package com.xieguiawu.rebirth.core

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.TimeoutCancellationException
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeout
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.longOrNull
import kotlinx.serialization.json.put
import java.io.BufferedReader
import java.io.BufferedWriter
import java.io.IOException
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong

/**
 * The JSON-lines transport: one request object per line in, one response
 * object per line out, correlated by an auto-incremented `id`.
 *
 * Decoupled from [java.lang.Process] so protocol behaviour is unit-testable
 * over piped streams (see CoreClientProtocolTest).
 *
 * Contract (docs/mobile-protocol.md §0): serial sends, responses in order,
 * one in-flight timeout per request.
 */
class CoreConnection(
    private val writer: BufferedWriter,
    private val reader: BufferedReader,
    scope: CoroutineScope,
    private val timeoutMs: Long = 30_000L,
    private val onUnexpectedLine: (String) -> Unit = {},
) {
    private val sendMutex = Mutex() // serialise writes (protocol forbids concurrency)
    private val ids = AtomicLong(0)
    private val pending = ConcurrentHashMap<Long, CompletableDeferred<JsonElement?>>()
    private val closed = java.util.concurrent.atomic.AtomicBoolean(false)

    init {
        scope.launch { pumpLoop() }
    }

    /**
     * Send one request and await its response data. Throws [CoreException]:
     * TIMEOUT if unanswered within [timeoutMs]; PROCESS_CRASHED if the stream
     * ends; PROTOCOL if the daemon answered ok:false.
     */
    suspend fun request(cmd: String, params: JsonObject = JsonObject(emptyMap())): JsonElement? {
        if (closed.get()) throw CoreException(CoreException.Kind.PROCESS_CRASHED, "connection closed")
        val id = ids.incrementAndGet()
        val deferred = CompletableDeferred<JsonElement?>()
        pending[id] = deferred

        val line = buildJsonObject {
            put("id", id)
            put("cmd", cmd)
            params.forEach { (k, v) -> put(k, v) }
        }.toString()

        try {
            sendMutex.withLock {
                withContext(Dispatchers.IO) {
                    writer.write(line)
                    writer.newLine()
                    writer.flush()
                }
            }
        } catch (e: IOException) {
            pending.remove(id)
            throw CoreException(CoreException.Kind.PROCESS_CRASHED, "write failed: ${e.message}", e)
        }

        return try {
            withTimeout(timeoutMs) { deferred.await() }
        } catch (e: TimeoutCancellationException) {
            pending.remove(id)
            throw CoreException(
                CoreException.Kind.TIMEOUT,
                "request '$cmd' timed out after ${timeoutMs}ms",
                e,
            )
        }
    }

    fun close() {
        if (!closed.compareAndSet(false, true)) return
        failAll(CoreException(CoreException.Kind.PROCESS_CRASHED, "connection closed"))
        try {
            writer.close()
        } catch (_: IOException) {
        }
    }

    private fun handleLine(line: String) {
        val obj = try {
            ProtocolJson.json.parseToJsonElement(line).jsonObject
        } catch (e: Exception) {
            onUnexpectedLine("unparseable line: $line")
            return
        }
        val id = (obj["id"] as? JsonPrimitive)?.longOrNull ?: run {
            onUnexpectedLine("line without id: $line")
            return
        }
        val deferred = pending.remove(id) ?: return // late response after timeout — drop

        val ok = (obj["ok"] as? JsonPrimitive)?.booleanOrNull ?: false
        if (ok) {
            deferred.complete(obj["data"])
        } else {
            val message = (obj["error"] as? JsonPrimitive)?.contentOrNull ?: "unknown error"
            deferred.completeExceptionally(CoreException.protocolError(message))
        }
    }

    private suspend fun pumpLoop() {
        withContext(Dispatchers.IO) {
            try {
                while (true) {
                    val line = reader.readLine() ?: break // EOF: process exited
                    if (line.isBlank()) continue
                    try {
                        handleLine(line)
                    } catch (e: Exception) {
                        onUnexpectedLine("handler error: ${e.message}")
                    }
                }
            } catch (_: IOException) {
                // stream closed underneath us
            } finally {
                if (!closed.getAndSet(true)) {
                    failAll(
                        CoreException(
                            CoreException.Kind.PROCESS_CRASHED,
                            "core process exited unexpectedly",
                        ),
                    )
                }
            }
        }
    }

    private fun failAll(e: CoreException) {
        pending.values.forEach { it.completeExceptionally(e) }
        pending.clear()
    }
}

package com.xieguiawu.rebirth.core

import android.content.Context
import android.util.Log
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import java.io.File
import java.io.IOException
import java.util.concurrent.TimeUnit

/**
 * Bridge to the Go core compiled as an executable ELF named
 * `librebirth_core.so` inside the APK's native library directory
 * (integrated by scripts/build-core.sh).
 *
 * - spawns `librebirth_core.so --dir <filesDir>/rebirth`
 * - speaks JSON-lines over stdin/stdout (see [CoreConnection])
 * - 30 s timeout per request; timeout or crash destroys the process and the
 *   next request transparently restarts it (session.json checkpoint keeps
 *   progress — the daemon replayed it on resume_session).
 * - stderr is logged with key redaction; keys never appear in logs.
 */
class CoreProcess(
    private val context: Context,
    private val scope: CoroutineScope = CoroutineScope(SupervisorJob() + Dispatchers.IO),
    private val requestTimeoutMs: Long = 30_000L,
) : CoreClient {

    private var process: Process? = null
    private var connection: CoreConnection? = null
    private var stderrJob: Job? = null

    /** Number of process restarts so far (for crash/recovery signalling). */
    @Volatile var crashCount: Int = 0
        private set

    @Volatile var lastCrashMessage: String? = null
        private set

    private val binaryPath: String
        get() = context.applicationInfo.nativeLibraryDir + "/librebirth_core.so"

    private val dataDir: String
        get() = File(context.filesDir, "rebirth").absolutePath

    override suspend fun request(cmd: String, params: JsonObject): JsonElement? {
        val conn = ensureStarted()
        return try {
            conn.request(cmd, params)
        } catch (e: CoreException) {
            if (e.kind == CoreException.Kind.PROCESS_CRASHED ||
                e.kind == CoreException.Kind.TIMEOUT
            ) {
                crash(e.message ?: "unknown failure")
            }
            throw e
        } catch (e: IOException) {
            crash(e.message ?: "io failure")
            throw CoreException(CoreException.Kind.PROCESS_CRASHED, e.message ?: "io failure", e)
        }
    }

    private fun ensureStarted(): CoreConnection {
        connection?.let { if (process?.isAlive == true) return it }
        try {
            start()
        } catch (e: IOException) {
            throw CoreException(
                CoreException.Kind.START_FAILED,
                "cannot exec core binary: ${e.message}",
                e,
            )
        }
        return connection!!
    }

    private fun start() {
        val cmd = listOf(binaryPath, "--dir", dataDir)
        Log.d(TAG, "starting core: $cmd")
        val p = ProcessBuilder(cmd)
            .directory(context.filesDir)
            .start()
        process = p

        connection = CoreConnection(
            writer = p.outputStream.bufferedWriter(),
            reader = p.inputStream.bufferedReader(),
            scope = scope,
            timeoutMs = requestTimeoutMs,
            onUnexpectedLine = { line -> Log.w(TAG, "unexpected core output: ${redact(line)}") },
        )
        stderrJob = scope.launch(Dispatchers.IO) {
            try {
                p.errorStream.bufferedReader().forEachLine { line ->
                    Log.d(TAG, redact(line))
                }
            } catch (_: Exception) {
                // stream teardown while process dies
            }
        }
    }

    private fun crash(message: String) {
        crashCount++
        lastCrashMessage = message
        connection?.close()
        process?.destroy()
        process = null
        connection = null
    }

    override suspend fun shutdown() {
        try {
            request("shutdown")
        } catch (_: CoreException) {
            // process is going away anyway
        }
        process?.let { p ->
            try {
                if (!p.waitFor(2, TimeUnit.SECONDS)) p.destroy()
            } catch (_: InterruptedException) {
                p.destroy()
            }
        }
        process = null
        connection = null
        stderrJob?.cancel()
    }

    companion object {
        private const val TAG = "RebirthCore"

        /**
         * Redact anything that looks like a secret before it reaches Logcat.
         * Logging red line (docs/mobile-protocol.md §4): never print keys.
         */
        fun redact(line: String): String = line
            .replace(Regex("""(sk|nvapi|ms|or)-[A-Za-z0-9]{6,}"""), "***")
            .replace(Regex("""(?i)authorization:\s*\S+"""), "authorization: ***")
            .replace(Regex("""(?i)bearer\s+\S+"""), "bearer ***")
    }
}

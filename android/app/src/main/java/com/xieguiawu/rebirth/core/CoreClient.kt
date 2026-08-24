package com.xieguiawu.rebirth.core

import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.encodeToJsonElement
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put

/** Error surfaced by the JSON-lines bridge (docs/mobile-protocol.md §3). */
class CoreException(
    val kind: Kind,
    override val message: String,
    cause: Throwable? = null,
) : Exception(message, cause) {
    enum class Kind {
        /** ok:false response from the daemon. */
        PROTOCOL,

        /** The core process died (panic / OOM / kill). */
        PROCESS_CRASHED,

        /** A single request exceeded the timeout. */
        TIMEOUT,

        /** The native binary could not be exec'd at all. */
        START_FAILED,
    }

    companion object {
        fun protocolError(message: String) = CoreException(Kind.PROTOCOL, message)
        fun missingData(cmd: String) =
            CoreException(Kind.PROTOCOL, "$cmd: response has no data field")
        fun decodeError(cmd: String, cause: Throwable) =
            CoreException(Kind.PROTOCOL, "$cmd: cannot decode data: ${cause.message}", cause)
    }
}

/**
 * The full rebirth mobile protocol as typed commands. Implementations provide
 * a single low-level [request]; everything else is built on top of it.
 * Serial requests only — never issue two commands concurrently.
 */
interface CoreClient {

    /** Send one JSON-lines request, await its response. Returns the `data` element (may be null). */
    suspend fun request(cmd: String, params: JsonObject = JsonObject(emptyMap())): JsonElement?

    // ---- protocol §1.1..§1.9 ----------------------------------------------

    suspend fun hello(): HelloData {
        val data = request("hello") ?: throw CoreException.missingData("hello")
        return try {
            ProtocolJson.json.decodeFromJsonElement(data)
        } catch (e: Exception) {
            throw CoreException.decodeError("hello", e)
        }
    }

    suspend fun bloodlineGet(): BloodlineData {
        val data = request("bloodline_get") ?: throw CoreException.missingData("bloodline_get")
        return try {
            ProtocolJson.json.decodeFromJsonElement(data)
        } catch (e: Exception) {
            throw CoreException.decodeError("bloodline_get", e)
        }
    }

    suspend fun drawBirths(seed: Long): List<Birth> {
        val data = request("draw_births", buildJsonObject { put("seed", seed) })
            ?: throw CoreException.missingData("draw_births")
        return try {
            data.jsonObject["births"]!!.jsonArray.map {
                ProtocolJson.json.decodeFromJsonElement<Birth>(it)
            }
        } catch (e: Exception) {
            throw CoreException.decodeError("draw_births", e)
        }
    }

    suspend fun drawTalents(seed: Long): List<Talent> {
        val data = request("draw_talents", buildJsonObject { put("seed", seed) })
            ?: throw CoreException.missingData("draw_talents")
        return try {
            data.jsonObject["talents"]!!.jsonArray.map {
                ProtocolJson.json.decodeFromJsonElement<Talent>(it)
            }
        } catch (e: Exception) {
            throw CoreException.decodeError("draw_talents", e)
        }
    }

    suspend fun newSession(
        seed: Long,
        lang: String,
        birth: Birth?,
        talents: List<Talent>,
        points: Points,
        maxAge: Int,
        narrator: NarratorConfig,
        traumaOverrides: TraumaOverrides?,
    ): NewSessionData {
        val params = buildJsonObject {
            put("seed", seed)
            put("lang", lang)
            put("birth", birth?.let { encodeBirth(it) } ?: JsonNull)
            put("talents", ProtocolJson.json.encodeToJsonElement(talents))
            put("points", buildJsonObject {
                put("chr", points.chr)
                put("int", points.int)
                put("str", points.str)
                put("mny", points.mny)
            })
            put("max_age", maxAge)
            put("narrator", buildJsonObject {
                put("enabled", narrator.enabled)
                put("providers", buildJsonArray {
                    narrator.providers.forEach { p ->
                        add(buildJsonObject {
                            put("provider", p.provider)
                            put("model", p.model)
                            put("url", p.url)
                            put("key", p.key)
                        })
                    }
                })
                put("budget", narrator.budget)
                put("ratio", narrator.ratio)
            })
            put("trauma_overrides", traumaOverrides?.let { o ->
                buildJsonObject {
                    o.enterAt?.let { put("enter_at", it) }
                    o.exitAt?.let { put("exit_at", it) }
                    o.drive?.let { put("drive", it) }
                    o.eventTraumaScale?.let { put("event_trauma_scale", it) }
                }
            } ?: JsonNull)
        }
        val data = request("new_session", params) ?: throw CoreException.missingData("new_session")
        return try {
            ProtocolJson.json.decodeFromJsonElement(data)
        } catch (e: Exception) {
            throw CoreException.decodeError("new_session", e)
        }
    }

    suspend fun next(): YearResult {
        val data = request("next") ?: throw CoreException.missingData("next")
        return try {
            ProtocolJson.json.decodeFromJsonElement(data)
        } catch (e: Exception) {
            throw CoreException.decodeError("next", e)
        }
    }

    suspend fun checkpointGet(): CheckpointData {
        val data = request("checkpoint_get") ?: throw CoreException.missingData("checkpoint_get")
        return try {
            ProtocolJson.json.decodeFromJsonElement(data)
        } catch (e: Exception) {
            throw CoreException.decodeError("checkpoint_get", e)
        }
    }

    /** Narrator config MUST be re-sent (keys never touch disk).
     *  Returns every replayed year (0..N) so the UI can rebuild the
     *  timeline lost in the crash (protocol §1.8). */
    suspend fun resumeSession(narrator: NarratorConfig): List<YearResult> {
        val params = buildJsonObject {
            put("narrator", buildJsonObject {
                put("enabled", narrator.enabled)
                put("providers", buildJsonArray {
                    narrator.providers.forEach { p ->
                        add(buildJsonObject {
                            put("provider", p.provider)
                            put("model", p.model)
                            put("url", p.url)
                            put("key", p.key)
                        })
                    }
                })
                put("budget", narrator.budget)
                put("ratio", narrator.ratio)
            })
        }
        val data = request("resume_session", params)
            ?: throw CoreException.missingData("resume_session")
        return try {
            data.jsonObject["years"]?.jsonArray?.map {
                ProtocolJson.json.decodeFromJsonElement<YearResult>(it)
            } ?: emptyList()
        } catch (e: Exception) {
            throw CoreException.decodeError("resume_session", e)
        }
    }

    suspend fun shutdown() {
        request("shutdown")
    }

    companion object {
        /** Exact Birth re-encoding for new_session (id/name/desc/weight/bonus/sensitivity_add). */
        private fun encodeBirth(b: Birth): JsonObject = buildJsonObject {
            put("id", b.id)
            put("name", b.name)
            put("desc", b.desc)
            put("weight", b.weight)
            put("bonus", buildJsonObject {
                put("chr", b.bonus.chr)
                put("int", b.bonus.int)
                put("str", b.bonus.str)
                put("mny", b.bonus.mny)
                put("spr", b.bonus.spr)
            })
            put("sensitivity_add", b.sensitivityAdd)
        }
    }
}

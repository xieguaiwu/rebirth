package com.xieguiawu.rebirth.core

import java.io.BufferedReader
import java.io.BufferedWriter
import java.io.InputStreamReader
import java.io.OutputStreamWriter
import java.io.PipedInputStream
import java.io.PipedOutputStream
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.double
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import kotlinx.serialization.json.put
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Protocol conformance tests against docs/mobile-protocol.md: JSON-lines
 * request/response correlation, exact new_session payload encoding, and
 * YearResult field mapping — exercised over a scripted pipe (no real
 * process spawn).
 */
class CoreClientProtocolTest {

    /** CoreConnection wired to a scripted responder over piped streams. */
    private class Harness(
        timeoutMs: Long = 2_000L,
        keepAlive: Boolean = false,
        responder: suspend (JsonObject) -> JsonObject?,
    ) {
        val received = mutableListOf<JsonObject>()
        val unexpected = mutableListOf<String>()
        private val scope = CoroutineScope(Dispatchers.IO)
        private val responderJob: Job
        val conn: CoreConnection
        val client: CoreClient

        init {
            val reqOut = PipedOutputStream() // conn writes → responder reads
            val reqIn = PipedInputStream(reqOut)
            val respOut = PipedOutputStream() // responder writes → conn reads
            val respIn = PipedInputStream(respOut)

            responderJob = scope.launch(Dispatchers.IO) {
                val reader = BufferedReader(InputStreamReader(reqIn))
                val writer = BufferedWriter(OutputStreamWriter(respOut))
                try {
                    while (true) {
                        val line = reader.readLine() ?: break
                        val req = ProtocolJson.json.parseToJsonElement(line).jsonObject
                        received += req
                        val resp = responder(req)
                        if (resp == null) {
                            if (!keepAlive) break
                        } else {
                            writer.write(resp.toString())
                            writer.newLine()
                            writer.flush()
                        }
                    }
                } catch (_: Exception) {
                    // stream teardown
                } finally {
                    try {
                        writer.close()
                    } catch (_: Exception) {
                    }
                }
            }
            conn = CoreConnection(
                writer = BufferedWriter(OutputStreamWriter(reqOut)),
                reader = BufferedReader(InputStreamReader(respIn)),
                scope = scope,
                timeoutMs = timeoutMs,
                onUnexpectedLine = { unexpected += it },
            )
            client = object : CoreClient {
                override suspend fun request(cmd: String, params: JsonObject): JsonElement? =
                    conn.request(cmd, params)
            }
        }

        fun close() {
            conn.close()
            responderJob.cancel()
            scope.cancel()
        }
    }

    private fun okResponse(data: JsonObject?): (JsonObject) -> JsonObject = { req ->
        buildJsonObject {
            put("id", req["id"]!!.jsonPrimitive.long)
            put("ok", true)
            if (data != null) put("data", data)
        }
    }

    // ---- request/response correlation -------------------------------------

    @Test
    fun hello_roundTrips() = runBlocking {
        val h = Harness(responder = okResponse(buildJsonObject { put("ver", "0.10.0"); put("proto", 1) }))
        val hello = h.client.hello()
        assertEquals("0.10.0", hello.ver)
        assertEquals(1, hello.proto)
        assertEquals("hello", h.received.single()["cmd"]!!.jsonPrimitive.content)
        assertEquals(1L, h.received.single()["id"]!!.jsonPrimitive.long)
        h.close()
    }

    @Test
    fun sequentialRequests_getIncrementingIds() = runBlocking {
        val h = Harness(responder = okResponse(buildJsonObject { put("ver", "x"); put("proto", 1) }))
        repeat(3) { h.client.hello() }
        assertEquals(listOf(1L, 2L, 3L), h.received.map { it["id"]!!.jsonPrimitive.long })
        assertEquals(listOf("hello", "hello", "hello"), h.received.map { it["cmd"]!!.jsonPrimitive.content })
        h.close()
    }

    // ---- new_session payload encoding -------------------------------------

    @Test
    fun newSession_payloadRoundTrips() = runBlocking {
        val h = Harness(responder = okResponse(buildJsonObject { put("generation", 3) }))
        val birth = Birth(
            id = "slum", name = "贫民窟", desc = "低矮棚户。", weight = 10.0,
            bonus = Effects(mny = -1.0), sensitivityAdd = 0.05,
        )
        val talents = listOf(
            Talent(
                name = "乐天派", desc = "心态好。", rarity = "rare", bonus = Effects(spr = 2.0),
                traumaMult = 1.0, luckBonus = 0.1, therapyMult = 1.0, inheritable = true,
            ),
        )
        val narrator = NarratorConfig(
            enabled = true,
            providers = listOf(
                NarratorProvider(provider = "deepseek", model = "deepseek-v4-flash", url = "", key = "sk-test-123456"),
                NarratorProvider(provider = "openrouter", model = "", url = "", key = "sk-or-999"),
            ),
            budget = 24,
            ratio = 0.5,
        )
        val gen = h.client.newSession(12345, "zh", birth, talents, Points(5, 5, 5, 5), 100, narrator, null)
        assertEquals(3, gen.generation)

        val req = h.received.single()
        assertEquals("new_session", req["cmd"]!!.jsonPrimitive.content)
        assertEquals(12345L, req["seed"]!!.jsonPrimitive.long)
        assertEquals("zh", req["lang"]!!.jsonPrimitive.content)
        assertEquals(100, req["max_age"]!!.jsonPrimitive.int)
        assertEquals(JsonNull, req["trauma_overrides"])

        val birthBack = ProtocolJson.json.decodeFromJsonElement<Birth>(req["birth"]!!)
        assertEquals("slum", birthBack.id)
        assertEquals("贫民窟", birthBack.name)
        assertEquals(0.05, birthBack.sensitivityAdd, 1e-9)
        assertEquals(-1.0, birthBack.bonus.mny, 1e-9)

        val talentBack = ProtocolJson.json.decodeFromJsonElement<List<Talent>>(req["talents"]!!).single()
        assertEquals("乐天派", talentBack.name)
        assertEquals("rare", talentBack.rarity)
        assertTrue(talentBack.inheritable)
        assertEquals(0.1, talentBack.luckBonus, 1e-9)

        val narratorBack = ProtocolJson.json.decodeFromJsonElement<NarratorConfig>(req["narrator"]!!)
        assertTrue(narratorBack.enabled)
        assertEquals(listOf("deepseek", "openrouter"), narratorBack.providers.map { it.provider })
        assertEquals("sk-test-123456", narratorBack.providers[0].key)
        assertEquals("deepseek-v4-flash", narratorBack.providers[0].model)
        assertEquals("", narratorBack.providers[0].url)
        assertEquals("sk-or-999", narratorBack.providers[1].key)
        assertEquals(24, narratorBack.budget)
        assertEquals(0.5, narratorBack.ratio, 1e-9)

        val pointsBack = req["points"]!!.jsonObject
        assertEquals(5, pointsBack["chr"]!!.jsonPrimitive.int)
        assertEquals(5, pointsBack["int"]!!.jsonPrimitive.int)
        assertEquals(5, pointsBack["str"]!!.jsonPrimitive.int)
        assertEquals(5, pointsBack["mny"]!!.jsonPrimitive.int)
        h.close()
    }

    @Test
    fun newSession_nullBirthAndDisabledNarrator_encodedAsNullAndFalse() = runBlocking {
        val h = Harness(responder = okResponse(buildJsonObject { put("generation", 1) }))
        h.client.newSession(7, "en", null, emptyList(), Points(5, 5, 5, 5), 100, NarratorConfig(enabled = false), null)
        val req = h.received.single()
        assertEquals(JsonNull, req["birth"])
        assertEquals(JsonNull, req["trauma_overrides"])
        val narratorBack = ProtocolJson.json.decodeFromJsonElement<NarratorConfig>(req["narrator"]!!)
        assertFalse(narratorBack.enabled)
        assertTrue(narratorBack.providers.isEmpty())
        h.close()
    }

    @Test
    fun newSession_traumaOverrides_onlyNonNullFieldsEncoded() = runBlocking {
        val h = Harness(responder = okResponse(buildJsonObject { put("generation", 1) }))
        h.client.newSession(
            7, "zh", null, emptyList(), Points(5, 5, 5, 5), 100, NarratorConfig(),
            TraumaOverrides(enterAt = 0.85, exitAt = 0.30),
        )
        val overrides = h.received.single()["trauma_overrides"]!!.jsonObject
        assertEquals(0.85, overrides["enter_at"]!!.jsonPrimitive.double, 1e-9)
        assertEquals(0.30, overrides["exit_at"]!!.jsonPrimitive.double, 1e-9)
        assertNull(overrides["drive"])
        assertNull(overrides["event_trauma_scale"])
        h.close()
    }

    @Test
    fun resumeSession_reEncodesNarratorWithKeys() = runBlocking {
        val h = Harness(responder = okResponse(
            buildJsonObject {
                put("resumed", true)
                put("age", 0)
                put("generation", 1)
            },
        ))
        h.client.resumeSession(
            NarratorConfig(
                enabled = true,
                providers = listOf(NarratorProvider("custom", "llama3", "http://10.0.0.2:8000/v1", "key-x")),
                budget = 10,
                ratio = 0.0,
            ),
        )
        val req = h.received.single()
        assertEquals("resume_session", req["cmd"]!!.jsonPrimitive.content)
        val narratorBack = ProtocolJson.json.decodeFromJsonElement<NarratorConfig>(req["narrator"]!!)
        assertEquals("key-x", narratorBack.providers.single().key)
        assertEquals(10, narratorBack.budget)
        h.close()
    }

    // ---- response decoding ------------------------------------------------

    @Test
    fun yearResult_deathYear_decodesPerProtocol() {
        val raw = """
            {"age":98,"lines":["[ 98 岁] 你在儿孙环绕中闭上了眼睛。"],
             "career":{"id":"teacher","name":"教师"},"career_change":"retire",
             "event":{"id":"fate_01","text":"命运眷顾。","good":true,"llm":true,"trauma_alpha":0.0,"therapy_q":0.2,
                      "delta":{"chr":0,"int":1,"str":0,"mny":0,"spr":2}},
             "stats":{"chr":4.1,"int":7.5,"str":3.9,"mny":5.2,"spr":8.9},
             "trauma":{"m":0.11,"a":0.22,"p":0.61,"load":0.15,"pathological":false},
             "luck":0.12,"llm_broken_notice":true,"died":true,
             "death_status":"安详离世","epitaph":"一生至此。","lineage_saved":true,
             "next_generation":3,"next_sensitivity":0.76}
        """.trimIndent()
        val year = ProtocolJson.json.decodeFromJsonElement<YearResult>(
            ProtocolJson.json.parseToJsonElement(raw),
        )
        assertEquals(98, year.age)
        assertEquals(1, year.lines.size)
        assertEquals("教师", year.career?.name)
        assertEquals("retire", year.careerChange)
        assertEquals("fate_01", year.event?.id)
        assertTrue(year.event?.good == true)
        assertTrue(year.event?.llm == true)
        assertEquals(0.2, year.event?.therapyQ ?: 0.0, 1e-9)
        assertEquals(2.0, year.event?.delta?.spr ?: 0.0, 1e-9)
        assertEquals(7.5, year.stats.int, 1e-9)
        assertEquals(5.2, year.stats.mny, 1e-9)
        assertEquals(0.11, year.trauma.m, 1e-9)
        assertEquals(0.15, year.trauma.load, 1e-9)
        assertEquals(0.12, year.luck, 1e-9)
        assertTrue(year.llmBrokenNotice)
        assertTrue(year.died)
        assertEquals("安详离世", year.deathStatus)
        assertEquals("一生至此。", year.epitaph)
        assertEquals(true, year.lineageSaved)
        assertEquals(3, year.nextGeneration)
        assertEquals(0.76, year.nextSensitivity ?: 0.0, 1e-9)
    }

    @Test
    fun yearResult_minimalYear_defaultsFill() {
        val year = ProtocolJson.json.decodeFromJsonElement<YearResult>(
            ProtocolJson.json.parseToJsonElement("""{"age":7}"""),
        )
        assertEquals(7, year.age)
        assertFalse(year.died)
        assertNull(year.deathStatus)
        assertNull(year.career)
        assertNull(year.event)
        assertTrue(year.lines.isEmpty())
    }

    // ---- error paths -------------------------------------------------------

    @Test
    fun errorResponse_throwsProtocolError() = runBlocking {
        val h = Harness(responder = { req ->
            buildJsonObject {
                put("id", req["id"]!!.jsonPrimitive.long)
                put("ok", false)
                put("error", "no checkpoint")
            }
        })
        val e = assertSuspendThrows<CoreException> { h.client.resumeSession(NarratorConfig()) }
        assertEquals(CoreException.Kind.PROTOCOL, e.kind)
        assertEquals("no checkpoint", e.message)
        h.close()
    }

    @Test
    fun timeout_throwsTimeoutError() = runBlocking {
        // keepAlive: the responder swallows the request but never replies.
        val h = Harness(timeoutMs = 150, keepAlive = true, responder = { null })
        val e = assertSuspendThrows<CoreException> { h.conn.request("hello") }
        assertEquals(CoreException.Kind.TIMEOUT, e.kind)
        assertTrue(e.message!!.contains("timed out"))
        h.close()
    }

    @Test
    fun processExit_failsPendingRequests() = runBlocking {
        // responder reads one line, then closes the stream (process died).
        val h = Harness(responder = { null })
        val e = assertSuspendThrows<CoreException> { h.conn.request("hello") }
        assertEquals(CoreException.Kind.PROCESS_CRASHED, e.kind)
        h.close()
    }

    // ---- FakeCore behaviour (also drives UI smoke tests) -------------------

    @Test
    fun fakeCore_nextSequence_reachesDeathThenErrors() = runBlocking {
        val fake = FakeCore()
        val years = mutableListOf<YearResult>()
        for (i in fake.scriptedYears.indices) years += fake.next()
        assertTrue(years.last().died)
        assertEquals(9, years.last().age)
        val e = try {
            fake.next()
            null
        } catch (t: CoreException) {
            t
        }
        assertNotNull(e)
        assertEquals(CoreException.Kind.PROTOCOL, e!!.kind)
        assertEquals("session finished", e.message)
    }

    // ---- helpers -----------------------------------------------------------

    private suspend inline fun <reified T : Throwable> assertSuspendThrows(block: suspend () -> Unit): T {
        var caught: Throwable? = null
        try {
            block()
        } catch (t: Throwable) {
            caught = t
        }
        assertNotNull("expected ${T::class.simpleName} but nothing was thrown", caught)
        assertTrue(
            "expected ${T::class.simpleName}, got ${caught!!::class.simpleName}",
            caught is T,
        )
        return caught as T
    }
}

package com.xieguiawu.rebirth.core

import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.encodeToJsonElement
import kotlinx.serialization.json.put

/**
 * In-memory [CoreClient] used by Robolectric UI tests (which cannot spawn the
 * real native process) and Compose previews. Scriptable via constructor
 * state; records every command in [requestLog].
 */
class FakeCore(
    var helloData: HelloData = HelloData(ver = "0.10.0", proto = 1),
    var bloodline: BloodlineData = BloodlineData(),
    var checkpoint: CheckpointData = CheckpointData(exists = false),
    var births: List<Birth> = FakeCore.defaultBirths(),
    var talents: List<Talent> = FakeCore.defaultTalents(),
    var scriptedYears: List<YearResult> = FakeCore.defaultLife(),
    var newSessionGeneration: Int = 1,
    var failNextWith: String? = null,
) : CoreClient {

    val requestLog = mutableListOf<String>()
    var yearIndex = 0
        private set

    override suspend fun request(cmd: String, params: JsonObject): JsonElement? {
        requestLog.add(cmd)
        return when (cmd) {
            "hello" -> ProtocolJson.json.encodeToJsonElement(helloData)
            "bloodline_get" -> ProtocolJson.json.encodeToJsonElement(bloodline)
            "draw_births" -> buildJsonObject {
                put("births", ProtocolJson.json.encodeToJsonElement(births))
            }
            "draw_talents" -> buildJsonObject {
                put("talents", ProtocolJson.json.encodeToJsonElement(talents))
            }
            "new_session" -> buildJsonObject { put("generation", newSessionGeneration) }
            "next" -> {
                failNextWith?.let { throw CoreException.protocolError(it) }
                if (yearIndex < scriptedYears.size) {
                    ProtocolJson.json.encodeToJsonElement(scriptedYears[yearIndex++])
                } else {
                    throw CoreException.protocolError("session finished")
                }
            }
            "checkpoint_get" -> ProtocolJson.json.encodeToJsonElement(checkpoint)
            "resume_session" -> {
                yearIndex = 0
                JsonNull
            }
            "shutdown" -> null
            else -> throw CoreException.protocolError("unknown command $cmd")
        }
    }

    companion object {
        fun defaultBirths(): List<Birth> = listOf(
            Birth(
                id = "slum", name = "贫民窟", desc = "低矮棚户，污水横流。",
                weight = 10.0, bonus = Effects(mny = -1.0, spr = -1.0), sensitivityAdd = 0.05,
            ),
            Birth(
                id = "rural", name = "农村", desc = "田野辽阔，日子清贫而安静。",
                weight = 10.0, bonus = Effects(str = 1.0), sensitivityAdd = 0.02,
            ),
            Birth(
                id = "urban", name = "城市", desc = "霓虹与机会并存的水泥森林。",
                weight = 10.0, bonus = Effects(mny = 1.0, chr = 1.0), sensitivityAdd = 0.03,
            ),
        )

        fun defaultTalents(): List<Talent> = listOf(
            Talent("乐天派", "心态总是很好。", "common", Effects(spr = 1.0)),
            Talent("过目不忘", "记忆力超群。", "rare", Effects(int = 2.0)),
            Talent("天生丽质", "颜值出众。", "rare", Effects(chr = 2.0)),
            Talent("钢筋铁骨", "体质极佳。", "rare", Effects(str = 2.0), therapyMult = 1.1),
            Talent("富贵之家", "含着金汤匙出生。", "epic", Effects(mny = 3.0)),
            Talent("心如止水", "情绪波动小。", "epic", Effects(spr = 2.0), traumaMult = 0.7),
            Talent("幸运星", "运气总是站在你这边。", "epic", Effects(), luckBonus = 0.2),
            Talent("苦难淬炼", "创伤让你更强大。", "epic", Effects(str = 2.0), traumaMult = 1.3, inheritable = true),
            Talent("天才少年", "智力超群。", "legendary", Effects(int = 4.0), inheritable = true),
            Talent("天选之人", "命运眷顾之人。", "legendary", Effects(chr = 2.0, int = 2.0, str = 2.0), luckBonus = 0.3),
        )

        fun defaultLife(): List<YearResult> = listOf(
            YearResult(
                age = 6,
                lines = listOf("[  6 岁] 你背着书包踏进了小学。"),
                career = Career("none", "无业"),
                stats = Stats(5.1, 5.4, 5.2, 4.9, 6.2),
                trauma = TraumaState(0.20, 0.30, 0.65, 0.26, false),
                luck = 0.10,
            ),
            YearResult(
                age = 7,
                lines = listOf("[  7 岁] 高年级的孩子堵在巷口，你学会了绕远路回家。"),
                career = Career("none", "无业"),
                event = GameEvent(
                    id = "bully_01",
                    text = "高年级的孩子堵在巷口，你学会了绕远路回家。",
                    good = false, llm = false, traumaAlpha = 0.3, therapyQ = 0.0,
                    delta = Effects(spr = -1.0),
                ),
                stats = Stats(5.1, 5.5, 5.0, 4.8, 5.4),
                trauma = TraumaState(0.31, 0.42, 0.63, 0.35, false),
                luck = 0.12,
            ),
            YearResult(
                age = 8,
                lines = listOf("[  8 岁] 巷口小卖部的阿婆分了你半根冰棍。"),
                career = Career("none", "无业"),
                event = GameEvent(
                    id = "heal_01",
                    text = "巷口小卖部的阿婆分了你半根冰棍。",
                    good = true, llm = false, traumaAlpha = 0.0, therapyQ = 0.4,
                    delta = Effects(spr = 1.0),
                ),
                stats = Stats(5.1, 5.5, 5.1, 4.8, 6.2),
                trauma = TraumaState(0.25, 0.35, 0.64, 0.29, false),
                luck = 0.05,
            ),
            YearResult(
                age = 9,
                lines = listOf("[  9 岁] 你没能熬过那个冬夜的高热。"),
                career = Career("none", "无业"),
                stats = Stats(5.1, 5.5, 5.1, 4.8, 4.0),
                trauma = TraumaState(0.50, 0.60, 0.40, 0.54, true),
                luck = -0.20,
                died = true,
                deathStatus = "幼年夭折",
                epitaph = "一生至此。",
                lineageSaved = true,
                nextGeneration = 2,
                nextSensitivity = 0.76,
            ),
        )
    }
}

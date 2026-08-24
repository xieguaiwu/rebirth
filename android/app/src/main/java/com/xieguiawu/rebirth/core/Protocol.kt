package com.xieguiawu.rebirth.core

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/**
 * Shared JSON codec for the frozen rebirth mobile protocol
 * (docs/mobile-protocol.md v1). Field names MUST match the Go side.
 */
object ProtocolJson {
    val json = Json {
        ignoreUnknownKeys = true // forward compatibility with the Go core
        encodeDefaults = false
        isLenient = true
    }
}

// ---- Responses / payloads -------------------------------------------------

@Serializable
data class HelloData(
    val ver: String = "",
    val proto: Int = 0,
)

@Serializable
data class BloodlineData(
    val generation: Int = 0,
    val sensitivity: Double = 0.0,
    @SerialName("inherited_talent") val inheritedTalent: String = "",
)

@Serializable
data class NewSessionData(
    val generation: Int = 0,
)

@Serializable
data class CheckpointData(
    val exists: Boolean = false,
    val age: Int = 0,
    val generation: Int = 0,
)

// ---- Game model (mirrors game.Birth / game.Talent / game.Stats JSON tags) --

@Serializable
data class Effects(
    val chr: Double = 0.0,
    val int: Double = 0.0,
    val str: Double = 0.0,
    val mny: Double = 0.0,
    val spr: Double = 0.0,
)

@Serializable
data class Birth(
    val id: String = "",
    val name: String = "",
    val desc: String = "",
    val weight: Double = 0.0,
    val bonus: Effects = Effects(),
    @SerialName("sensitivity_add") val sensitivityAdd: Double = 0.0,
)

@Serializable
data class Talent(
    val name: String = "",
    val desc: String = "",
    val rarity: String = "common",
    val bonus: Effects = Effects(),
    @SerialName("trauma_mult") val traumaMult: Double = 1.0,
    @SerialName("luck_bonus") val luckBonus: Double = 0.0,
    @SerialName("therapy_mult") val therapyMult: Double = 1.0,
    val inheritable: Boolean = false,
)

@Serializable
data class Stats(
    val chr: Double = 0.0,
    val int: Double = 0.0,
    val str: Double = 0.0,
    val mny: Double = 0.0,
    val spr: Double = 0.0,
)

@Serializable
data class TraumaState(
    val m: Double = 0.0,
    val a: Double = 0.0,
    val p: Double = 0.0,
    val load: Double = 0.0,
    val pathological: Boolean = false,
)

@Serializable
data class Career(
    val id: String = "none",
    val name: String = "",
)

@Serializable
data class GameEvent(
    val id: String = "",
    val text: String = "",
    val good: Boolean = false,
    val llm: Boolean = false,
    @SerialName("trauma_alpha") val traumaAlpha: Double = 0.0,
    @SerialName("therapy_q") val therapyQ: Double = 0.0,
    val delta: Effects = Effects(),
)

@Serializable
data class YearResult(
    val age: Int = 0,
    val lines: List<String> = emptyList(),
    val career: Career? = null,
    @SerialName("career_change") val careerChange: String? = null,
    val event: GameEvent? = null,
    val stats: Stats = Stats(),
    val trauma: TraumaState = TraumaState(),
    val luck: Double = 0.0,
    @SerialName("llm_broken_notice") val llmBrokenNotice: Boolean = false,
    val died: Boolean = false,
    // Death-year only fields (per protocol §1.6)
    @SerialName("death_status") val deathStatus: String? = null,
    val epitaph: String? = null,
    @SerialName("lineage_saved") val lineageSaved: Boolean? = null,
    @SerialName("next_generation") val nextGeneration: Int? = null,
    @SerialName("next_sensitivity") val nextSensitivity: Double? = null,
)

// ---- new_session request payloads ----------------------------------------

@Serializable
data class Points(
    val chr: Int = 0,
    val int: Int = 0,
    val str: Int = 0,
    val mny: Int = 0,
) {
    val total: Int get() = chr + int + str + mny
}

@Serializable
data class NarratorProvider(
    val provider: String = "",
    val model: String = "",
    val url: String = "",
    val key: String = "",
)

@Serializable
data class NarratorConfig(
    val enabled: Boolean = false,
    val providers: List<NarratorProvider> = emptyList(),
    val budget: Int = 24,
    val ratio: Double = 0.5,
)

@Serializable
data class TraumaOverrides(
    @SerialName("enter_at") val enterAt: Double? = null,
    @SerialName("exit_at") val exitAt: Double? = null,
    val drive: Double? = null,
    @SerialName("event_trauma_scale") val eventTraumaScale: Double? = null,
)

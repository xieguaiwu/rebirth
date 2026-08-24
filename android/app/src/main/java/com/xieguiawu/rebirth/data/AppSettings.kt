package com.xieguiawu.rebirth.data

import kotlinx.serialization.Serializable

/**
 * One LLM narration provider entry. The API key is intentionally NOT part of
 * this model — it lives in [com.xieguiawu.rebirth.security.SecureStorage]
 * under "provider_key_<id>" and is injected into the core process only.
 * List order = failover order (protocol §1.5).
 */
@Serializable
data class LlmProviderSetting(
    val id: String,
    val preset: String, // "deepseek" | "openrouter" | "custom" (OpenAI-compatible)
    val name: String,
    val model: String = "",
    val url: String = "",
    val enabled: Boolean = true,
)

@Serializable
data class AppSettings(
    /** "system" | "zh" | "en" — also decides the protocol `lang` field. */
    val language: String = "system",
    val providers: List<LlmProviderSetting> = emptyList(),
    val maxAge: Int = 100,
    val narrateRatio: Double = 0.5,
    val lastSeed: Long = 0L,
)

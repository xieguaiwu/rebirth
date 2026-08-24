package com.xieguiawu.rebirth.data

import android.content.Context
import com.xieguiawu.rebirth.core.ProtocolJson
import java.util.Locale
import kotlinx.serialization.encodeToString

/** Persists [AppSettings] as JSON in SharedPreferences. */
class SettingsRepository(private val context: Context) {

    private val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun load(): AppSettings = try {
        val raw = prefs.getString(KEY, null) ?: return AppSettings()
        ProtocolJson.json.decodeFromString<AppSettings>(raw)
    } catch (_: Exception) {
        AppSettings()
    }

    fun save(settings: AppSettings) {
        prefs.edit()
            .putString(KEY, ProtocolJson.json.encodeToString(settings))
            .apply()
    }

    companion object {
        private const val PREFS = "rebirth_settings"
        private const val KEY = "app_settings"

        /**
         * The protocol `lang` value: explicit zh/en wins, otherwise follow the
         * device locale (zh* → "zh", everything else → "en").
         */
        fun effectiveLanguage(language: String, locale: Locale): String = when (language) {
            "zh" -> "zh"
            "en" -> "en"
            else -> if (locale.language.lowercase(Locale.ROOT).startsWith("zh")) "zh" else "en"
        }

        /** Locale to apply in [android.content.ContextWrapper.attachBaseContext]; null = system. */
        fun localeForLanguage(language: String): Locale? = when (language) {
            "zh" -> Locale.SIMPLIFIED_CHINESE
            "en" -> Locale.ENGLISH
            else -> null
        }
    }
}

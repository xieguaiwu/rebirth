package com.xieguiawu.rebirth.data

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import java.util.Locale
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class LanguagePersistenceTest {

    private val context: Context = ApplicationProvider.getApplicationContext()

    @Test
    fun defaultLanguage_isSystem() {
        assertEquals("system", SettingsRepository(context).load().language)
    }

    @Test
    fun language_persistsAcrossRepositoryInstances() {
        val repo = SettingsRepository(context)
        repo.save(repo.load().copy(language = "zh"))
        assertEquals("zh", SettingsRepository(context).load().language)
    }

    @Test
    fun effectiveLanguage_resolvesForProtocol() {
        assertEquals("zh", SettingsRepository.effectiveLanguage("system", Locale.SIMPLIFIED_CHINESE))
        assertEquals("zh", SettingsRepository.effectiveLanguage("system", Locale.TRADITIONAL_CHINESE))
        assertEquals("en", SettingsRepository.effectiveLanguage("system", Locale.US))
        assertEquals("en", SettingsRepository.effectiveLanguage("en", Locale.SIMPLIFIED_CHINESE))
        assertEquals("zh", SettingsRepository.effectiveLanguage("zh", Locale.US))
    }

    @Test
    fun localeForLanguage_mapsExplicitOverridesOnly() {
        assertEquals(Locale.SIMPLIFIED_CHINESE, SettingsRepository.localeForLanguage("zh"))
        assertEquals(Locale.ENGLISH, SettingsRepository.localeForLanguage("en"))
        assertNull(SettingsRepository.localeForLanguage("system"))
    }

    @Test
    fun fullSettings_roundTripThroughJson() {
        val repo = SettingsRepository(context)
        val settings = AppSettings(
            language = "en",
            providers = listOf(
                LlmProviderSetting(
                    id = "a", preset = "deepseek", name = "DeepSeek",
                    model = "deepseek-v4-flash", enabled = true,
                ),
                LlmProviderSetting(
                    id = "b", preset = "custom", name = "Local",
                    model = "llama", url = "http://10.0.0.2:8000/v1", enabled = false,
                ),
            ),
            maxAge = 110,
            narrateRatio = 0.25,
            lastSeed = 42L,
        )
        repo.save(settings)
        assertEquals(settings, SettingsRepository(context).load())
    }
}

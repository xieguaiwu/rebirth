package com.xieguiawu.rebirth.ui

import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import com.xieguiawu.rebirth.core.Birth
import com.xieguiawu.rebirth.core.BloodlineData
import com.xieguiawu.rebirth.core.CheckpointData
import com.xieguiawu.rebirth.core.CoreClient
import com.xieguiawu.rebirth.core.CoreException
import com.xieguiawu.rebirth.core.CoreProcess
import com.xieguiawu.rebirth.core.NarratorConfig
import com.xieguiawu.rebirth.core.NarratorProvider
import com.xieguiawu.rebirth.core.Points
import com.xieguiawu.rebirth.core.Talent
import com.xieguiawu.rebirth.core.YearResult
import com.xieguiawu.rebirth.data.AppSettings
import com.xieguiawu.rebirth.data.LlmProviderSetting
import com.xieguiawu.rebirth.data.SettingsRepository
import com.xieguiawu.rebirth.security.KeystoreSecureStorage
import com.xieguiawu.rebirth.security.SecureStorage
import java.util.Locale
import java.util.UUID
import kotlin.random.Random
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Navigation destinations (single Activity, state-driven). */
sealed interface Screen {
    data object Home : Screen
    data object Create : Screen
    data object Timeline : Screen
    data object Trauma : Screen
    data object Settings : Screen
}

/** Test seam: UI tests inject fakes before the Activity starts. */
object ServiceLocator {
    @Volatile
    var coreClientFactory: (Context) -> CoreClient = { CoreProcess(it) }

    @Volatile
    var secureStorageFactory: (Context) -> SecureStorage = { KeystoreSecureStorage(it) }
}

class AppViewModel(
    private val client: CoreClient,
    private val secureStorage: SecureStorage,
    private val settingsRepo: SettingsRepository,
) : ViewModel() {

    data class CoreError(val kind: CoreException.Kind, val detail: String)

    data class UiState(
        val screen: Screen = Screen.Home,
        val loading: Boolean = false,
        val nextPending: Boolean = false,
        val coreError: CoreError? = null,
        val notice: String? = null,
        val bloodline: BloodlineData? = null,
        val checkpoint: CheckpointData? = null,
        val births: List<Birth> = emptyList(),
        val talents: List<Talent> = emptyList(),
        val years: List<YearResult> = emptyList(),
        val death: YearResult? = null,
        val currentSeed: Long = 0L,
        val settings: AppSettings = AppSettings(),
        // Create screen selection state (survives recomposition / rotation)
        val createBirth: Birth? = null,
        val createTalents: List<Talent> = emptyList(),
        val createPoints: Points = Points(5, 5, 5, 5),
    )

    private val _ui = MutableStateFlow(UiState())
    val ui: StateFlow<UiState> = _ui.asStateFlow()

    // viewModelScope is cancelled in onCleared, so the daemon shutdown
    // uses its own scope (graceful exit; checkpoint discipline is the
    // fallback if the process dies abruptly).
    private val shutdownScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    override fun onCleared() {
        shutdownScope.launch {
            try {
                client.shutdown()
            } catch (_: Exception) {
                // process is going away anyway
            }
        }
        super.onCleared()
    }

    /** Set by MainActivity to recreate() on in-app language switch. */
    var onLanguageChanged: (() -> Unit)? = null

    init {
        viewModelScope.launch {
            val settings = settingsRepo.load()
            _ui.update { it.copy(settings = settings) }
            refreshHome()
        }
    }

    // ---- navigation -------------------------------------------------------

    fun navigate(screen: Screen) {
        _ui.update { it.copy(screen = screen) }
    }

    fun consumeNotice() {
        _ui.update { it.copy(notice = null) }
    }

    // ---- home -------------------------------------------------------------

    private suspend fun refreshHome() {
        try {
            val bloodline = client.bloodlineGet()
            val checkpoint = client.checkpointGet()
            _ui.update { it.copy(bloodline = bloodline, checkpoint = checkpoint, coreError = null) }
        } catch (e: CoreException) {
            _ui.update { it.copy(coreError = CoreError(e.kind, e.message ?: "")) }
        }
    }

    fun refreshHomeFromUi() {
        viewModelScope.launch {
            _ui.update { it.copy(loading = true) }
            refreshHome()
            _ui.update { it.copy(loading = false) }
        }
    }

    // ---- create flow ------------------------------------------------------

    fun startCreate() {
        viewModelScope.launch {
            _ui.update {
                it.copy(
                    screen = Screen.Create,
                    loading = true,
                    births = emptyList(),
                    talents = emptyList(),
                    createBirth = null,
                    createTalents = emptyList(),
                    createPoints = Points(5, 5, 5, 5),
                )
            }
            val seed = Random.nextLong(1L, Int.MAX_VALUE.toLong())
            try {
                val births = client.drawBirths(seed)
                val talents = client.drawTalents(seed)
                _ui.update {
                    it.copy(births = births, talents = talents, currentSeed = seed, loading = false, coreError = null)
                }
            } catch (e: CoreException) {
                _ui.update {
                    it.copy(
                        loading = false,
                        screen = Screen.Home,
                        coreError = CoreError(e.kind, e.message ?: ""),
                    )
                }
            }
        }
    }

    fun selectBirth(birth: Birth) {
        _ui.update { it.copy(createBirth = birth) }
    }

    fun toggleTalent(talent: Talent) {
        _ui.update { state ->
            val selected = state.createTalents
            when {
                selected.any { it.name == talent.name } ->
                    state.copy(createTalents = selected.filterNot { it.name == talent.name })
                selected.size < 3 ->
                    state.copy(createTalents = selected + talent)
                else -> state
            }
        }
    }

    fun applyPoints(points: Points) {
        _ui.update { it.copy(createPoints = points) }
    }

    fun beginLife() {
        val state = _ui.value
        if (state.createBirth == null || state.createTalents.size != 3 ||
            state.createPoints.total != 20
        ) {
            return
        }
        viewModelScope.launch {
            _ui.update { it.copy(loading = true) }
            try {
                val settings = state.settings
                val lang = SettingsRepository.effectiveLanguage(settings.language, Locale.getDefault())
                client.newSession(
                    seed = state.currentSeed,
                    lang = lang,
                    birth = state.createBirth,
                    talents = state.createTalents,
                    points = state.createPoints,
                    maxAge = settings.maxAge,
                    narrator = buildNarrator(settings),
                    traumaOverrides = null,
                )
                val updated = settings.copy(lastSeed = state.currentSeed)
                settingsRepo.save(updated)
                _ui.update {
                    it.copy(
                        settings = updated,
                        years = emptyList(),
                        death = null,
                        loading = false,
                        screen = Screen.Timeline,
                    )
                }
                advance()
            } catch (e: CoreException) {
                _ui.update {
                    it.copy(loading = false, coreError = CoreError(e.kind, e.message ?: ""))
                }
            }
        }
    }

    // ---- timeline ---------------------------------------------------------

    fun advance() {
        if (_ui.value.nextPending) return
        viewModelScope.launch {
            _ui.update { it.copy(nextPending = true) }
            try {
                val year = client.next()
                _ui.update { s -> appendYear(s, year, pending = false).copy(coreError = null) }
            } catch (e: CoreException) {
                _ui.update {
                    it.copy(nextPending = false, coreError = CoreError(e.kind, e.message ?: ""))
                }
                if (e.kind == CoreException.Kind.PROCESS_CRASHED ||
                    e.kind == CoreException.Kind.TIMEOUT
                ) {
                    recoverAfterCrash()
                }
            }
        }
    }

    private fun appendYear(state: UiState, year: YearResult, pending: Boolean): UiState =
        if (year.died) {
            state.copy(years = state.years + year, death = year, nextPending = pending)
        } else {
            state.copy(years = state.years + year, nextPending = pending)
        }

    /** Process restarted itself: resume from checkpoint and rebuild the lost timeline. */
    private suspend fun recoverAfterCrash() {
        try {
            val cp = client.checkpointGet()
            if (!cp.exists) return
            val replayed = client.resumeSession(buildNarrator(_ui.value.settings))
            _ui.update { s ->
                s.copy(years = replayed, nextPending = false).copy(coreError = null)
            }
            val year = client.next()
            _ui.update { s -> appendYear(s, year, pending = false).copy(coreError = null) }
        } catch (_: CoreException) {
            // surfaced as coreError already; keep the UI consistent
        }
    }

    fun continueSession() {
        viewModelScope.launch {
            _ui.update { it.copy(loading = true) }
            try {
                val replayed = client.resumeSession(buildNarrator(_ui.value.settings))
                _ui.update {
                    it.copy(
                        years = replayed,
                        death = null,
                        loading = false,
                        screen = Screen.Timeline,
                    )
                }
                advance()
            } catch (e: CoreException) {
                _ui.update {
                    it.copy(loading = false, coreError = CoreError(e.kind, e.message ?: ""))
                }
            }
        }
    }

    fun nextGeneration() {
        viewModelScope.launch {
            _ui.update { it.copy(screen = Screen.Home, death = null, years = emptyList()) }
            refreshHome()
        }
    }

    fun backHome() = nextGeneration()

    // ---- settings ---------------------------------------------------------

    fun setLanguage(language: String) {
        val updated = _ui.value.settings.copy(language = language)
        settingsRepo.save(updated)
        _ui.update { it.copy(settings = updated) }
        onLanguageChanged?.invoke()
    }

    fun addProvider(
        preset: String,
        name: String,
        model: String,
        url: String,
        key: String,
    ) {
        val id = UUID.randomUUID().toString()
        val provider = LlmProviderSetting(
            id = id,
            preset = preset,
            name = name.ifBlank { preset },
            model = model,
            url = url,
            enabled = true,
        )
        saveSettings(_ui.value.settings.copy(providers = _ui.value.settings.providers + provider))
        if (key.isNotBlank()) secureStorage.put(providerKey(id), key)
    }

    fun updateProvider(
        id: String,
        name: String,
        model: String,
        url: String,
        newKey: String?,
    ) {
        val providers = _ui.value.settings.providers.map { p ->
            if (p.id == id) p.copy(name = name.ifBlank { p.preset }, model = model, url = url)
            else p
        }
        saveSettings(_ui.value.settings.copy(providers = providers))
        if (!newKey.isNullOrBlank()) secureStorage.put(providerKey(id), newKey)
    }

    fun removeProvider(id: String) {
        saveSettings(
            _ui.value.settings.copy(
                providers = _ui.value.settings.providers.filterNot { it.id == id },
            ),
        )
        secureStorage.remove(providerKey(id))
    }

    fun moveProvider(id: String, delta: Int) {
        val providers = _ui.value.settings.providers
        val from = providers.indexOfFirst { it.id == id }
        if (from < 0) return
        val to = (from + delta).coerceIn(0, providers.size - 1)
        if (to == from) return
        val reordered = providers.toMutableList().apply { add(to, removeAt(from)) }
        saveSettings(_ui.value.settings.copy(providers = reordered))
    }

    fun setProviderEnabled(id: String, enabled: Boolean) {
        val providers = _ui.value.settings.providers.map { p ->
            if (p.id == id) p.copy(enabled = enabled) else p
        }
        saveSettings(_ui.value.settings.copy(providers = providers))
    }

    fun setMaxAge(maxAge: Int) {
        saveSettings(_ui.value.settings.copy(maxAge = maxAge))
    }

    fun setNarrateRatio(ratio: Double) {
        saveSettings(_ui.value.settings.copy(narrateRatio = ratio))
    }

    fun providerHasKey(id: String): Boolean = secureStorage.get(providerKey(id)) != null

    private fun saveSettings(settings: AppSettings) {
        settingsRepo.save(settings)
        _ui.update { it.copy(settings = settings) }
    }

    private fun providerKey(id: String) = "provider_key_$id"

    /** Build the narrator config from settings; providers lacking a key are skipped. */
    private fun buildNarrator(settings: AppSettings): NarratorConfig {
        val providers = settings.providers.filter { it.enabled }.mapNotNull { p ->
            val key = secureStorage.get(providerKey(p.id)) ?: return@mapNotNull null
            if (p.model.isBlank()) return@mapNotNull null
            if (p.preset == "custom" && p.url.isBlank()) return@mapNotNull null
            NarratorProvider(provider = p.preset, model = p.model, url = p.url, key = key)
        }
        return NarratorConfig(
            enabled = providers.isNotEmpty(),
            providers = providers,
            budget = 24,
            ratio = settings.narrateRatio,
        )
    }

    class Factory(private val appContext: Context) : ViewModelProvider.Factory {
        @Suppress("UNCHECKED_CAST")
        override fun <T : ViewModel> create(modelClass: Class<T>): T =
            AppViewModel(
                client = ServiceLocator.coreClientFactory(appContext),
                secureStorage = ServiceLocator.secureStorageFactory(appContext),
                settingsRepo = SettingsRepository(appContext),
            ) as T
    }
}

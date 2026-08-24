package com.xieguiawu.rebirth

import android.content.Context
import android.content.res.Configuration
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import androidx.compose.runtime.getValue
import androidx.compose.runtime.collectAsState
import com.xieguiawu.rebirth.data.SettingsRepository
import com.xieguiawu.rebirth.ui.AppViewModel
import com.xieguiawu.rebirth.ui.Screen
import com.xieguiawu.rebirth.ui.CreateScreen
import com.xieguiawu.rebirth.ui.HomeScreen
import com.xieguiawu.rebirth.ui.SettingsScreen
import com.xieguiawu.rebirth.ui.TimelineScreen
import com.xieguiawu.rebirth.ui.TraumaPanelScreen
import com.xieguiawu.rebirth.ui.theme.RebirthTheme

class MainActivity : ComponentActivity() {

    private val viewModel: AppViewModel by viewModels { AppViewModel.Factory(applicationContext) }

    /** Apply the persisted in-app language before resources are loaded. */
    override fun attachBaseContext(newBase: Context) {
        val language = SettingsRepository(newBase).load().language
        val locale = SettingsRepository.localeForLanguage(language)
        val ctx = if (locale != null) {
            val config = Configuration(newBase.resources.configuration)
            config.setLocale(locale)
            newBase.createConfigurationContext(config)
        } else {
            newBase
        }
        super.attachBaseContext(ctx)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        viewModel.onLanguageChanged = { recreate() }
        setContent {
            RebirthTheme {
                val ui by viewModel.ui.collectAsState()
                when (ui.screen) {
                    Screen.Home -> HomeScreen(ui, viewModel)
                    Screen.Create -> CreateScreen(ui, viewModel)
                    Screen.Timeline -> TimelineScreen(ui, viewModel)
                    Screen.Trauma -> TraumaPanelScreen(ui, viewModel)
                    Screen.Settings -> SettingsScreen(ui, viewModel)
                }
            }
        }
    }
}

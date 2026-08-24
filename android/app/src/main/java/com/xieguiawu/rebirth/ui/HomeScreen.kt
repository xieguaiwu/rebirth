package com.xieguiawu.rebirth.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.xieguiawu.rebirth.R
import com.xieguiawu.rebirth.core.CoreException
import com.xieguiawu.rebirth.ui.theme.TerminalGreen
import com.xieguiawu.rebirth.ui.theme.TerminalTextDim

/** Localised rendering of a [AppViewModel.CoreError]. */
@Composable
fun coreErrorText(error: AppViewModel.CoreError?): String? {
    if (error == null) return null
    return when (error.kind) {
        CoreException.Kind.START_FAILED ->
            stringResource(R.string.core_start_failed, error.detail)
        CoreException.Kind.PROCESS_CRASHED ->
            stringResource(R.string.core_crashed, error.detail)
        CoreException.Kind.TIMEOUT -> stringResource(R.string.core_timeout)
        CoreException.Kind.PROTOCOL ->
            stringResource(R.string.core_protocol_error, error.detail)
    }
}

@Composable
fun HomeScreen(ui: AppViewModel.UiState, viewModel: AppViewModel) {
    val errorText = coreErrorText(ui.coreError)

    LaunchedEffect(ui.notice) {
        // notice is informational only; no auto-dismiss needed
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp),
    ) {
        // Header
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = stringResource(R.string.app_name),
                    style = MaterialTheme.typography.headlineMedium,
                    color = TerminalGreen,
                )
                Text(
                    text = stringResource(R.string.tagline),
                    style = MaterialTheme.typography.bodySmall,
                    color = TerminalTextDim,
                )
            }
            IconButton(onClick = { viewModel.navigate(Screen.Settings) }) {
                Icon(
                    imageVector = Icons.Filled.Settings,
                    contentDescription = stringResource(R.string.settings),
                    tint = TerminalGreen,
                )
            }
        }

        Spacer(Modifier.height(20.dp))

        if (errorText != null) {
            ErrorBanner(errorText)
            Spacer(Modifier.height(12.dp))
        }

        if (ui.notice != null) {
            Text(
                text = ui.notice,
                style = MaterialTheme.typography.bodySmall,
                color = TerminalGreen,
            )
            Spacer(Modifier.height(12.dp))
        }

        // Bloodline card
        val bloodline = ui.bloodline
        if (bloodline != null) {
            TerminalCard {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = stringResource(R.string.generation_label, bloodline.generation),
                        style = MaterialTheme.typography.titleLarge,
                        color = TerminalGreen,
                    )
                    if (ui.loading) {
                        Spacer(Modifier.width(8.dp))
                        CircularProgressIndicator(modifier = Modifier.size(14.dp), strokeWidth = 2.dp)
                    }
                }
                Spacer(Modifier.height(10.dp))
                Text(
                    text = stringResource(R.string.sensitivity_label),
                    style = MaterialTheme.typography.labelSmall,
                    color = TerminalTextDim,
                )
                Spacer(Modifier.height(4.dp))
                ValueBar(bloodline.sensitivity)
                Spacer(Modifier.height(2.dp))
                Text(
                    text = "%.3f".format(java.util.Locale.US, bloodline.sensitivity),
                    style = MaterialTheme.typography.labelSmall,
                    color = TerminalTextDim,
                )
                Spacer(Modifier.height(10.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = stringResource(R.string.inherited_talent_label),
                        style = MaterialTheme.typography.labelSmall,
                        color = TerminalTextDim,
                    )
                    Spacer(Modifier.width(8.dp))
                    Text(
                        text = bloodline.inheritedTalent.ifBlank {
                            stringResource(R.string.inherited_talent_none)
                        },
                        style = MaterialTheme.typography.bodyMedium,
                        color = if (bloodline.inheritedTalent.isBlank()) {
                            TerminalTextDim
                        } else {
                            MaterialTheme.colorScheme.secondary
                        },
                    )
                }
            }
            Spacer(Modifier.height(20.dp))
        }

        // Actions
        Button(
            onClick = { viewModel.startCreate() },
            modifier = Modifier.fillMaxWidth(),
            colors = ButtonDefaults.buttonColors(containerColor = TerminalGreen),
        ) {
            Text(stringResource(R.string.start_new_life))
        }

        Spacer(Modifier.height(10.dp))

        if (ui.checkpoint?.exists == true) {
            OutlinedButton(
                onClick = { viewModel.continueSession() },
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(stringResource(R.string.continue_life))
            }
            Spacer(Modifier.height(10.dp))
        }

        if (ui.years.isNotEmpty() || ui.death != null) {
            OutlinedButton(
                onClick = { viewModel.navigate(Screen.Trauma) },
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(stringResource(R.string.trauma_panel))
            }
        }

        Spacer(Modifier.height(24.dp))

        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(
                text = "rebirth v0.10.0 · protocol 1",
                style = MaterialTheme.typography.labelSmall,
                color = TerminalTextDim,
            )
            Text(
                text = "“A life is a seed; bloodlines are rivers.”",
                style = MaterialTheme.typography.labelSmall,
                color = TerminalTextDim.copy(alpha = 0.7f),
            )
        }
    }
}

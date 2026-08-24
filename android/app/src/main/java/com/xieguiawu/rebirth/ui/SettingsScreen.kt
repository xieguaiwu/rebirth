package com.xieguiawu.rebirth.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.KeyboardArrowUp
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import com.xieguiawu.rebirth.R
import com.xieguiawu.rebirth.data.LlmProviderSetting
import com.xieguiawu.rebirth.ui.theme.TerminalGreen
import com.xieguiawu.rebirth.ui.theme.TerminalRed
import com.xieguiawu.rebirth.ui.theme.TerminalTextDim
import java.util.Locale
import kotlin.math.roundToInt
import kotlinx.coroutines.delay

private sealed interface ProviderDialog {
    data class New(val preset: String = "deepseek") : ProviderDialog
    data class Edit(val provider: LlmProviderSetting) : ProviderDialog
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(ui: AppViewModel.UiState, viewModel: AppViewModel) {
    var dialog by remember { mutableStateOf<ProviderDialog?>(null) }
    var copied by remember { mutableStateOf(false) }
    val clipboard = LocalClipboardManager.current

    LaunchedEffect(copied) {
        if (copied) {
            delay(1500)
            copied = false
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            IconButton(onClick = { viewModel.navigate(Screen.Home) }) {
                Icon(
                    imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                    contentDescription = stringResource(R.string.back_home),
                    tint = TerminalGreen,
                )
            }
            Text(
                text = stringResource(R.string.settings),
                style = MaterialTheme.typography.titleLarge,
                color = TerminalGreen,
            )
        }

        // ---- Language -------------------------------------------------------
        TerminalCard {
            Text(stringResource(R.string.language), style = MaterialTheme.typography.titleSmall)
            Spacer(Modifier.height(6.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                LanguageChip(ui.settings.language, "system", stringResource(R.string.lang_system)) {
                    viewModel.setLanguage(it)
                }
                LanguageChip(ui.settings.language, "zh", stringResource(R.string.lang_chinese)) {
                    viewModel.setLanguage(it)
                }
                LanguageChip(ui.settings.language, "en", stringResource(R.string.lang_english)) {
                    viewModel.setLanguage(it)
                }
            }
        }

        Spacer(Modifier.height(10.dp))

        // ---- Game parameters -------------------------------------------------
        TerminalCard {
            Text(stringResource(R.string.settings_lifespan), style = MaterialTheme.typography.titleSmall)
            Row(verticalAlignment = Alignment.CenterVertically) {
                Slider(
                    value = ui.settings.maxAge.toFloat(),
                    onValueChange = { viewModel.setMaxAge(it.roundToInt()) },
                    valueRange = 40f..130f,
                    steps = 17, // 5-year increments
                    modifier = Modifier.weight(1f),
                )
                Text(
                    text = ui.settings.maxAge.toString(),
                    style = MaterialTheme.typography.labelLarge,
                    modifier = Modifier.width(40.dp),
                )
            }

            Spacer(Modifier.height(6.dp))
            Text(stringResource(R.string.settings_narrate_ratio), style = MaterialTheme.typography.titleSmall)
            Text(
                text = stringResource(R.string.narrate_ratio_desc),
                style = MaterialTheme.typography.labelSmall,
                color = TerminalTextDim,
            )
            Row(verticalAlignment = Alignment.CenterVertically) {
                Slider(
                    value = ui.settings.narrateRatio.toFloat(),
                    onValueChange = { viewModel.setNarrateRatio((it * 20).roundToInt() / 20.0) },
                    valueRange = 0f..1f,
                    steps = 19, // 0.05 increments
                    modifier = Modifier.weight(1f),
                )
                Text(
                    text = "%.2f".format(Locale.US, ui.settings.narrateRatio),
                    style = MaterialTheme.typography.labelLarge,
                    modifier = Modifier.width(40.dp),
                )
            }
        }

        Spacer(Modifier.height(10.dp))

        // ---- LLM providers ---------------------------------------------------
        TerminalCard {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = stringResource(R.string.settings_providers),
                    style = MaterialTheme.typography.titleSmall,
                    modifier = Modifier.weight(1f),
                )
                if (ui.settings.providers.none { it.enabled }) {
                    Badge(stringResource(R.string.offline_mode), TerminalGreen)
                }
            }
            Text(
                text = stringResource(R.string.providers_offline_note),
                style = MaterialTheme.typography.labelSmall,
                color = TerminalTextDim,
            )
            Spacer(Modifier.height(8.dp))

            ui.settings.providers.forEachIndexed { index, provider ->
                ProviderCard(
                    provider = provider,
                    index = index,
                    total = ui.settings.providers.size,
                    hasKey = viewModel.providerHasKey(provider.id),
                    onToggle = { viewModel.setProviderEnabled(provider.id, it) },
                    onMoveUp = { viewModel.moveProvider(provider.id, -1) },
                    onMoveDown = { viewModel.moveProvider(provider.id, +1) },
                    onDelete = { viewModel.removeProvider(provider.id) },
                    onEdit = { dialog = ProviderDialog.Edit(provider) },
                )
                Spacer(Modifier.height(6.dp))
            }

            OutlinedButton(
                onClick = { dialog = ProviderDialog.New() },
                modifier = Modifier.fillMaxWidth(),
            ) {
                Icon(Icons.Filled.Add, contentDescription = null)
                Spacer(Modifier.width(6.dp))
                Text(stringResource(R.string.add_provider))
            }
        }

        Spacer(Modifier.height(10.dp))

        // ---- Seed ------------------------------------------------------------
        TerminalCard {
            Text(stringResource(R.string.settings_seed), style = MaterialTheme.typography.titleSmall)
            Spacer(Modifier.height(4.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = if (ui.settings.lastSeed == 0L) "—" else ui.settings.lastSeed.toString(),
                    style = MaterialTheme.typography.bodyLarge,
                    modifier = Modifier.weight(1f),
                )
                TextButton(
                    enabled = ui.settings.lastSeed != 0L,
                    onClick = {
                        clipboard.setText(AnnotatedString(ui.settings.lastSeed.toString()))
                        copied = true
                    },
                ) {
                    Text(stringResource(if (copied) R.string.copied else R.string.copy))
                }
            }
        }

        Spacer(Modifier.height(20.dp))
    }

    dialog?.let { d ->
        ProviderEditDialog(
            dialog = d,
            onDismiss = { dialog = null },
            onSave = { preset, name, model, url, key ->
                when (d) {
                    is ProviderDialog.New -> viewModel.addProvider(preset, name, model, url, key)
                    is ProviderDialog.Edit -> viewModel.updateProvider(d.provider.id, name, model, url, key)
                }
                dialog = null
            },
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun LanguageChip(current: String, value: String, label: String, onSelect: (String) -> Unit) {
    FilterChip(
        selected = current == value,
        onClick = { if (current != value) onSelect(value) },
        label = { Text(label, style = MaterialTheme.typography.labelSmall) },
    )
}

@Composable
private fun ProviderCard(
    provider: LlmProviderSetting,
    index: Int,
    total: Int,
    hasKey: Boolean,
    onToggle: (Boolean) -> Unit,
    onMoveUp: () -> Unit,
    onMoveDown: () -> Unit,
    onDelete: () -> Unit,
    onEdit: () -> Unit,
) {
    Surface(
        modifier = Modifier.fillMaxWidth().clickable(onClick = onEdit),
        shape = RoundedCornerShape(8.dp),
        color = MaterialTheme.colorScheme.background,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.5f)),
    ) {
        Column(Modifier.padding(10.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = "#${index + 1} ${provider.name}",
                    style = MaterialTheme.typography.titleSmall,
                    modifier = Modifier.weight(1f),
                )
                Switch(
                    checked = provider.enabled,
                    onCheckedChange = onToggle,
                )
            }
            Text(
                text = provider.model.ifBlank { provider.preset },
                style = MaterialTheme.typography.bodySmall,
                color = TerminalTextDim,
            )
            if (provider.url.isNotBlank()) {
                Text(
                    text = provider.url,
                    style = MaterialTheme.typography.labelSmall,
                    color = TerminalTextDim,
                )
            }
            Text(
                text = if (hasKey) stringResource(R.string.provider_key_set)
                else stringResource(R.string.provider_key_missing),
                style = MaterialTheme.typography.labelSmall,
                color = if (hasKey) TerminalGreen else TerminalTextDim,
            )
            Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                IconButton(onClick = onMoveUp, enabled = index > 0) {
                    Icon(Icons.Filled.KeyboardArrowUp, contentDescription = stringResource(R.string.move_up))
                }
                IconButton(onClick = onMoveDown, enabled = index < total - 1) {
                    Icon(Icons.Filled.KeyboardArrowDown, contentDescription = stringResource(R.string.move_down))
                }
                IconButton(onClick = onDelete) {
                    Icon(
                        Icons.Filled.Delete,
                        contentDescription = stringResource(R.string.delete),
                        tint = TerminalRed,
                    )
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ProviderEditDialog(
    dialog: ProviderDialog,
    onDismiss: () -> Unit,
    onSave: (preset: String, name: String, model: String, url: String, key: String) -> Unit,
) {
    val initial = when (dialog) {
        is ProviderDialog.New -> LlmProviderSetting(
            id = "", preset = dialog.preset,
            name = when (dialog.preset) {
                "deepseek" -> "DeepSeek"
                "openrouter" -> "OpenRouter"
                else -> "Custom"
            },
            model = if (dialog.preset == "deepseek") "deepseek-v4-flash" else "",
        )
        is ProviderDialog.Edit -> dialog.provider
    }
    var preset by remember(initial.id) { mutableStateOf(initial.preset) }
    var name by remember(initial.id) { mutableStateOf(initial.name) }
    var model by remember(initial.id) { mutableStateOf(initial.model) }
    var url by remember(initial.id) { mutableStateOf(initial.url) }
    var key by remember(initial.id) { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = {
            Text(
                text = if (dialog is ProviderDialog.New) stringResource(R.string.add_provider)
                else stringResource(R.string.settings_providers),
                style = MaterialTheme.typography.titleMedium,
            )
        },
        text = {
            Column {
                if (dialog is ProviderDialog.New) {
                    Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        PresetChipInDialog(
                            selected = preset == "deepseek",
                            label = stringResource(R.string.provider_preset_deepseek),
                            onClick = {
                                preset = "deepseek"
                                if (model.isBlank()) model = "deepseek-v4-flash"
                            },
                        )
                        PresetChipInDialog(
                            selected = preset == "openrouter",
                            label = stringResource(R.string.provider_preset_openrouter),
                            onClick = { preset = "openrouter" },
                        )
                        PresetChipInDialog(
                            selected = preset == "custom",
                            label = stringResource(R.string.provider_preset_custom),
                            onClick = { preset = "custom" },
                        )
                    }
                    Spacer(Modifier.height(8.dp))
                }
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text(stringResource(R.string.provider_name)) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(
                    value = model,
                    onValueChange = { model = it },
                    label = { Text(stringResource(R.string.provider_model)) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(
                    value = url,
                    onValueChange = { url = it },
                    label = { Text(stringResource(R.string.provider_base_url)) },
                    placeholder = { Text("https://…") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(
                    value = key,
                    onValueChange = { key = it },
                    label = { Text(stringResource(R.string.provider_api_key)) },
                    placeholder = {
                        Text(
                            if (dialog is ProviderDialog.Edit) stringResource(R.string.provider_key_set)
                            else "sk-…",
                        )
                    },
                    visualTransformation = PasswordVisualTransformation(),
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        },
        confirmButton = {
            Button(
                onClick = {
                    val isCustom = preset == "custom"
                    val valid = model.isNotBlank() && (!isCustom || url.isNotBlank())
                    if (valid) onSave(preset, name, model, url, key)
                },
                enabled = model.isNotBlank() && (preset != "custom" || url.isNotBlank()),
            ) {
                Text(stringResource(R.string.save))
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) }
        },
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun PresetChipInDialog(selected: Boolean, label: String, onClick: () -> Unit) {
    FilterChip(selected = selected, onClick = onClick, label = { Text(label, style = MaterialTheme.typography.labelSmall) })
}

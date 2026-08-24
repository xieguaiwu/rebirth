package com.xieguiawu.rebirth.ui

import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.unit.dp
import com.xieguiawu.rebirth.R
import com.xieguiawu.rebirth.core.Stats
import com.xieguiawu.rebirth.core.YearResult
import com.xieguiawu.rebirth.ui.theme.TerminalGreen
import com.xieguiawu.rebirth.ui.theme.TerminalRed
import com.xieguiawu.rebirth.ui.theme.TerminalTextDim
import java.util.Locale
import kotlinx.coroutines.delay

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TimelineScreen(ui: AppViewModel.UiState, viewModel: AppViewModel) {
    val listState = rememberLazyListState()
    var autoplayMs by remember { mutableLongStateOf(0L) }

    // Autoplay loop: advance every autoplayMs until death.
    LaunchedEffect(autoplayMs) {
        while (autoplayMs > 0L && viewModel.ui.value.death == null) {
            delay(autoplayMs)
            viewModel.advance()
        }
    }

    // Follow the newest year card.
    LaunchedEffect(ui.years.size) {
        if (ui.years.isNotEmpty()) {
            listState.animateScrollToItem(ui.years.size - 1)
        }
    }

    Box(Modifier.fillMaxSize()) {
        Column(Modifier.fillMaxSize()) {
            // Header
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 8.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                IconButton(onClick = { viewModel.backHome() }) {
                    Icon(
                        imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                        contentDescription = stringResource(R.string.back_home),
                        tint = TerminalGreen,
                    )
                }
                Text(
                    text = stringResource(R.string.app_name),
                    style = MaterialTheme.typography.titleMedium,
                    color = TerminalGreen,
                    modifier = Modifier.weight(1f),
                )
                IconButton(onClick = { viewModel.navigate(Screen.Trauma) }) {
                    Icon(
                        imageVector = Icons.Filled.Info,
                        contentDescription = stringResource(R.string.trauma_panel),
                        tint = TerminalGreen,
                    )
                }
            }

            // Autoplay controls
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = stringResource(R.string.autoplay),
                    style = MaterialTheme.typography.labelSmall,
                    color = TerminalTextDim,
                )
                AutoChip(stringResource(R.string.speed_05), 500L, autoplayMs) { autoplayMs = it }
                AutoChip(stringResource(R.string.speed_1), 1000L, autoplayMs) { autoplayMs = it }
                AutoChip(stringResource(R.string.speed_2), 2000L, autoplayMs) { autoplayMs = it }
            }

            val errorText = coreErrorText(ui.coreError)
            if (errorText != null) {
                ErrorBanner(errorText, Modifier.padding(horizontal = 16.dp, vertical = 6.dp))
            }

            if (ui.death != null) {
                DeathScreen(ui, viewModel)
            } else {
                LazyColumn(
                    state = listState,
                    modifier = Modifier.weight(1f),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(
                        start = 16.dp, end = 16.dp, top = 8.dp, bottom = 96.dp,
                    ),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    if (ui.years.isEmpty() && !ui.nextPending) {
                        item { EmptyHint(stringResource(R.string.play_hint), Modifier.padding(top = 40.dp)) }
                    }
                    itemsIndexed(ui.years) { index, year ->
                        YearCard(
                            year = year,
                            previous = ui.years.getOrNull(index - 1),
                            key = year.age,
                        )
                    }
                    if (ui.nextPending) {
                        item { FateWeavingCard() }
                    }
                }
            }
        }

        // Bottom action
        if (ui.death == null) {
            ExtendedFloatingActionButton(
                onClick = { viewModel.advance() },
                modifier = Modifier
                    .align(Alignment.BottomEnd)
                    .padding(16.dp),
                icon = { Icon(Icons.Filled.PlayArrow, contentDescription = null) },
                text = { Text(stringResource(R.string.next_year)) },
                containerColor = TerminalGreen,
                contentColor = Color(0xFF001A08),
            )
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AutoChip(
    label: String,
    millis: Long,
    current: Long,
    onSelect: (Long) -> Unit,
) {
    val selected = current == millis
    FilterChip(
        selected = selected,
        onClick = { onSelect(if (selected) 0L else millis) },
        label = { Text(label, style = MaterialTheme.typography.labelSmall) },
    )
}

/** Pending-card shimmer while the core is advancing (possibly an LLM call). */
@Composable
private fun FateWeavingCard() {
    val transition = rememberInfiniteTransition(label = "fate")
    val alpha by transition.animateFloat(
        initialValue = 0.35f,
        targetValue = 0.95f,
        animationSpec = infiniteRepeatable(tween(650), RepeatMode.Reverse),
        label = "alpha",
    )
    Surface(
        modifier = Modifier.fillMaxWidth().alpha(alpha),
        shape = RoundedCornerShape(8.dp),
        color = MaterialTheme.colorScheme.surfaceVariant,
        border = BorderStroke(1.dp, TerminalGreen.copy(alpha = 0.6f)),
    ) {
        Row(
            modifier = Modifier.padding(14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            CircularProgressIndicator(
                modifier = Modifier.width(14.dp).height(14.dp),
                strokeWidth = 2.dp,
                color = TerminalGreen,
            )
            Spacer(Modifier.width(10.dp))
            Text(
                text = stringResource(R.string.fate_weaving),
                style = MaterialTheme.typography.bodyMedium,
                color = TerminalGreen,
            )
        }
    }
}

@Composable
private fun YearCard(year: YearResult, previous: YearResult?, key: Int) {
    var expanded by remember(key) { mutableStateOf(false) }
    val pathological = year.trauma.pathological
    val borderColor = if (pathological) TerminalRed else MaterialTheme.colorScheme.outline.copy(alpha = 0.5f)

    Surface(
        modifier = Modifier.fillMaxWidth().clickable { expanded = !expanded },
        shape = RoundedCornerShape(8.dp),
        color = MaterialTheme.colorScheme.surfaceVariant,
        border = BorderStroke(if (pathological) 2.dp else 1.dp, borderColor),
    ) {
        Column(Modifier.padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Badge(
                    text = stringResource(R.string.age_badge, year.age),
                    color = TerminalGreen,
                )
                year.career?.let { career ->
                    Spacer(Modifier.width(6.dp))
                    Badge(career.name.ifBlank { career.id }, MaterialTheme.colorScheme.secondary)
                }
                year.careerChange?.let { change ->
                    Spacer(Modifier.width(6.dp))
                    val label = when (change) {
                        "enter" -> stringResource(R.string.career_enter)
                        "quit" -> stringResource(R.string.career_quit)
                        "retire" -> stringResource(R.string.career_retire)
                        else -> change
                    }
                    Badge(label, MaterialTheme.colorScheme.tertiary)
                }
            }
            year.lines.forEach { line ->
                Text(
                    text = line,
                    style = MaterialTheme.typography.bodyMedium,
                    modifier = Modifier.padding(top = 6.dp),
                )
            }
            // Event badges
            year.event?.let { event ->
                Row(
                    modifier = Modifier.padding(top = 6.dp),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    if (event.traumaAlpha > 0.0) {
                        Badge("⚡ " + stringResource(R.string.badge_trauma), TerminalRed)
                    }
                    if (event.therapyQ > 0.0) {
                        Badge("🩹 " + stringResource(R.string.badge_heal), TerminalGreen)
                    }
                    if (event.llm) {
                        Badge("✨ " + stringResource(R.string.badge_llm), MaterialTheme.colorScheme.secondary)
                    }
                }
            }
            if (pathological) {
                Text(
                    text = "🔴 " + stringResource(R.string.badge_pathological),
                    style = MaterialTheme.typography.labelSmall,
                    color = TerminalRed,
                    modifier = Modifier.padding(top = 6.dp),
                )
            }
            // Expanded details: stats + deltas, trauma state, luck
            if (expanded) {
                Spacer(Modifier.height(8.dp))
                Text(
                    text = stringResource(R.string.details_title),
                    style = MaterialTheme.typography.labelSmall,
                    color = TerminalTextDim,
                )
                StatsRow(year.stats, previous?.stats)
                Text(
                    text = stringResource(R.string.trauma_panel) + ": m=%.2f a=%.2f p=%.2f L=%.2f".format(
                        Locale.US, year.trauma.m, year.trauma.a, year.trauma.p, year.trauma.load,
                    ),
                    style = MaterialTheme.typography.labelSmall,
                    color = TerminalTextDim,
                    modifier = Modifier.padding(top = 4.dp),
                )
                Text(
                    text = stringResource(R.string.luck_label) + ": %+.2f".format(Locale.US, year.luck),
                    style = MaterialTheme.typography.labelSmall,
                    color = TerminalTextDim,
                    modifier = Modifier.padding(top = 2.dp),
                )
            }
        }
    }
}

@Composable
private fun StatsRow(stats: Stats, previous: Stats?) {
    fun fmt(v: Double) = "%.1f".format(Locale.US, v)
    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        StatCell("CHR", fmt(stats.chr), previous?.chr)
        StatCell("INT", fmt(stats.int), previous?.int)
        StatCell("STR", fmt(stats.str), previous?.str)
        StatCell("MNY", fmt(stats.mny), previous?.mny)
        StatCell("SPR", fmt(stats.spr), previous?.spr)
    }
}

@Composable
private fun StatCell(label: String, value: String, previous: Double?) {
    Column {
        Text(label, style = MaterialTheme.typography.labelSmall, color = TerminalTextDim)
        Text(value, style = MaterialTheme.typography.bodySmall)
        if (previous != null) {
            val delta = value.toDoubleOrNull()?.minus(previous) ?: 0.0
            Text(
                text = "%+.1f".format(Locale.US, delta),
                style = MaterialTheme.typography.labelSmall,
                color = if (delta >= 0) TerminalGreen else TerminalRed,
            )
        }
    }
}

/** Full-screen end-of-life view. */
@Composable
private fun DeathScreen(ui: AppViewModel.UiState, viewModel: AppViewModel) {
    val death = ui.death ?: return
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(androidx.compose.foundation.rememberScrollState())
            .padding(16.dp),
    ) {
        Text(
            text = stringResource(R.string.died_label, death.age),
            style = MaterialTheme.typography.headlineMedium,
            color = TerminalRed,
        )
        Spacer(Modifier.height(12.dp))

        if (death.deathStatus != null) {
            Text(
                text = stringResource(R.string.death_cause),
                style = MaterialTheme.typography.labelSmall,
                color = TerminalTextDim,
            )
            Badge(death.deathStatus, TerminalRed)
            Spacer(Modifier.height(12.dp))
        }

        Text(
            text = stringResource(R.string.epitaph_label),
            style = MaterialTheme.typography.labelSmall,
            color = TerminalTextDim,
        )
        Surface(
            modifier = Modifier.fillMaxWidth().padding(top = 4.dp),
            shape = RoundedCornerShape(8.dp),
            color = MaterialTheme.colorScheme.surfaceVariant,
            border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline.copy(alpha = 0.5f)),
        ) {
            Text(
                text = death.epitaph ?: stringResource(R.string.session_finished),
                style = MaterialTheme.typography.bodyLarge,
                fontStyle = FontStyle.Italic,
                modifier = Modifier.padding(12.dp),
            )
        }

        Spacer(Modifier.height(16.dp))
        Text(
            text = stringResource(R.string.final_stats),
            style = MaterialTheme.typography.titleSmall,
        )
        Spacer(Modifier.height(6.dp))
        FinalStatBar(stringResource(R.string.attr_chr_full), death.stats.chr)
        FinalStatBar(stringResource(R.string.attr_int_full), death.stats.int)
        FinalStatBar(stringResource(R.string.attr_str_full), death.stats.str)
        FinalStatBar(stringResource(R.string.attr_mny_full), death.stats.mny)
        FinalStatBar(stringResource(R.string.attr_spr_full), death.stats.spr)

        Spacer(Modifier.height(12.dp))
        Text(
            text = stringResource(R.string.death_note),
            style = MaterialTheme.typography.bodySmall,
            color = TerminalTextDim,
        )

        Spacer(Modifier.height(20.dp))
        Button(
            onClick = { viewModel.nextGeneration() },
            modifier = Modifier.fillMaxWidth(),
            colors = ButtonDefaults.buttonColors(containerColor = TerminalGreen),
        ) {
            Text(stringResource(R.string.next_generation_btn))
        }
        Spacer(Modifier.height(8.dp))
        OutlinedButton(
            onClick = { viewModel.backHome() },
            modifier = Modifier.fillMaxWidth(),
        ) {
            Text(stringResource(R.string.back_home))
        }
    }
}

@Composable
private fun FinalStatBar(label: String, value: Double) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            color = TerminalTextDim,
            modifier = Modifier.width(64.dp),
        )
        ValueBar(value / 10.0, TerminalGreen, Modifier.weight(1f))
        Spacer(Modifier.width(8.dp))
        Text(
            text = "%.1f".format(Locale.US, value),
            style = MaterialTheme.typography.labelSmall,
            modifier = Modifier.width(32.dp),
        )
    }
}

package com.xieguiawu.rebirth.ui

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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.xieguiawu.rebirth.R
import com.xieguiawu.rebirth.core.Birth
import com.xieguiawu.rebirth.core.Effects
import com.xieguiawu.rebirth.core.Points
import com.xieguiawu.rebirth.core.Talent
import com.xieguiawu.rebirth.ui.theme.TerminalGreen
import com.xieguiawu.rebirth.ui.theme.TerminalTextDim
import com.xieguiawu.rebirth.ui.theme.rarityColor
import com.xieguiawu.rebirth.ui.theme.rarityStars
import kotlin.math.roundToInt

/** "CHR+1 MNY-2" style summary of a non-zero effect delta. */
private fun effectsText(e: Effects): String {
    val parts = mutableListOf<String>()
    if (e.chr != 0.0) parts += "CHR%+d".format(java.util.Locale.US, e.chr.toInt())
    if (e.int != 0.0) parts += "INT%+d".format(java.util.Locale.US, e.int.toInt())
    if (e.str != 0.0) parts += "STR%+d".format(java.util.Locale.US, e.str.toInt())
    if (e.mny != 0.0) parts += "MNY%+d".format(java.util.Locale.US, e.mny.toInt())
    if (e.spr != 0.0) parts += "SPR%+d".format(java.util.Locale.US, e.spr.toInt())
    return parts.joinToString(" ")
}

@Composable
fun CreateScreen(ui: AppViewModel.UiState, viewModel: AppViewModel) {
    val remaining = 20 - ui.createPoints.total
    val canBegin = ui.createBirth != null && ui.createTalents.size == 3 && remaining == 0

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(16.dp),
    ) {
        // Header
        Row(verticalAlignment = Alignment.CenterVertically) {
            IconButton(onClick = { viewModel.navigate(Screen.Home) }) {
                Icon(
                    imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                    contentDescription = stringResource(R.string.back_home),
                    tint = TerminalGreen,
                )
            }
            Text(
                text = stringResource(R.string.app_name),
                style = MaterialTheme.typography.titleLarge,
                color = TerminalGreen,
            )
        }

        if (ui.loading) {
            Box(Modifier.fillMaxWidth().padding(24.dp), contentAlignment = Alignment.Center) {
                CircularProgressIndicator()
            }
            return@Column
        }

        // ---- Step 1: birth ------------------------------------------------
        Spacer(Modifier.height(12.dp))
        Text(
            text = stringResource(R.string.create_origin_title),
            style = MaterialTheme.typography.titleMedium,
        )
        Spacer(Modifier.height(8.dp))
        ui.births.forEach { birth ->
            BirthCard(
                birth = birth,
                selected = ui.createBirth?.id == birth.id,
                onClick = { viewModel.selectBirth(birth) },
            )
            Spacer(Modifier.height(8.dp))
        }

        // ---- Step 2: talents ----------------------------------------------
        Spacer(Modifier.height(12.dp))
        Text(
            text = stringResource(R.string.create_talent_title),
            style = MaterialTheme.typography.titleMedium,
        )
        Text(
            text = "${ui.createTalents.size}/3",
            style = MaterialTheme.typography.labelMedium,
            color = if (ui.createTalents.size == 3) TerminalGreen else TerminalTextDim,
        )
        Spacer(Modifier.height(8.dp))
        ui.talents.chunked(2).forEach { row ->
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                row.forEach { talent ->
                    TalentCard(
                        talent = talent,
                        selected = ui.createTalents.any { it.name == talent.name },
                        onClick = { viewModel.toggleTalent(talent) },
                        modifier = Modifier.weight(1f),
                    )
                }
                if (row.size == 1) Spacer(Modifier.weight(1f))
            }
            Spacer(Modifier.height(8.dp))
        }

        // ---- Step 3: points ------------------------------------------------
        Spacer(Modifier.height(12.dp))
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                text = stringResource(R.string.create_points_title),
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.weight(1f),
            )
            Text(
                text = stringResource(R.string.points_remaining, remaining),
                style = MaterialTheme.typography.labelLarge,
                color = if (remaining == 0) TerminalGreen else TerminalTextDim,
            )
        }
        Spacer(Modifier.height(4.dp))
        PointSlider(
            label = stringResource(R.string.attr_chr),
            value = ui.createPoints.chr,
            remaining = remaining,
            onValue = { v -> viewModel.applyPoints(updatePoint(ui.createPoints, "chr", v)) },
        )
        PointSlider(
            label = stringResource(R.string.attr_int),
            value = ui.createPoints.int,
            remaining = remaining,
            onValue = { v -> viewModel.applyPoints(updatePoint(ui.createPoints, "int", v)) },
        )
        PointSlider(
            label = stringResource(R.string.attr_str),
            value = ui.createPoints.str,
            remaining = remaining,
            onValue = { v -> viewModel.applyPoints(updatePoint(ui.createPoints, "str", v)) },
        )
        PointSlider(
            label = stringResource(R.string.attr_mny),
            value = ui.createPoints.mny,
            remaining = remaining,
            onValue = { v -> viewModel.applyPoints(updatePoint(ui.createPoints, "mny", v)) },
        )

        Spacer(Modifier.height(8.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
            PresetChip(stringResource(R.string.preset_balanced), Points(5, 5, 5, 5)) { viewModel.applyPoints(it) }
            PresetChip(stringResource(R.string.preset_chr), Points(8, 4, 4, 4)) { viewModel.applyPoints(it) }
            PresetChip(stringResource(R.string.preset_int), Points(4, 8, 4, 4)) { viewModel.applyPoints(it) }
            PresetChip(stringResource(R.string.preset_str), Points(4, 4, 8, 4)) { viewModel.applyPoints(it) }
            PresetChip(stringResource(R.string.preset_mny), Points(4, 4, 4, 8)) { viewModel.applyPoints(it) }
        }

        // ---- Begin ----------------------------------------------------------
        Spacer(Modifier.height(20.dp))
        Button(
            onClick = { viewModel.beginLife() },
            enabled = canBegin,
            modifier = Modifier.fillMaxWidth(),
            colors = ButtonDefaults.buttonColors(
                containerColor = TerminalGreen,
                disabledContainerColor = TerminalGreen.copy(alpha = 0.25f),
            ),
        ) {
            Text(stringResource(R.string.begin_life))
        }
        Spacer(Modifier.height(20.dp))
    }
}

@Composable
private fun BirthCard(birth: Birth, selected: Boolean, onClick: () -> Unit) {
    val borderColor = if (selected) TerminalGreen else MaterialTheme.colorScheme.outline.copy(alpha = 0.5f)
    Surface(
        modifier = Modifier.fillMaxWidth().clickable(onClick = onClick),
        shape = RoundedCornerShape(8.dp),
        color = if (selected) TerminalGreen.copy(alpha = 0.10f) else MaterialTheme.colorScheme.surfaceVariant,
        border = BorderStroke(if (selected) 2.dp else 1.dp, borderColor),
    ) {
        Column(Modifier.padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = birth.name,
                    style = MaterialTheme.typography.titleMedium,
                    color = if (selected) TerminalGreen else MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.weight(1f),
                )
                if (selected) {
                    Icon(Icons.Filled.Check, contentDescription = null, tint = TerminalGreen)
                }
            }
            Text(
                text = birth.desc,
                style = MaterialTheme.typography.bodySmall,
                color = TerminalTextDim,
            )
            Row {
                val bonus = effectsText(birth.bonus)
                if (bonus.isNotBlank()) {
                    Badge(bonus, MaterialTheme.colorScheme.secondary)
                    Spacer(Modifier.width(6.dp))
                }
                if (birth.sensitivityAdd != 0.0) {
                    Badge(
                        stringResource(
                            R.string.sensitivity_delta,
                            "%.2f".format(java.util.Locale.US, birth.sensitivityAdd),
                        ),
                        MaterialTheme.colorScheme.error,
                    )
                }
            }
        }
    }
}

@Composable
private fun TalentCard(
    talent: Talent,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val rarity = rarityColor(talent.rarity)
    Surface(
        modifier = modifier.clickable(onClick = onClick),
        shape = RoundedCornerShape(8.dp),
        color = if (selected) TerminalGreen.copy(alpha = 0.10f) else MaterialTheme.colorScheme.surfaceVariant,
        border = BorderStroke(
            if (selected) 2.dp else 1.dp,
            if (selected) TerminalGreen else rarity.copy(alpha = 0.5f),
        ),
    ) {
        Column(Modifier.padding(10.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = talent.name,
                    style = MaterialTheme.typography.titleSmall,
                    color = if (selected) TerminalGreen else MaterialTheme.colorScheme.onSurface,
                    modifier = Modifier.weight(1f),
                )
                if (selected) {
                    Icon(Icons.Filled.Check, contentDescription = null, tint = TerminalGreen)
                }
            }
            Text(
                text = "★".repeat(rarityStars(talent.rarity)),
                style = MaterialTheme.typography.labelSmall,
                color = rarity,
            )
            Text(
                text = talent.desc,
                style = MaterialTheme.typography.bodySmall,
                color = TerminalTextDim,
            )
            val bonus = effectsText(talent.bonus)
            if (bonus.isNotBlank()) {
                Text(
                    text = bonus,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.secondary,
                )
            }
        }
    }
}

@Composable
private fun PointSlider(
    label: String,
    value: Int,
    remaining: Int,
    onValue: (Int) -> Unit,
) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text(
            text = label,
            style = MaterialTheme.typography.labelLarge,
            color = TerminalTextDim,
            modifier = Modifier.width(52.dp),
        )
        Slider(
            value = value.toFloat(),
            onValueChange = { onValue(it.roundToInt()) },
            valueRange = 0f..10f,
            modifier = Modifier.weight(1f),
        )
        Text(
            text = value.toString(),
            style = MaterialTheme.typography.labelLarge,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.width(24.dp),
        )
    }
}

@Composable
private fun PresetChip(label: String, points: Points, onClick: (Points) -> Unit) {
    OutlinedButton(onClick = { onClick(points) }) {
        Text(label, style = MaterialTheme.typography.labelSmall)
    }
}

/** Clamp a single attribute change so the total never exceeds 20. */
private fun updatePoint(points: Points, field: String, newValue: Int): Points {
    val current = when (field) {
        "chr" -> points.chr
        "int" -> points.int
        "str" -> points.str
        else -> points.mny
    }
    val remaining = 20 - points.total
    val clamped = newValue.coerceIn(0, 10)
    val value = if (clamped - current > remaining) current + remaining else clamped
    return when (field) {
        "chr" -> points.copy(chr = value)
        "int" -> points.copy(int = value)
        "str" -> points.copy(str = value)
        else -> points.copy(mny = value)
    }
}

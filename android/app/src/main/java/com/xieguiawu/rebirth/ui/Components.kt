package com.xieguiawu.rebirth.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.xieguiawu.rebirth.ui.theme.TerminalGreen
import com.xieguiawu.rebirth.ui.theme.TerminalRed
import com.xieguiawu.rebirth.ui.theme.TerminalTextDim

/** Standard card surface used across screens. */
@Composable
fun TerminalCard(
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(8.dp),
        color = MaterialTheme.colorScheme.surfaceVariant,
        border = androidx.compose.foundation.BorderStroke(
            1.dp,
            MaterialTheme.colorScheme.outline.copy(alpha = 0.5f),
        ),
    ) {
        Column(modifier = Modifier.padding(12.dp)) { content() }
    }
}

/** Horizontal bar (0..1) for sensitivity / stats. */
@Composable
fun ValueBar(
    value: Double,
    color: Color = TerminalGreen,
    modifier: Modifier = Modifier,
    height: Dp = 8.dp,
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(height)
            .clip(RoundedCornerShape(4.dp))
            .background(MaterialTheme.colorScheme.background),
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth(value.coerceIn(0.0, 1.0).toFloat())
                .height(height)
                .clip(RoundedCornerShape(4.dp))
                .background(color),
        )
    }
}

/** Small bordered chip (age badge / career / event badges). */
@Composable
fun Badge(
    text: String,
    color: Color = TerminalTextDim,
    modifier: Modifier = Modifier,
) {
    Surface(
        modifier = modifier,
        shape = RoundedCornerShape(4.dp),
        color = Color.Transparent,
        border = androidx.compose.foundation.BorderStroke(1.dp, color),
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.labelSmall,
            color = color,
            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp),
        )
    }
}

/** Centered monospace dim text for empty states. */
@Composable
fun EmptyHint(text: String, modifier: Modifier = Modifier) {
    Box(modifier = modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
        Text(text, style = MaterialTheme.typography.bodyMedium, color = TerminalTextDim)
    }
}

/** Row of legend dots + labels for the trauma chart. */
@Composable
fun LegendRow(entries: List<Pair<Color, String>>) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        entries.forEach { (color, label) ->
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    modifier = Modifier
                        .size(8.dp)
                        .clip(RoundedCornerShape(2.dp))
                        .background(color),
                )
                Text(
                    text = label,
                    style = MaterialTheme.typography.labelSmall,
                    color = TerminalTextDim,
                    modifier = Modifier.padding(start = 4.dp),
                )
            }
        }
    }
}

/** Red error surface for core failures. */
@Composable
fun ErrorBanner(text: String, modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(6.dp),
        color = TerminalRed.copy(alpha = 0.15f),
        border = androidx.compose.foundation.BorderStroke(1.dp, TerminalRed),
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.bodySmall,
            color = TerminalRed,
            modifier = Modifier.padding(10.dp),
        )
    }
}

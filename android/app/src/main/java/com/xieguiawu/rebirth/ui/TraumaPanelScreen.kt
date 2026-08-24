package com.xieguiawu.rebirth.ui

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.xieguiawu.rebirth.R
import com.xieguiawu.rebirth.core.TraumaState
import com.xieguiawu.rebirth.core.YearResult
import com.xieguiawu.rebirth.ui.theme.TerminalBlue
import com.xieguiawu.rebirth.ui.theme.TerminalGreen
import com.xieguiawu.rebirth.ui.theme.TerminalOrange
import com.xieguiawu.rebirth.ui.theme.TerminalPurple
import com.xieguiawu.rebirth.ui.theme.TerminalRed
import com.xieguiawu.rebirth.ui.theme.TerminalTextDim

/**
 * The core-selling visualisation: hand-drawn Canvas chart of the trauma ODE
 * (M/A/P traces + load line) with the hysteresis double-threshold band
 * (EnterAt 0.80 red dashed / ExitAt 0.35 green dashed).
 */
@Composable
fun TraumaPanelScreen(ui: AppViewModel.UiState, viewModel: AppViewModel) {
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
                text = stringResource(R.string.trauma_panel),
                style = MaterialTheme.typography.titleLarge,
                color = TerminalGreen,
            )
        }

        if (ui.years.isEmpty()) {
            EmptyHint(stringResource(R.string.trauma_empty), Modifier.padding(top = 48.dp))
            return@Column
        }

        Spacer(Modifier.height(8.dp))
        TerminalCard {
            TraumaChart(ui.years)
            Spacer(Modifier.height(8.dp))
            LegendRow(
                listOf(
                    TerminalBlue to stringResource(R.string.trauma_m),
                    TerminalOrange to stringResource(R.string.trauma_a),
                    TerminalPurple to stringResource(R.string.trauma_p),
                    TerminalGreen to stringResource(R.string.trauma_load),
                ),
            )
        }

        Spacer(Modifier.height(12.dp))
        TerminalCard {
            Text(
                text = stringResource(R.string.trauma_threshold_enter) + "  /  " +
                    stringResource(R.string.trauma_threshold_exit),
                style = MaterialTheme.typography.titleSmall,
                color = TerminalRed,
            )
            Spacer(Modifier.height(6.dp))
            Text(
                text = stringResource(R.string.trauma_hysteresis_note),
                style = MaterialTheme.typography.bodySmall,
                color = TerminalTextDim,
            )
        }

        Spacer(Modifier.height(12.dp))
        TerminalCard {
            Text(
                text = stringResource(R.string.trauma_bifurcation_note),
                style = MaterialTheme.typography.bodySmall,
                color = TerminalTextDim,
            )
        }
    }
}

private const val ENTER_AT = 0.80f
private const val EXIT_AT = 0.35f

@Composable
private fun TraumaChart(years: List<YearResult>) {
    val textMeasurer = rememberTextMeasurer()
    val points = years.map { it.age to it.trauma }
    val maxAge = maxOf(points.lastOrNull()?.first ?: 20, 20).toFloat()

    androidx.compose.foundation.Canvas(
        modifier = Modifier.fillMaxWidth().height(300.dp),
    ) {
        val padL = 36.dp.toPx()
        val padR = 10.dp.toPx()
        val padT = 12.dp.toPx()
        val padB = 22.dp.toPx()
        val plotW = size.width - padL - padR
        val plotH = size.height - padT - padB

        fun x(age: Float) = padL + (age / maxAge) * plotW
        fun y(v: Float) = padT + (1f - v.coerceIn(0f, 1f)) * plotH

        // Hysteresis band between the two thresholds.
        drawRect(
            color = TerminalPurple.copy(alpha = 0.08f),
            topLeft = Offset(padL, y(ENTER_AT)),
            size = Size(plotW, y(EXIT_AT) - y(ENTER_AT)),
        )

        // Threshold lines: red dashed (enter), green dashed (exit).
        val dash = PathEffect.dashPathEffect(floatArrayOf(12f, 8f), 0f)
        drawLine(
            color = TerminalRed.copy(alpha = 0.85f),
            start = Offset(padL, y(ENTER_AT)),
            end = Offset(padL + plotW, y(ENTER_AT)),
            strokeWidth = 1.5.dp.toPx(),
            pathEffect = dash,
        )
        drawLine(
            color = TerminalGreen.copy(alpha = 0.85f),
            start = Offset(padL, y(EXIT_AT)),
            end = Offset(padL + plotW, y(EXIT_AT)),
            strokeWidth = 1.5.dp.toPx(),
            pathEffect = dash,
        )

        // Axes.
        val axisColor = TerminalTextDim.copy(alpha = 0.4f)
        drawLine(axisColor, Offset(padL, padT + plotH), Offset(padL + plotW, padT + plotH), 1.dp.toPx())
        drawLine(axisColor, Offset(padL, padT), Offset(padL, padT + plotH), 1.dp.toPx())

        // M / A / P series + thick load line.
        fun series(selector: (TraumaState) -> Double, color: Color, width: Float) {
            if (points.size < 2) {
                points.firstOrNull()?.let { (age, t) ->
                    drawCircle(color, 4.dp.toPx(), Offset(x(age.toFloat()), y(selector(t).toFloat())))
                }
                return
            }
            val path = Path()
            points.forEachIndexed { i, (age, t) ->
                val px = x(age.toFloat())
                val py = y(selector(t).toFloat())
                if (i == 0) path.moveTo(px, py) else path.lineTo(px, py)
            }
            drawPath(path, color, style = Stroke(width, cap = StrokeCap.Round, join = StrokeJoin.Round))
        }
        series({ it.m }, TerminalBlue, 2.dp.toPx())
        series({ it.a }, TerminalOrange, 2.dp.toPx())
        series({ it.p }, TerminalPurple, 2.dp.toPx())
        series({ it.load }, TerminalGreen, 4.dp.toPx())

        // Pathological years: red dot on the load line.
        points.filter { it.second.pathological }.forEach { (age, t) ->
            drawCircle(TerminalRed, 4.dp.toPx(), Offset(x(age.toFloat()), y(t.load.toFloat())))
        }

        // Labels.
        val labelStyle = TextStyle(color = TerminalTextDim, fontSize = 9.sp)
        drawText(textMeasurer, "0.80", Offset(2f, y(ENTER_AT) - 5f), labelStyle)
        drawText(textMeasurer, "0.35", Offset(2f, y(EXIT_AT) - 5f), labelStyle)
        drawText(textMeasurer, "1", Offset(padL - 10f, padT - 5f), labelStyle)
        drawText(textMeasurer, "0", Offset(padL - 10f, padT + plotH - 5f), labelStyle)
        drawText(textMeasurer, maxAge.toInt().toString(), Offset(padL + plotW - 14f, padT + plotH + 4f), labelStyle)
    }
}

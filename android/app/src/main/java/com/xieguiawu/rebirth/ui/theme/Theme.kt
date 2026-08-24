package com.xieguiawu.rebirth.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

// Terminal / CRT palette (PICO-8 inspired)
val TerminalGreen = Color(0xFF00E436)
val TerminalGreenDim = Color(0xFF0F5A2B)
val TerminalBg = Color(0xFF0C1116)
val TerminalSurface = Color(0xFF141B22)
val TerminalSurfaceHigh = Color(0xFF1B242E)
val TerminalRed = Color(0xFFFF4D5E)
val TerminalOrange = Color(0xFFFFA300)
val TerminalBlue = Color(0xFF29ADFF)
val TerminalPurple = Color(0xFF83769C)
val TerminalAmber = Color(0xFFFFCCAA)
val TerminalText = Color(0xFFD6E4D6)
val TerminalTextDim = Color(0xFF8A9A8A)

/** Rarity colour ramp for talents: common / rare / epic / legendary. */
fun rarityColor(rarity: String): Color = when (rarity) {
    "rare" -> TerminalBlue
    "epic" -> TerminalPurple
    "legendary" -> TerminalOrange
    else -> TerminalTextDim
}

/** Rarity star count for talents (matches the CLI RarityStars marker). */
fun rarityStars(rarity: String): Int = when (rarity) {
    "rare" -> 2
    "epic" -> 3
    "legendary" -> 4
    else -> 1
}

private val DarkColors = darkColorScheme(
    primary = TerminalGreen,
    onPrimary = Color(0xFF001A08),
    secondary = TerminalBlue,
    onSecondary = Color(0xFF001220),
    tertiary = TerminalPurple,
    background = TerminalBg,
    onBackground = TerminalText,
    surface = TerminalSurface,
    onSurface = TerminalText,
    surfaceVariant = TerminalSurfaceHigh,
    onSurfaceVariant = TerminalTextDim,
    error = TerminalRed,
    onError = Color(0xFF2D0006),
    outline = TerminalGreenDim,
)

private val TerminalTypography = Typography(
    headlineMedium = TextStyle(
        fontFamily = FontFamily.Monospace,
        fontWeight = FontWeight.Bold,
        fontSize = 26.sp,
    ),
    titleLarge = TextStyle(
        fontFamily = FontFamily.Monospace,
        fontWeight = FontWeight.Bold,
        fontSize = 21.sp,
    ),
    titleMedium = TextStyle(
        fontFamily = FontFamily.Monospace,
        fontWeight = FontWeight.Bold,
        fontSize = 16.sp,
    ),
    titleSmall = TextStyle(
        fontFamily = FontFamily.Monospace,
        fontWeight = FontWeight.Bold,
        fontSize = 14.sp,
    ),
    bodyLarge = TextStyle(fontFamily = FontFamily.Monospace, fontSize = 15.sp),
    bodyMedium = TextStyle(fontFamily = FontFamily.Monospace, fontSize = 13.sp),
    bodySmall = TextStyle(fontFamily = FontFamily.Monospace, fontSize = 11.sp),
    labelLarge = TextStyle(
        fontFamily = FontFamily.Monospace,
        fontWeight = FontWeight.Bold,
        fontSize = 14.sp,
    ),
    labelMedium = TextStyle(fontFamily = FontFamily.Monospace, fontSize = 12.sp),
    labelSmall = TextStyle(fontFamily = FontFamily.Monospace, fontSize = 10.sp),
)

@Composable
fun RebirthTheme(
    @Suppress("UNUSED_PARAMETER") darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    // The terminal theme is dark-only by design.
    MaterialTheme(
        colorScheme = DarkColors,
        typography = TerminalTypography,
        content = content,
    )
}

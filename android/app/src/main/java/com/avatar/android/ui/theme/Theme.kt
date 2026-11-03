package com.avatar.android.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable

private val DarkColorScheme = darkColorScheme(
    primary = AccentGreen,
    onPrimary = OnAccentGreen,
    secondary = AccentCyan,
    onSecondary = BgDark,
    tertiary = AccentPurple,
    onTertiary = BgDark,
    background = BgDark,
    surface = CardDark,
    surfaceVariant = CardElevated,
    outline = CardBorder,
    onBackground = TextPrimary,
    onSurface = TextPrimary,
    error = AccentRed
)

@Composable
fun AvatarTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = DarkColorScheme,
        content = content
    )
}

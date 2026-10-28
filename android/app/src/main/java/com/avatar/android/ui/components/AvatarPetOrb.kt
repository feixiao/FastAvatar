package com.avatar.android.ui.components

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.avatar.android.core.state.SessionState
import com.avatar.android.ui.theme.AccentCyan
import com.avatar.android.ui.theme.AccentGreen
import com.avatar.android.ui.theme.AccentPurple
import com.avatar.android.ui.theme.AccentYellow
import com.avatar.android.ui.theme.TextMuted
import com.avatar.android.ui.theme.TextPrimary
import com.avatar.android.ui.theme.TextSecondary
import kotlinx.coroutines.launch
import kotlin.math.cos
import kotlin.math.sin

@Composable
fun AvatarPetOrb(
    sessionState: SessionState,
    speechVolume: Float = 0f,
    activePersona: String = "companion",
    onPet: () -> Unit,
    modifier: Modifier = Modifier
) {
    val haptic = LocalHapticFeedback.current
    val coroutineScope = rememberCoroutineScope()
    val petScale = remember { Animatable(1f) }
    var petFeedbackText by remember { mutableStateOf<String?>(null) }

    val infiniteTransition = rememberInfiniteTransition(label = "orb_anim")

    // Breathing pulse
    val breathScale by infiniteTransition.animateFloat(
        initialValue = 0.95f,
        targetValue = 1.05f,
        animationSpec = infiniteRepeatable(
            animation = tween(1800, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "breath"
    )

    // Orbital rotation
    val orbitAngle by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(
            animation = tween(5000, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "orbit"
    )

    // Sound wave pulse
    val waveAlpha by infiniteTransition.animateFloat(
        initialValue = 0.8f,
        targetValue = 0.1f,
        animationSpec = infiniteRepeatable(
            animation = tween(1200, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "wave"
    )

    val primaryColor = when (sessionState) {
        SessionState.LISTENING -> AccentGreen
        SessionState.SPEAKING -> AccentCyan
        SessionState.ROUTING, SessionState.LOCAL_PROCESSING,
        SessionState.CLOUD_CONNECTING, SessionState.CLOUD_TURN -> AccentPurple
        SessionState.PAUSED, SessionState.OFFLINE -> AccentYellow
        SessionState.STOPPED, SessionState.LOCAL_ONLY -> Color(0xFF58A6FF)
        else -> AccentCyan
    }

    val secondaryColor = when (sessionState) {
        SessionState.SPEAKING -> AccentPurple
        SessionState.LISTENING -> AccentCyan
        else -> Color(0xFF1F6FEB)
    }

    val personaEmoji = when (activePersona) {
        "roaster" -> "🔥 毒舌损友"
        "jarvis" -> "💼 高效幕僚"
        "pet" -> "🐾 元气小猫"
        else -> "🌙 知心伴侣"
    }

    Column(
        modifier = modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Box(
            contentAlignment = Alignment.Center,
            modifier = Modifier
                .size(170.dp)
                .clickable(
                    interactionSource = remember { MutableInteractionSource() },
                    indication = null
                ) {
                    haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                    coroutineScope.launch {
                        petScale.animateTo(1.22f, spring(dampingRatio = 0.4f, stiffness = 600f))
                        petScale.animateTo(1.0f, spring(dampingRatio = 0.6f, stiffness = 400f))
                    }
                    petFeedbackText = listOf("嘻嘻，摸摸头~", "蹭蹭你！我在呢~", "主人今天也要开心哦！", "心跳加速中~✨").random()
                    onPet()
                }
        ) {
            Canvas(modifier = Modifier.size(160.dp)) {
                val center = Offset(size.width / 2f, size.height / 2f)
                val baseRadius = (size.minDimension / 2.8f) * breathScale * petScale.value

                // 1. External soundwave / energy ripples (Active when listening or speaking)
                if (sessionState == SessionState.LISTENING || sessionState == SessionState.SPEAKING) {
                    val volumeBoost = speechVolume * 30f
                    val rippleRadius = baseRadius + 14f + volumeBoost
                    drawCircle(
                        color = primaryColor.copy(alpha = (waveAlpha * 0.45f).coerceIn(0f, 1f)),
                        radius = rippleRadius,
                        center = center,
                        style = Stroke(width = 3.dp.toPx())
                    )
                    drawCircle(
                        color = secondaryColor.copy(alpha = (waveAlpha * 0.25f).coerceIn(0f, 1f)),
                        radius = rippleRadius + 10f,
                        center = center,
                        style = Stroke(width = 1.5.dp.toPx())
                    )
                }

                // 2. Outer soft aura glow
                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            primaryColor.copy(alpha = 0.45f),
                            secondaryColor.copy(alpha = 0.2f),
                            Color.Transparent
                        ),
                        center = center,
                        radius = baseRadius * 1.5f
                    ),
                    radius = baseRadius * 1.45f,
                    center = center
                )

                // 3. Central living Sprite Orb
                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            Color.White,
                            primaryColor,
                            secondaryColor,
                            Color(0xFF0D1117)
                        ),
                        center = Offset(center.x - baseRadius * 0.25f, center.y - baseRadius * 0.3f),
                        radius = baseRadius * 1.1f
                    ),
                    radius = baseRadius,
                    center = center
                )

                // 4. Orbital particle / thinking halo
                val isThinking = sessionState == SessionState.ROUTING ||
                        sessionState == SessionState.LOCAL_PROCESSING ||
                        sessionState == SessionState.CLOUD_CONNECTING ||
                        sessionState == SessionState.CLOUD_TURN

                val ringRadius = baseRadius * 1.18f
                drawCircle(
                    color = primaryColor.copy(alpha = if (isThinking) 0.6f else 0.25f),
                    radius = ringRadius,
                    center = center,
                    style = Stroke(width = if (isThinking) 2.5.dp.toPx() else 1.dp.toPx())
                )

                // Orbiting satellite dot
                val rad = Math.toRadians(orbitAngle.toDouble())
                val orbitX = center.x + ringRadius * cos(rad).toFloat()
                val orbitY = center.y + ringRadius * sin(rad).toFloat()
                drawCircle(
                    color = Color.White,
                    radius = if (isThinking) 4.5.dp.toPx() else 3.dp.toPx(),
                    center = Offset(orbitX, orbitY)
                )

                // Second orbit dot
                val rad2 = Math.toRadians((orbitAngle + 180.0) % 360.0)
                val orbit2X = center.x + ringRadius * cos(rad2).toFloat()
                val orbit2Y = center.y + ringRadius * sin(rad2).toFloat()
                drawCircle(
                    color = primaryColor,
                    radius = if (isThinking) 3.5.dp.toPx() else 2.dp.toPx(),
                    center = Offset(orbit2X, orbit2Y)
                )
            }

            // Center status / emotional expression icon
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                val statusEmoji = when (sessionState) {
                    SessionState.LISTENING -> "👂"
                    SessionState.SPEAKING -> "💬"
                    SessionState.ROUTING, SessionState.LOCAL_PROCESSING,
                    SessionState.CLOUD_CONNECTING, SessionState.CLOUD_TURN -> "💭"
                    SessionState.STOPPED, SessionState.PAUSED -> "💤"
                    else -> "✨"
                }
                Text(
                    text = statusEmoji,
                    fontSize = 24.sp
                )
            }
        }

        Spacer(modifier = Modifier.height(6.dp))

        // Interactive status hint & persona tag
        Text(
            text = petFeedbackText ?: when (sessionState) {
                SessionState.LISTENING -> "正在倾听中... (随时对我说话)"
                SessionState.SPEAKING -> "正在语音回复中... (点击'打断'可随时插话)"
                SessionState.ROUTING, SessionState.LOCAL_PROCESSING,
                SessionState.CLOUD_CONNECTING, SessionState.CLOUD_TURN -> "思考与分流中..."
                SessionState.STOPPED -> "已待机 • 轻触精灵球抚摸或开启对话"
                SessionState.PAUSED -> "已暂停 • 轻触以恢复交互"
                else -> "全双工伴侣 • 轻触精灵球互动"
            },
            fontSize = 12.sp,
            fontWeight = FontWeight.Medium,
            color = if (petFeedbackText != null) AccentYellow else TextSecondary
        )

        Spacer(modifier = Modifier.height(2.dp))

        Text(
            text = "人设: $personaEmoji",
            fontSize = 11.sp,
            color = TextMuted
        )
    }

    LaunchedEffect(petFeedbackText) {
        if (petFeedbackText != null) {
            kotlinx.coroutines.delay(2200)
            petFeedbackText = null
        }
    }
}

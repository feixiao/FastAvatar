package com.avatar.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.avatar.android.core.config.AppSettings
import com.avatar.android.ui.theme.AccentCyan
import com.avatar.android.ui.theme.AccentGreen
import com.avatar.android.ui.theme.CardBorder
import com.avatar.android.ui.theme.CardDark
import com.avatar.android.ui.theme.TextMuted
import com.avatar.android.ui.theme.TextPrimary
import com.avatar.android.ui.theme.TextSecondary

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsSheet(
    appSettings: AppSettings,
    onSaveSettings: (
        serverUrl: String,
        isCloudConsented: Boolean,
        autoTts: Boolean,
        speechRate: Float,
        activePersona: String,
        customLlmUrl: String,
        customLlmKey: String,
        customLlmModel: String
    ) -> Unit,
    onDismiss: () -> Unit
) {
    var serverUrl by remember { mutableStateOf(appSettings.serverUrl) }
    var isCloudConsented by remember { mutableStateOf(appSettings.isCloudConsented) }
    var autoTts by remember { mutableStateOf(appSettings.isAutoTtsEnabled) }
    var speechRate by remember { mutableFloatStateOf(appSettings.speechRate) }
    var activePersona by remember { mutableStateOf(appSettings.activePersona) }
    var customLlmUrl by remember { mutableStateOf(appSettings.customLlmUrl) }
    var customLlmKey by remember { mutableStateOf(appSettings.customLlmKey) }
    var customLlmModel by remember { mutableStateOf(appSettings.customLlmModel) }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        containerColor = CardDark
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 10.dp)
                .verticalScroll(rememberScrollState())
        ) {
            Text(
                text = "⚙️ 伴侣设置与连接配置",
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                color = TextPrimary
            )
            Spacer(modifier = Modifier.height(14.dp))

            // 1. 人设选择
            Text(
                text = "陪伴人设与音色风格",
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
                color = AccentCyan
            )
            Spacer(modifier = Modifier.height(6.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                val personas = listOf(
                    "companion" to "🌙 知心伴侣",
                    "pet" to "🐾 元气小猫",
                    "jarvis" to "💼 高效幕僚",
                    "roaster" to "🔥 毒舌损友"
                )
                personas.forEach { (id, label) ->
                    FilterChip(
                        selected = activePersona == id,
                        onClick = { activePersona = id },
                        label = { Text(label, fontSize = 11.sp) },
                        colors = FilterChipDefaults.filterChipColors(
                            selectedContainerColor = AccentCyan,
                            selectedLabelColor = Color.Black
                        )
                    )
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            // 2. 语音播报设置
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column {
                    Text(text = "语音回复自动朗读 (TTS)", fontSize = 14.sp, color = TextPrimary)
                    Text(text = "助手回复时通过扬声器播放语音", fontSize = 11.sp, color = TextSecondary)
                }
                Switch(
                    checked = autoTts,
                    onCheckedChange = { autoTts = it },
                    colors = SwitchDefaults.colors(checkedThumbColor = AccentCyan)
                )
            }

            Spacer(modifier = Modifier.height(8.dp))

            Text(
                text = "语速调节: ${String.format("%.1f", speechRate)}x",
                fontSize = 12.sp,
                color = TextSecondary
            )
            Slider(
                value = speechRate,
                onValueChange = { speechRate = it },
                valueRange = 0.8f..1.6f,
                steps = 7,
                colors = SliderDefaults.colors(
                    thumbColor = AccentCyan,
                    activeTrackColor = AccentCyan
                )
            )

            Spacer(modifier = Modifier.height(16.dp))

            // 3. 云端实时处理授权
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(text = "云端开放式对话授权", fontSize = 14.sp, color = TextPrimary)
                    Text(
                        text = "开启后开放式问题连接云端/大模型；本地车控与记忆始终在端侧安全闭环",
                        fontSize = 11.sp,
                        color = TextSecondary
                    )
                }
                Spacer(modifier = Modifier.width(10.dp))
                Switch(
                    checked = isCloudConsented,
                    onCheckedChange = { isCloudConsented = it },
                    colors = SwitchDefaults.colors(checkedThumbColor = AccentGreen)
                )
            }

            Spacer(modifier = Modifier.height(16.dp))

            // 4. 服务器地址配置
            Text(
                text = "云端网关服务地址 (Avatar Gateway)",
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
                color = AccentCyan
            )
            Spacer(modifier = Modifier.height(6.dp))
            OutlinedTextField(
                value = serverUrl,
                onValueChange = { serverUrl = it },
                modifier = Modifier.fillMaxWidth(),
                placeholder = { Text("http://192.168.x.x:8088", color = TextMuted) },
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = AccentCyan,
                    unfocusedBorderColor = CardBorder,
                    focusedTextColor = TextPrimary,
                    unfocusedTextColor = TextPrimary
                ),
                singleLine = true
            )
            Spacer(modifier = Modifier.height(6.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                OutlinedButton(
                    onClick = { serverUrl = "http://10.0.2.2:8088" },
                    colors = ButtonDefaults.outlinedButtonColors(contentColor = TextSecondary)
                ) {
                    Text("模拟器 (10.0.2.2)", fontSize = 10.sp)
                }
                OutlinedButton(
                    onClick = { serverUrl = "http://127.0.0.1:8088" },
                    colors = ButtonDefaults.outlinedButtonColors(contentColor = TextSecondary)
                ) {
                    Text("本机 (127.0.0.1)", fontSize = 10.sp)
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            // 5. 自定义外部大模型端点 (可选)
            Text(
                text = "直连大模型端点 (可选: Ollama / DeepSeek / OpenAI)",
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
                color = AccentCyan
            )
            Spacer(modifier = Modifier.height(6.dp))
            OutlinedTextField(
                value = customLlmUrl,
                onValueChange = { customLlmUrl = it },
                modifier = Modifier.fillMaxWidth(),
                placeholder = { Text("例如: http://192.168.1.100:11434/v1", color = TextMuted) },
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = AccentCyan,
                    unfocusedBorderColor = CardBorder,
                    focusedTextColor = TextPrimary,
                    unfocusedTextColor = TextPrimary
                ),
                singleLine = true
            )
            Spacer(modifier = Modifier.height(6.dp))
            OutlinedTextField(
                value = customLlmModel,
                onValueChange = { customLlmModel = it },
                modifier = Modifier.fillMaxWidth(),
                placeholder = { Text("模型名称: 例如 qwen2.5:7b 或 deepseek-chat", color = TextMuted) },
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = AccentCyan,
                    unfocusedBorderColor = CardBorder,
                    focusedTextColor = TextPrimary,
                    unfocusedTextColor = TextPrimary
                ),
                singleLine = true
            )
            Spacer(modifier = Modifier.height(6.dp))
            OutlinedTextField(
                value = customLlmKey,
                onValueChange = { customLlmKey = it },
                modifier = Modifier.fillMaxWidth(),
                placeholder = { Text("API Key (如果需要)", color = TextMuted) },
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = AccentCyan,
                    unfocusedBorderColor = CardBorder,
                    focusedTextColor = TextPrimary,
                    unfocusedTextColor = TextPrimary
                ),
                singleLine = true
            )

            Spacer(modifier = Modifier.height(20.dp))

            Button(
                onClick = {
                    onSaveSettings(
                        serverUrl,
                        isCloudConsented,
                        autoTts,
                        speechRate,
                        activePersona,
                        customLlmUrl,
                        customLlmKey,
                        customLlmModel
                    )
                    onDismiss()
                },
                colors = ButtonDefaults.buttonColors(containerColor = AccentCyan),
                modifier = Modifier
                    .fillMaxWidth()
                    .height(44.dp),
                shape = RoundedCornerShape(12.dp)
            ) {
                Text("保存设置", color = Color.Black, fontWeight = FontWeight.Bold)
            }

            Spacer(modifier = Modifier.height(20.dp))
        }
    }
}

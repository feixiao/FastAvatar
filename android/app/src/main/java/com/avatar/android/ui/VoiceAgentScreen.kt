package com.avatar.android.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.Cloud
import androidx.compose.material.icons.filled.CloudOff
import androidx.compose.material.icons.filled.DirectionsCar
import androidx.compose.material.icons.filled.Headphones
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Mic
import androidx.compose.material.icons.filled.MicOff
import androidx.compose.material.icons.filled.Send
import androidx.compose.material.icons.filled.Stop
import androidx.compose.material.icons.filled.VolumeUp
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ElevatedButton
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.avatar.android.core.dispatcher.RouteTarget
import com.avatar.android.core.state.SessionState
import com.avatar.android.ui.components.AuditLogSheet
import com.avatar.android.ui.components.MemorySheet
import com.avatar.android.ui.theme.AccentCyan
import com.avatar.android.ui.theme.AccentGreen
import com.avatar.android.ui.theme.AccentPurple
import com.avatar.android.ui.theme.AccentRed
import com.avatar.android.ui.theme.AccentYellow
import com.avatar.android.ui.theme.BgDark
import com.avatar.android.ui.theme.CardBorder
import com.avatar.android.ui.theme.CardDark
import com.avatar.android.ui.theme.TextMuted
import com.avatar.android.ui.theme.TextPrimary
import com.avatar.android.ui.theme.TextSecondary

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun VoiceAgentScreen(
    viewModel: VoiceAgentViewModel,
    hasMicPermission: Boolean,
    onRequestMicPermission: () -> Unit
) {
    val sessionState by viewModel.sessionState.collectAsState()
    val messages by viewModel.messages.collectAsState()
    val isCloudConsented by viewModel.isCloudConsented.collectAsState()
    val isOnline by viewModel.isOnline.collectAsState()
    val pendingMemory by viewModel.pendingMemory.collectAsState()
    val memories by viewModel.memories.collectAsState()
    val auditLogs by viewModel.auditLogs.collectAsState()
    val isPlaying by viewModel.isPlaying.collectAsState()

    var showMemorySheet by remember { mutableStateOf(false) }
    var showAuditSheet by remember { mutableStateOf(false) }
    var showConsentDialog by remember { mutableStateOf(false) }
    var inputText by remember { mutableStateOf("") }

    val listState = rememberLazyListState()
    LaunchedEffect(messages.size) {
        if (messages.isNotEmpty()) {
            listState.animateScrollToItem(messages.size - 1)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                colors = TopAppBarDefaults.topAppBarColors(containerColor = BgDark),
                title = {
                    Column {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                text = "Avatar AI 语音伴侣",
                                fontSize = 17.sp,
                                fontWeight = FontWeight.Bold,
                                color = TextPrimary
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                            Box(
                                modifier = Modifier
                                    .clip(RoundedCornerShape(4.dp))
                                    .background(CardDark)
                                    .padding(horizontal = 6.dp, vertical = 2.dp)
                            ) {
                                Text(
                                    text = "Pixel 10 • Android 16",
                                    fontSize = 10.sp,
                                    color = AccentCyan
                                )
                            }
                        }
                    }
                },
                actions = {
                    // Online/Offline switch
                    IconButton(onClick = { viewModel.setOnline(!isOnline) }) {
                        Icon(
                            imageVector = if (isOnline) Icons.Default.Cloud else Icons.Default.CloudOff,
                            contentDescription = "网络状态切换",
                            tint = if (isOnline) AccentGreen else AccentRed
                        )
                    }

                    // Memory sheet button
                    IconButton(onClick = {
                        viewModel.refreshMemories()
                        showMemorySheet = true
                    }) {
                        Icon(
                            imageVector = Icons.Default.Bookmark,
                            contentDescription = "本地记忆",
                            tint = AccentPurple
                        )
                    }

                    // Audit sheet button
                    IconButton(onClick = {
                        viewModel.refreshAuditLogs()
                        showAuditSheet = true
                    }) {
                        Icon(
                            imageVector = Icons.Default.DirectionsCar,
                            contentDescription = "车控审计",
                            tint = AccentYellow
                        )
                    }
                }
            )
        },
        containerColor = BgDark
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(horizontal = 16.dp)
        ) {
            // 1. Session State & Media Gate Banner
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = CardDark),
                shape = RoundedCornerShape(12.dp),
                border = CardDefaults.outlinedCardBorder().copy(brush = androidx.compose.ui.graphics.SolidColor(CardBorder))
            ) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            val stateColor = when (sessionState) {
                                SessionState.STOPPED -> TextMuted
                                SessionState.PREPARING -> AccentYellow
                                SessionState.LISTENING -> AccentGreen
                                SessionState.ROUTING, SessionState.LOCAL_PROCESSING -> AccentCyan
                                SessionState.CLOUD_CONNECTING, SessionState.CLOUD_TURN -> AccentPurple
                                SessionState.SPEAKING -> AccentCyan
                                SessionState.PAUSED -> AccentYellow
                                SessionState.OFFLINE, SessionState.LOCAL_ONLY -> AccentRed
                            }

                            Box(
                                modifier = Modifier
                                    .size(10.dp)
                                    .clip(CircleShape)
                                    .background(stateColor)
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                            Text(
                                text = sessionState.displayName,
                                fontSize = 16.sp,
                                fontWeight = FontWeight.Bold,
                                color = TextPrimary
                            )
                        }

                        // Cloud consent status toggle
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                text = if (isCloudConsented) "云端已授权" else "仅限本地",
                                fontSize = 12.sp,
                                color = if (isCloudConsented) AccentCyan else TextMuted
                            )
                            Spacer(modifier = Modifier.width(4.dp))
                            Switch(
                                checked = isCloudConsented,
                                onCheckedChange = { checked ->
                                    if (checked) {
                                        showConsentDialog = true
                                    } else {
                                        viewModel.setCloudConsent(false)
                                    }
                                },
                                colors = SwitchDefaults.colors(
                                    checkedThumbColor = AccentCyan,
                                    checkedTrackColor = CardBorder,
                                    uncheckedThumbColor = TextMuted,
                                    uncheckedTrackColor = BgDark
                                ),
                                modifier = Modifier.size(width = 44.dp, height = 24.dp)
                            )
                        }
                    }

                    Spacer(modifier = Modifier.height(6.dp))
                    Text(
                        text = sessionState.description,
                        fontSize = 12.sp,
                        color = TextSecondary
                    )

                    Spacer(modifier = Modifier.height(10.dp))

                    // Audio Gate Guardrail Display
                    val isMuted = viewModel.isAudioMuted
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clip(RoundedCornerShape(6.dp))
                            .background(if (isMuted) Color(0xFF1E2631) else Color(0xFF231B34))
                            .padding(horizontal = 10.dp, vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Icon(
                            imageVector = if (isMuted) Icons.Default.Lock else Icons.Default.VolumeUp,
                            contentDescription = "媒体门禁",
                            tint = if (isMuted) AccentGreen else AccentPurple,
                            modifier = Modifier.size(14.dp)
                        )
                        Spacer(modifier = Modifier.width(6.dp))
                        Text(
                            text = if (isMuted)
                                "媒体门禁: 已安全闭锁 (Muted=true，零网络音频泄露)"
                            else
                                "媒体门禁: 临时推流中 (Audio Gate Active，云端流式传输)",
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Medium,
                            color = if (isMuted) AccentGreen else AccentPurple
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(8.dp))

            // 2. Pending Memory Confirmation Card (Two-phase commit per Section 4.3)
            AnimatedVisibility(visible = pendingMemory != null) {
                pendingMemory?.let { pending ->
                    Card(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = 4.dp),
                        colors = CardDefaults.cardColors(containerColor = Color(0xFF1C2D38)),
                        shape = RoundedCornerShape(10.dp),
                        border = CardDefaults.outlinedCardBorder().copy(brush = androidx.compose.ui.graphics.SolidColor(AccentCyan))
                    ) {
                        Column(modifier = Modifier.padding(12.dp)) {
                            Text(
                                text = "💡 本地记忆保存确认 (需用户明确授权)",
                                fontSize = 13.sp,
                                fontWeight = FontWeight.Bold,
                                color = AccentCyan
                            )
                            Spacer(modifier = Modifier.height(4.dp))
                            Text(
                                text = pending.prompt,
                                fontSize = 14.sp,
                                color = TextPrimary
                            )
                            Spacer(modifier = Modifier.height(8.dp))
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.End
                            ) {
                                OutlinedButton(
                                    onClick = { viewModel.rejectStagedMemory() },
                                    colors = ButtonDefaults.outlinedButtonColors(contentColor = AccentRed),
                                    modifier = Modifier.height(34.dp)
                                ) {
                                    Text("放弃", fontSize = 12.sp)
                                }
                                Spacer(modifier = Modifier.width(8.dp))
                                Button(
                                    onClick = { viewModel.confirmStagedMemory() },
                                    colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
                                    modifier = Modifier.height(34.dp)
                                ) {
                                    Text("确认记住", fontSize = 12.sp, color = Color.White)
                                }
                            }
                        }
                    }
                }
            }

            // 3. Conversation Message Stream
            LazyColumn(
                state = listState,
                modifier = Modifier
                    .weight(1f)
                    .fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                items(messages, key = { it.id }) { msg ->
                    MessageBubble(msg)
                }
            }

            Spacer(modifier = Modifier.height(8.dp))

            // 4. Quick Test Chips
            Text(
                text = "验收快捷场景 (点击快速评测):",
                fontSize = 11.sp,
                color = TextMuted
            )
            LazyRow(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(vertical = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                item {
                    TestChip("把空调调到24度") {
                        ensureSessionStarted(sessionState, hasMicPermission, onRequestMicPermission, viewModel)
                        viewModel.submitUtterance("把空调调到24度")
                    }
                }
                item {
                    TestChip("切换到漂移模式 (危险拦截)") {
                        ensureSessionStarted(sessionState, hasMicPermission, onRequestMicPermission, viewModel)
                        viewModel.submitUtterance("切换到漂移模式")
                    }
                }
                item {
                    TestChip("帮我记一下我喜欢美式咖啡") {
                        ensureSessionStarted(sessionState, hasMicPermission, onRequestMicPermission, viewModel)
                        viewModel.submitUtterance("帮我记一下我喜欢美式咖啡")
                    }
                }
                item {
                    TestChip("为什么海水是蓝色的？") {
                        ensureSessionStarted(sessionState, hasMicPermission, onRequestMicPermission, viewModel)
                        viewModel.submitUtterance("为什么海水是蓝色的？")
                    }
                }
            }

            // 5. Input and Control Bar
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                OutlinedTextField(
                    value = inputText,
                    onValueChange = { inputText = it },
                    placeholder = { Text("输入语音模拟文字...", fontSize = 13.sp, color = TextMuted) },
                    modifier = Modifier.weight(1f),
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedTextColor = TextPrimary,
                        unfocusedTextColor = TextPrimary,
                        focusedBorderColor = AccentCyan,
                        unfocusedBorderColor = CardBorder,
                        focusedContainerColor = CardDark,
                        unfocusedContainerColor = CardDark
                    ),
                    singleLine = true,
                    shape = RoundedCornerShape(20.dp)
                )

                Spacer(modifier = Modifier.width(6.dp))

                IconButton(
                    onClick = {
                        if (inputText.isNotBlank()) {
                            ensureSessionStarted(sessionState, hasMicPermission, onRequestMicPermission, viewModel)
                            viewModel.submitUtterance(inputText.trim())
                            inputText = ""
                        }
                    },
                    modifier = Modifier
                        .size(42.dp)
                        .clip(CircleShape)
                        .background(AccentCyan)
                ) {
                    Icon(
                        imageVector = Icons.Default.Send,
                        contentDescription = "发送",
                        tint = BgDark,
                        modifier = Modifier.size(18.dp)
                    )
                }

                Spacer(modifier = Modifier.width(6.dp))

                // Barge-in button (Enabled when speaking)
                Button(
                    onClick = { viewModel.bargeIn() },
                    colors = ButtonDefaults.buttonColors(
                        containerColor = if (isPlaying) AccentRed else CardBorder
                    ),
                    shape = RoundedCornerShape(20.dp),
                    modifier = Modifier.height(42.dp)
                ) {
                    Text(
                        text = "打断",
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                        color = if (isPlaying) Color.White else TextSecondary
                    )
                }
            }

            // 6. Session Lifecycle Toggle
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(bottom = 12.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                if (sessionState == SessionState.STOPPED) {
                    Button(
                        onClick = {
                            if (!hasMicPermission) {
                                onRequestMicPermission()
                            } else {
                                viewModel.startSession(true)
                            }
                        },
                        colors = ButtonDefaults.buttonColors(containerColor = AccentGreen),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Icon(Icons.Default.Mic, contentDescription = null, modifier = Modifier.size(18.dp))
                        Spacer(modifier = Modifier.width(8.dp))
                        Text("启动语音会话 (Start Session)")
                    }
                } else {
                    Button(
                        onClick = { viewModel.stopSession() },
                        colors = ButtonDefaults.buttonColors(containerColor = CardBorder),
                        modifier = Modifier.fillMaxWidth()
                    ) {
                        Icon(Icons.Default.Stop, contentDescription = null, tint = AccentRed, modifier = Modifier.size(18.dp))
                        Spacer(modifier = Modifier.width(8.dp))
                        Text("结束语音会话 (Stop Session)", color = AccentRed)
                    }
                }
            }
        }
    }

    // Consent Confirmation Dialog
    if (showConsentDialog) {
        AlertDialog(
            onDismissRequest = { showConsentDialog = false },
            title = { Text("启用云端实时处理授权") },
            text = {
                Text(
                    "开启后，开放式对话语音回合将通过加密通道发送至云端服务进行流式生成。\n\n" +
                            "• 本地记忆与车控指令仍在端侧本地处理，不会上传云端；\n" +
                            "• 您可随时在此撤销云端授权；\n" +
                            "• 断网时本地功能仍然可用，不会排队或自动重发语音。"
                )
            },
            confirmButton = {
                Button(onClick = {
                    viewModel.setCloudConsent(true)
                    showConsentDialog = false
                }) {
                    Text("同意开启")
                }
            },
            dismissButton = {
                TextButton(onClick = { showConsentDialog = false }) {
                    Text("取消")
                }
            },
            containerColor = CardDark
        )
    }

    // Memory BottomSheet
    if (showMemorySheet) {
        MemorySheet(
            memories = memories,
            deviceId = viewModel.deviceId,
            onDelete = { viewModel.deleteMemory(it) },
            onClearAll = { viewModel.clearAllMemories() },
            onDismiss = { showMemorySheet = false }
        )
    }

    // Audit Log BottomSheet
    if (showAuditSheet) {
        AuditLogSheet(
            auditLogs = auditLogs,
            onClearAll = { viewModel.clearAuditLogs() },
            onDismiss = { showAuditSheet = false }
        )
    }
}

private fun ensureSessionStarted(
    sessionState: SessionState,
    hasMicPermission: Boolean,
    onRequestMicPermission: () -> Unit,
    viewModel: VoiceAgentViewModel
) {
    if (sessionState == SessionState.STOPPED) {
        if (!hasMicPermission) {
            onRequestMicPermission()
        } else {
            viewModel.startSession(true)
        }
    }
}

@Composable
fun TestChip(label: String, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(16.dp))
            .background(CardDark)
            .border(1.dp, CardBorder, RoundedCornerShape(16.dp))
            .padding(horizontal = 10.dp, vertical = 6.dp)
    ) {
        TextButton(
            onClick = onClick,
            contentPadding = androidx.compose.foundation.layout.PaddingValues(0.dp),
            modifier = Modifier.height(20.dp)
        ) {
            Text(label, fontSize = 11.sp, color = AccentCyan)
        }
    }
}

@Composable
fun MessageBubble(msg: MessageItem) {
    val isUser = msg.sender == MessageSender.USER
    val isSystem = msg.sender == MessageSender.SYSTEM

    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = when {
            isUser -> Alignment.End
            isSystem -> Alignment.CenterHorizontally
            else -> Alignment.Start
        }
    ) {
        if (isSystem) {
            Box(
                modifier = Modifier
                    .clip(RoundedCornerShape(6.dp))
                    .background(Color(0xFF21262D))
                    .padding(horizontal = 10.dp, vertical = 4.dp)
            ) {
                Text(
                    text = msg.text,
                    fontSize = 11.sp,
                    color = if (msg.isInterrupted) AccentYellow else TextSecondary
                )
            }
        } else {
            Card(
                colors = CardDefaults.cardColors(
                    containerColor = if (isUser) Color(0xFF1F6FEB) else CardDark
                ),
                shape = RoundedCornerShape(12.dp),
                border = if (!isUser) CardDefaults.outlinedCardBorder().copy(brush = androidx.compose.ui.graphics.SolidColor(CardBorder)) else null,
                modifier = Modifier.fillMaxWidth(0.85f)
            ) {
                Column(modifier = Modifier.padding(10.dp)) {
                    Text(
                        text = msg.text,
                        fontSize = 14.sp,
                        color = Color.White
                    )

                    if (!isUser && msg.routeTarget != null) {
                        Spacer(modifier = Modifier.height(6.dp))
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.SpaceBetween,
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            val badgeColor = when (msg.routeTarget) {
                                RouteTarget.LOCAL_MEMORY, RouteTarget.LOCAL_PERSONA -> AccentGreen
                                RouteTarget.MOCK_VEHICLE -> AccentYellow
                                RouteTarget.CLOUD_CHAT -> AccentPurple
                                else -> TextMuted
                            }

                            val routeLabel = when (msg.routeTarget) {
                                RouteTarget.LOCAL_MEMORY -> "本地记忆快环"
                                RouteTarget.LOCAL_PERSONA -> "本地人设快环"
                                RouteTarget.MOCK_VEHICLE -> "Mock车控快环"
                                RouteTarget.CLOUD_CHAT -> "云端实时对话"
                                RouteTarget.LOCAL_ONLY_PROMPT -> "未授权降级"
                                RouteTarget.OFFLINE_PROMPT -> "离线降级"
                            }

                            Text(
                                text = "路由: $routeLabel",
                                fontSize = 10.sp,
                                fontWeight = FontWeight.Bold,
                                color = badgeColor
                            )

                            msg.latencyMs?.let { latency ->
                                Text(
                                    text = "响应: ${latency}ms",
                                    fontSize = 10.sp,
                                    color = if (latency <= 150) AccentGreen else AccentYellow
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

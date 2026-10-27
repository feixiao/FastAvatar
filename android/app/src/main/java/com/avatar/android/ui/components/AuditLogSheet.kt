package com.avatar.android.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.avatar.android.core.mockvehicle.AuditLogEntry
import com.avatar.android.core.mockvehicle.DecisionType
import com.avatar.android.ui.theme.AccentCyan
import com.avatar.android.ui.theme.AccentGreen
import com.avatar.android.ui.theme.AccentRed
import com.avatar.android.ui.theme.BgDark
import com.avatar.android.ui.theme.CardBorder
import com.avatar.android.ui.theme.CardDark
import com.avatar.android.ui.theme.TextMuted
import com.avatar.android.ui.theme.TextPrimary
import com.avatar.android.ui.theme.TextSecondary
import com.google.gson.GsonBuilder

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun AuditLogSheet(
    auditLogs: List<AuditLogEntry>,
    onClearAll: () -> Unit,
    onDismiss: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val gson = GsonBuilder().setPrettyPrinting().create()

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = CardDark
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column {
                    Text(
                        text = "Mock 车控审计日志 (JSON Lines)",
                        fontSize = 17.sp,
                        fontWeight = FontWeight.Bold,
                        color = TextPrimary
                    )
                    Text(
                        text = "PRD 规范: 单条日志 / 零音频 / simulation=true",
                        fontSize = 11.sp,
                        color = AccentCyan
                    )
                }

                if (auditLogs.isNotEmpty()) {
                    TextButton(
                        onClick = onClearAll,
                        colors = ButtonDefaults.textButtonColors(contentColor = AccentRed)
                    ) {
                        Text("清空日志")
                    }
                }
            }

            Spacer(modifier = Modifier.height(12.dp))

            if (auditLogs.isEmpty()) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(32.dp),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        text = "暂无车控审计日志\n发送车控指令（如\"把空调调到24度\"）即可生成",
                        color = TextMuted,
                        fontSize = 13.sp
                    )
                }
            } else {
                LazyColumn(
                    modifier = Modifier.fillMaxWidth(),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    items(auditLogs.reversed()) { entry ->
                        val isAccepted = entry.decision == DecisionType.MOCK_ACCEPTED
                        Card(
                            modifier = Modifier.fillMaxWidth(),
                            colors = CardDefaults.cardColors(containerColor = BgDark),
                            shape = RoundedCornerShape(8.dp),
                            border = CardDefaults.outlinedCardBorder().copy(brush = androidx.compose.ui.graphics.SolidColor(CardBorder))
                        ) {
                            Column(modifier = Modifier.padding(12.dp)) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween
                                ) {
                                    Text(
                                        text = entry.intent,
                                        fontWeight = FontWeight.Bold,
                                        fontSize = 13.sp,
                                        color = AccentCyan
                                    )
                                    Text(
                                        text = entry.decision.name,
                                        fontWeight = FontWeight.Bold,
                                        fontSize = 12.sp,
                                        color = if (isAccepted) AccentGreen else AccentRed
                                    )
                                }

                                Spacer(modifier = Modifier.height(4.dp))
                                Text(
                                    text = "原因代码: ${entry.reason}",
                                    fontSize = 11.sp,
                                    color = TextSecondary
                                )
                                Text(
                                    text = "时间戳: ${entry.timestamp}",
                                    fontSize = 11.sp,
                                    color = TextMuted
                                )

                                Spacer(modifier = Modifier.height(6.dp))
                                Text(
                                    text = "Raw JSON:",
                                    fontSize = 10.sp,
                                    color = TextMuted
                                )
                                Text(
                                    text = gson.toJson(entry),
                                    fontSize = 11.sp,
                                    fontFamily = FontFamily.Monospace,
                                    color = TextPrimary,
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .background(CardDark, RoundedCornerShape(4.dp))
                                        .padding(6.dp)
                                )
                            }
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(24.dp))
        }
    }
}

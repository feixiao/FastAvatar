package com.avatar.android.core.memory

import java.util.UUID

data class MemoryItem(
    val id: String = UUID.randomUUID().toString(),
    val content: String,
    val category: String = "general",
    val createdAt: Long = System.currentTimeMillis(),
    val deviceId: String
)

data class PendingMemory(
    val content: String,
    val category: String = "general",
    val prompt: String
)

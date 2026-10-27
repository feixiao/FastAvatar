package com.avatar.android.core.memory

import com.google.gson.Gson
import com.google.gson.reflect.TypeToken
import java.io.File
import java.nio.charset.StandardCharsets

class MemoryVault(
    private val storageFile: File,
    val deviceId: String
) {
    private val gson = Gson()
    private val lock = Any()
    private val memories = mutableListOf<MemoryItem>()

    var pendingMemory: PendingMemory? = null
        private set

    init {
        storageFile.parentFile?.mkdirs()
        loadFromFile()
    }

    private fun loadFromFile() {
        synchronized(lock) {
            memories.clear()
            if (storageFile.exists() && storageFile.length() > 0) {
                try {
                    val json = storageFile.readText(StandardCharsets.UTF_8)
                    val type = object : TypeToken<List<MemoryItem>>() {}.type
                    val list: List<MemoryItem>? = gson.fromJson(json, type)
                    if (list != null) {
                        // Device isolation: only load memories that match this deviceId
                        memories.addAll(list.filter { it.deviceId == deviceId })
                    }
                } catch (e: Exception) {
                    memories.clear()
                }
            }
        }
    }

    private fun persistToFile() {
        val allOtherDevices = mutableListOf<MemoryItem>()
        if (storageFile.exists() && storageFile.length() > 0) {
            try {
                val json = storageFile.readText(StandardCharsets.UTF_8)
                val type = object : TypeToken<List<MemoryItem>>() {}.type
                val existing: List<MemoryItem>? = gson.fromJson(json, type)
                if (existing != null) {
                    allOtherDevices.addAll(existing.filter { it.deviceId != deviceId })
                }
            } catch (e: Exception) {
                // Ignore parse errors on raw file
            }
        }
        val combined = allOtherDevices + memories
        val json = gson.toJson(combined)
        storageFile.writeText(json, StandardCharsets.UTF_8)
    }

    /**
     * Stage a memory for two-phase confirmation per PRD Section 4.3.
     * Content is NOT saved until confirmPendingMemory() is called.
     */
    fun stageMemory(content: String, category: String = "preference"): PendingMemory {
        val pending = PendingMemory(
            content = content,
            category = category,
            prompt = "请确认是否记住：\"$content\"？"
        )
        pendingMemory = pending
        return pending
    }

    /**
     * Confirm staged memory and persist to private local storage.
     */
    fun confirmPendingMemory(): MemoryItem? {
        synchronized(lock) {
            val pending = pendingMemory ?: return null
            val item = MemoryItem(
                content = pending.content,
                category = pending.category,
                deviceId = deviceId
            )
            memories.add(item)
            persistToFile()
            pendingMemory = null
            return item
        }
    }

    fun rejectPendingMemory() {
        pendingMemory = null
    }

    fun addDirect(content: String, category: String = "general"): MemoryItem {
        synchronized(lock) {
            val item = MemoryItem(
                content = content,
                category = category,
                deviceId = deviceId
            )
            memories.add(item)
            persistToFile()
            return item
        }
    }

    fun getAll(): List<MemoryItem> {
        synchronized(lock) {
            return memories.toList()
        }
    }

    fun getById(id: String): MemoryItem? {
        synchronized(lock) {
            return memories.find { it.id == id }
        }
    }

    fun delete(id: String): Boolean {
        synchronized(lock) {
            val removed = memories.removeAll { it.id == id }
            if (removed) {
                persistToFile()
            }
            return removed
        }
    }

    fun clearAll() {
        synchronized(lock) {
            memories.clear()
            persistToFile()
        }
    }

    fun parseMemoryCommand(text: String): MemoryCommandResult {
        val clean = text.trim()

        if (clean.startsWith("清除记忆") || clean.startsWith("清空记忆") || clean.startsWith("忘掉所有") || clean.startsWith("清除所有记忆")) {
            clearAll()
            return MemoryCommandResult(
                action = MemoryAction.CLEAR,
                feedback = "已清空本地所有记忆条目。"
            )
        }

        if (clean.startsWith("忘掉") || clean.startsWith("忘了")) {
            val target = clean.removePrefix("忘掉").removePrefix("忘了").trim()
            if (target.isBlank()) {
                return MemoryCommandResult(
                    action = MemoryAction.QUERY,
                    feedback = "请告诉我需要忘掉的具体内容。"
                )
            }
            val match = memories.find { it.content.contains(target) }
            return if (match != null) {
                delete(match.id)
                MemoryCommandResult(
                    action = MemoryAction.DELETE,
                    feedback = "已删除本地记忆：\"${match.content}\"。"
                )
            } else {
                MemoryCommandResult(
                    action = MemoryAction.DELETE,
                    feedback = "未找到包含\"$target\"的本地记忆。"
                )
            }
        }

        // Add / Remember
        val content = clean
            .removePrefix("帮我记一下")
            .removePrefix("记一下")
            .removePrefix("记住")
            .trim()

        if (content.isBlank()) {
            return MemoryCommandResult(
                action = MemoryAction.STAGE,
                feedback = "请问需要为您记住什么内容？"
            )
        }

        stageMemory(content)
        return MemoryCommandResult(
            action = MemoryAction.STAGE,
            feedback = "收到，请确认是否记住：\"$content\"？",
            stagedContent = content
        )
    }
}

enum class MemoryAction {
    STAGE,
    CONFIRM,
    DELETE,
    CLEAR,
    QUERY
}

data class MemoryCommandResult(
    val action: MemoryAction,
    val feedback: String,
    val stagedContent: String? = null
)

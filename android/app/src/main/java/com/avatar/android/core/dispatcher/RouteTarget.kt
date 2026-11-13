package com.avatar.android.core.dispatcher

enum class RouteTarget(val targetName: String) {
    LOCAL_MEMORY("LOCAL_MEMORY"),
    LOCAL_PERSONA("LOCAL_PERSONA"),
    LOCAL_PET_ACTION("LOCAL_PET_ACTION"),
    CLOUD_CHAT("CLOUD_CHAT"),
    LOCAL_ONLY_PROMPT("LOCAL_ONLY_PROMPT"),
    OFFLINE_PROMPT("OFFLINE_PROMPT")
}

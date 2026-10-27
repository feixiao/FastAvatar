package com.avatar.android.core.state

/**
 * Client session state strictly per PRD Section 3.3.
 */
enum class SessionState(val displayName: String, val description: String) {
    STOPPED("已停止", "会话已关闭，无音频采集"),
    PREPARING("准备中", "检查麦克风权限、音频设备与本地能力"),
    LISTENING("聆听中", "等待用户语音输入，麦克风处于就绪状态"),
    ROUTING("分流中", "端侧毫秒级意图判断与路由"),
    LOCAL_PROCESSING("本地处理中", "执行本地记忆或 Mock 车控逻辑"),
    LOCAL_ONLY("仅本地可用", "开放式问题未获云端授权，提示用户并保持本地就绪"),
    CLOUD_CONNECTING("云端连接中", "建立实时房间连接并验证 Token"),
    CLOUD_TURN("云端对话中", "等待流式首包与回复音频到达"),
    SPEAKING("播放中", "正在播放本地或云端回复音频"),
    PAUSED("已暂停", "系统音频冲突、权限撤销或输出设备变更触发暂停"),
    OFFLINE("离线模式", "网络断开，仅保留本地快环能力")
}

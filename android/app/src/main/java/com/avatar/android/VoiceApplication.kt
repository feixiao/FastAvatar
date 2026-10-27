package com.avatar.android

import android.app.Application
import com.avatar.android.core.audio.AudioFocusHelper
import com.avatar.android.core.audio.AudioFocusListener
import com.avatar.android.core.audio.AudioPlaybackController
import com.avatar.android.core.audio.AudioPrivacyGuard
import com.avatar.android.core.audio.AudioPrivacyListener
import com.avatar.android.core.cloud.CloudClient
import com.avatar.android.core.diagnostics.DiagnosticLogger
import com.avatar.android.core.dispatcher.FastDispatcher
import com.avatar.android.core.memory.MemoryVault
import com.avatar.android.core.mockvehicle.MockVehicleEngine
import com.avatar.android.core.state.SessionStateMachine
import java.io.File
import java.util.UUID

class VoiceApplication : Application() {

    lateinit var dispatcher: FastDispatcher
        private set
    lateinit var vehicleEngine: MockVehicleEngine
        private set
    lateinit var memoryVault: MemoryVault
        private set
    lateinit var cloudClient: CloudClient
        private set
    lateinit var playbackController: AudioPlaybackController
        private set
    lateinit var privacyGuard: AudioPrivacyGuard
        private set
    lateinit var audioFocusHelper: AudioFocusHelper
        private set
    lateinit var diagnosticLogger: DiagnosticLogger
        private set
    lateinit var stateMachine: SessionStateMachine
        private set

    val deviceId: String by lazy {
        val prefs = getSharedPreferences("avatar_prefs", MODE_PRIVATE)
        var id = prefs.getString("device_id", null)
        if (id == null) {
            id = "pixel10_${UUID.randomUUID().toString().take(8)}"
            prefs.edit().putString("device_id", id).apply()
        }
        id
    }

    override fun onCreate() {
        super.onCreate()

        dispatcher = FastDispatcher()
        val logFile = File(filesDir, "mock_vehicle.log")
        vehicleEngine = MockVehicleEngine(logFile)

        val memoryFile = File(filesDir, "local_memory.json")
        memoryVault = MemoryVault(memoryFile, deviceId)

        val diagFile = File(filesDir, "diagnostics.log")
        diagnosticLogger = DiagnosticLogger(diagFile)

        playbackController = AudioPlaybackController()
        privacyGuard = AudioPrivacyGuard(this, playbackController)

        cloudClient = CloudClient(
            serverUrl = "http://10.0.2.2:8088", // Default Android emulator loopback or LAN IP
            userId = "user_pixel",
            deviceId = deviceId,
            clientType = "pixel_10"
        )

        stateMachine = SessionStateMachine(
            dispatcher = dispatcher,
            vehicleEngine = vehicleEngine,
            memoryVault = memoryVault,
            cloudClient = cloudClient,
            playbackController = playbackController,
            diagnosticLogger = diagnosticLogger
        )

        audioFocusHelper = AudioFocusHelper(this, object : AudioFocusListener {
            override fun onAudioFocusLost(isTransient: Boolean) {
                stateMachine.pauseSession(reason = if (isTransient) "audio_focus_loss_transient" else "audio_focus_loss")
            }

            override fun onAudioFocusGained() {
                // Focus regained
            }
        })

        privacyGuard.addListener(object : AudioPrivacyListener {
            override fun onAudioRouteBecameNoisy() {
                stateMachine.pauseSession(reason = "audio_becoming_noisy")
            }

            override fun onAudioDevicesChanged(isPrivateOutputConnected: Boolean) {
                // Route changed
            }
        })
    }
}

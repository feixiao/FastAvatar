package com.avatar.android.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import com.avatar.android.VoiceApplication
import com.avatar.android.core.audio.SpeechRecognitionHelper
import com.avatar.android.core.state.SessionState
import com.avatar.android.service.VoiceSessionService
import com.avatar.android.ui.theme.AvatarTheme
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {

    private lateinit var viewModel: VoiceAgentViewModel
    private var speechHelper: SpeechRecognitionHelper? = null
    private var hasMicPermission: Boolean = false

    private val permissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions()
    ) { permissions ->
        hasMicPermission = permissions[Manifest.permission.RECORD_AUDIO] == true
        if (hasMicPermission) {
            initSpeechHelper()
            if (viewModel.sessionState.value == SessionState.STOPPED) {
                viewModel.startSession(true)
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val app = application as VoiceApplication
        checkPermissions()
        if (hasMicPermission) {
            initSpeechHelper()
        }

        viewModel = VoiceAgentViewModel(
            stateMachine = app.stateMachine,
            memoryVault = app.memoryVault,
            vehicleEngine = app.vehicleEngine,
            playbackController = app.playbackController,
            privacyGuard = app.privacyGuard,
            cloudClient = app.cloudClient,
            deviceId = app.deviceId,
            appSettings = app.appSettings,
            speechRecognitionHelper = speechHelper
        )

        // Automatically start session if permission is granted so user doesn't face dead STOPPED state
        if (hasMicPermission && viewModel.sessionState.value == SessionState.STOPPED) {
            viewModel.startSession(true)
        }

        // Sync foreground service with session state
        lifecycleScope.launch {
            viewModel.sessionState.collect { state ->
                when (state) {
                    SessionState.LISTENING,
                    SessionState.ROUTING,
                    SessionState.LOCAL_PROCESSING,
                    SessionState.CLOUD_CONNECTING,
                    SessionState.CLOUD_TURN,
                    SessionState.SPEAKING -> {
                        VoiceSessionService.start(this@MainActivity)
                    }
                    SessionState.STOPPED -> {
                        VoiceSessionService.stop(this@MainActivity)
                    }
                    else -> Unit
                }
            }
        }

        setContent {
            AvatarTheme {
                VoiceAgentScreen(
                    viewModel = viewModel,
                    hasMicPermission = hasMicPermission,
                    onRequestMicPermission = { checkPermissions() }
                )
            }
        }
    }

    private fun initSpeechHelper() {
        if (speechHelper == null) {
            speechHelper = SpeechRecognitionHelper(this) { text ->
                viewModel.submitUtterance(text)
            }
        }
    }

    private fun checkPermissions() {
        hasMicPermission = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.RECORD_AUDIO
        ) == PackageManager.PERMISSION_GRANTED

        val permissionsToRequest = mutableListOf<String>()
        if (!hasMicPermission) {
            permissionsToRequest.add(Manifest.permission.RECORD_AUDIO)
        }

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(
                    this,
                    Manifest.permission.POST_NOTIFICATIONS
                ) != PackageManager.PERMISSION_GRANTED
            ) {
                permissionsToRequest.add(Manifest.permission.POST_NOTIFICATIONS)
            }
        }

        if (permissionsToRequest.isNotEmpty()) {
            permissionLauncher.launch(permissionsToRequest.toTypedArray())
        }
    }

    override fun onDestroy() {
        speechHelper?.destroy()
        speechHelper = null
        super.onDestroy()
    }
}

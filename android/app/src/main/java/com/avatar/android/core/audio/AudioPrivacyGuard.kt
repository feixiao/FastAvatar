package com.avatar.android.core.audio

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.media.AudioDeviceInfo
import android.media.AudioManager
import android.os.Build

interface AudioPrivacyListener {
    fun onAudioRouteBecameNoisy()
    fun onAudioDevicesChanged(isPrivateOutputConnected: Boolean)
}

class AudioPrivacyGuard(
    private val context: Context,
    private val audioPlaybackController: AudioPlaybackController
) {
    private val audioManager = context.getSystemService(Context.AUDIO_SERVICE) as? AudioManager
    private var isRegistered = false
    private val listeners = mutableListOf<AudioPrivacyListener>()

    private val noisyReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            if (intent?.action == AudioManager.ACTION_AUDIO_BECOMING_NOISY) {
                // Immediate pause to protect privacy!
                audioPlaybackController.stopPlayback(reason = "becoming_noisy")
                listeners.forEach { it.onAudioRouteBecameNoisy() }
            }
        }
    }

    fun addListener(listener: AudioPrivacyListener) {
        listeners.add(listener)
    }

    fun removeListener(listener: AudioPrivacyListener) {
        listeners.remove(listener)
    }

    fun register() {
        if (!isRegistered) {
            val filter = IntentFilter(AudioManager.ACTION_AUDIO_BECOMING_NOISY)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                context.registerReceiver(noisyReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
            } else {
                context.registerReceiver(noisyReceiver, filter)
            }
            isRegistered = true
        }
    }

    fun unregister() {
        if (isRegistered) {
            try {
                context.unregisterReceiver(noisyReceiver)
            } catch (e: Exception) {
                // Ignore if not registered
            }
            isRegistered = false
        }
    }

    /**
     * Checks if a private audio output device (wired or Bluetooth headset) is currently connected.
     */
    fun isPrivateOutputConnected(): Boolean {
        val am = audioManager ?: return false
        val devices = am.getDevices(AudioManager.GET_DEVICES_OUTPUTS)
        return devices.any { device ->
            device.type == AudioDeviceInfo.TYPE_WIRED_HEADSET ||
                    device.type == AudioDeviceInfo.TYPE_WIRED_HEADPHONES ||
                    device.type == AudioDeviceInfo.TYPE_BLUETOOTH_A2DP ||
                    device.type == AudioDeviceInfo.TYPE_BLUETOOTH_SCO ||
                    device.type == AudioDeviceInfo.TYPE_USB_HEADSET ||
                    device.type == AudioDeviceInfo.TYPE_BLE_HEADSET
        }
    }
}

import Foundation
import Speech
import AVFoundation

class SpeechManager: NSObject {
    private let audioEngine = AVAudioEngine()
    private var speechRecognizer: SFSpeechRecognizer?
    private var recognitionRequest: SFSpeechAudioBufferRecognitionRequest?
    private var recognitionTask: SFSpeechRecognitionTask?

    private var localeId: String = "zh-CN"
    private var mode: String = "hands_free" // "hands_free" | "ptt"
    private var isAvatarSpeaking: Bool = false
    private var sessionStartTime: Date = Date()
    private var lastTranscriptionTime: Date = Date.distantPast
    private var lastText: String = ""
    private var hasSpeech: Bool = false
    private var isFinalizing: Bool = false
    private var isListening: Bool = false
    private var isPttActive: Bool = false
    private var tapInstalled: Bool = false
    init(localeId: String = "zh-CN") {
        super.init()
        self.localeId = localeId
        setupRecognizer(localeId: localeId)
    }

    private func setupRecognizer(localeId: String) {
        self.localeId = localeId
        let locale = Locale(identifier: localeId)
        speechRecognizer = SFSpeechRecognizer(locale: locale)
    }

    func start() {
        guard !isListening else { return }

        let inputNode = audioEngine.inputNode
        do {
            try inputNode.setVoiceProcessingEnabled(true)
        } catch {
            // Some virtual devices may not support voice processing, continue gracefully
        }

        if !tapInstalled {
            let format = inputNode.outputFormat(forBus: 0)
            inputNode.installTap(onBus: 0, bufferSize: 1024, format: format) { [weak self] buffer, time in
                self?.processAudioBuffer(buffer)
            }
            tapInstalled = true
        }

        do {
            try audioEngine.start()
            isListening = true
            if mode == "hands_free" {
                startRecognitionSession()
            }
            sendJSON([
                "event": "ready",
                "locale": localeId,
                "on_device": speechRecognizer?.supportsOnDeviceRecognition ?? false
            ])
            sendJSON(["event": "state", "state": "listening"])
        } catch {
            sendJSON(["event": "error", "message": "Failed to start audio engine: \(error.localizedDescription)"])
        }
    }

    func stop() {
        guard isListening else { return }
        isListening = false
        audioEngine.stop()
        if tapInstalled {
            audioEngine.inputNode.removeTap(onBus: 0)
            tapInstalled = false
        }
        recognitionRequest?.endAudio()
        recognitionTask?.cancel()
        recognitionRequest = nil
        recognitionTask = nil
        sendJSON(["event": "state", "state": "idle"])
    }

    private func startRecognitionSession() {
        recognitionTask?.cancel()
        recognitionTask = nil
        recognitionRequest = nil
        sessionStartTime = Date()
        lastTranscriptionTime = Date.distantPast
        lastText = ""
        hasSpeech = false
        isFinalizing = false

        let request = SFSpeechAudioBufferRecognitionRequest()
        request.shouldReportPartialResults = true
        if speechRecognizer?.supportsOnDeviceRecognition == true {
            request.requiresOnDeviceRecognition = true
        }
        self.recognitionRequest = request

        guard let recognizer = speechRecognizer, recognizer.isAvailable else {
            sendJSON(["event": "error", "message": "Speech recognizer unavailable for locale \(localeId)"])
            return
        }

        recognitionTask = recognizer.recognitionTask(with: request) { [weak self] result, error in
            guard let self = self else { return }

            if let result = result {
                let text = result.bestTranscription.formattedString.trimmingCharacters(in: .whitespacesAndNewlines)
                if !text.isEmpty {
                    self.lastText = text
                    self.lastTranscriptionTime = Date()
                    self.hasSpeech = true

                    if self.isAvatarSpeaking {
                        self.sendJSON(["event": "barge_in", "text": text])
                    }
                    self.sendJSON(["event": "partial", "text": text])
                }

                if result.isFinal {
                    if !text.isEmpty {
                        self.sendJSON(["event": "final", "text": text])
                    }
                    self.lastText = ""
                    self.hasSpeech = false
                    self.isFinalizing = false
                    if self.mode == "hands_free" && self.isListening {
                        DispatchQueue.main.async {
                            self.startRecognitionSession()
                        }
                    }
                }
            }

            if let error = error as NSError? {
                // Code 203 (silence/timeout) or 216 (cancelled) are expected flow terminations
                if (error.code == 203 || error.code == 216) && self.mode == "hands_free" && self.isListening {
                    if !self.lastText.isEmpty && !self.isFinalizing {
                        self.sendJSON(["event": "final", "text": self.lastText])
                        self.lastText = ""
                    }
                    DispatchQueue.main.async {
                        self.startRecognitionSession()
                    }
                }
            }
        }
    }

    private func processAudioBuffer(_ buffer: AVAudioPCMBuffer) {
        guard let channelData = buffer.floatChannelData?[0] else { return }
        let frames = Int(buffer.frameLength)
        if frames <= 0 { return }

        var sum: Float = 0
        for i in 0..<frames {
            let s = channelData[i]
            sum += s * s
        }
        let rms = sqrt(sum / Float(frames))

        // Feed audio buffer while listening and not currently finalizing
        if (mode == "hands_free" && isListening && !isFinalizing) || (mode == "ptt" && isPttActive) {
            recognitionRequest?.append(buffer)
        }

        // Report energy for UI soundwave/pet animations
        if rms > 0.0005 {
            sendJSON(["event": "vad", "speaking": hasSpeech, "energy": rms])
        }

        let now = Date()

        // Hands-free utterance finalization based on recognizer transcription silence
        if mode == "hands_free" && hasSpeech && !lastText.isEmpty && !isFinalizing {
            let silenceDuration = now.timeIntervalSince(lastTranscriptionTime)
            if silenceDuration >= 0.75 { // 750ms of silence after transcription
                isFinalizing = true
                hasSpeech = false
                DispatchQueue.main.async {
                    self.recognitionRequest?.endAudio()
                }
            }
        }

        // Refresh recognition session periodically if idle for > 25s to avoid Apple 60s timeout
        if mode == "hands_free" && !hasSpeech && !isFinalizing && now.timeIntervalSince(sessionStartTime) > 25.0 {
            sessionStartTime = now
            DispatchQueue.main.async {
                self.startRecognitionSession()
            }
        }
    }

    func pttStart() {
        mode = "ptt"
        isPttActive = true
        if !isListening {
            start()
        }
        startRecognitionSession()
        sendJSON(["event": "state", "state": "listening"])
    }

    func pttStop() {
        isPttActive = false
        recognitionRequest?.endAudio()
    }

    func setAvatarSpeaking(_ speaking: Bool) {
        isAvatarSpeaking = speaking
    }

    func setMode(_ m: String) {
        let prev = mode
        mode = m
        if prev == "ptt" && m == "hands_free" && isListening {
            startRecognitionSession()
        }
    }

    private func sendJSON(_ dict: [String: Any]) {
        if let data = try? JSONSerialization.data(withJSONObject: dict),
           let str = String(data: data, encoding: .utf8) {
            print(str)
            fflush(stdout)
        }
    }
}

// ==================== CLI Entry Point ====================

var selectedLocale = "zh-CN"
let args = CommandLine.arguments
for i in 0..<args.count {
    if args[i] == "--locale" && i + 1 < args.count {
        selectedLocale = args[i + 1]
    }
}

let manager = SpeechManager(localeId: selectedLocale)

// Read commands from stdin line-by-line
DispatchQueue.global(qos: .userInteractive).async {
    while let line = readLine() {
        let trimmed = line.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty { continue }

        guard let data = trimmed.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let cmd = json["cmd"] as? String else {
            continue
        }

        DispatchQueue.main.async {
            switch cmd {
            case "start":
                manager.start()
            case "stop":
                manager.stop()
            case "ptt_start":
                manager.pttStart()
            case "ptt_stop":
                manager.pttStop()
            case "set_speaking":
                if let speaking = json["speaking"] as? Bool {
                    manager.setAvatarSpeaking(speaking)
                }
            case "set_mode":
                if let mode = json["mode"] as? String {
                    manager.setMode(mode)
                }
            default:
                break
            }
        }
    }

    // Stdin closed -> clean shutdown
    DispatchQueue.main.async {
        manager.stop()
        exit(0)
    }
}

// Auto-start on launch
DispatchQueue.main.async {
    manager.start()
}

RunLoop.main.run()

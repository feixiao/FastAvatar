// DOM 元素引用
const capsule = document.getElementById("capsule");
const statusIndicator = document.getElementById("statusIndicator");
const statusText = document.getElementById("statusText");
const modeBadge = document.getElementById("modeBadge");
const subtitleText = document.getElementById("subtitleText");
const btnInput = document.getElementById("btnInput");
const btnInterrupt = document.getElementById("btnInterrupt");
const inputDrawer = document.getElementById("inputDrawer");
const textInput = document.getElementById("textInput");
const waveCanvas = document.getElementById("waveCanvas");
const ctx = waveCanvas ? waveCanvas.getContext("2d") : null;

let currentMode = "hands_free";
let currentState = "idle";
let recognition = null;
let isRecognizing = false;

// 状态文字映射
const stateLabels = {
  idle: "待命 (Idle)",
  listening: "倾听中...",
  thinking: "思考中...",
  speaking: "说话中..."
};

// 更新状态 UI
function applyState(state) {
  currentState = state;
  capsule.className = `capsule state-${state}`;
  statusText.textContent = stateLabels[state] || state;

  if (state === "idle") {
    subtitleText.textContent = currentMode === "hands_free" ? "随时开口与我畅聊..." : "按住空格说话...";
  } else if (state === "thinking") {
    subtitleText.textContent = "正在组织回答...";
  }
}

// 绑定 Wails 事件与方法
if (window.runtime) {
  window.runtime.EventsOn("avatar:state", (s) => applyState(s));
  window.runtime.EventsOn("avatar:speech", (sentence) => {
    subtitleText.textContent = sentence;
  });
  window.runtime.EventsOn("avatar:error", (err) => {
    subtitleText.textContent = "❌ " + err;
  });
  window.runtime.EventsOn("avatar:mode", (m) => {
    currentMode = m;
    modeBadge.textContent = m === "hands_free" ? "免唤醒" : "快捷键";
    if (currentState === "idle") {
      subtitleText.textContent = m === "hands_free" ? "随时开口与我畅聊..." : "按住空格说话...";
    }
  });
}

// 初始化状态
applyState("idle");

// 模式切换点击
modeBadge.addEventListener("click", () => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.ToggleMode().then((m) => {
      currentMode = m;
      modeBadge.textContent = m === "hands_free" ? "免唤醒" : "快捷键";
    });
  } else {
    currentMode = currentMode === "hands_free" ? "ptt" : "hands_free";
    modeBadge.textContent = currentMode === "hands_free" ? "免唤醒" : "快捷键";
  }
});

// 打断点击
btnInterrupt.addEventListener("click", () => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.Interrupt();
  }
  applyState("idle");
});

// 文字输入抽屉
btnInput.addEventListener("click", () => {
  inputDrawer.classList.toggle("open");
  if (inputDrawer.classList.contains("open")) {
    textInput.focus();
  }
});

textInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    const val = textInput.value.trim();
    if (val) {
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SendTextMessage(val);
      }
      textInput.value = "";
      inputDrawer.classList.remove("open");
      subtitleText.textContent = "你说: " + val;
    }
  } else if (e.key === "Escape") {
    inputDrawer.classList.remove("open");
  }
});

// 语音识别初始化 (内置 Web Speech API / AEC 麦克风)
function initSpeechRecognition() {
  const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
  if (!SpeechRecognition) {
    console.log("Web Speech API not natively available in this webview, fallback to text/stream");
    return;
  }

  recognition = new SpeechRecognition();
  recognition.continuous = true;
  recognition.interimResults = true;
  recognition.lang = "zh-CN";

  recognition.onstart = () => {
    isRecognizing = true;
  };

  recognition.onresult = (event) => {
    if (currentState === "speaking") {
      // 伴侣正在说话时用户开口 -> 触发打断!
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.Interrupt();
      }
      applyState("listening");
    }

    let finalTranscript = "";
    for (let i = event.resultIndex; i < event.results.length; i++) {
      const transcript = event.results[i][0].transcript;
      if (event.results[i].isFinal) {
        finalTranscript += transcript;
      } else {
        subtitleText.textContent = "正在听: " + transcript;
        applyState("listening");
      }
    }

    if (finalTranscript.trim()) {
      subtitleText.textContent = "你说: " + finalTranscript;
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SendTextMessage(finalTranscript.trim());
      }
    }
  };

  recognition.onerror = (e) => {
    console.warn("Speech recognition error:", e);
  };

  recognition.onend = () => {
    isRecognizing = false;
    if (currentMode === "hands_free") {
      try {
        recognition.start();
      } catch (err) {}
    }
  };

  try {
    recognition.start();
  } catch (err) {
    console.warn("Auto-start speech recognition error:", err);
  }
}

// 启动录音检测
window.addEventListener("DOMContentLoaded", () => {
  initSpeechRecognition();
});

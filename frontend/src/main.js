// DOM 元素引用
const capsule = document.getElementById("capsule");
const statusIndicator = document.getElementById("statusIndicator");
const statusText = document.getElementById("statusText");
const personaBadge = document.getElementById("personaBadge");
const modeBadge = document.getElementById("modeBadge");
const subtitleText = document.getElementById("subtitleText");
const btnMemory = document.getElementById("btnMemory");
const btnInput = document.getElementById("btnInput");
const btnInterrupt = document.getElementById("btnInterrupt");
const inputDrawer = document.getElementById("inputDrawer");
const textInput = document.getElementById("textInput");
const manageDrawer = document.getElementById("manageDrawer");
const btnCloseDrawer = document.getElementById("btnCloseDrawer");

// 抽屉内部元素
const tabButtons = document.querySelectorAll(".tab-btn");
const tabPanes = document.querySelectorAll(".tab-pane");
const profileList = document.getElementById("profileList");
const newProfKey = document.getElementById("newProfKey");
const newProfVal = document.getElementById("newProfVal");
const btnAddProfile = document.getElementById("btnAddProfile");
const episodeList = document.getElementById("episodeList");
const slotList = document.getElementById("slotList");
const chkAutoSchedule = document.getElementById("chkAutoSchedule");
const bondLevelText = document.getElementById("bondLevelText");
const bondStreakText = document.getElementById("bondStreakText");
const xpBarFill = document.getElementById("xpBarFill");
const xpLabelText = document.getElementById("xpLabelText");

let currentMode = "hands_free";
let currentState = "idle";
let recognition = null;
let isDrawerOpen = false;

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
    subtitleText.textContent = "正在思考并组织回答...";
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
  window.runtime.EventsOn("avatar:persona", (slot) => {
    if (slot && slot.name) {
      personaBadge.textContent = `${slot.badge || "🌙"} ${slot.name}`;
      refreshPersonas();
    }
  });
  window.runtime.EventsOn("avatar:bond", (bond) => {
    if (bond) {
      updateBondUI(bond);
    }
  });
  window.runtime.EventsOn("avatar:memory_updated", () => {
    refreshMemory();
  });
}

// 初始化状态
applyState("idle");

// ==================== 抽屉管理与窗口伸缩 ====================

function setDrawerOpen(open) {
  isDrawerOpen = open;
  if (open) {
    manageDrawer.classList.add("open");
    inputDrawer.classList.remove("open");
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetWindowExpanded(true);
    }
    refreshMemory();
    refreshPersonas();
    refreshBond();
  } else {
    manageDrawer.classList.remove("open");
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetWindowExpanded(false);
    }
  }
}

btnMemory.addEventListener("click", () => {
  setDrawerOpen(!isDrawerOpen);
});

btnCloseDrawer.addEventListener("click", () => {
  setDrawerOpen(false);
});

// Tab 切换
tabButtons.forEach((btn) => {
  btn.addEventListener("click", () => {
    tabButtons.forEach((b) => b.classList.remove("active"));
    tabPanes.forEach((p) => p.classList.remove("active"));

    btn.classList.add("active");
    const target = btn.getAttribute("data-tab");
    document.getElementById(target).classList.add("active");
  });
});

// ==================== 记忆库刷新与增删 ====================

function refreshMemory() {
  if (!window.go || !window.go.main || !window.go.main.App) return;

  window.go.main.App.GetMemoryData().then((data) => {
    if (!data) return;

    // 1. 渲染画像标签
    profileList.innerHTML = "";
    if (!data.profile || data.profile.length === 0) {
      profileList.innerHTML = '<span style="font-size:11px;color:#64748b;">暂无核心画像</span>';
    } else {
      data.profile.forEach((item) => {
        const tag = document.createElement("div");
        tag.className = "profile-tag";
        tag.innerHTML = `<span><b>${item.key}:</b> ${item.value}</span><span class="tag-del" title="删除">✕</span>`;
        tag.querySelector(".tag-del").addEventListener("click", () => {
          window.go.main.App.DeleteProfile(item.id).then(() => refreshMemory());
        });
        profileList.appendChild(tag);
      });
    }

    // 2. 渲染近期事件
    episodeList.innerHTML = "";
    if (!data.episodes || data.episodes.length === 0) {
      episodeList.innerHTML = '<span style="font-size:11px;color:#64748b;">暂无事件记录</span>';
    } else {
      data.episodes.forEach((ep) => {
        const card = document.createElement("div");
        card.className = "episode-card";
        const d = new Date(ep.created_at * 1000);
        const timeStr = `${d.getMonth() + 1}月${d.getDate()}日`;
        card.innerHTML = `
          <div class="episode-info">
            <span class="episode-text">${ep.content}</span>
            <span class="episode-meta">${timeStr} ${ep.tags ? `[${ep.tags.join(",")}]` : ""}</span>
          </div>
          <span class="tag-del" title="删除">✕</span>
        `;
        card.querySelector(".tag-del").addEventListener("click", () => {
          window.go.main.App.DeleteEpisode(ep.id).then(() => refreshMemory());
        });
        episodeList.appendChild(card);
      });
    }
  });
}

// 手动添加画像
btnAddProfile.addEventListener("click", () => {
  const k = newProfKey.value.trim();
  const v = newProfVal.value.trim();
  if (k && v && window.go && window.go.main && window.go.main.App) {
    window.go.main.App.AddOrUpdateProfile("custom", k, v).then(() => {
      newProfKey.value = "";
      newProfVal.value = "";
      refreshMemory();
    });
  }
});

// ==================== 人格卡槽管理 ====================

function refreshPersonas() {
  if (!window.go || !window.go.main || !window.go.main.App) return;

  window.go.main.App.GetPersonaData().then((data) => {
    if (!data) return;
    chkAutoSchedule.checked = data.auto_schedule;

    slotList.innerHTML = "";
    data.slots.forEach((s) => {
      const card = document.createElement("div");
      card.className = `slot-card ${s.id === data.active_slot_id ? "active" : ""}`;
      card.innerHTML = `
        <div class="slot-left">
          <span class="slot-badge">${s.badge || "🎭"}</span>
          <div>
            <div class="slot-name">${s.name}</div>
            <div class="slot-desc">${s.tone_description || ""}</div>
          </div>
        </div>
        <button class="badge-btn">${s.id === data.active_slot_id ? "当前激活" : "启用"}</button>
      `;
      card.addEventListener("click", () => {
        window.go.main.App.SetActivePersona(s.id).then(() => {
          refreshPersonas();
        });
      });
      slotList.appendChild(card);
    });
  });
}

chkAutoSchedule.addEventListener("change", (e) => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.SetAutoSchedule(e.target.checked);
  }
});

// 点击胶囊上的人格 Badge 快速轮转
personaBadge.addEventListener("click", () => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.RotatePersona();
  }
});

// ==================== 羁绊管理 ====================

function updateBondUI(bond) {
  if (!bond) return;
  bondLevelText.textContent = bond.level_name || `Lv.${bond.level}`;
  bondStreakText.textContent = `🔥 已连续陪伴 ${bond.streak_days} 天`;

  const thresholds = [0, 100, 300, 700, 1500, 3000];
  const nextTarget = thresholds[bond.level] || 1500;
  const pct = Math.min(100, Math.round((bond.xp / nextTarget) * 100));
  xpBarFill.style.width = `${pct}%`;
  xpLabelText.textContent = `${bond.xp} / ${nextTarget} XP`;
}

function refreshBond() {
  if (!window.go || !window.go.main || !window.go.main.App) return;
  window.go.main.App.GetBondData().then((bond) => updateBondUI(bond));
}

// 模式切换点击
modeBadge.addEventListener("click", () => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.ToggleMode();
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
  if (isDrawerOpen) setDrawerOpen(false);
  inputDrawer.classList.toggle("open");
  if (inputDrawer.classList.contains("open")) {
    textInput.focus();
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetWindowExpanded(true);
    }
  } else {
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetWindowExpanded(false);
    }
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
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SetWindowExpanded(false);
      }
      subtitleText.textContent = "你说: " + val;
    }
  } else if (e.key === "Escape") {
    inputDrawer.classList.remove("open");
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetWindowExpanded(false);
    }
  }
});

// 语音识别初始化 (内置 Web Speech API / AEC 麦克风)
function initSpeechRecognition() {
  const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
  if (!SpeechRecognition) {
    return;
  }

  recognition = new SpeechRecognition();
  recognition.continuous = true;
  recognition.interimResults = true;
  recognition.lang = "zh-CN";

  recognition.onresult = (event) => {
    if (currentState === "speaking") {
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

  recognition.onend = () => {
    if (currentMode === "hands_free") {
      try {
        recognition.start();
      } catch (err) {}
    }
  };

  try {
    recognition.start();
  } catch (err) {}
}

window.addEventListener("DOMContentLoaded", () => {
  initSpeechRecognition();
  setTimeout(() => {
    refreshPersonas();
    refreshBond();
  }, 300);
});

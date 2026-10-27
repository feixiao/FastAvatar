// main.js - 伴侣与桌面宠物交互总控

import { PetRenderer } from "./pet-renderer.js";

// ==================== 1. DOM 元素引用 ====================
// 桌面宠物 DOM
const desktopPet = document.getElementById("desktopPet");
const petBubble = document.getElementById("petBubble");
const petBubbleText = document.getElementById("petBubbleText");
const petStage = document.getElementById("petStage");
const petCanvas = document.getElementById("petCanvas");
const particleLayer = document.getElementById("particleLayer");
const petStatusPill = document.getElementById("petStatusPill");
const petStatusDot = document.getElementById("petStatusDot");
const petStatusText = document.getElementById("petStatusText");

const petInputBar = document.getElementById("petInputBar");
const petTextInput = document.getElementById("petTextInput");
const btnPetSend = document.getElementById("btnPetSend");

const btnPetMic = document.getElementById("btnPetMic");
const btnPetChat = document.getElementById("btnPetChat");
const btnPetFeed = document.getElementById("btnPetFeed");
const btnPetTouch = document.getElementById("btnPetTouch");
const btnPetPersona = document.getElementById("btnPetPersona");
const btnPetToCapsule = document.getElementById("btnPetToCapsule");
const btnPetDrawer = document.getElementById("btnPetDrawer");
const btnPetStop = document.getElementById("btnPetStop");

// 右键菜单
const petContextMenu = document.getElementById("petContextMenu");
const menuPetTitle = document.getElementById("menuPetTitle");
const menuActionPet = document.getElementById("menuActionPet");
const menuActionFeed = document.getElementById("menuActionFeed");
const menuActionChat = document.getElementById("menuActionChat");
const skinSubmenu = document.getElementById("skinSubmenu");
const personaSubmenu = document.getElementById("personaSubmenu");
const menuToggleForm = document.getElementById("menuToggleForm");
const menuToggleTop = document.getElementById("menuToggleTop");
const topStatusText = document.getElementById("topStatusText");
const menuOpenDrawer = document.getElementById("menuOpenDrawer");
const menuQuitApp = document.getElementById("menuQuitApp");

// 胶囊 DOM
const capsule = document.getElementById("capsule");
const statusIndicator = document.getElementById("statusIndicator");
const statusText = document.getElementById("statusText");
const personaBadge = document.getElementById("personaBadge");
const modeBadge = document.getElementById("modeBadge");
const subtitleText = document.getElementById("subtitleText");
const btnSwitchToPet = document.getElementById("btnSwitchToPet");
const btnMemory = document.getElementById("btnMemory");
const btnInput = document.getElementById("btnInput");
const btnInterrupt = document.getElementById("btnInterrupt");
const inputDrawer = document.getElementById("inputDrawer");
const textInput = document.getElementById("textInput");

// 中枢抽屉 DOM
const manageDrawer = document.getElementById("manageDrawer");
const btnCloseDrawer = document.getElementById("btnCloseDrawer");
const tabButtons = document.querySelectorAll(".tab-btn");
const tabPanes = document.querySelectorAll(".tab-pane");

const btnFormPet = document.getElementById("btnFormPet");
const btnFormCapsule = document.getElementById("btnFormCapsule");
const skinGrid = document.getElementById("skinGrid");
const txtSkinImportPath = document.getElementById("txtSkinImportPath");
const btnDoImportSkin = document.getElementById("btnDoImportSkin");

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

// ==================== 2. 全局状态与宠物渲染器初始化 ====================
let currentForm = "pet"; // "pet" | "capsule"
let currentSkin = "cat"; // "cat" | "fox" | "spirit" | 自定义皮肤 ID
let currentMode = "hands_free";
let currentState = "idle";
let recognition = null;
let isDrawerOpen = false;
let isAlwaysOnTop = true;
let sleepTimeout = null;

// 初始化桌宠动画播放引擎
const petRenderer = new PetRenderer(petCanvas, particleLayer);

// 状态文字与主题色映射
const stateLabels = {
  idle: "待命 (Idle)",
  listening: "倾听中...",
  thinking: "思考中...",
  speaking: "说话中...",
  sleep: "打盹中 (Zzz)",
  error: "异常",
};

const stateColors = {
  idle: "#38bdf8",
  listening: "#10b981",
  thinking: "#a855f7",
  speaking: "#f59e0b",
  sleep: "#64748b",
  error: "#ef4444",
};

// ==================== 3. 形态与皮肤控制 ====================

function applyDisplayForm(form) {
  currentForm = form === "capsule" ? "capsule" : "pet";
  document.body.className = `mode-${currentForm}`;

  if (btnFormPet && btnFormCapsule) {
    btnFormPet.classList.toggle("active", currentForm === "pet");
    btnFormCapsule.classList.toggle("active", currentForm === "capsule");
  }

  if (currentForm === "capsule") {
    menuToggleForm.textContent = "🐾 切换为桌面宠物形态";
  } else {
    menuToggleForm.textContent = "🔄 切换为极简胶囊形态";
  }
}

function switchForm(targetForm) {
  applyDisplayForm(targetForm);
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.SetDisplayForm(targetForm);
  }
}

function applySkin(skinId) {
  currentSkin = skinId || "cat";
  petRenderer.loadSkin(currentSkin);

  const skinNames = {
    cat: "🐾 橘宝喵 (桌宠)",
    fox: "🦊 灵狐小九 (桌宠)",
    spirit: "✨ 星灵露露 (桌宠)",
  };
  menuPetTitle.textContent = skinNames[currentSkin] || `🐾 ${currentSkin} (桌宠)`;

  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.SetPetSkin(currentSkin);
  }
  refreshSkinUI();
}

// 刷新皮肤卡片与子菜单
function refreshSkins() {
  if (!window.go || !window.go.main || !window.go.main.App) return;

  window.go.main.App.GetSkinsList().then((list) => {
    if (!list || !Array.isArray(list)) return;

    // 1. 渲染抽屉内的皮肤网格
    skinGrid.innerHTML = "";
    list.forEach((s) => {
      const card = document.createElement("div");
      card.className = `skin-card ${s.id === currentSkin ? "active" : ""}`;
      card.innerHTML = `
        <div class="skin-card-header">
          <span class="skin-card-title">${s.display_name}</span>
          <span class="skin-card-badge">${s.id === currentSkin ? "当前使用" : s.type === "builtin" ? "内置" : "自定义"}</span>
        </div>
        <div class="skin-card-desc">${s.description || "轻触切换"}</div>
      `;
      card.addEventListener("click", () => {
        applySkin(s.id);
      });
      skinGrid.appendChild(card);
    });

    // 2. 渲染右键菜单内的皮肤子项
    skinSubmenu.innerHTML = "";
    list.forEach((s) => {
      const item = document.createElement("div");
      item.className = "menu-item";
      item.textContent = `${s.id === currentSkin ? "✓ " : ""}${s.display_name}`;
      item.addEventListener("click", (e) => {
        e.stopPropagation();
        applySkin(s.id);
        closeContextMenu();
      });
      skinSubmenu.appendChild(item);
    });
  });
}

function refreshSkinUI() {
  const cards = skinGrid.querySelectorAll(".skin-card");
  cards.forEach((c) => {
    const title = c.querySelector(".skin-card-title")?.textContent;
    const badge = c.querySelector(".skin-card-badge");
    // 更新高亮
  });
  refreshSkins();
}

// ==================== 4. 状态机与休眠流转 ====================

function resetSleepTimer() {
  clearTimeout(sleepTimeout);
  if (currentState === "sleep") {
    applyState("idle");
  }
  // 45 秒无交互自动打盹
  sleepTimeout = setTimeout(() => {
    if (currentState === "idle" && currentForm === "pet") {
      applyState("sleep");
    }
  }, 45000);
}

function applyState(state) {
  currentState = state;
  const label = stateLabels[state] || state;
  const color = stateColors[state] || "#38bdf8";

  // 1. 更新胶囊 UI
  capsule.className = `capsule state-${state}`;
  statusText.textContent = label;

  // 2. 更新桌宠 UI
  petRenderer.setState(state);
  petStatusText.textContent = label.split(" ")[0];
  petStatusDot.style.background = color;
  petStatusDot.style.boxShadow = `0 0 8px ${color}`;

  if (state === "idle") {
    const cur = petBubbleText.textContent || "";
    if (!cur || cur.startsWith("👂") || cur.startsWith("💭") || cur.startsWith("正在思考")) {
      const defaultText =
        currentMode === "hands_free"
          ? "随时开口或摸摸我畅聊..."
          : "按住空格说话...";
      subtitleText.textContent = defaultText;
      petBubbleText.textContent = defaultText;
    }
  } else if (state === "thinking") {
    subtitleText.textContent = "正在思考并组织回答...";
    petBubbleText.textContent = "💭 思考中，请稍候片刻...";
  } else if (state === "listening") {
    subtitleText.textContent = "倾听中...";
    petBubbleText.textContent = "👂 正在听你说话...";
  } else if (state === "sleep") {
    petBubbleText.textContent = "💤 Zzz... 正在小憩打盹中 (摸摸唤醒)";
  }

  resetSleepTimer();
}

// ==================== 5. 桌面宠物交互事件 ====================

// 轻戳 / 抚摸 / 拖拽方向检测
let dragStartX = 0;
let dragStartY = 0;
let isDragging = false;
let lastMoveTime = 0;

petStage.addEventListener("pointerdown", (e) => {
  if (e.button !== 0) return; // 仅左键
  dragStartX = e.clientX;
  dragStartY = e.clientY;
  isDragging = false;
  lastMoveTime = performance.now();
  resetSleepTimer();
});

window.addEventListener("pointermove", (e) => {
  if (dragStartX === 0 && dragStartY === 0) return;
  const dx = e.clientX - dragStartX;
  const dy = e.clientY - dragStartY;
  if (Math.hypot(dx, dy) > 8) {
    isDragging = true;
    const now = performance.now();
    if (now - lastMoveTime > 80) {
      if (dx > 4) {
        petRenderer.setDragDirection("right");
      } else if (dx < -4) {
        petRenderer.setDragDirection("left");
      }
      lastMoveTime = now;
    }
  }
});

window.addEventListener("pointerup", (e) => {
  if (dragStartX === 0 && dragStartY === 0) return;
  const dx = e.clientX - dragStartX;
  const dy = e.clientY - dragStartY;
  const wasClick = Math.hypot(dx, dy) < 6;

  dragStartX = 0;
  dragStartY = 0;
  petRenderer.setDragDirection("");

  if (wasClick && currentForm === "pet") {
    // 视为单击戳了桌宠一下
    handlePetAction("poke");
  }
});

// 抚摸互动
function handlePetAction(action) {
  resetSleepTimer();
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.InteractPet(action).then((res) => {
      if (!res) return;
      petRenderer.triggerCue(res.cue || "jumping", 1200);

      const xpLabel = res.xp ? `+5 XP (${res.xp} XP)` : "+5 XP";
      if (action === "pet") {
        petRenderer.spawnHearts(100, 70, `❤️ ${xpLabel}`);
      } else if (action === "feed") {
        petRenderer.spawnHearts(100, 70, `🐟 饱腹 ${xpLabel}`);
      } else {
        petRenderer.spawnHearts(100, 70, xpLabel);
      }

      if (res.message) {
        petBubbleText.textContent = res.message;
      }
    });
  }
}

// 双击桌宠触发摸摸头
petStage.addEventListener("dblclick", () => {
  handlePetAction("pet");
});

// 快捷工具栏按钮事件
btnPetTouch.addEventListener("click", (e) => {
  e.stopPropagation();
  handlePetAction("pet");
});

btnPetFeed.addEventListener("click", (e) => {
  e.stopPropagation();
  handlePetAction("feed");
});

btnPetStop.addEventListener("click", (e) => {
  e.stopPropagation();
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.Interrupt();
  }
  applyState("idle");
  petBubbleText.textContent = "已打断发音，随时开口~";
});

btnPetMic.addEventListener("click", (e) => {
  e.stopPropagation();
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.ToggleMode();
  }
});

btnPetPersona.addEventListener("click", (e) => {
  e.stopPropagation();
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.RotatePersona();
  }
});

btnPetToCapsule.addEventListener("click", (e) => {
  e.stopPropagation();
  switchForm("capsule");
});

btnSwitchToPet.addEventListener("click", (e) => {
  e.stopPropagation();
  switchForm("pet");
});

btnPetDrawer.addEventListener("click", (e) => {
  e.stopPropagation();
  setDrawerOpen(!isDrawerOpen);
});

// 桌宠文字聊天输入条
btnPetChat.addEventListener("click", (e) => {
  e.stopPropagation();
  petInputBar.classList.toggle("open");
  if (petInputBar.classList.contains("open")) {
    petTextInput.focus();
  }
});

function submitPetText() {
  const val = petTextInput.value.trim();
  if (val && window.go && window.go.main && window.go.main.App) {
    window.go.main.App.SendTextMessage(val);
    petBubbleText.textContent = "你说: " + val;
    petTextInput.value = "";
    petInputBar.classList.remove("open");
    applyState("thinking");
  }
}

btnPetSend.addEventListener("click", submitPetText);
petTextInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    submitPetText();
  } else if (e.key === "Escape") {
    petInputBar.classList.remove("open");
  }
});

// ==================== 6. 右键自定义上下文菜单 ====================

petStage.addEventListener("contextmenu", (e) => {
  e.preventDefault();
  openContextMenu(e.clientX, e.clientY);
});

function openContextMenu(x, y) {
  petContextMenu.classList.add("open");
  const winW = window.innerWidth;
  const winH = window.innerHeight;
  const menuW = 185;
  const menuH = 290;

  let left = Math.min(x, winW - menuW - 10);
  let top = Math.min(y, winH - menuH - 10);
  left = Math.max(10, left);
  top = Math.max(10, top);

  petContextMenu.style.left = `${left}px`;
  petContextMenu.style.top = `${top}px`;
}

function closeContextMenu() {
  petContextMenu.classList.remove("open");
}

window.addEventListener("click", (e) => {
  if (!petContextMenu.contains(e.target)) {
    closeContextMenu();
  }
});

menuActionPet.addEventListener("click", () => {
  handlePetAction("pet");
  closeContextMenu();
});

menuActionFeed.addEventListener("click", () => {
  handlePetAction("feed");
  closeContextMenu();
});

menuActionChat.addEventListener("click", () => {
  petInputBar.classList.add("open");
  petTextInput.focus();
  closeContextMenu();
});

menuToggleForm.addEventListener("click", () => {
  switchForm(currentForm === "pet" ? "capsule" : "pet");
  closeContextMenu();
});

menuToggleTop.addEventListener("click", () => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.ToggleAlwaysOnTop().then((isTop) => {
      isAlwaysOnTop = isTop;
      topStatusText.textContent = isTop ? "已开启" : "已关闭";
    });
  }
  closeContextMenu();
});

menuOpenDrawer.addEventListener("click", () => {
  setDrawerOpen(true);
  closeContextMenu();
});

menuQuitApp.addEventListener("click", () => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.QuitApp();
  }
});

// ==================== 7. 中枢抽屉与形态按钮绑定 ====================

btnFormPet.addEventListener("click", () => switchForm("pet"));
btnFormCapsule.addEventListener("click", () => switchForm("capsule"));

btnDoImportSkin.addEventListener("click", () => {
  const p = txtSkinImportPath.value.trim();
  if (p && window.go && window.go.main && window.go.main.App) {
    window.go.main.App.ImportSkinPackage(p)
      .then((info) => {
        txtSkinImportPath.value = "";
        applySkin(info.id);
        refreshSkins();
        alert(`皮肤 ${info.display_name} 导入成功！`);
      })
      .catch((err) => {
        alert("导入失败: " + err);
      });
  }
});

function setDrawerOpen(open) {
  isDrawerOpen = open;
  if (open) {
    manageDrawer.classList.add("open");
    inputDrawer.classList.remove("open");
    petInputBar.classList.remove("open");
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetWindowExpanded(true);
    }
    refreshSkins();
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

btnMemory.addEventListener("click", () => setDrawerOpen(!isDrawerOpen));
btnCloseDrawer.addEventListener("click", () => setDrawerOpen(false));

// Tab 切换
tabButtons.forEach((btn) => {
  btn.addEventListener("click", () => {
    tabButtons.forEach((b) => b.classList.remove("active"));
    tabPanes.forEach((p) => p.classList.remove("active"));

    btn.classList.add("active");
    const target = btn.getAttribute("data-tab");
    document.getElementById(target)?.classList.add("active");
  });
});

// ==================== 8. 记忆库与画像管理 ====================

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

// ==================== 9. 人格卡槽管理 ====================

function refreshPersonas() {
  if (!window.go || !window.go.main || !window.go.main.App) return;

  window.go.main.App.GetPersonaData().then((data) => {
    if (!data) return;
    chkAutoSchedule.checked = data.auto_schedule;

    // 渲染卡槽
    slotList.innerHTML = "";
    personaSubmenu.innerHTML = "";

    data.slots.forEach((s) => {
      const isAct = s.id === data.active_slot_id;

      // 抽屉卡片
      const card = document.createElement("div");
      card.className = `slot-card ${isAct ? "active" : ""}`;
      card.innerHTML = `
        <div class="slot-left">
          <span class="slot-badge">${s.badge || "🎭"}</span>
          <div>
            <div class="slot-name">${s.name}</div>
            <div class="slot-desc">${s.tone_description || ""}</div>
          </div>
        </div>
        <button class="badge-btn">${isAct ? "当前激活" : "启用"}</button>
      `;
      card.addEventListener("click", () => {
        window.go.main.App.SetActivePersona(s.id).then(() => refreshPersonas());
      });
      slotList.appendChild(card);

      // 右键子菜单
      const item = document.createElement("div");
      item.className = "menu-item";
      item.textContent = `${isAct ? "✓ " : ""}${s.badge || ""} ${s.name}`;
      item.addEventListener("click", (e) => {
        e.stopPropagation();
        window.go.main.App.SetActivePersona(s.id).then(() => refreshPersonas());
        closeContextMenu();
      });
      personaSubmenu.appendChild(item);
    });
  });
}

chkAutoSchedule.addEventListener("change", (e) => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.SetAutoSchedule(e.target.checked);
  }
});

personaBadge.addEventListener("click", () => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.RotatePersona();
  }
});

// ==================== 10. 羁绊管理 ====================

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

// 胶囊快捷打断
btnInterrupt.addEventListener("click", () => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.Interrupt();
  }
  applyState("idle");
});

modeBadge.addEventListener("click", () => {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.ToggleMode();
  }
});

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
    if (val && window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SendTextMessage(val);
      textInput.value = "";
      inputDrawer.classList.remove("open");
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SetWindowExpanded(false);
      }
      subtitleText.textContent = "你说: " + val;
      applyState("thinking");
    }
  } else if (e.key === "Escape") {
    inputDrawer.classList.remove("open");
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetWindowExpanded(false);
    }
  }
});

// ==================== 11. 按住空格说话 (PTT 快捷键交互) ====================

let isSpaceHeld = false;
let pttWatchdogTimer = null;
function initPTTKeyboard() {
  window.addEventListener("keydown", (e) => {
    const active = document.activeElement;
    if (active === textInput || active === petTextInput) {
      return;
    }

    if (e.code === "Space" && !isSpaceHeld) {
      if (currentMode === "ptt") {
        isSpaceHeld = true;
        e.preventDefault();
        resetSleepTimer();
        if (window.go && window.go.main && window.go.main.App) {
          window.go.main.App.StartPTT();
        }
        applyState("listening");
        subtitleText.textContent = "🎙️ 正在倾听，松开空格发送...";
        petBubbleText.textContent = "👂 正在听，松开发送...";
      }
    }
  });

  window.addEventListener("keyup", (e) => {
    if (e.code === "Space" && isSpaceHeld) {
      isSpaceHeld = false;
      e.preventDefault();
      resetSleepTimer();
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.StopPTT();
      }
      subtitleText.textContent = "正在识别并组织回答...";
      petBubbleText.textContent = "💭 正在识别...";

      clearTimeout(pttWatchdogTimer);
      pttWatchdogTimer = setTimeout(() => {
        if (petBubbleText.textContent.includes("正在识别")) {
          petBubbleText.textContent = "👂 刚才好像没听清，请按住空格再说一次哦~";
          subtitleText.textContent = "按住空格说话...";
          applyState("idle");
        }
      }, 3000);
    }
  });
}

// ==================== 12. Wails 后端事件广播绑定 ====================

if (window.runtime) {
  window.runtime.EventsOn("avatar:state", (s) => {
    clearTimeout(pttWatchdogTimer);
    applyState(s);
  });
  window.runtime.EventsOn("avatar:speech_partial", (text) => {
    resetSleepTimer();
    subtitleText.textContent = "正在听: " + text;
    petBubbleText.textContent = "👂 " + text;
    applyState("listening");
  });
  window.runtime.EventsOn("avatar:speech_final", (text) => {
    clearTimeout(pttWatchdogTimer);
    resetSleepTimer();
    subtitleText.textContent = "你说: " + text;
    petBubbleText.textContent = "你说: " + text;
    applyState("thinking");
  });
  window.runtime.EventsOn("avatar:mic_energy", (energy) => {
    if ((currentState === "listening" || currentState === "idle") && energy > 0.04) {
      resetSleepTimer();
    }
  });
  window.runtime.EventsOn("avatar:speech", (sentence) => {
    resetSleepTimer();
    subtitleText.textContent = sentence;
    petBubbleText.textContent = sentence;
  });
  window.runtime.EventsOn("avatar:error", (err) => {
    subtitleText.textContent = "❌ " + err;
    petBubbleText.textContent = "❌ " + err;
    applyState("error");
  });
  window.runtime.EventsOn("avatar:display_form", (f) => {
    applyDisplayForm(f);
  });
  window.runtime.EventsOn("avatar:pet_skin", (s) => {
    applySkin(s);
  });
  window.runtime.EventsOn("avatar:mode", (m) => {
    currentMode = m;
    modeBadge.textContent = m === "hands_free" ? "免唤醒" : "快捷键";
    const desc = m === "hands_free" ? "免唤醒模式 (随时开口)" : "按住空格说话模式";
    petBubbleText.textContent = `🎙️ ${desc}`;
    if (currentState === "idle") {
      const defaultText =
        m === "hands_free" ? "随时开口与我畅聊..." : "按住空格说话...";
      subtitleText.textContent = defaultText;
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

// ==================== 13. DOM 就绪自启 ====================

window.addEventListener("DOMContentLoaded", () => {
  // 获取后端保存的形态和皮肤
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.GetDisplayForm().then((form) => applyDisplayForm(form));
    window.go.main.App.GetPetSkin().then((skin) => applySkin(skin));
    window.go.main.App.GetMode().then((m) => {
      currentMode = m;
      modeBadge.textContent = m === "hands_free" ? "免唤醒" : "快捷键";
      if (currentState === "idle") {
        const defaultText = m === "hands_free" ? "随时开口与我畅聊..." : "按住空格说话...";
        subtitleText.textContent = defaultText;
        petBubbleText.textContent = defaultText;
      }
    });
  } else {
    applyDisplayForm("pet");
    applySkin("cat");
  }

  applyState("idle");
  initPTTKeyboard();

  setTimeout(() => {
    refreshSkins();
    refreshPersonas();
    refreshBond();
  }, 300);
});

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
const petCharacterView = document.getElementById("petCharacterView");
const whaleCharacterBox = document.getElementById("whaleCharacterBox");
const whaleBaseImg = document.getElementById("whaleBaseImg");
const whaleMouth = document.getElementById("whaleMouth");
const whalePoseContainer = document.getElementById("whalePoseContainer");
const whalePoseImg = document.getElementById("whalePoseImg");
const whaleFoxtailLure = document.getElementById("whaleFoxtailLure");

const petInputBar = document.getElementById("petInputBar");
const petTextInput = document.getElementById("petTextInput");
const btnPetSend = document.getElementById("btnPetSend");

const btnPetMic = document.getElementById("btnPetMic");
const btnPetChat = document.getElementById("btnPetChat");
const btnPetFeed = document.getElementById("btnPetFeed");
const btnPetRoam = document.getElementById("btnPetRoam");
const btnPetTouch = document.getElementById("btnPetTouch");
const btnPetRice = document.getElementById("btnPetRice");
const btnPetTurbo = document.getElementById("btnPetTurbo");
const btnPetDialKit = document.getElementById("btnPetDialKit");
const btnPetPersona = document.getElementById("btnPetPersona");
const btnPetToCapsule = document.getElementById("btnPetToCapsule");
const btnPetDrawer = document.getElementById("btnPetDrawer");
const btnPetStop = document.getElementById("btnPetStop");

const roamBubble = document.getElementById("roamBubble");
const macPetShadow = document.getElementById("macPetShadow");

// 右键菜单
const petContextMenu = document.getElementById("petContextMenu");
const menuPetTitle = document.getElementById("menuPetTitle");
const menuActionPet = document.getElementById("menuActionPet");
const menuActionFish = document.getElementById("menuActionFish");
const menuActionRice = document.getElementById("menuActionRice");
const menuActionTurbo = document.getElementById("menuActionTurbo");
const menuActionGrass = document.getElementById("menuActionGrass");
const menuToggleRoam = document.getElementById("menuToggleRoam");
const roamStatusMenuText = document.getElementById("roamStatusMenuText");
const menuActionChat = document.getElementById("menuActionChat");
const skinSubmenu = document.getElementById("skinSubmenu");
const personaSubmenu = document.getElementById("personaSubmenu");
const menuToggleForm = document.getElementById("menuToggleForm");
const menuToggleTop = document.getElementById("menuToggleTop");
const topStatusText = document.getElementById("topStatusText");
const menuOpenAudit = document.getElementById("menuOpenAudit");
const menuOpenDrawer = document.getElementById("menuOpenDrawer");
const menuQuitApp = document.getElementById("menuQuitApp");

// Things 3 速记流与模型端侧抽屉 DOM
const memoCardsList = document.getElementById("memoCardsList");
const newMemoInput = document.getElementById("newMemoInput");
const btnQuickAddMemo = document.getElementById("btnQuickAddMemo");
const fishFeedCount = document.getElementById("fishFeedCount");
const roamPatrolCount = document.getElementById("roamPatrolCount");

const cfgEdgeBaseUrl = document.getElementById("cfgEdgeBaseUrl");
const cfgEdgeModel = document.getElementById("cfgEdgeModel");
const cfgCloudUrl = document.getElementById("cfgCloudUrl");
const chkCloudConsent = document.getElementById("chkCloudConsent");
const btnSaveModelConfig = document.getElementById("btnSaveModelConfig");

// TPM 实时审计与 DialKit HUD
const tpmHudPanel = document.getElementById("tpmHudPanel");
const btnCloseTpmHud = document.getElementById("btnCloseTpmHud");
const tpmRoamCoord = document.getElementById("tpmRoamCoord");
const tpmRoamDesc = document.getElementById("tpmRoamDesc");
const tpmRmsValue = document.getElementById("tpmRmsValue");
const tpmRmsBar = document.getElementById("tpmRmsBar");
const tpmActiveTrack = document.getElementById("tpmActiveTrack");

const dialKitHud = document.getElementById("dialKitHud");
const btnCloseDialKit = document.getElementById("btnCloseDialKit");
const sliderScale = document.getElementById("sliderScale");
const sliderSpeed = document.getElementById("sliderSpeed");
const valScale = document.getElementById("valScale");
const valSpeed = document.getElementById("valSpeed");
const btnCopyDialTokens = document.getElementById("btnCopyDialTokens");
// 胶囊 DOM
const capsule = document.getElementById("capsule");
const orbContainer = document.getElementById("orbContainer");
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
const newEpContent = document.getElementById("newEpContent");
const btnAddEpisode = document.getElementById("btnAddEpisode");
const slotList = document.getElementById("slotList");
const chkAutoSchedule = document.getElementById("chkAutoSchedule");
const bondStreakHeader = document.getElementById("bondStreakHeader");
const bondLevelText = document.getElementById("bondLevelText");
const bondStreakText = document.getElementById("bondStreakText");
const xpBarFill = document.getElementById("xpBarFill");
const xpLabelText = document.getElementById("xpLabelText");
// ==================== 2. 全局状态与宠物渲染器初始化 ====================
let currentForm = "pet"; // "pet" | "capsule"
let currentSkin = "luoxiaohei2d"; // 默认官方宠伴: 罗小黑 2D
let isAutoRoamingEnabled = true;
let roamSpeedFactor = 1.2;
let petScale = 1.15;
let currentRms = 0.0;
let isRoamingMoving = false;
let roamCurrentOffset = 0;
let roamTargetOffset = 0;
let roamTimer = null;
let roamAnimId = null;
let roamBubbleTimer = null;
let isDirectedRoaming = false;
let roamWaypoints = [];
let roamLastFrameTs = 0;
let fishFedTotal = 28;
let patrolStepsTotal = 142;
let dragInitialMouseX = 0;
let dragInitialMouseY = 0;
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
  idle: "待命",
  listening: "倾听中",
  thinking: "思考中",
  speaking: "说话中",
  sleep: "打盹中",
  error: "异常",
};

const stateColors = {
  idle: "#0051ba",
  listening: "#047857",
  thinking: "#6d28d9",
  speaking: "#b45309",
  sleep: "#4b5563",
  error: "#b91c1c",
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
    menuToggleForm.textContent = "切换为灵动桌宠形态";
  } else {
    menuToggleForm.textContent = "切换为挺括胶囊形态";
  }
}

function switchForm(targetForm) {
  applyDisplayForm(targetForm);
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.SetDisplayForm(targetForm);
  }
}

let activePoseTimer = null;
let isWhalePosePlaying = false;
let activeWhalePose = null;

function playWhalePose(poseName, durationMs = 0) {
  if (currentSkin !== "whale" || !petCharacterView || !whaleCharacterBox || !whalePoseContainer || !whalePoseImg) {
    return;
  }
  clearTimeout(activePoseTimer);
  isWhalePosePlaying = true;
  activeWhalePose = poseName;

  whaleCharacterBox.style.display = "none";
  whalePoseContainer.style.display = "flex";
  whalePoseContainer.className = `whale-pose-container pose-${poseName}`;
  whalePoseImg.src = `./assets/whale/${poseName}.webp`;

  if (durationMs > 0) {
    activePoseTimer = setTimeout(() => {
      stopWhalePose();
    }, durationMs);
  }
}

function stopWhalePose() {
  clearTimeout(activePoseTimer);
  isWhalePosePlaying = false;
  activeWhalePose = null;
  if (whaleCharacterBox && whalePoseContainer) {
    whaleCharacterBox.style.display = "flex";
    whalePoseContainer.style.display = "none";
  }
}

function playFoxtailInteraction() {
  if (currentSkin !== "whale" || !whaleFoxtailLure) return;
  whaleFoxtailLure.style.display = "block";
  whaleFoxtailLure.style.left = "62%";
  whaleFoxtailLure.style.top = "24%";
  setTimeout(() => {
    whaleFoxtailLure.style.display = "none";
  }, 2200);
}

function applySkin(skinId, notifyBackend = true) {
  const targetSkin = skinId || "luoxiaohei2d";
  currentSkin = targetSkin;
  stopWhalePose();

  if (currentSkin === "whale") {
    if (petCharacterView) petCharacterView.style.display = "flex";
    if (petCanvas) petCanvas.style.display = "none";
  } else {
    if (petCharacterView) petCharacterView.style.display = "none";
    if (petCanvas) petCanvas.style.display = "block";
    petRenderer.loadSkin(currentSkin);
  }

  const skinNames = {
    luoxiaohei2d: "罗小黑 2D (官方宠伴)",
    whale: "大肥鱼 (鲸鱼娘)",
    cat: "橘宝喵 (桌宠伴侣)",
    fox: "灵狐小九 (桌宠伴侣)",
    spirit: "星灵露露 (桌宠伴侣)",
  };
  if (menuPetTitle) {
    menuPetTitle.textContent = skinNames[currentSkin] || `${currentSkin} (桌宠伴侣)`;
  }
  if (notifyBackend && window.go && window.go.main && window.go.main.App) {
    window.go.main.App.SetPetSkin(currentSkin);
  }
  refreshSkinUI();
}

// 刷新皮肤卡片与子菜单
const defaultSkins = [
  { id: "luoxiaohei2d", display_name: "罗小黑 2D", description: "官方默认宠伴：纯黑灵动小猫、黄绿大眼、呆萌耳朵与分叉尾巴，支持桌面自由漫步与全双工打断", type: "builtin" },
  { id: "whale", display_name: "大肥鱼 (鲸鱼娘)", description: "呆萌温和、爱吃白饭，嘴型随说话起伏自然张合", type: "builtin" },
  { id: "cat", display_name: "橘宝喵", description: "软萌可爱的橘猫伴侣，活泼好动，拥有眨眼等完整动作帧", type: "builtin" },
  { id: "fox", display_name: "灵狐小九", description: "仙气灵动的治愈系赤狐，摇着毛茸茸大尾巴", type: "builtin" },
  { id: "spirit", display_name: "星灵露露", description: "软萌发光的奇幻小水滴星灵，律动跳跃，温暖治愈", type: "builtin" },
];

function renderSkinList(list) {
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
}

function refreshSkins() {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.GetSkinsList().then((list) => {
      if (list && Array.isArray(list) && list.length > 0) {
        renderSkinList(list);
      } else {
        renderSkinList(defaultSkins);
      }
    }).catch(() => renderSkinList(defaultSkins));
  } else {
    renderSkinList(defaultSkins);
  }
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

// 桌宠气泡默认隐藏，只在有实际内容（识别文本/回复/提示）时短暂浮现
let bubbleHideTimer = null;
function showBubble(text, holdMs = 4000) {
  petBubbleText.textContent = text;
  petBubble.classList.add("visible");
  clearTimeout(bubbleHideTimer);
  bubbleHideTimer = setTimeout(hideBubble, holdMs);
}

function hideBubble() {
  clearTimeout(bubbleHideTimer);
  petBubble.classList.remove("visible");
}
petBubble.addEventListener("click", hideBubble);


let thinkingWatchdog = null;
function scheduleThinkingWatchdog() {
  clearTimeout(thinkingWatchdog);
  thinkingWatchdog = setTimeout(() => {
    if (currentState === "thinking") {
      showBubble("⏳ 回复超时，请检查大模型连接", 4000);
      applyState("idle");
    }
  }, 18000);
}

function applyState(state) {
  const previousState = currentState;
  currentState = state;
  const label = stateLabels[state] || state;
  const color = stateColors[state] || "#38bdf8";

  if (state === "thinking") {
    scheduleThinkingWatchdog();
  } else {
    clearTimeout(thinkingWatchdog);
  }

  // 1. 更新胶囊 UI
  if (capsule) capsule.className = `capsule state-${state}`;
  if (statusText) statusText.textContent = label;

  // 2. 更新桌宠 UI
  if (desktopPet) desktopPet.dataset.state = state;
  petRenderer.setState(state);
  if (petStatusText) petStatusText.textContent = label.split(" ")[0];
  if (petStatusDot) {
    petStatusDot.style.background = color;
    petStatusDot.style.boxShadow = `0 0 8px ${color}`;
  }

  // 3. 严格闭环大肥鱼嘴型状态：只有在 speaking 发音时嘴型才张合，其余状态强制闭合锁紧
  if (whaleMouth) {
    if (state === "speaking") {
      whaleMouth.style.opacity = "1";
    } else {
      whaleMouth.style.opacity = "0";
    }
  }

  // 4. 休眠与唤醒大肥鱼动效联动
  if (state === "sleep" && currentSkin === "whale") {
    playWhalePose("sleep-asleep", 0);
  } else if (previousState === "sleep" && state !== "sleep" && currentSkin === "whale") {
    playWhalePose("sleep-waking", 1600);
  }

  // 5. 状态机动作轨道流转与自主漫游联动 (模式 B 规范)
  if (state === "idle") {
    if (!isDirectedRoaming) {
      updateAuditTrack("Row 0 (idle)");
    }
    const cur = subtitleText.textContent || "";
    if (!cur || cur.startsWith("正在") || cur.startsWith("倾听") || cur.startsWith("🎙️")) {
      subtitleText.textContent =
        currentMode === "hands_free" ? "随时开口或摸摸我畅聊..." : "按住空格说话...";
    }
    if (petBubble.classList.contains("visible")) {
      clearTimeout(bubbleHideTimer);
      bubbleHideTimer = setTimeout(hideBubble, 3000);
    }
    scheduleNextRoamDecision();
  } else if (state === "thinking") {
    if (!isDirectedRoaming) {
      stopRoamingMove();
      updateAuditTrack("Row 8 (review)");
    }
    subtitleText.textContent = "正在思考并组织回答...";
  } else if (state === "listening") {
    // 打断或进入倾听：≤50ms 强制收口闭嘴，瞬切 Row 6 前倾倾听
    stopRoamingMove();
    updateAuditTrack("Row 6 (waiting)");
    subtitleText.textContent = "倾听中...";
  } else if (state === "speaking") {
    if (!isDirectedRoaming) {
      stopRoamingMove();
      updateAuditTrack("Row 3 (waving)");
    }
  } else if (state === "sleep") {
    stopRoamingMove();
    hideBubble();
  } else if (state === "error") {
    stopRoamingMove();
  }
  resetSleepTimer();
}

// ==================== 5. 桌面宠物交互事件 & 活体自主自由漫步 ====================

function showRoamBubble(text, durationMs = 2500) {
  if (!roamBubble) return;
  roamBubble.textContent = text;
  roamBubble.style.display = "flex";
  roamBubble.style.opacity = "1";
  clearTimeout(roamBubbleTimer);
  roamBubbleTimer = setTimeout(() => {
    roamBubble.style.opacity = "0";
    setTimeout(() => {
      if (roamBubble.style.opacity === "0") {
        roamBubble.style.display = "none";
      }
    }, 200);
  }, durationMs);
}

function scheduleNextRoamDecision() {
  clearTimeout(roamTimer);
  if (!isAutoRoamingEnabled) return;

  const delay = Math.random() * 4000 + 3500; // 3.5 ~ 7.5 秒思考时间
  roamTimer = setTimeout(() => {
    if (currentState === "idle" && !isDragging && currentForm === "pet" && !isDrawerOpen) {
      makeAutonomousMoveDecision();
    }
    scheduleNextRoamDecision();
  }, delay);
}

function makeAutonomousMoveDecision() {
  if (currentState !== "idle" || isDragging || isDrawerOpen || isDirectedRoaming) return;

  roamTargetOffset = (Math.random() - 0.5) * 60; // -30px ~ +30px 漫游
  const dx = roamTargetOffset - roamCurrentOffset;
  if (Math.abs(dx) < 6) return;

  isDirectedRoaming = false;
  roamWaypoints = [];
  isRoamingMoving = true;
  roamLastFrameTs = 0;
  patrolStepsTotal += Math.round(Math.abs(dx));
  if (roamPatrolCount) roamPatrolCount.textContent = `${patrolStepsTotal} 步`;

  const dir = dx > 0 ? "right" : "left";
  petRenderer.setDragDirection(dir);
  showRoamBubble(dx > 0 ? "🐾 向右边巡视小跑..." : "🐾 向左边巡视小跑...", 2500);
  updateAuditTrack(dir === "right" ? "Row 1 (running-right)" : "Row 2 (running-left)");
  if (tpmRoamCoord) tpmRoamCoord.textContent = `巡视中 → ${Math.round(roamTargetOffset)}px`;

  animateRoamStep();
}

function animateRoamStep(nowTs) {
  const stateBlocked = isDirectedRoaming
    ? currentState === "listening" || currentState === "sleep" || currentState === "error"
    : currentState !== "idle";
  if (!isRoamingMoving || stateBlocked || isDragging) {
    stopRoamingMove();
    return;
  }

  const ts = typeof nowTs === "number" ? nowTs : performance.now();
  const dtSec = roamLastFrameTs > 0 ? Math.min(0.05, Math.max(0.004, (ts - roamLastFrameTs) / 1000)) : 1 / 60;
  roamLastFrameTs = ts;

  const diff = roamTargetOffset - roamCurrentOffset;
  if (Math.abs(diff) < 2) {
    roamCurrentOffset = roamTargetOffset;
    petStage.style.transform = `translateX(${roamCurrentOffset}px) scale(var(--pet-scale))`;

    if (roamWaypoints.length > 0) {
      const nextTarget = roamWaypoints.shift();
      roamTargetOffset = Math.max(-45, Math.min(45, nextTarget));
      const nextDiff = roamTargetOffset - roamCurrentOffset;
      if (Math.abs(nextDiff) >= 2) {
        const nextDir = nextDiff > 0 ? "right" : "left";
        petRenderer.setDragDirection(nextDir);
        updateAuditTrack(nextDir === "right" ? "Row 1 (running-right)" : "Row 2 (running-left)");
        if (tpmRoamCoord) tpmRoamCoord.textContent = `巡逻中 → ${Math.round(roamTargetOffset)}px`;
        roamAnimId = requestAnimationFrame(animateRoamStep);
        return;
      }
    }

    const wasDirected = isDirectedRoaming;
    stopRoamingMove();
    if (wasDirected && currentState === "speaking") {
      updateAuditTrack("Row 3 (waving)");
      if (tpmRoamCoord) tpmRoamCoord.textContent = `巡逻完成 (${Math.round(roamCurrentOffset)}px)`;
    } else if (currentState === "idle") {
      showRoamBubble("🐾 原地蹲下休息打盹...", 2000);
      updateAuditTrack("Row 0 (idle)");
      if (tpmRoamCoord) tpmRoamCoord.textContent = `休憩中 (${Math.round(roamCurrentOffset)}px)`;
    }
    return;
  }

  const pxPerSec = 90 * roamSpeedFactor;
  const step = Math.sign(diff) * Math.min(Math.abs(diff), pxPerSec * dtSec);
  roamCurrentOffset += step;
  petStage.style.transform = `translateX(${roamCurrentOffset}px) scale(var(--pet-scale))`;
  roamAnimId = requestAnimationFrame(animateRoamStep);
}

function stopRoamingMove() {
  isRoamingMoving = false;
  isDirectedRoaming = false;
  roamWaypoints = [];
  roamLastFrameTs = 0;
  if (roamAnimId) {
    cancelAnimationFrame(roamAnimId);
    roamAnimId = null;
  }
  petRenderer.setDragDirection("");
}

function startRoamSequence(waypoints, bubbleText) {
  if (isDragging || isDrawerOpen || currentForm !== "pet") return false;
  if (!Array.isArray(waypoints) || waypoints.length === 0) return false;

  const clamped = waypoints.map((w) => Math.max(-45, Math.min(45, w)));
  const firstTarget = clamped.shift();
  const diff = firstTarget - roamCurrentOffset;
  if (Math.abs(diff) < 4 && clamped.length === 0) return false;

  if (roamAnimId) {
    cancelAnimationFrame(roamAnimId);
    roamAnimId = null;
  }
  clearTimeout(roamTimer);

  isDirectedRoaming = true;
  isRoamingMoving = true;
  roamLastFrameTs = 0;
  roamTargetOffset = firstTarget;
  roamWaypoints = clamped;

  let totalDist = Math.abs(firstTarget - roamCurrentOffset);
  let prev = firstTarget;
  for (const wp of clamped) {
    totalDist += Math.abs(wp - prev);
    prev = wp;
  }
  patrolStepsTotal += Math.round(totalDist);
  if (roamPatrolCount) roamPatrolCount.textContent = `${patrolStepsTotal} 步`;

  const dir = (roamTargetOffset - roamCurrentOffset) >= 0 ? "right" : "left";
  petRenderer.clearCue();
  petRenderer.setDragDirection(dir);
  showRoamBubble(bubbleText || (dir === "right" ? "🐾 收到指令，向右巡视..." : "🐾 收到指令，向左巡视..."), 2500);
  updateAuditTrack(dir === "right" ? "Row 1 (running-right)" : "Row 2 (running-left)");
  if (tpmRoamCoord) tpmRoamCoord.textContent = `巡逻中 → ${Math.round(roamTargetOffset)}px`;
  animateRoamStep();
  return true;
}

function triggerDirectedRoam(offsetDelta) {
  const target = Math.max(-45, Math.min(45, roamCurrentOffset + offsetDelta));
  if (Math.abs(target - roamCurrentOffset) < 10) {
    const bounce = offsetDelta > 0 ? [-18, 42] : [18, -42];
    return startRoamSequence(bounce);
  }
  return startRoamSequence([target]);
}

function toggleAutoRoaming() {
  isAutoRoamingEnabled = !isAutoRoamingEnabled;
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.SetAutoRoaming(isAutoRoamingEnabled);
  }
  updateRoamingUI();
}

function updateRoamingUI() {
  if (roamStatusMenuText) {
    roamStatusMenuText.textContent = isAutoRoamingEnabled ? "已开启" : "已暂停";
  }
  if (btnPetRoam) {
    btnPetRoam.style.opacity = isAutoRoamingEnabled ? "1" : "0.5";
    btnPetRoam.title = isAutoRoamingEnabled ? "桌面自由漫步：开启中 (点击暂停)" : "自由漫步：已暂停 (点击开启)";
  }
  if (isAutoRoamingEnabled) {
    scheduleNextRoamDecision();
    showRoamBubble("桌面自由漫步：开启中", 2000);
  } else {
    stopRoamingMove();
    roamCurrentOffset = 0;
    petStage.style.transform = `translateX(0px) scale(var(--pet-scale))`;
    showRoamBubble("自由漫步：已暂停", 2000);
  }
}

// 轻戳 / 抚摸 / 拖拽方向检测
let dragStartX = 0;
let dragStartY = 0;
let isDragging = false;
let lastMoveTime = 0;

function resetDragState() {
  dragStartX = 0;
  dragStartY = 0;
  isDragging = false;
  petRenderer.setDragDirection("");
}

petStage.addEventListener("pointerdown", (e) => {
  if (e.button !== 0) return; // 仅左键
  dragStartX = e.clientX;
  dragStartY = e.clientY;
  isDragging = false;
  stopRoamingMove();
  lastMoveTime = performance.now();
  resetSleepTimer();
});

window.addEventListener("pointermove", (e) => {
  if (dragStartX === 0 && dragStartY === 0) return;
  const dx = e.clientX - dragStartX;
  const dy = e.clientY - dragStartY;
  if (Math.hypot(dx, dy) > 8) {
    if (!isDragging) {
      isDragging = true;
      showRoamBubble("⚡ 被主人揪起后颈皮！", 3000);
    }
    const now = performance.now();
    if (now - lastMoveTime > 60) {
      if (dx > 4) {
        petRenderer.setDragDirection("right");
        updateAuditTrack("Row 1 (running-right)");
      } else if (dx < -4) {
        petRenderer.setDragDirection("left");
        updateAuditTrack("Row 2 (running-left)");
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

  if (isDragging) {
    showRoamBubble("🐾 平稳落地，继续巡视...", 2000);
    // 边缘吸附检查
    if (window.go && window.go.main && window.go.main.App && window.go.main.App.GetWindowPosition) {
      window.go.main.App.GetWindowPosition().then((pos) => {
        if (pos && pos.length >= 2) {
          const [wx, wy] = pos;
          if (wx < 30 || wy < 30) {
            window.go.main.App.SetWindowPosition(wx, wy);
          }
        }
      });
    }
  }

  resetDragState();

  if (wasClick && currentForm === "pet") {
    handlePetAction("poke");
  } else {
    scheduleNextRoamDecision();
  }
});

window.addEventListener("pointercancel", resetDragState);
window.addEventListener("blur", resetDragState);
window.addEventListener("mouseleave", resetDragState);

// 双击罗小黑身体：平滑滑出 Things 3 纯白极简手账中枢面板 (Companion Journal)
petStage.addEventListener("dblclick", (e) => {
  e.stopPropagation();
  setDrawerOpen(!isDrawerOpen);
  petRenderer.triggerCue("jumping", 800);
  petRenderer.spawnHearts(100, 70, "📘 案头手账");
});
function handlePetAction(action) {
  resetSleepTimer();
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.InteractPet(action).then((res) => {
      if (!res) return;
      petRenderer.triggerCue(res.cue || "jumping", 1200);

      // 大肥鱼专属动作呈现
      if (currentSkin === "whale") {
        if (action === "rice") {
          playWhalePose("rice", 2600);
        } else if (action === "turbo") {
          playWhalePose("turbo", 1600);
        } else if (action === "grass") {
          playFoxtailInteraction();
        } else if (action === "wake") {
          playWhalePose("sleep-waking", 1600);
        }
      }

      const xpLabel = res.xp ? `+5 XP (${res.xp} XP)` : "+5 XP";
      if (action === "pet") {
        updateAuditTrack("Row 4 (jumping)");
        petRenderer.spawnHearts(100, 70, `❤️ ${xpLabel}`);
      } else if (action === "rice" || action === "turbo") {
        updateAuditTrack("Row 7 (feed)");
        petRenderer.spawnHearts(100, 70, `🍚 干饭 ${xpLabel}`);
      } else if (action === "feed") {
        fishFedTotal += 1;
        if (fishFeedCount) fishFeedCount.textContent = `${fishFedTotal} 次`;
        updateAuditTrack("Row 7 (feed)");
        petRenderer.spawnHearts(100, 70, `🐟 饱腹 ${xpLabel}`);
      } else if (action === "grass") {
        updateAuditTrack("Row 4 (jumping)");
        petRenderer.spawnHearts(100, 70, `🌾 逗草 ${xpLabel}`);
      } else {
        updateAuditTrack("Row 4 (jumping)");
        petRenderer.spawnHearts(100, 70, xpLabel);
      }
      if (res.message) {
        showBubble(res.message);
      }
    });
  }
}



// 快捷工具栏按钮事件
btnPetTouch.addEventListener("click", (e) => {
  e.stopPropagation();
  handlePetAction("pet");
});

if (btnPetFeed) {
  btnPetFeed.addEventListener("click", (e) => {
    e.stopPropagation();
    handlePetAction("feed");
  });
}
if (btnPetRoam) {
  btnPetRoam.addEventListener("click", (e) => {
    e.stopPropagation();
    toggleAutoRoaming();
  });
}
if (btnPetDialKit) {
  btnPetDialKit.addEventListener("click", (e) => {
    e.stopPropagation();
    toggleDialKit();
  });
}
if (btnPetRice) {
  btnPetRice.addEventListener("click", (e) => {
    e.stopPropagation();
    handlePetAction("rice");
  });
}
if (btnPetTurbo) {
  btnPetTurbo.addEventListener("click", (e) => {
    e.stopPropagation();
    handlePetAction("turbo");
  });
}
if (menuActionFish) {
  menuActionFish.addEventListener("click", () => {
    handlePetAction("feed");
    closeContextMenu();
  });
}
if (menuToggleRoam) {
  menuToggleRoam.addEventListener("click", () => {
    toggleAutoRoaming();
    closeContextMenu();
  });
}
if (menuOpenAudit) {
  menuOpenAudit.addEventListener("click", () => {
    toggleDialKit();
    closeContextMenu();
  });
}

if (menuActionRice) {
  menuActionRice.addEventListener("click", () => {
    handlePetAction("rice");
    closeContextMenu();
  });
}

if (menuActionTurbo) {
  menuActionTurbo.addEventListener("click", () => {
    handlePetAction("turbo");
    closeContextMenu();
  });
}

if (menuActionGrass) {
  menuActionGrass.addEventListener("click", () => {
    handlePetAction("grass");
    closeContextMenu();
  });
}

btnPetStop.addEventListener("click", (e) => {
  e.stopPropagation();
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.Interrupt();
  }
  applyState("idle");
  showBubble("已打断发音，随时开口~", 2500);
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
    showBubble("你说: " + val, 5000);
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
  petContextMenu.classList.add("visible");
  const winW = window.innerWidth;
  const winH = window.innerHeight;
  const menuW = 185;
  const menuH = 300;

  let left = Math.min(x, winW - menuW - 10);
  let top = Math.min(y, winH - menuH - 10);
  left = Math.max(10, left);
  top = Math.max(10, top);

  petContextMenu.style.left = `${left}px`;
  petContextMenu.style.top = `${top}px`;

  const submenus = petContextMenu.querySelectorAll(".submenu");
  submenus.forEach((sub) => {
    if (left + menuW + 145 > winW) {
      sub.classList.add("flip-left");
    } else {
      sub.classList.remove("flip-left");
    }
  });
}

function closeContextMenu() {
  petContextMenu.classList.remove("open");
  petContextMenu.classList.remove("visible");
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
    stopRoamingMove();
    document.body.classList.add("drawer-open");
    manageDrawer.classList.add("open");
    inputDrawer.classList.remove("open");
    petInputBar.classList.remove("open");
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetWindowExpanded(true);
    }
    refreshSkins();
    refreshMemory();
    renderMemos();
    refreshPersonas();
    refreshBond();
    refreshModelConfig();
  } else {
    document.body.classList.remove("drawer-open");
    manageDrawer.classList.remove("open");
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetWindowExpanded(false);
    }
    scheduleNextRoamDecision();
  }
}

// ==================== 7.5 TPM 实时审计与 DialKit 手感调参 ====================

function updateAuditTrack(trackName) {
  if (tpmActiveTrack) tpmActiveTrack.textContent = trackName;
  const tagMap = {
    "idle": "tagRow0",
    "Row 0 (idle)": "tagRow0",
    "running-right": "tagRow1",
    "Row 1 (running-right)": "tagRow1",
    "running-left": "tagRow2",
    "Row 2 (running-left)": "tagRow2",
    "speaking": "tagRow3",
    "waving": "tagRow3",
    "Row 3 (waving)": "tagRow3",
    "Row 3 (waving-excited)": "tagRow3",
    "jumping": "tagRow4",
    "Row 4 (jumping)": "tagRow4",
    "listening": "tagRow6",
    "waiting": "tagRow6",
    "Row 6 (waiting)": "tagRow6",
    "running": "tagRow7",
    "feed": "tagRow7",
    "Row 7 (feed)": "tagRow7",
    "thinking": "tagRow8",
    "review": "tagRow8",
    "Row 8 (review)": "tagRow8",
  };
  document.querySelectorAll(".track-tag").forEach((el) => el.classList.remove("active"));
  let tagId = tagMap[trackName];
  if (!tagId && trackName && trackName.includes("->")) {
    const parts = trackName.split("->");
    const targetCue = parts[parts.length - 1].trim();
    tagId = tagMap[targetCue];
  }
  if (tagId && document.getElementById(tagId)) {
    document.getElementById(tagId).classList.add("active");
  }
}

function updateRmsMeter(energy) {
  if (tpmRmsValue) tpmRmsValue.textContent = energy.toFixed(2);
  if (tpmRmsBar) {
    const pct = Math.min(100, Math.round(energy * 120));
    tpmRmsBar.style.width = `${pct}%`;
  }
}

function toggleDialKit() {
  if (!dialKitHud) return;
  dialKitHud.classList.toggle("hidden");
  if (tpmHudPanel) {
    tpmHudPanel.classList.toggle("hidden");
  }
}

if (btnCloseDialKit) {
  btnCloseDialKit.addEventListener("click", () => {
    dialKitHud.classList.add("hidden");
    if (tpmHudPanel) tpmHudPanel.classList.add("hidden");
  });
}
if (btnCloseTpmHud) {
  btnCloseTpmHud.addEventListener("click", () => {
    if (tpmHudPanel) tpmHudPanel.classList.add("hidden");
  });
}

if (sliderScale) {
  sliderScale.addEventListener("input", (e) => updateDialParam("scale", e.target.value));
}
if (sliderSpeed) {
  sliderSpeed.addEventListener("input", (e) => updateDialParam("speed", e.target.value));
}

function updateDialParam(key, val) {
  const num = parseFloat(val);
  if (key === "scale") {
    petScale = num;
    document.documentElement.style.setProperty("--pet-scale", num);
    if (valScale) valScale.textContent = `${num.toFixed(2)}x`;
  } else if (key === "speed") {
    roamSpeedFactor = num;
    if (valSpeed) valSpeed.textContent = `${num.toFixed(1)}x`;
  }
  if (window.go && window.go.main && window.go.main.App && window.go.main.App.SetDialParam) {
    window.go.main.App.SetDialParam(key, num);
  }
}

function copyDialTokens() {
  const tokens = {
    pet_scale: petScale,
    roam_speed: roamSpeedFactor,
    pet_skin: currentSkin,
    auto_roaming: isAutoRoamingEnabled,
  };
  navigator.clipboard.writeText(JSON.stringify(tokens, null, 2)).then(() => {
    showBubble("📋 配置 Tokens 已复制到剪贴板！", 2500);
  });
}
if (btnCopyDialTokens) {
  btnCopyDialTokens.addEventListener("click", copyDialTokens);
}

// 快捷键 D / d 唤起调参 HUD
window.addEventListener("keydown", (e) => {
  if ((e.key === "d" || e.key === "D") && !["INPUT", "TEXTAREA"].includes(document.activeElement?.tagName)) {
    toggleDialKit();
  }
});

// ==================== 7.6 Things 3 灵感速记流与模型端侧抽屉 ====================

let localMemos = [
  { id: "1", time: "09:41 · 语音捕获", content: "下午 4 点找架构师对齐全双工打断协议" },
  { id: "2", time: "10:15 · 快捷速记", content: "优化罗小黑 2D 贴边吸附物理手感与声学分级" },
];

function renderMemos() {
  if (!memoCardsList) return;
  memoCardsList.innerHTML = "";
  if (localMemos.length === 0) {
    memoCardsList.innerHTML = '<div style="font-size:12px;color:#94a3b8;text-align:center;padding:12px 0;">暂无灵感速记，随手记一条吧~</div>';
    return;
  }
  localMemos.forEach((memo) => {
    const card = document.createElement("div");
    card.className = "memo-card-item";
    card.innerHTML = `
      <div class="memo-card-header">
        <span>${memo.time}</span>
      </div>
      <div class="memo-card-content">${memo.content}</div>
      <div class="memo-card-actions">
        <button class="memo-action-btn" data-act="copy">复制</button>
        <button class="memo-action-btn" data-act="archive">归档</button>
      </div>
    `;
    card.querySelector('[data-act="copy"]')?.addEventListener("click", () => {
      navigator.clipboard.writeText(memo.content);
      showBubble("已复制灵感内容到剪贴板！", 2000);
    });
    card.querySelector('[data-act="archive"]')?.addEventListener("click", () => {
      localMemos = localMemos.filter((m) => m.id !== memo.id);
      renderMemos();
      showBubble("已归档该条速记", 1500);
    });
    memoCardsList.appendChild(card);
  });
}

function addQuickMemo(text) {
  if (!text) return;
  const now = new Date();
  const timeStr = `${String(now.getHours()).padStart(2, "0")}:${String(now.getMinutes()).padStart(2, "0")} · 案头速记`;
  localMemos.unshift({
    id: String(Date.now()),
    time: timeStr,
    content: text,
  });
  renderMemos();
  showBubble("💡 灵感已捕获至案头手账！", 2500);
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.AddEpisode(text, ["灵感速记"]);
  }
}

if (btnQuickAddMemo && newMemoInput) {
  btnQuickAddMemo.addEventListener("click", () => {
    addQuickMemo(newMemoInput.value.trim());
    newMemoInput.value = "";
  });
  newMemoInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      addQuickMemo(newMemoInput.value.trim());
      newMemoInput.value = "";
    }
  });
}

function refreshModelConfig() {
  if (window.go && window.go.main && window.go.main.App) {
    if (window.go.main.App.GetDualModelInfo) {
      window.go.main.App.GetDualModelInfo().then((info) => {
        if (!info) return;
        if (cfgEdgeBaseUrl && info.edge_url) cfgEdgeBaseUrl.value = info.edge_url;
        if (cfgEdgeModel && info.edge_model) cfgEdgeModel.value = info.edge_model;
        if (cfgCloudUrl && info.cloud_gateway) cfgCloudUrl.value = info.cloud_gateway;
      });
    }
    if (window.go.main.App.GetCloudConsent) {
      window.go.main.App.GetCloudConsent().then((c) => {
        if (chkCloudConsent) chkCloudConsent.checked = c;
      });
    }
  }
}

if (btnSaveModelConfig) {
  btnSaveModelConfig.addEventListener("click", () => {
    if (window.go && window.go.main && window.go.main.App) {
      if (cfgCloudUrl && window.go.main.App.SetCloudServerURL) {
        window.go.main.App.SetCloudServerURL(cfgCloudUrl.value.trim());
      }
      if (chkCloudConsent && window.go.main.App.SetCloudConsent) {
        window.go.main.App.SetCloudConsent(chkCloudConsent.checked);
      }
    }
    showBubble("✅ 模型与端侧配置已保存！", 2500);
  });
}

if (orbContainer) {
  orbContainer.addEventListener("click", () => {
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.Interrupt();
    }
    applyState("idle");
    showBubble("已重置并待命~", 2000);
  });
}

if (subtitleText) {
  subtitleText.addEventListener("click", () => {
    const text = subtitleText.textContent;
    if (text && text.length > 20) {
      alert(text);
    }
  });
}

if (btnAddEpisode && newEpContent) {
  btnAddEpisode.addEventListener("click", () => {
    const val = newEpContent.value.trim();
    if (val && window.go && window.go.main && window.go.main.App) {
      window.go.main.App.AddEpisode(val, ["备忘"]).then(() => {
        newEpContent.value = "";
        refreshMemory();
      });
    }
  });
  newEpContent.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      btnAddEpisode.click();
    }
  });
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

const defaultPersonas = {
  active_slot_id: "whale",
  auto_schedule: true,
  slots: [
    { id: "whale", name: "大肥鱼 (鲸鱼娘)", badge: "🐋", tone_description: "温和开朗、爱吃白饭，像好朋友闲聊" },
    { id: "companion", name: "知心伴侣", badge: "🌙", tone_description: "温柔共情、松弛随性，关注对方情绪" },
    { id: "jarvis", name: "高效幕僚", badge: "⚡", tone_description: "理性严谨、结果导向、条理分明" },
    { id: "coder", name: "极客搭子", badge: "💻", tone_description: "直切要点、代码精炼、极客美学" },
  ]
};

function renderPersonaList(data) {
  chkAutoSchedule.checked = data.auto_schedule;
  slotList.innerHTML = "";
  personaSubmenu.innerHTML = "";

  data.slots.forEach((s) => {
    const isAct = s.id === data.active_slot_id;
    const card = document.createElement("div");
    card.className = `slot-card ${isAct ? "active" : ""}`;
    card.innerHTML = `
      <div class="slot-left">
        <span class="slot-badge">${s.badge || "🎭"}</span>
        <div style="min-width:0;">
          <div class="slot-name">${s.name}</div>
          <div class="slot-desc">${s.tone_description || ""}</div>
        </div>
      </div>
      <button class="badge-btn">${isAct ? "当前激活" : "启用"}</button>
    `;
    card.addEventListener("click", () => {
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SetActivePersona(s.id).then(() => refreshPersonas());
      } else {
        data.active_slot_id = s.id;
        renderPersonaList(data);
      }
    });
    slotList.appendChild(card);

    const item = document.createElement("div");
    item.className = "menu-item";
    item.textContent = `${isAct ? "✓ " : ""}${s.badge || ""} ${s.name}`;
    item.addEventListener("click", (e) => {
      e.stopPropagation();
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SetActivePersona(s.id).then(() => refreshPersonas());
      } else {
        data.active_slot_id = s.id;
        renderPersonaList(data);
      }
      closeContextMenu();
    });
    personaSubmenu.appendChild(item);
  });
}

function refreshPersonas() {
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.GetPersonaData().then((data) => {
      if (data && data.slots && data.slots.length > 0) {
        renderPersonaList(data);
      } else {
        renderPersonaList(defaultPersonas);
      }
    }).catch(() => renderPersonaList(defaultPersonas));
  } else {
    renderPersonaList(defaultPersonas);
  }
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
  if (bondStreakHeader) {
    bondStreakHeader.textContent = `连续陪伴 ${bond.streak_days} 天`;
  }

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
        subtitleText.textContent = "正在倾听，松开空格发送...";
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

      clearTimeout(pttWatchdogTimer);
      pttWatchdogTimer = setTimeout(() => {
        if (subtitleText.textContent.includes("正在识别")) {
          showBubble("👂 刚才好像没听清，请按住空格再说一次哦~", 3000);
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
    showBubble("👂 " + text, 3000);
    applyState("listening");
  });
  window.runtime.EventsOn("avatar:speech_final", (text) => {
    clearTimeout(pttWatchdogTimer);
    resetSleepTimer();
    subtitleText.textContent = "你说: " + text;
    subtitleText.title = text;
    showBubble("你说: " + text, 5000);
    applyState("thinking");
  });
  window.runtime.EventsOn("avatar:mic_energy", (energy) => {
    currentRms = energy;
    updateRmsMeter(energy);

    // 模式 B 声学驱动规范：
    // E < 0.05 语间停顿/换气强制归位待机姿势，严禁假唱抽搐
    // 0.05 <= E < 0.45 平滑播放 Row 3 动作
    // E >= 0.45 重音兴奋挥爪
    if (currentState === "speaking") {
      if (energy < 0.05) {
        // 停顿归位呼吸
      } else if (energy >= 0.45) {
        updateAuditTrack("Row 3 (waving-excited)");
      } else {
        updateAuditTrack("Row 3 (waving)");
      }
    }

    if ((currentState === "listening" || currentState === "idle") && energy > 0.05) {
      resetSleepTimer();
    }
  });
  window.runtime.EventsOn("avatar:auto_roaming", (enabled) => {
    isAutoRoamingEnabled = enabled;
    updateRoamingUI();
  });
  window.runtime.EventsOn("avatar:dial_param", (param) => {
    if (param && param.key) {
      if (param.key === "scale") {
        petScale = param.val;
        document.documentElement.style.setProperty("--pet-scale", param.val);
        if (sliderScale) sliderScale.value = param.val;
        if (valScale) valScale.textContent = `${param.val.toFixed(2)}x`;
      } else if (param.key === "speed") {
        roamSpeedFactor = param.val;
        if (sliderSpeed) sliderSpeed.value = param.val;
        if (valSpeed) valSpeed.textContent = `${param.val.toFixed(1)}x`;
      }
    }
  });
  window.runtime.EventsOn("avatar:speech", (sentence) => {
    clearTimeout(thinkingWatchdog);
    resetSleepTimer();
    subtitleText.textContent = sentence;
    subtitleText.title = sentence;
    showBubble(sentence, Math.max(4000, sentence.length * 300));
  });
  window.runtime.EventsOn("avatar:error", (err) => {
    clearTimeout(thinkingWatchdog);
    subtitleText.textContent = "连接异常: " + err;
    subtitleText.title = err;
    showBubble("连接异常: " + err, 5000);
    applyState("error");
    setTimeout(() => {
      if (currentState === "error") {
        applyState("idle");
      }
    }, 3500);
  });
  window.runtime.EventsOn("avatar:display_form", (f) => {
    applyDisplayForm(f);
  });
  window.runtime.EventsOn("avatar:pet_skin", (s) => {
    applySkin(s, false);
  });
  window.runtime.EventsOn("avatar:mode", (m) => {
    currentMode = m;
    modeBadge.textContent = m === "hands_free" ? "免唤醒" : "快捷键";
    const desc = m === "hands_free" ? "免唤醒模式 (随时开口)" : "按住空格说话模式";
    showBubble(desc, 2500);
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
  window.runtime.EventsOn("avatar:pet_action", (act) => {
    if (!act) return;
    resetSleepTimer();

    // 1. 漫游与空间位移联动判定
    let roamStarted = false;
    if (act.move === "right") {
      roamStarted = triggerDirectedRoam(42);
    } else if (act.move === "left") {
      roamStarted = triggerDirectedRoam(-42);
    } else if (act.action === "roam") {
      const patrolPath = roamCurrentOffset > 0 ? [-42, 42, 0] : [42, -42, 0];
      roamStarted = startRoamSequence(patrolPath, "🐾 巡逻奔跑中...");
    }

    // 2. 触发对应的动画轨道 (若未启动奔跑漫游，则回退为瞬时 Cue 动画)
    const cue = act.cue || "jumping";
    if (!roamStarted) {
      petRenderer.triggerCue(cue, 1400);
    }

    // 3. 特效与气泡
    const xpText = act.xp > 0 ? `+${act.xp} XP` : "";
    if (act.action === "feed" || act.action === "rice" || act.action === "turbo") {
      fishFedTotal += 1;
      if (fishFeedCount) fishFeedCount.textContent = `${fishFedTotal} 次`;
      petRenderer.spawnHearts(100, 70, `🐟 ${xpText}`);
    } else if (act.action === "wave") {
      petRenderer.spawnHearts(100, 70, `🐾 ${xpText}`);
    } else if (act.action === "jump") {
      petRenderer.spawnHearts(100, 70, `✨ ${xpText}`);
    } else if (act.action === "grass") {
      petRenderer.spawnHearts(100, 70, `🌾 ${xpText}`);
    } else if (act.action === "sleep") {
      stopRoamingMove();
      applyState("sleep");
    } else if (act.action === "sit") {
      stopRoamingMove();
      if (desktopPet) {
        desktopPet.style.transition = "transform 0.25s cubic-bezier(0.25, 1, 0.5, 1)";
        desktopPet.style.transform = "translateY(8px) scale(1.04, 0.94)";
        setTimeout(() => {
          if (desktopPet) {
            desktopPet.style.transform = "";
            setTimeout(() => {
              if (desktopPet) desktopPet.style.transition = "";
            }, 300);
          }
        }, 1400);
      }
      petRenderer.spawnHearts(100, 70, `🐾 ${xpText}`);
      showRoamBubble("🐾 乖乖蹲坐好啦...", 2500);
    } else {
      petRenderer.spawnHearts(100, 70, `❤️ ${xpText}`);
    }

    if (act.message) {
      showBubble(act.message, 3000);
    }

    // 4. 羁绊数值同步
    if (act.bond) {
      updateBondUI(act.bond);
    }
    if (!roamStarted) {
      updateAuditTrack(`Voice Action: ${act.action} -> ${cue}`);
    }
  });
}

// ==================== 13. DOM 就绪自启 ====================

function initApp() {
  // 获取后端保存的形态、皮肤与自主漫游配置
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.GetDisplayForm().then((form) => applyDisplayForm(form));
    window.go.main.App.GetPetSkin().then((skin) => applySkin(skin, false));
    window.go.main.App.GetMode().then((m) => {
      currentMode = m;
      modeBadge.textContent = m === "hands_free" ? "免唤醒" : "快捷键";
      if (currentState === "idle") {
        subtitleText.textContent = m === "hands_free" ? "随时开口与我畅聊..." : "按住空格说话...";
      }
    });
    if (window.go.main.App.GetAutoRoaming) {
      window.go.main.App.GetAutoRoaming().then((r) => {
        isAutoRoamingEnabled = r;
        updateRoamingUI();
      });
    }
    if (window.go.main.App.GetDialParams) {
      window.go.main.App.GetDialParams().then((dial) => {
        if (dial && dial.scale) {
          updateDialParam("scale", dial.scale);
          if (sliderScale) sliderScale.value = dial.scale;
        }
        if (dial && dial.speed) {
          updateDialParam("speed", dial.speed);
          if (sliderSpeed) sliderSpeed.value = dial.speed;
        }
      });
    }
  } else {
    applyDisplayForm("pet");
    applySkin("luoxiaohei2d", false);
    updateRoamingUI();
  }

  applyState("idle");
  renderMemos();
  initPTTKeyboard();

  setTimeout(() => {
    refreshSkins();
    refreshPersonas();
    refreshBond();
    refreshModelConfig();
  }, 300);
}

if (document.readyState === "loading") {
  window.addEventListener("DOMContentLoaded", initApp);
} else {
  initApp();
}

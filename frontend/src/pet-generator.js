// pet-generator.js - 内置桌宠图集生成器
// 生成符合 Codex Pet 规范的标准 1536×1872 8列×9行 精灵图集

import {
  DEFAULT_COLUMNS,
  DEFAULT_ROWS,
  DEFAULT_FRAME_WIDTH,
  DEFAULT_FRAME_HEIGHT,
} from "./sprite-orb.js";

const sheetCache = new Map();

export function getBuiltinPetSpritesheet(skinId) {
  if (sheetCache.has(skinId)) {
    return sheetCache.get(skinId);
  }

  const canvas = document.createElement("canvas");
  canvas.width = DEFAULT_COLUMNS * DEFAULT_FRAME_WIDTH;   // 1536
  canvas.height = DEFAULT_ROWS * DEFAULT_FRAME_HEIGHT;    // 1872
  const ctx = canvas.getContext("2d");

  renderSpritesheet(ctx, skinId);

  sheetCache.set(skinId, canvas);
  return canvas;
}

function renderSpritesheet(ctx, skinId) {
  for (let row = 0; row < DEFAULT_ROWS; row++) {
    for (let col = 0; col < DEFAULT_COLUMNS; col++) {
      const cx = col * DEFAULT_FRAME_WIDTH + DEFAULT_FRAME_WIDTH / 2;
      const cy = row * DEFAULT_FRAME_HEIGHT + DEFAULT_FRAME_HEIGHT / 2 + 10;
      ctx.save();
      ctx.translate(cx, cy);

      switch (skinId) {
        case "fox":
          drawFoxFrame(ctx, row, col);
          break;
        case "spirit":
          drawSpiritFrame(ctx, row, col);
          break;
        case "cat":
        default:
          drawCatFrame(ctx, row, col);
          break;
      }

      ctx.restore();
    }
  }
}

// ==================== 1. 橘宝喵 (Cat) ====================
function drawCatFrame(ctx, row, col) {
  // 颜色配置
  const bodyColor = "#fb923c"; // 暖橘色
  const bellyColor = "#ffedd5"; // 奶油白
  const stripeColor = "#ea580c"; // 橘红斑纹
  const earInner = "#f472b6"; // 粉色内耳
  const eyeColor = "#10b981"; // 翡翠绿大眼
  const blushColor = "rgba(244, 114, 182, 0.55)";

  let bobY = 0;
  let stretchX = 1;
  let stretchY = 1;
  let eyeState = "open"; // "open", "blink", "happy", "dizzy", "listening"
  let mouthState = "smile"; // "smile", "open", "round", "sad"
  let pawWave = 0;
  let tailAngle = Math.sin(col * 0.8) * 0.25;
  let leanX = 0;
  let blush = false;
  let fxType = null; // "hearts", "sweat", "thought", "wave"

  // 根据行（动作类型）与列计算关键帧动态
  switch (row) {
    case 0: // idle 待机
      bobY = Math.sin((col / 6) * Math.PI * 2) * 2;
      stretchY = 1 + Math.sin((col / 6) * Math.PI * 2) * 0.03;
      stretchX = 1 - Math.sin((col / 6) * Math.PI * 2) * 0.03;
      if (col === 2 || col === 3) eyeState = "blink";
      tailAngle = Math.sin(col * 0.9) * 0.35;
      break;

    case 1: // running-right 向右跑
      bobY = Math.abs(Math.sin((col / 4) * Math.PI)) * -8;
      leanX = 0.15;
      tailAngle = -0.5 + Math.sin(col * 1.2) * 0.2;
      pawWave = Math.sin(col * 0.8) * 8;
      break;

    case 2: // running-left 向左跑
      bobY = Math.abs(Math.sin((col / 4) * Math.PI)) * -8;
      leanX = -0.15;
      tailAngle = 0.5 - Math.sin(col * 1.2) * 0.2;
      pawWave = -Math.sin(col * 0.8) * 8;
      break;

    case 3: // waving 挥手 / 说话
      bobY = Math.sin(col * 1.5) * 3;
      mouthState = col % 2 === 0 ? "open" : "smile";
      pawWave = -18 - (col % 2) * 6;
      tailAngle = 0.3;
      blush = true;
      break;

    case 4: // jumping 开心跳跃
      bobY = -Math.sin((col / 5) * Math.PI) * 22;
      stretchY = col === 0 ? 0.85 : col === 2 ? 1.15 : 1;
      stretchX = col === 0 ? 1.15 : col === 2 ? 0.9 : 1;
      eyeState = "happy";
      mouthState = "open";
      blush = true;
      fxType = "hearts";
      break;

    case 5: // failed 失败/委屈
      bobY = 4;
      eyeState = "dizzy";
      mouthState = "sad";
      fxType = "sweat";
      tailAngle = 0.1;
      break;

    case 6: // waiting 倾听
      bobY = Math.sin((col / 6) * Math.PI * 2) * 1.5;
      leanX = 0.08;
      eyeState = "listening";
      mouthState = "round";
      fxType = "wave";
      tailAngle = Math.sin(col * 0.6) * 0.2;
      break;

    case 7: // running 冲刺
      bobY = Math.abs(Math.sin((col / 3) * Math.PI)) * -10;
      leanX = 0.2;
      tailAngle = -0.6;
      mouthState = "open";
      break;

    case 8: // review 思考
      bobY = Math.sin(col * 0.8) * 2;
      leanX = -0.06;
      eyeState = "thinking";
      mouthState = "round";
      fxType = "thought";
      break;
  }

  // 应用身体形变
  ctx.translate(0, bobY);
  ctx.scale(stretchX, stretchY);
  ctx.rotate(leanX);

  // 1. 尾巴
  ctx.save();
  ctx.translate(28, 22);
  ctx.rotate(tailAngle);
  ctx.beginPath();
  ctx.moveTo(0, 0);
  ctx.quadraticCurveTo(24, -18, 30, -32);
  ctx.quadraticCurveTo(34, -36, 38, -32);
  ctx.quadraticCurveTo(36, -14, 8, 8);
  ctx.fillStyle = bodyColor;
  ctx.fill();
  // 尾尖白边
  ctx.beginPath();
  ctx.arc(33, -31, 7, 0, Math.PI * 2);
  ctx.fillStyle = bellyColor;
  ctx.fill();
  ctx.restore();

  // 2. 猫耳朵 (左右)
  // 左耳
  ctx.beginPath();
  ctx.moveTo(-36, -26);
  ctx.lineTo(-44, -58);
  ctx.lineTo(-18, -36);
  ctx.closePath();
  ctx.fillStyle = bodyColor;
  ctx.fill();
  ctx.beginPath();
  ctx.moveTo(-34, -28);
  ctx.lineTo(-40, -52);
  ctx.lineTo(-22, -36);
  ctx.closePath();
  ctx.fillStyle = earInner;
  ctx.fill();

  // 右耳
  ctx.beginPath();
  ctx.moveTo(36, -26);
  ctx.lineTo(44, -58);
  ctx.lineTo(18, -36);
  ctx.closePath();
  ctx.fillStyle = bodyColor;
  ctx.fill();
  ctx.beginPath();
  ctx.moveTo(34, -28);
  ctx.lineTo(40, -52);
  ctx.lineTo(22, -36);
  ctx.closePath();
  ctx.fillStyle = earInner;
  ctx.fill();

  // 3. 圆滚滚身体
  ctx.beginPath();
  ctx.ellipse(0, 8, 42, 38, 0, 0, Math.PI * 2);
  ctx.fillStyle = bodyColor;
  ctx.fill();

  // 白色大肚皮
  ctx.beginPath();
  ctx.ellipse(0, 16, 26, 22, 0, 0, Math.PI * 2);
  ctx.fillStyle = bellyColor;
  ctx.fill();

  // 额头花纹 (橘红斑纹)
  ctx.fillStyle = stripeColor;
  ctx.beginPath();
  ctx.moveTo(0, -38);
  ctx.lineTo(-4, -28);
  ctx.lineTo(4, -28);
  ctx.closePath();
  ctx.fill();
  ctx.beginPath();
  ctx.moveTo(-10, -34);
  ctx.lineTo(-12, -26);
  ctx.lineTo(-7, -26);
  ctx.closePath();
  ctx.fill();
  ctx.beginPath();
  ctx.moveTo(10, -34);
  ctx.lineTo(12, -26);
  ctx.lineTo(7, -26);
  ctx.closePath();
  ctx.fill();

  // 4. 猫脸与大眼睛
  drawCatEyes(ctx, eyeState, eyeColor);

  // 5. 腮红
  if (blush || eyeState === "happy") {
    ctx.fillStyle = blushColor;
    ctx.beginPath();
    ctx.ellipse(-24, 0, 7, 4, 0, 0, Math.PI * 2);
    ctx.ellipse(24, 0, 7, 4, 0, 0, Math.PI * 2);
    ctx.fill();
  }

  // 6. 鼻子与小嘴
  ctx.fillStyle = "#f472b6";
  ctx.beginPath();
  ctx.moveTo(0, -2);
  ctx.lineTo(-3, -6);
  ctx.lineTo(3, -6);
  ctx.closePath();
  ctx.fill();

  // 嘴巴
  ctx.strokeStyle = "#7c2d12";
  ctx.lineWidth = 2;
  ctx.lineCap = "round";
  if (mouthState === "smile") {
    ctx.beginPath();
    ctx.moveTo(-6, 2);
    ctx.quadraticCurveTo(-3, 6, 0, 2);
    ctx.quadraticCurveTo(3, 6, 6, 2);
    ctx.stroke();
  } else if (mouthState === "open") {
    ctx.beginPath();
    ctx.ellipse(0, 5, 5, 6, 0, 0, Math.PI * 2);
    ctx.fillStyle = "#dc2626";
    ctx.fill();
    ctx.fillStyle = "#f472b6";
    ctx.beginPath();
    ctx.ellipse(0, 7, 3, 3, 0, 0, Math.PI * 2);
    ctx.fill();
  } else if (mouthState === "round") {
    ctx.beginPath();
    ctx.arc(0, 4, 3, 0, Math.PI * 2);
    ctx.stroke();
  } else if (mouthState === "sad") {
    ctx.beginPath();
    ctx.arc(0, 7, 5, Math.PI, Math.PI * 2);
    ctx.stroke();
  }

  // 7. 胡须 (左右各两条)
  ctx.strokeStyle = "#fed7aa";
  ctx.lineWidth = 1.5;
  // 左胡须
  ctx.beginPath();
  ctx.moveTo(-20, -1);
  ctx.lineTo(-38, -4);
  ctx.moveTo(-20, 3);
  ctx.lineTo(-36, 6);
  // 右胡须
  ctx.moveTo(20, -1);
  ctx.lineTo(38, -4);
  ctx.moveTo(20, 3);
  ctx.lineTo(36, 6);
  ctx.stroke();

  // 8. 爪子
  ctx.fillStyle = bellyColor;
  // 左爪
  ctx.beginPath();
  ctx.ellipse(-18, 28 + (row === 3 ? pawWave : 0), 8, 7, 0, 0, Math.PI * 2);
  ctx.fill();
  // 右爪
  ctx.beginPath();
  ctx.ellipse(18, 28, 8, 7, 0, 0, Math.PI * 2);
  ctx.fill();

  // 9. 特效装饰
  if (fxType === "hearts") {
    drawHeart(ctx, -28, -42, 6, "#f43f5e");
    drawHeart(ctx, 28, -46, 8, "#fb7185");
  } else if (fxType === "sweat") {
    drawSweatDrop(ctx, 30, -32);
  } else if (fxType === "wave") {
    drawSoundWaves(ctx, -40, -10, col);
    drawSoundWaves(ctx, 40, -10, col);
  } else if (fxType === "thought") {
    drawThoughtBubble(ctx, 28, -48, col);
  }
}

function drawCatEyes(ctx, state, eyeColor) {
  if (state === "blink") {
    ctx.strokeStyle = "#431407";
    ctx.lineWidth = 2.5;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(-22, -10);
    ctx.quadraticCurveTo(-15, -6, -8, -10);
    ctx.moveTo(8, -10);
    ctx.quadraticCurveTo(15, -6, 22, -10);
    ctx.stroke();
  } else if (state === "happy") {
    ctx.strokeStyle = "#431407";
    ctx.lineWidth = 3;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(-22, -8);
    ctx.quadraticCurveTo(-15, -16, -8, -8);
    ctx.moveTo(8, -8);
    ctx.quadraticCurveTo(15, -16, 22, -8);
    ctx.stroke();
  } else if (state === "dizzy") {
    ctx.strokeStyle = "#431407";
    ctx.lineWidth = 2;
    drawSpiral(ctx, -15, -10, 6);
    drawSpiral(ctx, 15, -10, 6);
  } else if (state === "thinking") {
    // 向上看
    drawSingleEye(ctx, -15, -12, eyeColor, 0, -3);
    drawSingleEye(ctx, 15, -12, eyeColor, 0, -3);
  } else if (state === "listening") {
    // 瞪大眼
    drawSingleEye(ctx, -15, -10, eyeColor, 0, 0, 1.15);
    drawSingleEye(ctx, 15, -10, eyeColor, 0, 0, 1.15);
  } else {
    // 正常眼睛
    drawSingleEye(ctx, -15, -10, eyeColor, 0, 0);
    drawSingleEye(ctx, 15, -10, eyeColor, 0, 0);
  }
}

function drawSingleEye(ctx, x, y, irisColor, pupilOffX = 0, pupilOffY = 0, scale = 1) {
  ctx.save();
  ctx.translate(x, y);
  ctx.scale(scale, scale);

  // 眼白
  ctx.beginPath();
  ctx.ellipse(0, 0, 8, 10, 0, 0, Math.PI * 2);
  ctx.fillStyle = "#ffffff";
  ctx.fill();

  // 虹膜
  ctx.beginPath();
  ctx.ellipse(pupilOffX, pupilOffY, 6, 8, 0, 0, Math.PI * 2);
  ctx.fillStyle = irisColor;
  ctx.fill();

  // 瞳孔
  ctx.beginPath();
  ctx.ellipse(pupilOffX, pupilOffY, 3, 5, 0, 0, Math.PI * 2);
  ctx.fillStyle = "#0f172a";
  ctx.fill();

  // 高光 (双高光更生动)
  ctx.beginPath();
  ctx.arc(pupilOffX - 2, pupilOffY - 3, 2.5, 0, Math.PI * 2);
  ctx.fillStyle = "#ffffff";
  ctx.fill();
  ctx.beginPath();
  ctx.arc(pupilOffX + 2, pupilOffY + 2, 1.2, 0, Math.PI * 2);
  ctx.fill();

  ctx.restore();
}

// ==================== 2. 灵狐小九 (Fox) ====================
function drawFoxFrame(ctx, row, col) {
  const foxRed = "#ea580c";
  const foxWhite = "#fff7ed";
  const earBlack = "#1e293b";
  const goldEye = "#f59e0b";

  let bobY = Math.sin((col / 6) * Math.PI * 2) * 2;
  if (row === 4) bobY = -Math.sin((col / 5) * Math.PI) * 20;

  ctx.translate(0, bobY);

  // 蓬松狐狸大尾巴
  ctx.save();
  ctx.translate(26, 18);
  ctx.rotate(0.3 + Math.sin(col * 0.7) * 0.25);
  ctx.beginPath();
  ctx.ellipse(18, -12, 18, 30, 0.4, 0, Math.PI * 2);
  ctx.fillStyle = foxRed;
  ctx.fill();
  // 尾尖白毛
  ctx.beginPath();
  ctx.ellipse(26, -32, 10, 12, 0.4, 0, Math.PI * 2);
  ctx.fillStyle = foxWhite;
  ctx.fill();
  ctx.restore();

  // 尖尖的大狐狸耳
  // 左耳
  ctx.beginPath();
  ctx.moveTo(-34, -22);
  ctx.lineTo(-44, -62);
  ctx.lineTo(-14, -34);
  ctx.closePath();
  ctx.fillStyle = foxRed;
  ctx.fill();
  ctx.beginPath();
  ctx.moveTo(-38, -50);
  ctx.lineTo(-44, -62);
  ctx.lineTo(-30, -56);
  ctx.closePath();
  ctx.fillStyle = earBlack;
  ctx.fill();
  ctx.beginPath();
  ctx.moveTo(-30, -26);
  ctx.lineTo(-38, -48);
  ctx.lineTo(-18, -34);
  ctx.closePath();
  ctx.fillStyle = foxWhite;
  ctx.fill();

  // 右耳
  ctx.beginPath();
  ctx.moveTo(34, -22);
  ctx.lineTo(44, -62);
  ctx.lineTo(14, -34);
  ctx.closePath();
  ctx.fillStyle = foxRed;
  ctx.fill();
  ctx.beginPath();
  ctx.moveTo(38, -50);
  ctx.lineTo(44, -62);
  ctx.lineTo(30, -56);
  ctx.closePath();
  ctx.fillStyle = earBlack;
  ctx.fill();
  ctx.beginPath();
  ctx.moveTo(30, -26);
  ctx.lineTo(38, -48);
  ctx.lineTo(18, -34);
  ctx.closePath();
  ctx.fillStyle = foxWhite;
  ctx.fill();

  // 身体与脸颊
  ctx.beginPath();
  ctx.ellipse(0, 8, 38, 36, 0, 0, Math.PI * 2);
  ctx.fillStyle = foxRed;
  ctx.fill();

  // 白狐脸颊毛
  ctx.beginPath();
  ctx.moveTo(0, 0);
  ctx.quadraticCurveTo(-26, 12, -36, 6);
  ctx.quadraticCurveTo(-38, 22, -22, 28);
  ctx.quadraticCurveTo(0, 34, 22, 28);
  ctx.quadraticCurveTo(38, 22, 36, 6);
  ctx.quadraticCurveTo(26, 12, 0, 0);
  ctx.fillStyle = foxWhite;
  ctx.fill();

  // 眼睛
  drawSingleEye(ctx, -14, -8, goldEye);
  drawSingleEye(ctx, 14, -8, goldEye);

  // 黑色小巧鼻子
  ctx.fillStyle = "#0f172a";
  ctx.beginPath();
  ctx.arc(0, 8, 3.5, 0, Math.PI * 2);
  ctx.fill();

  // 爪爪
  ctx.fillStyle = foxBlackPaws();
  ctx.beginPath();
  ctx.ellipse(-16, 28, 7, 6, 0, 0, Math.PI * 2);
  ctx.ellipse(16, 28, 7, 6, 0, 0, Math.PI * 2);
  ctx.fill();
}

function foxBlackPaws() {
  return "#1e293b";
}

// ==================== 3. 星灵露露 (Spirit) ====================
function drawSpiritFrame(ctx, row, col) {
  const glowGrad = ctx.createRadialGradient(0, 0, 8, 0, 0, 48);
  glowGrad.addColorStop(0, "rgba(56, 189, 248, 0.95)");
  glowGrad.addColorStop(0.6, "rgba(168, 85, 247, 0.85)");
  glowGrad.addColorStop(1, "rgba(56, 189, 248, 0.2)");

  let jiggle = Math.sin((col / 4) * Math.PI * 2);
  let bobY = Math.sin((col / 6) * Math.PI * 2) * 4;
  if (row === 4) bobY = -Math.sin((col / 5) * Math.PI) * 25;

  ctx.translate(0, bobY);

  // 光环
  ctx.beginPath();
  ctx.arc(0, 0, 44 + jiggle * 2, 0, Math.PI * 2);
  ctx.fillStyle = "rgba(168, 85, 247, 0.15)";
  ctx.fill();

  // 水滴胶质身体
  ctx.beginPath();
  ctx.moveTo(0, -38);
  ctx.bezierCurveTo(34, -20, 42 + jiggle * 3, 24, 0, 36);
  ctx.bezierCurveTo(-42 - jiggle * 3, 24, -34, -20, 0, -38);
  ctx.fillStyle = glowGrad;
  ctx.fill();

  // 头顶发光小星星
  drawStar(ctx, 0, -42, 5, 8, 4, "#fef08a");

  // 治愈系眨眼与可爱表情
  ctx.fillStyle = "#ffffff";
  if (row === 5) {
    // dizzy
    drawSpiral(ctx, -12, 0, 4);
    drawSpiral(ctx, 12, 0, 4);
  } else {
    ctx.beginPath();
    ctx.ellipse(-12, 0, 4, 6, 0, 0, Math.PI * 2);
    ctx.ellipse(12, 0, 4, 6, 0, 0, Math.PI * 2);
    ctx.fill();
  }

  // 腮红
  ctx.fillStyle = "rgba(244, 114, 182, 0.6)";
  ctx.beginPath();
  ctx.arc(-18, 8, 4, 0, Math.PI * 2);
  ctx.arc(18, 8, 4, 0, Math.PI * 2);
  ctx.fill();

  // 环绕发光粒子
  for (let i = 0; i < 3; i++) {
    const angle = (col * 0.4) + (i * Math.PI * 2) / 3;
    const px = Math.cos(angle) * 32;
    const py = Math.sin(angle) * 22;
    ctx.beginPath();
    ctx.arc(px, py, 2.5, 0, Math.PI * 2);
    ctx.fillStyle = "#38bdf8";
    ctx.fill();
  }
}

// 辅助绘图工具函数
function drawHeart(ctx, x, y, size, color) {
  ctx.save();
  ctx.translate(x, y);
  ctx.beginPath();
  ctx.moveTo(0, size / 4);
  ctx.quadraticCurveTo(0, -size / 2, size / 2, -size / 2);
  ctx.quadraticCurveTo(size, -size / 2, size, 0);
  ctx.quadraticCurveTo(size, size / 2, 0, size);
  ctx.quadraticCurveTo(-size, size / 2, -size, 0);
  ctx.quadraticCurveTo(-size, -size / 2, -size / 2, -size / 2);
  ctx.quadraticCurveTo(0, -size / 2, 0, size / 4);
  ctx.fillStyle = color;
  ctx.fill();
  ctx.restore();
}

function drawSweatDrop(ctx, x, y) {
  ctx.save();
  ctx.translate(x, y);
  ctx.beginPath();
  ctx.moveTo(0, -8);
  ctx.quadraticCurveTo(6, 0, 4, 6);
  ctx.arc(0, 6, 4, 0, Math.PI);
  ctx.quadraticCurveTo(-6, 0, 0, -8);
  ctx.fillStyle = "#38bdf8";
  ctx.fill();
  ctx.restore();
}

function drawSoundWaves(ctx, x, y, col) {
  ctx.save();
  ctx.translate(x, y);
  ctx.strokeStyle = "rgba(56, 189, 248, 0.7)";
  ctx.lineWidth = 1.8;
  ctx.beginPath();
  ctx.arc(0, 0, 8 + (col % 3) * 3, -Math.PI / 3, Math.PI / 3);
  ctx.stroke();
  ctx.restore();
}

function drawThoughtBubble(ctx, x, y, col) {
  ctx.save();
  ctx.translate(x, y);
  ctx.fillStyle = "rgba(255, 255, 255, 0.85)";
  ctx.beginPath();
  ctx.arc(-8, 6, 2, 0, Math.PI * 2);
  ctx.arc(-4, 0, 3.5, 0, Math.PI * 2);
  ctx.arc(4, -8, 6, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
}

function drawSpiral(ctx, x, y, radius) {
  ctx.beginPath();
  for (let i = 0; i < 20; i++) {
    const angle = 0.5 * i;
    const r = (radius / 20) * i;
    const px = x + Math.cos(angle) * r;
    const py = y + Math.sin(angle) * r;
    if (i === 0) ctx.moveTo(px, py);
    else ctx.lineTo(px, py);
  }
  ctx.stroke();
}

function drawStar(ctx, cx, cy, spikes, outerRadius, innerRadius, color) {
  let rot = (Math.PI / 2) * 3;
  let x = cx;
  let y = cy;
  const step = Math.PI / spikes;

  ctx.beginPath();
  ctx.moveTo(cx, cy - outerRadius);
  for (let i = 0; i < spikes; i++) {
    x = cx + Math.cos(rot) * outerRadius;
    y = cy + Math.sin(rot) * outerRadius;
    ctx.lineTo(x, y);
    rot += step;

    x = cx + Math.cos(rot) * innerRadius;
    y = cy + Math.sin(rot) * innerRadius;
    ctx.lineTo(x, y);
    rot += step;
  }
  ctx.lineTo(cx, cy - outerRadius);
  ctx.closePath();
  ctx.fillStyle = color;
  ctx.fill();
}

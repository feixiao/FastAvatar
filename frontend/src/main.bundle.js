(() => {
  // frontend/src/sprite-orb.js
  var DEFAULT_COLUMNS = 8;
  var DEFAULT_FRAME_WIDTH = 192;
  var DEFAULT_FRAME_HEIGHT = 208;
  var DEFAULT_ROWS = 9;
  var V2_ROWS = 11;
  var DEFAULT_FPS = 8;
  var MAX_FPS = 60;
  function idleAnimation() {
    const durations = [1680, 660, 660, 840, 840, 1920];
    return {
      frames: durations.map((durationMs, spriteIndex) => ({
        spriteIndex,
        durationMs
      })),
      loopStart: 0,
      fallback: "idle"
    };
  }
  function rowAnimation(rowIndex, frameCount, frameDurationMs, finalFrameDurationMs) {
    return {
      frames: Array.from({ length: frameCount }, (_, column) => ({
        spriteIndex: rowIndex * DEFAULT_COLUMNS + column,
        durationMs: column === frameCount - 1 ? finalFrameDurationMs : frameDurationMs
      })),
      loopStart: null,
      fallback: "idle"
    };
  }
  function defaultAnimations() {
    return {
      idle: idleAnimation(),
      "running-right": rowAnimation(1, 8, 120, 220),
      "running-left": rowAnimation(2, 8, 120, 220),
      waving: rowAnimation(3, 4, 140, 280),
      jumping: rowAnimation(4, 5, 140, 280),
      failed: rowAnimation(5, 8, 140, 240),
      waiting: rowAnimation(6, 6, 150, 260),
      running: rowAnimation(7, 6, 120, 220),
      review: rowAnimation(8, 6, 150, 280)
    };
  }
  var PET_STATE_ANIMATION_POLICY = Object.freeze({
    idle: { name: "idle", mode: "loop" },
    sleep: { name: "idle", mode: "loop" },
    listening: { name: "waiting", mode: "loop", blocksCues: true },
    thinking: { name: "review", mode: "loop", blocksCues: true },
    speaking: { name: "waving", mode: "loop", blocksCues: true },
    error: { name: "failed", mode: "once", blocksCues: true }
  });
  var PET_EVENT_ANIMATION_POLICY = Object.freeze({
    hover: "jumping",
    click: "jumping",
    wake: "jumping",
    pet: "jumping",
    feed: "jumping"
  });
  function spritePlaybackSelection({
    state,
    baseWorking = false,
    dragDirection = "",
    cue = null
  } = {}) {
    const fallback = baseWorking || state === "working" ? "running" : "idle";
    const statePolicy = PET_STATE_ANIMATION_POLICY[state] || { name: fallback, mode: "loop" };
    if (dragDirection === "left" || dragDirection === "right") {
      return {
        name: `running-${dragDirection}`,
        key: `drag:${dragDirection}`,
        loop: true,
        completion: "none",
        fallback
      };
    }
    if (cue?.id && cue.name && (!statePolicy.blocksCues || cue.name === statePolicy.name)) {
      return {
        name: cue.name,
        key: `cue:${cue.id}`,
        loop: false,
        completion: "cue",
        completionId: cue.id,
        fallback
      };
    }
    return {
      name: statePolicy.name,
      key: `state:${state}:${statePolicy.name}`,
      loop: statePolicy.mode === "loop",
      completion: "none",
      completionId: null,
      fallback
    };
  }
  function spriteGeometry(manifest = {}) {
    const version = manifest.spriteVersionNumber ?? 1;
    const frame = manifest.frame && typeof manifest.frame === "object" ? manifest.frame : {
      width: DEFAULT_FRAME_WIDTH,
      height: DEFAULT_FRAME_HEIGHT,
      columns: DEFAULT_COLUMNS,
      rows: version === 2 ? V2_ROWS : DEFAULT_ROWS
    };
    for (const key of ["width", "height", "columns", "rows"]) {
      if (!Number.isInteger(frame[key]) || frame[key] <= 0) return null;
    }
    return {
      width: frame.width,
      height: frame.height,
      columns: frame.columns,
      rows: frame.rows,
      frameCount: frame.columns * frame.rows
    };
  }
  function frameRect(geometry, spriteIndex) {
    return {
      x: spriteIndex % geometry.columns * geometry.width,
      y: Math.floor(spriteIndex / geometry.columns) * geometry.height,
      width: geometry.width,
      height: geometry.height
    };
  }
  function resolveAnimations(manifest = {}, frameCount = 0) {
    const animations = defaultAnimations();
    const specs = manifest.animations;
    if (specs && typeof specs === "object" && !Array.isArray(specs)) {
      for (const [name, spec] of Object.entries(specs)) {
        if (!spec || !Array.isArray(spec.frames) || spec.frames.length === 0) {
          continue;
        }
        const fps = spec.fps === void 0 ? DEFAULT_FPS : spec.fps;
        if (!Number.isFinite(fps) || fps <= 0 || fps > MAX_FPS) {
          continue;
        }
        const durationMs = 1e3 / fps;
        animations[name] = {
          frames: spec.frames.map((spriteIndex) => ({ spriteIndex, durationMs })),
          loopStart: name === "idle" ? 0 : null,
          fallback: "idle"
        };
      }
    }
    return animations;
  }
  function totalDuration(animation) {
    return animation.frames.reduce((sum, frame) => sum + frame.durationMs, 0);
  }
  function frameAtElapsed(animation, elapsedMs) {
    const total = totalDuration(animation);
    if (total <= 0) return null;
    let position = Math.max(0, elapsedMs);
    if (position >= total) {
      if (animation.loopStart === null) return null;
      const introDuration = animation.frames.slice(0, animation.loopStart).reduce((sum, frame) => sum + frame.durationMs, 0);
      const loopDuration = total - introDuration;
      if (loopDuration <= 0) return null;
      position = introDuration + (position - total) % loopDuration;
    }
    let cursor = 0;
    for (const frame of animation.frames) {
      if (position < cursor + frame.durationMs) {
        return {
          spriteIndex: frame.spriteIndex,
          remainingMs: cursor + frame.durationMs - position
        };
      }
      cursor += frame.durationMs;
    }
    return null;
  }

  // frontend/src/pet-generator.js
  var sheetCache = /* @__PURE__ */ new Map();
  function getBuiltinPetSpritesheet(skinId) {
    if (sheetCache.has(skinId)) {
      return sheetCache.get(skinId);
    }
    const canvas = document.createElement("canvas");
    canvas.width = DEFAULT_COLUMNS * DEFAULT_FRAME_WIDTH;
    canvas.height = DEFAULT_ROWS * DEFAULT_FRAME_HEIGHT;
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
          case "whale":
            drawWhaleFrame(ctx, row, col);
            break;
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
  function drawWhaleFrame(ctx, row, col) {
    const whaleBlue = "#2563eb";
    const whaleDark = "#1e3a8a";
    const whaleBelly = "#f8fafc";
    const skinColor = "#ffe4d6";
    const eyeColor = "#0284c7";
    const blushColor = "rgba(244, 114, 182, 0.65)";
    const waterColor = "#38bdf8";
    let bobY = 0;
    let stretchX = 1;
    let stretchY = 1;
    let eyeState = "open";
    let mouthState = "smile";
    let finWave = 0;
    let tailAngle = Math.sin(col * 0.8) * 0.25;
    let leanX = 0;
    let blush = true;
    let fxType = null;
    let spoutHeight = 6;
    switch (row) {
      case 0:
        bobY = Math.sin(col / 6 * Math.PI * 2) * 2.5;
        stretchY = 1 + Math.sin(col / 6 * Math.PI * 2) * 0.03;
        stretchX = 1 - Math.sin(col / 6 * Math.PI * 2) * 0.03;
        if (col === 2 || col === 3) eyeState = "blink";
        tailAngle = Math.sin(col * 0.9) * 0.35;
        spoutHeight = 6 + Math.sin(col * 0.8) * 3;
        break;
      case 1:
        bobY = Math.abs(Math.sin(col / 4 * Math.PI)) * -6;
        leanX = 0.12;
        tailAngle = -0.4 + Math.sin(col * 1.2) * 0.25;
        finWave = Math.sin(col * 0.8) * 8;
        break;
      case 2:
        bobY = Math.abs(Math.sin(col / 4 * Math.PI)) * -6;
        leanX = -0.12;
        tailAngle = 0.4 - Math.sin(col * 1.2) * 0.25;
        finWave = -Math.sin(col * 0.8) * 8;
        break;
      case 3:
        bobY = Math.sin(col * 1.5) * 3;
        mouthState = col % 2 === 0 ? "open" : "smile";
        finWave = -14 - col % 2 * 6;
        tailAngle = 0.25;
        spoutHeight = 8 + col % 2 * 4;
        break;
      case 4:
        bobY = -Math.sin(col / 5 * Math.PI) * 22;
        stretchY = col === 0 ? 0.85 : col === 2 ? 1.15 : 1;
        stretchX = col === 0 ? 1.15 : col === 2 ? 0.9 : 1;
        eyeState = "happy";
        mouthState = "open";
        fxType = "hearts";
        spoutHeight = 12;
        break;
      case 5:
        bobY = 3;
        eyeState = "dizzy";
        mouthState = "sad";
        fxType = "sweat";
        tailAngle = 0.1;
        break;
      case 6:
        bobY = Math.sin(col / 6 * Math.PI * 2) * 1.2;
        leanX = 0.06;
        eyeState = "listening";
        mouthState = "closed";
        fxType = "wave";
        tailAngle = Math.sin(col * 0.5) * 0.15;
        break;
      case 7:
        bobY = Math.abs(Math.sin(col / 3 * Math.PI)) * -8;
        leanX = 0.18;
        tailAngle = -0.5;
        mouthState = "open";
        break;
      case 8:
        bobY = Math.sin(col * 0.8) * 1.8;
        leanX = -0.05;
        eyeState = "thinking";
        mouthState = "closed";
        fxType = "thought";
        break;
    }
    ctx.translate(0, bobY);
    ctx.scale(stretchX, stretchY);
    ctx.rotate(leanX);
    ctx.save();
    ctx.translate(0, 36);
    ctx.rotate(tailAngle);
    ctx.fillStyle = whaleBlue;
    ctx.beginPath();
    ctx.moveTo(-10, 0);
    ctx.quadraticCurveTo(-24, 18, -32, 28);
    ctx.quadraticCurveTo(-16, 26, 0, 16);
    ctx.quadraticCurveTo(16, 26, 32, 28);
    ctx.quadraticCurveTo(24, 18, 10, 0);
    ctx.closePath();
    ctx.fill();
    ctx.restore();
    ctx.beginPath();
    ctx.ellipse(0, 4, 46, 42, 0, 0, Math.PI * 2);
    ctx.fillStyle = whaleBlue;
    ctx.fill();
    ctx.save();
    ctx.translate(0, -38);
    ctx.fillStyle = waterColor;
    ctx.beginPath();
    ctx.arc(0, -spoutHeight, 3.5, 0, Math.PI * 2);
    ctx.arc(-5, -spoutHeight * 0.7, 2.5, 0, Math.PI * 2);
    ctx.arc(5, -spoutHeight * 0.7, 2.5, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
    ctx.beginPath();
    ctx.ellipse(0, 18, 30, 24, 0, 0, Math.PI * 2);
    ctx.fillStyle = whaleBelly;
    ctx.fill();
    ctx.beginPath();
    ctx.ellipse(0, -4, 32, 26, 0, 0, Math.PI * 2);
    ctx.fillStyle = skinColor;
    ctx.fill();
    ctx.fillStyle = whaleDark;
    ctx.beginPath();
    ctx.moveTo(-30, -10);
    ctx.quadraticCurveTo(-20, -28, 0, -28);
    ctx.quadraticCurveTo(20, -28, 30, -10);
    ctx.quadraticCurveTo(15, -16, 0, -12);
    ctx.quadraticCurveTo(-15, -16, -30, -10);
    ctx.fill();
    drawWhaleEyes(ctx, eyeState, eyeColor);
    if (blush || eyeState === "happy") {
      ctx.fillStyle = blushColor;
      ctx.beginPath();
      ctx.ellipse(-20, 2, 6, 3.5, 0, 0, Math.PI * 2);
      ctx.ellipse(20, 2, 6, 3.5, 0, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.strokeStyle = "#991b1b";
    ctx.lineWidth = 1.8;
    ctx.lineCap = "round";
    if (mouthState === "smile") {
      ctx.beginPath();
      ctx.arc(0, 3, 3.5, 0.2, Math.PI - 0.2);
      ctx.stroke();
    } else if (mouthState === "open") {
      ctx.beginPath();
      ctx.ellipse(0, 5, 4.5, 5.5, 0, 0, Math.PI * 2);
      ctx.fillStyle = "#e11d48";
      ctx.fill();
      ctx.fillStyle = "#fda4af";
      ctx.beginPath();
      ctx.ellipse(0, 7, 3, 2.5, 0, 0, Math.PI * 2);
      ctx.fill();
    } else if (mouthState === "closed") {
      ctx.beginPath();
      ctx.moveTo(-3, 4);
      ctx.lineTo(3, 4);
      ctx.stroke();
    } else if (mouthState === "sad") {
      ctx.beginPath();
      ctx.arc(0, 6, 4, Math.PI * 1.1, Math.PI * 1.9);
      ctx.stroke();
    }
    ctx.fillStyle = whaleBlue;
    ctx.save();
    ctx.translate(-34, 12 + (row === 3 ? finWave : 0));
    ctx.rotate(0.3 + (row === 3 ? -0.4 : 0));
    ctx.beginPath();
    ctx.ellipse(0, 0, 12, 6, 0.2, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
    ctx.save();
    ctx.translate(34, 12);
    ctx.rotate(-0.3);
    ctx.beginPath();
    ctx.ellipse(0, 0, 12, 6, -0.2, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
    if (fxType === "hearts") {
      drawHeart(ctx, -26, -38, 6, "#f43f5e");
      drawHeart(ctx, 26, -42, 8, "#fb7185");
    } else if (fxType === "sweat") {
      drawSweatDrop(ctx, 28, -28);
    } else if (fxType === "wave") {
      drawSoundWaves(ctx, -38, -6, col);
      drawSoundWaves(ctx, 38, -6, col);
    } else if (fxType === "thought") {
      drawThoughtBubble(ctx, 26, -44, col);
    }
  }
  function drawWhaleEyes(ctx, state, eyeColor) {
    if (state === "blink") {
      ctx.strokeStyle = "#1e3a8a";
      ctx.lineWidth = 2.2;
      ctx.lineCap = "round";
      ctx.beginPath();
      ctx.moveTo(-19, -4);
      ctx.quadraticCurveTo(-14, 0, -9, -4);
      ctx.moveTo(9, -4);
      ctx.quadraticCurveTo(14, 0, 19, -4);
      ctx.stroke();
    } else if (state === "happy") {
      ctx.strokeStyle = "#1e3a8a";
      ctx.lineWidth = 2.4;
      ctx.lineCap = "round";
      ctx.beginPath();
      ctx.moveTo(-20, -2);
      ctx.quadraticCurveTo(-14, -8, -8, -2);
      ctx.moveTo(8, -2);
      ctx.quadraticCurveTo(14, -8, 20, -2);
      ctx.stroke();
    } else if (state === "dizzy") {
      ctx.strokeStyle = "#1e3a8a";
      ctx.lineWidth = 1.8;
      drawSpiral(ctx, -14, -4, 4);
      drawSpiral(ctx, 14, -4, 4);
    } else {
      let offX = 0;
      let offY = 0;
      if (state === "thinking") {
        offX = 1;
        offY = -2;
      } else if (state === "listening") {
        offY = 1;
      }
      drawSingleEye(ctx, -14, -4, eyeColor, offX, offY, 1.15);
      drawSingleEye(ctx, 14, -4, eyeColor, offX, offY, 1.15);
    }
  }
  function drawCatFrame(ctx, row, col) {
    const bodyColor = "#fb923c";
    const bellyColor = "#ffedd5";
    const stripeColor = "#ea580c";
    const earInner = "#f472b6";
    const eyeColor = "#10b981";
    const blushColor = "rgba(244, 114, 182, 0.55)";
    let bobY = 0;
    let stretchX = 1;
    let stretchY = 1;
    let eyeState = "open";
    let mouthState = "smile";
    let pawWave = 0;
    let tailAngle = Math.sin(col * 0.8) * 0.25;
    let leanX = 0;
    let blush = false;
    let fxType = null;
    switch (row) {
      case 0:
        bobY = Math.sin(col / 6 * Math.PI * 2) * 2;
        stretchY = 1 + Math.sin(col / 6 * Math.PI * 2) * 0.03;
        stretchX = 1 - Math.sin(col / 6 * Math.PI * 2) * 0.03;
        if (col === 2 || col === 3) eyeState = "blink";
        tailAngle = Math.sin(col * 0.9) * 0.35;
        break;
      case 1:
        bobY = Math.abs(Math.sin(col / 4 * Math.PI)) * -8;
        leanX = 0.15;
        tailAngle = -0.5 + Math.sin(col * 1.2) * 0.2;
        pawWave = Math.sin(col * 0.8) * 8;
        break;
      case 2:
        bobY = Math.abs(Math.sin(col / 4 * Math.PI)) * -8;
        leanX = -0.15;
        tailAngle = 0.5 - Math.sin(col * 1.2) * 0.2;
        pawWave = -Math.sin(col * 0.8) * 8;
        break;
      case 3:
        bobY = Math.sin(col * 1.5) * 3;
        mouthState = col % 2 === 0 ? "open" : "smile";
        pawWave = -18 - col % 2 * 6;
        tailAngle = 0.3;
        blush = true;
        break;
      case 4:
        bobY = -Math.sin(col / 5 * Math.PI) * 22;
        stretchY = col === 0 ? 0.85 : col === 2 ? 1.15 : 1;
        stretchX = col === 0 ? 1.15 : col === 2 ? 0.9 : 1;
        eyeState = "happy";
        mouthState = "open";
        blush = true;
        fxType = "hearts";
        break;
      case 5:
        bobY = 4;
        eyeState = "dizzy";
        mouthState = "sad";
        fxType = "sweat";
        tailAngle = 0.1;
        break;
      case 6:
        bobY = Math.sin(col / 6 * Math.PI * 2) * 1.5;
        leanX = 0.08;
        eyeState = "listening";
        mouthState = "closed";
        fxType = "wave";
        tailAngle = Math.sin(col * 0.6) * 0.2;
        break;
      case 7:
        bobY = Math.abs(Math.sin(col / 3 * Math.PI)) * -10;
        leanX = 0.2;
        tailAngle = -0.6;
        mouthState = "open";
        break;
      case 8:
        bobY = Math.sin(col * 0.8) * 2;
        leanX = -0.06;
        eyeState = "thinking";
        mouthState = "closed";
        fxType = "thought";
        break;
    }
    ctx.translate(0, bobY);
    ctx.scale(stretchX, stretchY);
    ctx.rotate(leanX);
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
    ctx.beginPath();
    ctx.arc(33, -31, 7, 0, Math.PI * 2);
    ctx.fillStyle = bellyColor;
    ctx.fill();
    ctx.restore();
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
    ctx.beginPath();
    ctx.ellipse(0, 8, 42, 38, 0, 0, Math.PI * 2);
    ctx.fillStyle = bodyColor;
    ctx.fill();
    ctx.beginPath();
    ctx.ellipse(0, 16, 26, 22, 0, 0, Math.PI * 2);
    ctx.fillStyle = bellyColor;
    ctx.fill();
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
    drawCatEyes(ctx, eyeState, eyeColor);
    if (blush || eyeState === "happy") {
      ctx.fillStyle = blushColor;
      ctx.beginPath();
      ctx.ellipse(-24, 0, 7, 4, 0, 0, Math.PI * 2);
      ctx.ellipse(24, 0, 7, 4, 0, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.fillStyle = "#f472b6";
    ctx.beginPath();
    ctx.moveTo(0, -2);
    ctx.lineTo(-3, -6);
    ctx.lineTo(3, -6);
    ctx.closePath();
    ctx.fill();
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
    } else if (mouthState === "closed") {
      ctx.beginPath();
      ctx.moveTo(-4, 3);
      ctx.lineTo(4, 3);
      ctx.stroke();
    } else if (mouthState === "round") {
      ctx.beginPath();
      ctx.arc(0, 4, 3, 0, Math.PI * 2);
      ctx.stroke();
    } else if (mouthState === "sad") {
      ctx.beginPath();
      ctx.arc(0, 7, 5, Math.PI, Math.PI * 2);
      ctx.stroke();
    }
    ctx.strokeStyle = "#fed7aa";
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(-20, -1);
    ctx.lineTo(-38, -4);
    ctx.moveTo(-20, 3);
    ctx.lineTo(-36, 6);
    ctx.moveTo(20, -1);
    ctx.lineTo(38, -4);
    ctx.moveTo(20, 3);
    ctx.lineTo(36, 6);
    ctx.stroke();
    ctx.fillStyle = bellyColor;
    ctx.beginPath();
    ctx.ellipse(-18, 28 + (row === 3 ? pawWave : 0), 8, 7, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.beginPath();
    ctx.ellipse(18, 28, 8, 7, 0, 0, Math.PI * 2);
    ctx.fill();
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
      drawSingleEye(ctx, -15, -12, eyeColor, 0, -3);
      drawSingleEye(ctx, 15, -12, eyeColor, 0, -3);
    } else if (state === "listening") {
      drawSingleEye(ctx, -15, -10, eyeColor, 0, 0, 1.15);
      drawSingleEye(ctx, 15, -10, eyeColor, 0, 0, 1.15);
    } else {
      drawSingleEye(ctx, -15, -10, eyeColor, 0, 0);
      drawSingleEye(ctx, 15, -10, eyeColor, 0, 0);
    }
  }
  function drawSingleEye(ctx, x, y, irisColor, pupilOffX = 0, pupilOffY = 0, scale = 1) {
    ctx.save();
    ctx.translate(x, y);
    ctx.scale(scale, scale);
    ctx.beginPath();
    ctx.ellipse(0, 0, 8, 10, 0, 0, Math.PI * 2);
    ctx.fillStyle = "#ffffff";
    ctx.fill();
    ctx.beginPath();
    ctx.ellipse(pupilOffX, pupilOffY, 6, 8, 0, 0, Math.PI * 2);
    ctx.fillStyle = irisColor;
    ctx.fill();
    ctx.beginPath();
    ctx.ellipse(pupilOffX, pupilOffY, 3, 5, 0, 0, Math.PI * 2);
    ctx.fillStyle = "#0f172a";
    ctx.fill();
    ctx.beginPath();
    ctx.arc(pupilOffX - 2, pupilOffY - 3, 2.5, 0, Math.PI * 2);
    ctx.fillStyle = "#ffffff";
    ctx.fill();
    ctx.beginPath();
    ctx.arc(pupilOffX + 2, pupilOffY + 2, 1.2, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
  }
  function drawFoxFrame(ctx, row, col) {
    const foxRed = "#ea580c";
    const foxWhite = "#fff7ed";
    const earBlack = "#1e293b";
    const goldEye = "#f59e0b";
    let bobY = Math.sin(col / 6 * Math.PI * 2) * 2;
    if (row === 4) bobY = -Math.sin(col / 5 * Math.PI) * 20;
    ctx.translate(0, bobY);
    ctx.save();
    ctx.translate(26, 18);
    ctx.rotate(0.3 + Math.sin(col * 0.7) * 0.25);
    ctx.beginPath();
    ctx.ellipse(18, -12, 18, 30, 0.4, 0, Math.PI * 2);
    ctx.fillStyle = foxRed;
    ctx.fill();
    ctx.beginPath();
    ctx.ellipse(26, -32, 10, 12, 0.4, 0, Math.PI * 2);
    ctx.fillStyle = foxWhite;
    ctx.fill();
    ctx.restore();
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
    ctx.beginPath();
    ctx.ellipse(0, 8, 38, 36, 0, 0, Math.PI * 2);
    ctx.fillStyle = foxRed;
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(0, 0);
    ctx.quadraticCurveTo(-26, 12, -36, 6);
    ctx.quadraticCurveTo(-38, 22, -22, 28);
    ctx.quadraticCurveTo(0, 34, 22, 28);
    ctx.quadraticCurveTo(38, 22, 36, 6);
    ctx.quadraticCurveTo(26, 12, 0, 0);
    ctx.fillStyle = foxWhite;
    ctx.fill();
    drawSingleEye(ctx, -14, -8, goldEye);
    drawSingleEye(ctx, 14, -8, goldEye);
    ctx.fillStyle = "#0f172a";
    ctx.beginPath();
    ctx.arc(0, 8, 3.5, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = foxBlackPaws();
    ctx.beginPath();
    ctx.ellipse(-16, 28, 7, 6, 0, 0, Math.PI * 2);
    ctx.ellipse(16, 28, 7, 6, 0, 0, Math.PI * 2);
    ctx.fill();
  }
  function foxBlackPaws() {
    return "#1e293b";
  }
  function drawSpiritFrame(ctx, row, col) {
    const glowGrad = ctx.createRadialGradient(0, 0, 8, 0, 0, 48);
    glowGrad.addColorStop(0, "rgba(56, 189, 248, 0.95)");
    glowGrad.addColorStop(0.6, "rgba(168, 85, 247, 0.85)");
    glowGrad.addColorStop(1, "rgba(56, 189, 248, 0.2)");
    let jiggle = Math.sin(col / 4 * Math.PI * 2);
    let bobY = Math.sin(col / 6 * Math.PI * 2) * 4;
    if (row === 4) bobY = -Math.sin(col / 5 * Math.PI) * 25;
    ctx.translate(0, bobY);
    ctx.beginPath();
    ctx.arc(0, 0, 44 + jiggle * 2, 0, Math.PI * 2);
    ctx.fillStyle = "rgba(168, 85, 247, 0.15)";
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(0, -38);
    ctx.bezierCurveTo(34, -20, 42 + jiggle * 3, 24, 0, 36);
    ctx.bezierCurveTo(-42 - jiggle * 3, 24, -34, -20, 0, -38);
    ctx.fillStyle = glowGrad;
    ctx.fill();
    drawStar(ctx, 0, -42, 5, 8, 4, "#fef08a");
    ctx.fillStyle = "#ffffff";
    if (row === 5) {
      drawSpiral(ctx, -12, 0, 4);
      drawSpiral(ctx, 12, 0, 4);
    } else {
      ctx.beginPath();
      ctx.ellipse(-12, 0, 4, 6, 0, 0, Math.PI * 2);
      ctx.ellipse(12, 0, 4, 6, 0, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.fillStyle = "rgba(244, 114, 182, 0.6)";
    ctx.beginPath();
    ctx.arc(-18, 8, 4, 0, Math.PI * 2);
    ctx.arc(18, 8, 4, 0, Math.PI * 2);
    ctx.fill();
    for (let i = 0; i < 3; i++) {
      const angle = col * 0.4 + i * Math.PI * 2 / 3;
      const px = Math.cos(angle) * 32;
      const py = Math.sin(angle) * 22;
      ctx.beginPath();
      ctx.arc(px, py, 2.5, 0, Math.PI * 2);
      ctx.fillStyle = "#38bdf8";
      ctx.fill();
    }
  }
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
    ctx.arc(0, 0, 8 + col % 3 * 3, -Math.PI / 3, Math.PI / 3);
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
      const r = radius / 20 * i;
      const px = x + Math.cos(angle) * r;
      const py = y + Math.sin(angle) * r;
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.stroke();
  }
  function drawStar(ctx, cx, cy, spikes, outerRadius, innerRadius, color) {
    let rot = Math.PI / 2 * 3;
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

  // frontend/src/pet-renderer.js
  var PetRenderer = class {
    constructor(canvasElement, particleContainer) {
      this.canvas = canvasElement;
      this.ctx = canvasElement.getContext("2d");
      this.particleContainer = particleContainer;
      this.skinId = "cat";
      this.state = "idle";
      this.dragDirection = "";
      this.cue = null;
      this.assets = null;
      this.playback = null;
      this.track = null;
      this.startedAt = performance.now();
      this.timer = null;
      this.isDestroyed = false;
      this.geometry = spriteGeometry({});
      this.animations = defaultAnimations();
      this.loadSkin(this.skinId);
    }
    loadSkin(skinId, customSpritesheetPath = null, customManifest = null) {
      this.skinId = skinId;
      if (customSpritesheetPath && customManifest) {
        const img = new Image();
        img.onload = () => {
          this.geometry = spriteGeometry(customManifest) || spriteGeometry({});
          this.animations = resolveAnimations(customManifest, this.geometry.frameCount);
          this.assets = { image: img, geometry: this.geometry, animations: this.animations };
          this.resetTrack();
        };
        img.onerror = () => {
          this.loadBuiltin(skinId);
        };
        img.src = customSpritesheetPath;
      } else {
        this.loadBuiltin(skinId);
      }
    }
    loadBuiltin(skinId) {
      const sheetCanvas = getBuiltinPetSpritesheet(skinId);
      this.geometry = spriteGeometry({});
      this.animations = defaultAnimations();
      this.assets = {
        image: sheetCanvas,
        geometry: this.geometry,
        animations: this.animations
      };
      this.resetTrack();
    }
    setState(newState) {
      if (this.state === newState) return;
      this.state = newState;
      this.resetTrack();
    }
    setDragDirection(dir) {
      if (this.dragDirection === dir) return;
      this.dragDirection = dir;
      this.resetTrack();
    }
    triggerCue(name, durationMs = 1200) {
      this.cue = {
        id: Date.now() + Math.random(),
        name
      };
      this.resetTrack();
      if (durationMs > 0) {
        setTimeout(() => {
          if (this.cue && this.cue.name === name) {
            this.cue = null;
            this.resetTrack();
          }
        }, durationMs);
      }
    }
    resetTrack() {
      if (!this.assets) return;
      this.playback = spritePlaybackSelection({
        state: this.state,
        dragDirection: this.dragDirection,
        cue: this.cue
      });
      const activePlayback = this.playback;
      const selected = this.assets.animations[activePlayback.name] || this.assets.animations.idle;
      this.track = {
        ...selected,
        loopStart: activePlayback.loop ? 0 : null,
        fallback: activePlayback.fallback || "idle"
      };
      this.startedAt = performance.now();
      if (!this.timer) {
        this.loop();
      }
    }
    loop() {
      if (this.isDestroyed) return;
      if (!this.assets || !this.track) {
        this.timer = requestAnimationFrame(() => this.loop());
        return;
      }
      const elapsed = performance.now() - this.startedAt;
      let frame = frameAtElapsed(this.track, elapsed);
      if (!frame) {
        const fallbackName = this.track.fallback || "idle";
        const fallback = this.assets.animations[fallbackName] || this.assets.animations.idle;
        this.track = {
          ...fallback,
          loopStart: 0,
          fallback: "idle"
        };
        this.cue = null;
        this.startedAt = performance.now();
        frame = frameAtElapsed(this.track, 0);
      }
      if (frame) {
        const source = frameRect(this.assets.geometry, frame.spriteIndex);
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        this.ctx.imageSmoothingEnabled = false;
        this.ctx.drawImage(
          this.assets.image,
          source.x,
          source.y,
          source.width,
          source.height,
          0,
          0,
          this.canvas.width,
          this.canvas.height
        );
      }
      const nextDelay = frame ? Math.max(16, frame.remainingMs) : 60;
      this.timer = setTimeout(() => this.loop(), nextDelay);
    }
    // 触发粒子爱心与 XP 提示特效
    spawnHearts(x = 100, y = 60, xpText = "+5 XP") {
      if (!this.particleContainer) return;
      const icons = ["\u2764\uFE0F", "\u{1F496}", "\u2728", "\u{1F43E}", "\u{1F338}"];
      const count = 5;
      for (let i = 0; i < count; i++) {
        const p = document.createElement("div");
        p.className = "floating-particle";
        p.textContent = icons[Math.floor(Math.random() * icons.length)];
        const offsetX = (Math.random() - 0.5) * 60;
        const offsetY = (Math.random() - 0.5) * 30;
        p.style.left = `${x + offsetX}px`;
        p.style.top = `${y + offsetY}px`;
        this.particleContainer.appendChild(p);
        setTimeout(() => {
          p.remove();
        }, 1e3);
      }
      if (xpText) {
        const badge = document.createElement("div");
        badge.className = "floating-xp-badge";
        badge.textContent = xpText;
        badge.style.left = `${x}px`;
        badge.style.top = `${y - 20}px`;
        this.particleContainer.appendChild(badge);
        setTimeout(() => {
          badge.remove();
        }, 1200);
      }
    }
    destroy() {
      this.isDestroyed = true;
      if (this.timer) {
        clearTimeout(this.timer);
        this.timer = null;
      }
    }
  };

  // frontend/src/main.js
  var desktopPet = document.getElementById("desktopPet");
  var petBubble = document.getElementById("petBubble");
  var petBubbleText = document.getElementById("petBubbleText");
  var petStage = document.getElementById("petStage");
  var petCanvas = document.getElementById("petCanvas");
  var particleLayer = document.getElementById("particleLayer");
  var petStatusPill = document.getElementById("petStatusPill");
  var petStatusDot = document.getElementById("petStatusDot");
  var petStatusText = document.getElementById("petStatusText");
  var petCharacterView = document.getElementById("petCharacterView");
  var whaleCharacterBox = document.getElementById("whaleCharacterBox");
  var whaleBaseImg = document.getElementById("whaleBaseImg");
  var whaleMouth = document.getElementById("whaleMouth");
  var whalePoseContainer = document.getElementById("whalePoseContainer");
  var whalePoseImg = document.getElementById("whalePoseImg");
  var whaleFoxtailLure = document.getElementById("whaleFoxtailLure");
  var petInputBar = document.getElementById("petInputBar");
  var petTextInput = document.getElementById("petTextInput");
  var btnPetSend = document.getElementById("btnPetSend");
  var btnPetMic = document.getElementById("btnPetMic");
  var btnPetChat = document.getElementById("btnPetChat");
  var btnPetFeed = document.getElementById("btnPetFeed");
  var btnPetTouch = document.getElementById("btnPetTouch");
  var btnPetPersona = document.getElementById("btnPetPersona");
  var btnPetToCapsule = document.getElementById("btnPetToCapsule");
  var btnPetDrawer = document.getElementById("btnPetDrawer");
  var btnPetStop = document.getElementById("btnPetStop");
  var btnPetRice = document.getElementById("btnPetRice");
  var btnPetTurbo = document.getElementById("btnPetTurbo");
  var petContextMenu = document.getElementById("petContextMenu");
  var menuPetTitle = document.getElementById("menuPetTitle");
  var menuActionPet = document.getElementById("menuActionPet");
  var menuActionFeed = document.getElementById("menuActionFeed");
  var menuActionChat = document.getElementById("menuActionChat");
  var menuActionRice = document.getElementById("menuActionRice");
  var menuActionTurbo = document.getElementById("menuActionTurbo");
  var menuActionGrass = document.getElementById("menuActionGrass");
  var skinSubmenu = document.getElementById("skinSubmenu");
  var personaSubmenu = document.getElementById("personaSubmenu");
  var menuToggleForm = document.getElementById("menuToggleForm");
  var menuToggleTop = document.getElementById("menuToggleTop");
  var topStatusText = document.getElementById("topStatusText");
  var menuOpenDrawer = document.getElementById("menuOpenDrawer");
  var menuQuitApp = document.getElementById("menuQuitApp");
  var capsule = document.getElementById("capsule");
  var statusIndicator = document.getElementById("statusIndicator");
  var statusText = document.getElementById("statusText");
  var personaBadge = document.getElementById("personaBadge");
  var modeBadge = document.getElementById("modeBadge");
  var subtitleText = document.getElementById("subtitleText");
  var btnSwitchToPet = document.getElementById("btnSwitchToPet");
  var btnMemory = document.getElementById("btnMemory");
  var btnInput = document.getElementById("btnInput");
  var btnInterrupt = document.getElementById("btnInterrupt");
  var inputDrawer = document.getElementById("inputDrawer");
  var textInput = document.getElementById("textInput");
  var manageDrawer = document.getElementById("manageDrawer");
  var btnCloseDrawer = document.getElementById("btnCloseDrawer");
  var tabButtons = document.querySelectorAll(".tab-btn");
  var tabPanes = document.querySelectorAll(".tab-pane");
  var btnFormPet = document.getElementById("btnFormPet");
  var btnFormCapsule = document.getElementById("btnFormCapsule");
  var skinGrid = document.getElementById("skinGrid");
  var txtSkinImportPath = document.getElementById("txtSkinImportPath");
  var btnDoImportSkin = document.getElementById("btnDoImportSkin");
  var profileList = document.getElementById("profileList");
  var newProfKey = document.getElementById("newProfKey");
  var newProfVal = document.getElementById("newProfVal");
  var btnAddProfile = document.getElementById("btnAddProfile");
  var episodeList = document.getElementById("episodeList");
  var slotList = document.getElementById("slotList");
  var chkAutoSchedule = document.getElementById("chkAutoSchedule");
  var bondLevelText = document.getElementById("bondLevelText");
  var bondStreakText = document.getElementById("bondStreakText");
  var xpBarFill = document.getElementById("xpBarFill");
  var xpLabelText = document.getElementById("xpLabelText");
  var currentForm = "pet";
  var currentSkin = "whale";
  var currentMode = "hands_free";
  var currentState = "idle";
  var isDrawerOpen = false;
  var isAlwaysOnTop = true;
  var sleepTimeout = null;
  var petRenderer = new PetRenderer(petCanvas, particleLayer);
  var stateLabels = {
    idle: "\u5F85\u547D (Idle)",
    listening: "\u503E\u542C\u4E2D...",
    thinking: "\u601D\u8003\u4E2D...",
    speaking: "\u8BF4\u8BDD\u4E2D...",
    sleep: "\u6253\u76F9\u4E2D (Zzz)",
    error: "\u5F02\u5E38"
  };
  var stateColors = {
    idle: "#0051ba",
    listening: "#047857",
    thinking: "#6d28d9",
    speaking: "#b45309",
    sleep: "#4b5563",
    error: "#b91c1c"
  };
  function applyDisplayForm(form) {
    currentForm = form === "capsule" ? "capsule" : "pet";
    document.body.className = `mode-${currentForm}`;
    if (btnFormPet && btnFormCapsule) {
      btnFormPet.classList.toggle("active", currentForm === "pet");
      btnFormCapsule.classList.toggle("active", currentForm === "capsule");
    }
    if (currentForm === "capsule") {
      menuToggleForm.textContent = "\u5207\u6362\u4E3A\u7075\u52A8\u684C\u5BA0\u5F62\u6001";
    } else {
      menuToggleForm.textContent = "\u5207\u6362\u4E3A\u633A\u62EC\u80F6\u56CA\u5F62\u6001";
    }
  }
  function switchForm(targetForm) {
    applyDisplayForm(targetForm);
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetDisplayForm(targetForm);
    }
  }
  var activePoseTimer = null;
  var isWhalePosePlaying = false;
  var activeWhalePose = null;
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
  function applySkin(skinId) {
    currentSkin = skinId || "whale";
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
      whale: "\u5927\u80A5\u9C7C (\u9CB8\u9C7C\u5A18)",
      cat: "\u6A58\u5B9D\u55B5 (\u684C\u5BA0\u4F34\u4FA3)",
      fox: "\u7075\u72D0\u5C0F\u4E5D (\u684C\u5BA0\u4F34\u4FA3)",
      spirit: "\u661F\u7075\u9732\u9732 (\u684C\u5BA0\u4F34\u4FA3)"
    };
    menuPetTitle.textContent = skinNames[currentSkin] || `${currentSkin} (\u684C\u5BA0\u4F34\u4FA3)`;
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetPetSkin(currentSkin);
    }
    refreshSkinUI();
  }
  function refreshSkins() {
    if (!window.go || !window.go.main || !window.go.main.App) return;
    window.go.main.App.GetSkinsList().then((list) => {
      if (!list || !Array.isArray(list)) return;
      skinGrid.innerHTML = "";
      list.forEach((s) => {
        const card = document.createElement("div");
        card.className = `skin-card ${s.id === currentSkin ? "active" : ""}`;
        card.innerHTML = `
        <div class="skin-card-header">
          <span class="skin-card-title">${s.display_name}</span>
          <span class="skin-card-badge">${s.id === currentSkin ? "\u5F53\u524D\u4F7F\u7528" : s.type === "builtin" ? "\u5185\u7F6E" : "\u81EA\u5B9A\u4E49"}</span>
        </div>
        <div class="skin-card-desc">${s.description || "\u8F7B\u89E6\u5207\u6362"}</div>
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
        item.textContent = `${s.id === currentSkin ? "\u2713 " : ""}${s.display_name}`;
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
    });
    refreshSkins();
  }
  function resetSleepTimer() {
    clearTimeout(sleepTimeout);
    if (currentState === "sleep") {
      applyState("idle");
    }
    sleepTimeout = setTimeout(() => {
      if (currentState === "idle" && currentForm === "pet") {
        applyState("sleep");
      }
    }, 45e3);
  }
  var bubbleHideTimer = null;
  function showBubble(text, holdMs = 4e3) {
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
  function applyState(state) {
    const previousState = currentState;
    currentState = state;
    const label = stateLabels[state] || state;
    const color = stateColors[state] || "#38bdf8";
    capsule.className = `capsule state-${state}`;
    statusText.textContent = label;
    desktopPet.dataset.state = state;
    petRenderer.setState(state);
    petStatusText.textContent = label.split(" ")[0];
    petStatusDot.style.background = color;
    petStatusDot.style.boxShadow = `0 0 8px ${color}`;
    if (whaleMouth) {
      if (state === "speaking") {
        whaleMouth.style.opacity = "1";
      } else {
        whaleMouth.style.opacity = "0";
      }
    }
    if (state === "sleep" && currentSkin === "whale") {
      playWhalePose("sleep-asleep", 0);
    } else if (previousState === "sleep" && state !== "sleep" && currentSkin === "whale") {
      playWhalePose("sleep-waking", 1600);
    }
    if (state === "idle") {
      const cur = subtitleText.textContent || "";
      if (!cur || cur.startsWith("\u6B63\u5728") || cur.startsWith("\u503E\u542C") || cur.startsWith("\u{1F399}\uFE0F")) {
        subtitleText.textContent = currentMode === "hands_free" ? "\u968F\u65F6\u5F00\u53E3\u6216\u6478\u6478\u6211\u7545\u804A..." : "\u6309\u4F4F\u7A7A\u683C\u8BF4\u8BDD...";
      }
      if (petBubble.classList.contains("visible")) {
        clearTimeout(bubbleHideTimer);
        bubbleHideTimer = setTimeout(hideBubble, 3e3);
      }
    } else if (state === "thinking") {
      subtitleText.textContent = "\u6B63\u5728\u601D\u8003\u5E76\u7EC4\u7EC7\u56DE\u7B54...";
    } else if (state === "listening") {
      subtitleText.textContent = "\u503E\u542C\u4E2D...";
    } else if (state === "sleep") {
      hideBubble();
    }
    resetSleepTimer();
  }
  var dragStartX = 0;
  var dragStartY = 0;
  var isDragging = false;
  var lastMoveTime = 0;
  petStage.addEventListener("pointerdown", (e) => {
    if (e.button !== 0) return;
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
      handlePetAction("poke");
    }
  });
  function handlePetAction(action) {
    resetSleepTimer();
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.InteractPet(action).then((res) => {
        if (!res) return;
        petRenderer.triggerCue(res.cue || "jumping", 1200);
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
          petRenderer.spawnHearts(100, 70, `\u2764\uFE0F ${xpLabel}`);
        } else if (action === "rice" || action === "turbo") {
          petRenderer.spawnHearts(100, 70, `\u{1F35A} \u5E72\u996D ${xpLabel}`);
        } else if (action === "feed") {
          petRenderer.spawnHearts(100, 70, `\u{1F41F} \u9971\u8179 ${xpLabel}`);
        } else if (action === "grass") {
          petRenderer.spawnHearts(100, 70, `\u{1F33E} \u9017\u8349 ${xpLabel}`);
        } else {
          petRenderer.spawnHearts(100, 70, xpLabel);
        }
        if (res.message) {
          showBubble(res.message);
        }
      });
    }
  }
  petStage.addEventListener("dblclick", () => {
    handlePetAction("pet");
  });
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
    showBubble("\u5DF2\u6253\u65AD\u53D1\u97F3\uFF0C\u968F\u65F6\u5F00\u53E3~", 2500);
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
      showBubble("\u4F60\u8BF4: " + val, 3e4);
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
  if (menuActionFeed) {
    menuActionFeed.addEventListener("click", () => {
      handlePetAction("feed");
      closeContextMenu();
    });
  }
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
        topStatusText.textContent = isTop ? "\u5DF2\u5F00\u542F" : "\u5DF2\u5173\u95ED";
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
  btnFormPet.addEventListener("click", () => switchForm("pet"));
  btnFormCapsule.addEventListener("click", () => switchForm("capsule"));
  btnDoImportSkin.addEventListener("click", () => {
    const p = txtSkinImportPath.value.trim();
    if (p && window.go && window.go.main && window.go.main.App) {
      window.go.main.App.ImportSkinPackage(p).then((info) => {
        txtSkinImportPath.value = "";
        applySkin(info.id);
        refreshSkins();
        alert(`\u76AE\u80A4 ${info.display_name} \u5BFC\u5165\u6210\u529F\uFF01`);
      }).catch((err) => {
        alert("\u5BFC\u5165\u5931\u8D25: " + err);
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
  tabButtons.forEach((btn) => {
    btn.addEventListener("click", () => {
      tabButtons.forEach((b) => b.classList.remove("active"));
      tabPanes.forEach((p) => p.classList.remove("active"));
      btn.classList.add("active");
      const target = btn.getAttribute("data-tab");
      document.getElementById(target)?.classList.add("active");
    });
  });
  function refreshMemory() {
    if (!window.go || !window.go.main || !window.go.main.App) return;
    window.go.main.App.GetMemoryData().then((data) => {
      if (!data) return;
      profileList.innerHTML = "";
      if (!data.profile || data.profile.length === 0) {
        profileList.innerHTML = '<span style="font-size:11px;color:#64748b;">\u6682\u65E0\u6838\u5FC3\u753B\u50CF</span>';
      } else {
        data.profile.forEach((item) => {
          const tag = document.createElement("div");
          tag.className = "profile-tag";
          tag.innerHTML = `<span><b>${item.key}:</b> ${item.value}</span><span class="tag-del" title="\u5220\u9664">\u2715</span>`;
          tag.querySelector(".tag-del").addEventListener("click", () => {
            window.go.main.App.DeleteProfile(item.id).then(() => refreshMemory());
          });
          profileList.appendChild(tag);
        });
      }
      episodeList.innerHTML = "";
      if (!data.episodes || data.episodes.length === 0) {
        episodeList.innerHTML = '<span style="font-size:11px;color:#64748b;">\u6682\u65E0\u4E8B\u4EF6\u8BB0\u5F55</span>';
      } else {
        data.episodes.forEach((ep) => {
          const card = document.createElement("div");
          card.className = "episode-card";
          const d = new Date(ep.created_at * 1e3);
          const timeStr = `${d.getMonth() + 1}\u6708${d.getDate()}\u65E5`;
          card.innerHTML = `
          <div class="episode-info">
            <span class="episode-text">${ep.content}</span>
            <span class="episode-meta">${timeStr} ${ep.tags ? `[${ep.tags.join(",")}]` : ""}</span>
          </div>
          <span class="tag-del" title="\u5220\u9664">\u2715</span>
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
  function refreshPersonas() {
    if (!window.go || !window.go.main || !window.go.main.App) return;
    window.go.main.App.GetPersonaData().then((data) => {
      if (!data) return;
      chkAutoSchedule.checked = data.auto_schedule;
      slotList.innerHTML = "";
      personaSubmenu.innerHTML = "";
      data.slots.forEach((s) => {
        const isAct = s.id === data.active_slot_id;
        const card = document.createElement("div");
        card.className = `slot-card ${isAct ? "active" : ""}`;
        card.innerHTML = `
        <div class="slot-left">
          <span class="slot-badge">${s.badge || "\u{1F3AD}"}</span>
          <div>
            <div class="slot-name">${s.name}</div>
            <div class="slot-desc">${s.tone_description || ""}</div>
          </div>
        </div>
        <button class="badge-btn">${isAct ? "\u5F53\u524D\u6FC0\u6D3B" : "\u542F\u7528"}</button>
      `;
        card.addEventListener("click", () => {
          window.go.main.App.SetActivePersona(s.id).then(() => refreshPersonas());
        });
        slotList.appendChild(card);
        const item = document.createElement("div");
        item.className = "menu-item";
        item.textContent = `${isAct ? "\u2713 " : ""}${s.badge || ""} ${s.name}`;
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
  function updateBondUI(bond) {
    if (!bond) return;
    bondLevelText.textContent = bond.level_name || `Lv.${bond.level}`;
    bondStreakText.textContent = `\u{1F525} \u5DF2\u8FDE\u7EED\u966A\u4F34 ${bond.streak_days} \u5929`;
    const thresholds = [0, 100, 300, 700, 1500, 3e3];
    const nextTarget = thresholds[bond.level] || 1500;
    const pct = Math.min(100, Math.round(bond.xp / nextTarget * 100));
    xpBarFill.style.width = `${pct}%`;
    xpLabelText.textContent = `${bond.xp} / ${nextTarget} XP`;
  }
  function refreshBond() {
    if (!window.go || !window.go.main || !window.go.main.App) return;
    window.go.main.App.GetBondData().then((bond) => updateBondUI(bond));
  }
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
        subtitleText.textContent = "\u4F60\u8BF4: " + val;
        applyState("thinking");
      }
    } else if (e.key === "Escape") {
      inputDrawer.classList.remove("open");
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.SetWindowExpanded(false);
      }
    }
  });
  var isSpaceHeld = false;
  var pttWatchdogTimer = null;
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
          subtitleText.textContent = "\u6B63\u5728\u503E\u542C\uFF0C\u677E\u5F00\u7A7A\u683C\u53D1\u9001...";
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
        subtitleText.textContent = "\u6B63\u5728\u8BC6\u522B\u5E76\u7EC4\u7EC7\u56DE\u7B54...";
        clearTimeout(pttWatchdogTimer);
        pttWatchdogTimer = setTimeout(() => {
          if (subtitleText.textContent.includes("\u6B63\u5728\u8BC6\u522B")) {
            showBubble("\u{1F442} \u521A\u624D\u597D\u50CF\u6CA1\u542C\u6E05\uFF0C\u8BF7\u6309\u4F4F\u7A7A\u683C\u518D\u8BF4\u4E00\u6B21\u54E6~", 3e3);
            subtitleText.textContent = "\u6309\u4F4F\u7A7A\u683C\u8BF4\u8BDD...";
            applyState("idle");
          }
        }, 3e3);
      }
    });
  }
  if (window.runtime) {
    window.runtime.EventsOn("avatar:state", (s) => {
      clearTimeout(pttWatchdogTimer);
      applyState(s);
    });
    window.runtime.EventsOn("avatar:speech_partial", (text) => {
      resetSleepTimer();
      subtitleText.textContent = "\u6B63\u5728\u542C: " + text;
      showBubble("\u{1F442} " + text, 3e3);
      applyState("listening");
    });
    window.runtime.EventsOn("avatar:speech_final", (text) => {
      clearTimeout(pttWatchdogTimer);
      resetSleepTimer();
      subtitleText.textContent = "\u4F60\u8BF4: " + text;
      showBubble("\u4F60\u8BF4: " + text, 3e4);
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
      showBubble(sentence, Math.max(4e3, sentence.length * 300));
    });
    window.runtime.EventsOn("avatar:error", (err) => {
      subtitleText.textContent = "\u8FDE\u63A5\u5F02\u5E38: " + err;
      showBubble("\u8FDE\u63A5\u5F02\u5E38: " + err, 6e3);
    });
    window.runtime.EventsOn("avatar:display_form", (f) => {
      applyDisplayForm(f);
    });
    window.runtime.EventsOn("avatar:pet_skin", (s) => {
      applySkin(s);
    });
    window.runtime.EventsOn("avatar:mode", (m) => {
      currentMode = m;
      modeBadge.textContent = m === "hands_free" ? "\u514D\u5524\u9192" : "\u5FEB\u6377\u952E";
      const desc = m === "hands_free" ? "\u514D\u5524\u9192\u6A21\u5F0F (\u968F\u65F6\u5F00\u53E3)" : "\u6309\u4F4F\u7A7A\u683C\u8BF4\u8BDD\u6A21\u5F0F";
      showBubble(desc, 2500);
      if (currentState === "idle") {
        const defaultText = m === "hands_free" ? "\u968F\u65F6\u5F00\u53E3\u4E0E\u6211\u7545\u804A..." : "\u6309\u4F4F\u7A7A\u683C\u8BF4\u8BDD...";
        subtitleText.textContent = defaultText;
      }
    });
    window.runtime.EventsOn("avatar:persona", (slot) => {
      if (slot && slot.name) {
        personaBadge.textContent = `${slot.badge || "\u{1F319}"} ${slot.name}`;
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
  window.addEventListener("DOMContentLoaded", () => {
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.GetDisplayForm().then((form) => applyDisplayForm(form));
      window.go.main.App.GetPetSkin().then((skin) => applySkin(skin));
      window.go.main.App.GetMode().then((m) => {
        currentMode = m;
        modeBadge.textContent = m === "hands_free" ? "\u514D\u5524\u9192" : "\u5FEB\u6377\u952E";
        if (currentState === "idle") {
          subtitleText.textContent = m === "hands_free" ? "\u968F\u65F6\u5F00\u53E3\u4E0E\u6211\u7545\u804A..." : "\u6309\u4F4F\u7A7A\u683C\u8BF4\u8BDD...";
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
})();

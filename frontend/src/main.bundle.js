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
      "running-right": rowAnimation(1, 8, 91, 91),
      "running-left": rowAnimation(2, 8, 91, 91),
      waving: rowAnimation(3, 4, 125, 125),
      jumping: rowAnimation(4, 5, 83, 120),
      failed: rowAnimation(5, 8, 125, 125),
      waiting: rowAnimation(6, 6, 166, 166),
      running: rowAnimation(7, 6, 100, 100),
      review: rowAnimation(8, 6, 166, 166)
    };
  }
  var PET_STATE_ANIMATION_POLICY = Object.freeze({
    idle: { name: "idle", mode: "loop" },
    sleep: { name: "idle", mode: "loop" },
    listening: { name: "waiting", mode: "loop", blocksCues: true },
    thinking: { name: "review", mode: "loop", blocksCues: true },
    speaking: { name: "waving", mode: "loop", blocksCues: false },
    error: { name: "failed", mode: "once", blocksCues: true }
  });
  var PET_EVENT_ANIMATION_POLICY = Object.freeze({
    hover: "jumping",
    click: "jumping",
    wake: "jumping",
    pet: "jumping",
    poke: "jumping",
    feed: "running",
    rice: "running",
    turbo: "running",
    grass: "jumping"
  });
  function spritePlaybackSelection({
    state,
    baseWorking = false,
    dragDirection = "",
    cue = null
  } = {}) {
    const fallback = state === "speaking" ? "waving" : baseWorking || state === "working" ? "running" : "idle";
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
          case "luoxiaohei2d":
            drawLuoxiaoheiFrame(ctx, row, col);
            break;
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
            drawCatFrame(ctx, row, col);
            break;
          default:
            drawLuoxiaoheiFrame(ctx, row, col);
            break;
        }
        ctx.restore();
      }
    }
  }
  function drawLuoxiaoheiFrame(ctx, row, col) {
    const catBlack = "#0f131c";
    const eyeGreen = "#bef264";
    const eyePupil = "#090d16";
    const earInner = "#2a3447";
    const mouthPink = "#f43f5e";
    let bodyScaleX = 1;
    let bodyScaleY = 1;
    let offsetY = 0;
    let offsetX = 0;
    let isBlinking = false;
    let tailAngle = 0;
    let pawLift = 0;
    let mouthOpen = false;
    switch (row) {
      case 0:
        offsetY = Math.sin(col * 1.05) * 3;
        bodyScaleY = 1 + Math.sin(col * 1.05) * 0.04;
        tailAngle = Math.sin(col * 1.2) * 12;
        if (col === 3 || col === 4) isBlinking = true;
        break;
      case 1:
        offsetX = 6;
        offsetY = col % 2 === 0 ? -4 : 2;
        tailAngle = -25 + col % 2 * 15;
        pawLift = col % 2 === 0 ? 6 : -4;
        break;
      case 2:
        offsetX = -6;
        offsetY = col % 2 === 0 ? -4 : 2;
        tailAngle = 25 - col % 2 * 15;
        pawLift = col % 2 === 0 ? -4 : 6;
        break;
      case 3:
        offsetY = col % 2 * -3;
        mouthOpen = true;
        pawLift = 12 + col % 2 * 6;
        tailAngle = Math.sin(col * 1.5) * 15;
        break;
      case 4:
        offsetY = -12 - col * 3;
        bodyScaleY = 1.08;
        bodyScaleX = 0.94;
        pawLift = 14;
        tailAngle = 10;
        break;
      case 5:
        offsetY = 4;
        bodyScaleY = 0.92;
        bodyScaleX = 1.06;
        tailAngle = -30;
        break;
      case 6:
        offsetY = -2;
        tailAngle = 5;
        break;
      case 7:
        offsetY = -4 + col % 2 * 4;
        mouthOpen = true;
        tailAngle = 20;
        pawLift = 8;
        break;
      case 8:
        offsetY = Math.sin(col * 0.8) * 2;
        tailAngle = -10 + col * 4;
        break;
    }
    ctx.save();
    ctx.translate(offsetX, offsetY);
    ctx.scale(bodyScaleX, bodyScaleY);
    ctx.save();
    ctx.translate(-28, 28);
    ctx.rotate(tailAngle * Math.PI / 180);
    ctx.strokeStyle = catBlack;
    ctx.lineWidth = 9;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(0, 0);
    ctx.quadraticCurveTo(-18, -10, -28, -22);
    ctx.stroke();
    ctx.lineWidth = 5;
    ctx.beginPath();
    ctx.moveTo(-28, -22);
    ctx.lineTo(-36, -30);
    ctx.moveTo(-28, -22);
    ctx.lineTo(-32, -35);
    ctx.stroke();
    ctx.restore();
    ctx.fillStyle = catBlack;
    ctx.beginPath();
    ctx.ellipse(0, 16, 44, 38, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(-36, -14);
    ctx.lineTo(-48, -54);
    ctx.lineTo(-14, -30);
    ctx.closePath();
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(36, -14);
    ctx.lineTo(48, -54);
    ctx.lineTo(14, -30);
    ctx.closePath();
    ctx.fill();
    ctx.fillStyle = earInner;
    ctx.beginPath();
    ctx.moveTo(-32, -18);
    ctx.lineTo(-42, -48);
    ctx.lineTo(-18, -28);
    ctx.closePath();
    ctx.fill();
    ctx.beginPath();
    ctx.moveTo(32, -18);
    ctx.lineTo(42, -48);
    ctx.lineTo(18, -28);
    ctx.closePath();
    ctx.fill();
    if (!isBlinking && row !== 5) {
      ctx.fillStyle = eyeGreen;
      ctx.beginPath();
      ctx.ellipse(-19, -2, 14, 16, 0, 0, Math.PI * 2);
      ctx.ellipse(19, -2, 14, 16, 0, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = eyePupil;
      let pupilOffsetX = 0;
      if (row === 1) pupilOffsetX = 4;
      else if (row === 2) pupilOffsetX = -4;
      else if (row === 8) pupilOffsetX = Math.sin(col) * 5;
      ctx.beginPath();
      ctx.ellipse(-19 + pupilOffsetX, -2, 6, 11, 0, 0, Math.PI * 2);
      ctx.ellipse(19 + pupilOffsetX, -2, 6, 11, 0, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = "#ffffff";
      ctx.beginPath();
      ctx.arc(-22 + pupilOffsetX, -6, 3.5, 0, Math.PI * 2);
      ctx.arc(16 + pupilOffsetX, -6, 3.5, 0, Math.PI * 2);
      ctx.fill();
    } else if (row === 5) {
      ctx.strokeStyle = eyeGreen;
      ctx.lineWidth = 3.5;
      ctx.beginPath();
      ctx.moveTo(-26, -8);
      ctx.lineTo(-12, 4);
      ctx.moveTo(-12, -8);
      ctx.lineTo(-26, 4);
      ctx.moveTo(12, -8);
      ctx.lineTo(26, 4);
      ctx.moveTo(26, -8);
      ctx.lineTo(12, 4);
      ctx.stroke();
    } else {
      ctx.strokeStyle = eyeGreen;
      ctx.lineWidth = 3.5;
      ctx.beginPath();
      ctx.arc(-19, -2, 10, 0.1 * Math.PI, 0.9 * Math.PI);
      ctx.arc(19, -2, 10, 0.1 * Math.PI, 0.9 * Math.PI);
      ctx.stroke();
    }
    if (mouthOpen) {
      ctx.fillStyle = mouthPink;
      ctx.beginPath();
      ctx.ellipse(0, 14, 5, 6, 0, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.fillStyle = catBlack;
    ctx.beginPath();
    ctx.arc(-18, 42 - (row === 3 ? pawLift : 0), 9, 0, Math.PI * 2);
    ctx.arc(18, 42 - (row === 1 || row === 7 ? pawLift : 0), 9, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
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
      this.skinId = "luoxiaohei2d";
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
      if (skinId === "luoxiaohei2d") {
        const img = new Image();
        img.onload = () => {
          this.geometry = spriteGeometry({});
          this.animations = defaultAnimations();
          this.assets = {
            image: img,
            geometry: this.geometry,
            animations: this.animations
          };
          this.resetTrack();
        };
        img.onerror = () => {
          const sheetCanvas2 = getBuiltinPetSpritesheet("luoxiaohei2d");
          this.geometry = spriteGeometry({});
          this.animations = defaultAnimations();
          this.assets = {
            image: sheetCanvas2,
            geometry: this.geometry,
            animations: this.animations
          };
          this.resetTrack();
        };
        img.src = "./assets/luoxiaohei2d/spritesheet.webp";
        return;
      }
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
      const cueId = Date.now() + Math.random();
      this.cue = {
        id: cueId,
        name
      };
      this.resetTrack();
      if (durationMs > 0) {
        setTimeout(() => {
          if (this.cue && this.cue.id === cueId) {
            this.cue = null;
            this.resetTrack();
          }
        }, durationMs);
      }
    }
    clearCue() {
      if (!this.cue) return;
      this.cue = null;
      this.resetTrack();
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
      if (this.skinId === "whale" || this.canvas.style.display === "none") {
        this.timer = setTimeout(() => this.loop(), 300);
        return;
      }
      if (!this.assets || !this.track) {
        this.timer = setTimeout(() => this.loop(), 100);
        return;
      }
      const elapsed = performance.now() - this.startedAt;
      let frame = frameAtElapsed(this.track, elapsed);
      if (!frame) {
        const defaultFallback = this.state === "speaking" ? "waving" : "idle";
        const fallbackName = this.track.fallback || defaultFallback;
        const fallback = this.assets.animations[fallbackName] || this.assets.animations.idle;
        this.track = {
          ...fallback,
          loopStart: 0,
          fallback: defaultFallback
        };
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
  var btnPetRoam = document.getElementById("btnPetRoam");
  var btnPetTouch = document.getElementById("btnPetTouch");
  var btnPetRice = document.getElementById("btnPetRice");
  var btnPetTurbo = document.getElementById("btnPetTurbo");
  var btnPetDialKit = document.getElementById("btnPetDialKit");
  var btnPetPersona = document.getElementById("btnPetPersona");
  var btnPetToCapsule = document.getElementById("btnPetToCapsule");
  var btnPetDrawer = document.getElementById("btnPetDrawer");
  var btnPetStop = document.getElementById("btnPetStop");
  var roamBubble = document.getElementById("roamBubble");
  var macPetShadow = document.getElementById("macPetShadow");
  var petContextMenu = document.getElementById("petContextMenu");
  var menuPetTitle = document.getElementById("menuPetTitle");
  var menuActionPet = document.getElementById("menuActionPet");
  var menuActionFish = document.getElementById("menuActionFish");
  var menuActionRice = document.getElementById("menuActionRice");
  var menuActionTurbo = document.getElementById("menuActionTurbo");
  var menuActionGrass = document.getElementById("menuActionGrass");
  var menuToggleRoam = document.getElementById("menuToggleRoam");
  var roamStatusMenuText = document.getElementById("roamStatusMenuText");
  var menuActionChat = document.getElementById("menuActionChat");
  var skinSubmenu = document.getElementById("skinSubmenu");
  var personaSubmenu = document.getElementById("personaSubmenu");
  var menuToggleForm = document.getElementById("menuToggleForm");
  var menuToggleTop = document.getElementById("menuToggleTop");
  var topStatusText = document.getElementById("topStatusText");
  var menuOpenAudit = document.getElementById("menuOpenAudit");
  var menuOpenDrawer = document.getElementById("menuOpenDrawer");
  var menuQuitApp = document.getElementById("menuQuitApp");
  var memoCardsList = document.getElementById("memoCardsList");
  var newMemoInput = document.getElementById("newMemoInput");
  var btnQuickAddMemo = document.getElementById("btnQuickAddMemo");
  var fishFeedCount = document.getElementById("fishFeedCount");
  var roamPatrolCount = document.getElementById("roamPatrolCount");
  var cfgEdgeBaseUrl = document.getElementById("cfgEdgeBaseUrl");
  var cfgEdgeModel = document.getElementById("cfgEdgeModel");
  var cfgCloudUrl = document.getElementById("cfgCloudUrl");
  var chkCloudConsent = document.getElementById("chkCloudConsent");
  var btnSaveModelConfig = document.getElementById("btnSaveModelConfig");
  var tpmHudPanel = document.getElementById("tpmHudPanel");
  var btnCloseTpmHud = document.getElementById("btnCloseTpmHud");
  var tpmRoamCoord = document.getElementById("tpmRoamCoord");
  var tpmRoamDesc = document.getElementById("tpmRoamDesc");
  var tpmRmsValue = document.getElementById("tpmRmsValue");
  var tpmRmsBar = document.getElementById("tpmRmsBar");
  var tpmActiveTrack = document.getElementById("tpmActiveTrack");
  var dialKitHud = document.getElementById("dialKitHud");
  var btnCloseDialKit = document.getElementById("btnCloseDialKit");
  var sliderScale = document.getElementById("sliderScale");
  var sliderSpeed = document.getElementById("sliderSpeed");
  var valScale = document.getElementById("valScale");
  var valSpeed = document.getElementById("valSpeed");
  var btnCopyDialTokens = document.getElementById("btnCopyDialTokens");
  var capsule = document.getElementById("capsule");
  var orbContainer = document.getElementById("orbContainer");
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
  var newEpContent = document.getElementById("newEpContent");
  var btnAddEpisode = document.getElementById("btnAddEpisode");
  var slotList = document.getElementById("slotList");
  var chkAutoSchedule = document.getElementById("chkAutoSchedule");
  var bondStreakHeader = document.getElementById("bondStreakHeader");
  var bondLevelText = document.getElementById("bondLevelText");
  var bondStreakText = document.getElementById("bondStreakText");
  var xpBarFill = document.getElementById("xpBarFill");
  var xpLabelText = document.getElementById("xpLabelText");
  var currentForm = "pet";
  var currentSkin = "luoxiaohei2d";
  var isAutoRoamingEnabled = true;
  var roamSpeedFactor = 1.2;
  var petScale = 1.15;
  var currentRms = 0;
  var isRoamingMoving = false;
  var roamCurrentOffset = 0;
  var roamTargetOffset = 0;
  var roamTimer = null;
  var roamAnimId = null;
  var roamBubbleTimer = null;
  var isDirectedRoaming = false;
  var roamWaypoints = [];
  var roamLastFrameTs = 0;
  var fishFedTotal = 28;
  var patrolStepsTotal = 142;
  var currentMode = "hands_free";
  var currentState = "idle";
  var isDrawerOpen = false;
  var isAlwaysOnTop = true;
  var sleepTimeout = null;
  var petRenderer = new PetRenderer(petCanvas, particleLayer);
  var stateLabels = {
    idle: "\u5F85\u547D",
    listening: "\u503E\u542C\u4E2D",
    thinking: "\u601D\u8003\u4E2D",
    speaking: "\u8BF4\u8BDD\u4E2D",
    sleep: "\u6253\u76F9\u4E2D",
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
      luoxiaohei2d: "\u7F57\u5C0F\u9ED1 2D (\u5B98\u65B9\u5BA0\u4F34)",
      whale: "\u5927\u80A5\u9C7C (\u9CB8\u9C7C\u5A18)",
      cat: "\u6A58\u5B9D\u55B5 (\u684C\u5BA0\u4F34\u4FA3)",
      fox: "\u7075\u72D0\u5C0F\u4E5D (\u684C\u5BA0\u4F34\u4FA3)",
      spirit: "\u661F\u7075\u9732\u9732 (\u684C\u5BA0\u4F34\u4FA3)"
    };
    if (menuPetTitle) {
      menuPetTitle.textContent = skinNames[currentSkin] || `${currentSkin} (\u684C\u5BA0\u4F34\u4FA3)`;
    }
    if (notifyBackend && window.go && window.go.main && window.go.main.App) {
      window.go.main.App.SetPetSkin(currentSkin);
    }
    refreshSkinUI();
  }
  var defaultSkins = [
    { id: "luoxiaohei2d", display_name: "\u7F57\u5C0F\u9ED1 2D", description: "\u5B98\u65B9\u9ED8\u8BA4\u5BA0\u4F34\uFF1A\u7EAF\u9ED1\u7075\u52A8\u5C0F\u732B\u3001\u9EC4\u7EFF\u5927\u773C\u3001\u5446\u840C\u8033\u6735\u4E0E\u5206\u53C9\u5C3E\u5DF4\uFF0C\u652F\u6301\u684C\u9762\u81EA\u7531\u6F2B\u6B65\u4E0E\u5168\u53CC\u5DE5\u6253\u65AD", type: "builtin" },
    { id: "whale", display_name: "\u5927\u80A5\u9C7C (\u9CB8\u9C7C\u5A18)", description: "\u5446\u840C\u6E29\u548C\u3001\u7231\u5403\u767D\u996D\uFF0C\u5634\u578B\u968F\u8BF4\u8BDD\u8D77\u4F0F\u81EA\u7136\u5F20\u5408", type: "builtin" },
    { id: "cat", display_name: "\u6A58\u5B9D\u55B5", description: "\u8F6F\u840C\u53EF\u7231\u7684\u6A58\u732B\u4F34\u4FA3\uFF0C\u6D3B\u6CFC\u597D\u52A8\uFF0C\u62E5\u6709\u7728\u773C\u7B49\u5B8C\u6574\u52A8\u4F5C\u5E27", type: "builtin" },
    { id: "fox", display_name: "\u7075\u72D0\u5C0F\u4E5D", description: "\u4ED9\u6C14\u7075\u52A8\u7684\u6CBB\u6108\u7CFB\u8D64\u72D0\uFF0C\u6447\u7740\u6BDB\u8338\u8338\u5927\u5C3E\u5DF4", type: "builtin" },
    { id: "spirit", display_name: "\u661F\u7075\u9732\u9732", description: "\u8F6F\u840C\u53D1\u5149\u7684\u5947\u5E7B\u5C0F\u6C34\u6EF4\u661F\u7075\uFF0C\u5F8B\u52A8\u8DF3\u8DC3\uFF0C\u6E29\u6696\u6CBB\u6108", type: "builtin" }
  ];
  function renderSkinList(list) {
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
  var thinkingWatchdog = null;
  function scheduleThinkingWatchdog() {
    clearTimeout(thinkingWatchdog);
    thinkingWatchdog = setTimeout(() => {
      if (currentState === "thinking") {
        showBubble("\u23F3 \u56DE\u590D\u8D85\u65F6\uFF0C\u8BF7\u68C0\u67E5\u5927\u6A21\u578B\u8FDE\u63A5", 4e3);
        applyState("idle");
      }
    }, 18e3);
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
    if (capsule) capsule.className = `capsule state-${state}`;
    if (statusText) statusText.textContent = label;
    if (desktopPet) desktopPet.dataset.state = state;
    petRenderer.setState(state);
    if (petStatusText) petStatusText.textContent = label.split(" ")[0];
    if (petStatusDot) {
      petStatusDot.style.background = color;
      petStatusDot.style.boxShadow = `0 0 8px ${color}`;
    }
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
      if (!isDirectedRoaming) {
        updateAuditTrack("Row 0 (idle)");
      }
      const cur = subtitleText.textContent || "";
      if (!cur || cur.startsWith("\u6B63\u5728") || cur.startsWith("\u503E\u542C") || cur.startsWith("\u{1F399}\uFE0F")) {
        subtitleText.textContent = currentMode === "hands_free" ? "\u968F\u65F6\u5F00\u53E3\u6216\u6478\u6478\u6211\u7545\u804A..." : "\u6309\u4F4F\u7A7A\u683C\u8BF4\u8BDD...";
      }
      if (petBubble.classList.contains("visible")) {
        clearTimeout(bubbleHideTimer);
        bubbleHideTimer = setTimeout(hideBubble, 3e3);
      }
      scheduleNextRoamDecision();
    } else if (state === "thinking") {
      if (!isDirectedRoaming) {
        stopRoamingMove();
        updateAuditTrack("Row 8 (review)");
      }
      subtitleText.textContent = "\u6B63\u5728\u601D\u8003\u5E76\u7EC4\u7EC7\u56DE\u7B54...";
    } else if (state === "listening") {
      stopRoamingMove();
      updateAuditTrack("Row 6 (waiting)");
      subtitleText.textContent = "\u503E\u542C\u4E2D...";
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
    const delay = Math.random() * 4e3 + 3500;
    roamTimer = setTimeout(() => {
      if (currentState === "idle" && !isDragging && currentForm === "pet" && !isDrawerOpen) {
        makeAutonomousMoveDecision();
      }
      scheduleNextRoamDecision();
    }, delay);
  }
  function makeAutonomousMoveDecision() {
    if (currentState !== "idle" || isDragging || isDrawerOpen || isDirectedRoaming) return;
    roamTargetOffset = (Math.random() - 0.5) * 60;
    const dx = roamTargetOffset - roamCurrentOffset;
    if (Math.abs(dx) < 6) return;
    isDirectedRoaming = false;
    roamWaypoints = [];
    isRoamingMoving = true;
    roamLastFrameTs = 0;
    patrolStepsTotal += Math.round(Math.abs(dx));
    if (roamPatrolCount) roamPatrolCount.textContent = `${patrolStepsTotal} \u6B65`;
    const dir = dx > 0 ? "right" : "left";
    petRenderer.setDragDirection(dir);
    showRoamBubble(dx > 0 ? "\u{1F43E} \u5411\u53F3\u8FB9\u5DE1\u89C6\u5C0F\u8DD1..." : "\u{1F43E} \u5411\u5DE6\u8FB9\u5DE1\u89C6\u5C0F\u8DD1...", 2500);
    updateAuditTrack(dir === "right" ? "Row 1 (running-right)" : "Row 2 (running-left)");
    if (tpmRoamCoord) tpmRoamCoord.textContent = `\u5DE1\u89C6\u4E2D \u2192 ${Math.round(roamTargetOffset)}px`;
    animateRoamStep();
  }
  function animateRoamStep(nowTs) {
    const stateBlocked = isDirectedRoaming ? currentState === "listening" || currentState === "sleep" || currentState === "error" : currentState !== "idle";
    if (!isRoamingMoving || stateBlocked || isDragging) {
      stopRoamingMove();
      return;
    }
    const ts = typeof nowTs === "number" ? nowTs : performance.now();
    const dtSec = roamLastFrameTs > 0 ? Math.min(0.05, Math.max(4e-3, (ts - roamLastFrameTs) / 1e3)) : 1 / 60;
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
          if (tpmRoamCoord) tpmRoamCoord.textContent = `\u5DE1\u903B\u4E2D \u2192 ${Math.round(roamTargetOffset)}px`;
          roamAnimId = requestAnimationFrame(animateRoamStep);
          return;
        }
      }
      const wasDirected = isDirectedRoaming;
      stopRoamingMove();
      if (wasDirected && currentState === "speaking") {
        updateAuditTrack("Row 3 (waving)");
        if (tpmRoamCoord) tpmRoamCoord.textContent = `\u5DE1\u903B\u5B8C\u6210 (${Math.round(roamCurrentOffset)}px)`;
      } else if (currentState === "idle") {
        showRoamBubble("\u{1F43E} \u539F\u5730\u8E72\u4E0B\u4F11\u606F\u6253\u76F9...", 2e3);
        updateAuditTrack("Row 0 (idle)");
        if (tpmRoamCoord) tpmRoamCoord.textContent = `\u4F11\u61A9\u4E2D (${Math.round(roamCurrentOffset)}px)`;
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
    if (roamPatrolCount) roamPatrolCount.textContent = `${patrolStepsTotal} \u6B65`;
    const dir = roamTargetOffset - roamCurrentOffset >= 0 ? "right" : "left";
    petRenderer.clearCue();
    petRenderer.setDragDirection(dir);
    showRoamBubble(bubbleText || (dir === "right" ? "\u{1F43E} \u6536\u5230\u6307\u4EE4\uFF0C\u5411\u53F3\u5DE1\u89C6..." : "\u{1F43E} \u6536\u5230\u6307\u4EE4\uFF0C\u5411\u5DE6\u5DE1\u89C6..."), 2500);
    updateAuditTrack(dir === "right" ? "Row 1 (running-right)" : "Row 2 (running-left)");
    if (tpmRoamCoord) tpmRoamCoord.textContent = `\u5DE1\u903B\u4E2D \u2192 ${Math.round(roamTargetOffset)}px`;
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
      roamStatusMenuText.textContent = isAutoRoamingEnabled ? "\u5DF2\u5F00\u542F" : "\u5DF2\u6682\u505C";
    }
    if (btnPetRoam) {
      btnPetRoam.style.opacity = isAutoRoamingEnabled ? "1" : "0.5";
      btnPetRoam.title = isAutoRoamingEnabled ? "\u684C\u9762\u81EA\u7531\u6F2B\u6B65\uFF1A\u5F00\u542F\u4E2D (\u70B9\u51FB\u6682\u505C)" : "\u81EA\u7531\u6F2B\u6B65\uFF1A\u5DF2\u6682\u505C (\u70B9\u51FB\u5F00\u542F)";
    }
    if (isAutoRoamingEnabled) {
      scheduleNextRoamDecision();
      showRoamBubble("\u684C\u9762\u81EA\u7531\u6F2B\u6B65\uFF1A\u5F00\u542F\u4E2D", 2e3);
    } else {
      stopRoamingMove();
      roamCurrentOffset = 0;
      petStage.style.transform = `translateX(0px) scale(var(--pet-scale))`;
      showRoamBubble("\u81EA\u7531\u6F2B\u6B65\uFF1A\u5DF2\u6682\u505C", 2e3);
    }
  }
  var dragStartX = 0;
  var dragStartY = 0;
  var isDragging = false;
  var lastMoveTime = 0;
  function resetDragState() {
    dragStartX = 0;
    dragStartY = 0;
    isDragging = false;
    petRenderer.setDragDirection("");
  }
  petStage.addEventListener("pointerdown", (e) => {
    if (e.button !== 0) return;
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
        showRoamBubble("\u26A1 \u88AB\u4E3B\u4EBA\u63EA\u8D77\u540E\u9888\u76AE\uFF01", 3e3);
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
      showRoamBubble("\u{1F43E} \u5E73\u7A33\u843D\u5730\uFF0C\u7EE7\u7EED\u5DE1\u89C6...", 2e3);
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
  petStage.addEventListener("dblclick", (e) => {
    e.stopPropagation();
    setDrawerOpen(!isDrawerOpen);
    petRenderer.triggerCue("jumping", 800);
    petRenderer.spawnHearts(100, 70, "\u{1F4D8} \u6848\u5934\u624B\u8D26");
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
          updateAuditTrack("Row 4 (jumping)");
          petRenderer.spawnHearts(100, 70, `\u2764\uFE0F ${xpLabel}`);
        } else if (action === "rice" || action === "turbo") {
          updateAuditTrack("Row 7 (feed)");
          petRenderer.spawnHearts(100, 70, `\u{1F35A} \u5E72\u996D ${xpLabel}`);
        } else if (action === "feed") {
          fishFedTotal += 1;
          if (fishFeedCount) fishFeedCount.textContent = `${fishFedTotal} \u6B21`;
          updateAuditTrack("Row 7 (feed)");
          petRenderer.spawnHearts(100, 70, `\u{1F41F} \u9971\u8179 ${xpLabel}`);
        } else if (action === "grass") {
          updateAuditTrack("Row 4 (jumping)");
          petRenderer.spawnHearts(100, 70, `\u{1F33E} \u9017\u8349 ${xpLabel}`);
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
      showBubble("\u4F60\u8BF4: " + val, 5e3);
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
      "Row 8 (review)": "tagRow8"
    };
    document.querySelectorAll(".track-tag").forEach((el) => el.classList.remove("active"));
    const tagId = tagMap[trackName];
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
      auto_roaming: isAutoRoamingEnabled
    };
    navigator.clipboard.writeText(JSON.stringify(tokens, null, 2)).then(() => {
      showBubble("\u{1F4CB} \u914D\u7F6E Tokens \u5DF2\u590D\u5236\u5230\u526A\u8D34\u677F\uFF01", 2500);
    });
  }
  if (btnCopyDialTokens) {
    btnCopyDialTokens.addEventListener("click", copyDialTokens);
  }
  window.addEventListener("keydown", (e) => {
    if ((e.key === "d" || e.key === "D") && !["INPUT", "TEXTAREA"].includes(document.activeElement?.tagName)) {
      toggleDialKit();
    }
  });
  var localMemos = [
    { id: "1", time: "09:41 \xB7 \u8BED\u97F3\u6355\u83B7", content: "\u4E0B\u5348 4 \u70B9\u627E\u67B6\u6784\u5E08\u5BF9\u9F50\u5168\u53CC\u5DE5\u6253\u65AD\u534F\u8BAE" },
    { id: "2", time: "10:15 \xB7 \u5FEB\u6377\u901F\u8BB0", content: "\u4F18\u5316\u7F57\u5C0F\u9ED1 2D \u8D34\u8FB9\u5438\u9644\u7269\u7406\u624B\u611F\u4E0E\u58F0\u5B66\u5206\u7EA7" }
  ];
  function renderMemos() {
    if (!memoCardsList) return;
    memoCardsList.innerHTML = "";
    if (localMemos.length === 0) {
      memoCardsList.innerHTML = '<div style="font-size:12px;color:#94a3b8;text-align:center;padding:12px 0;">\u6682\u65E0\u7075\u611F\u901F\u8BB0\uFF0C\u968F\u624B\u8BB0\u4E00\u6761\u5427~</div>';
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
        <button class="memo-action-btn" data-act="copy">\u590D\u5236</button>
        <button class="memo-action-btn" data-act="archive">\u5F52\u6863</button>
      </div>
    `;
      card.querySelector('[data-act="copy"]')?.addEventListener("click", () => {
        navigator.clipboard.writeText(memo.content);
        showBubble("\u5DF2\u590D\u5236\u7075\u611F\u5185\u5BB9\u5230\u526A\u8D34\u677F\uFF01", 2e3);
      });
      card.querySelector('[data-act="archive"]')?.addEventListener("click", () => {
        localMemos = localMemos.filter((m) => m.id !== memo.id);
        renderMemos();
        showBubble("\u5DF2\u5F52\u6863\u8BE5\u6761\u901F\u8BB0", 1500);
      });
      memoCardsList.appendChild(card);
    });
  }
  function addQuickMemo(text) {
    if (!text) return;
    const now = /* @__PURE__ */ new Date();
    const timeStr = `${String(now.getHours()).padStart(2, "0")}:${String(now.getMinutes()).padStart(2, "0")} \xB7 \u6848\u5934\u901F\u8BB0`;
    localMemos.unshift({
      id: String(Date.now()),
      time: timeStr,
      content: text
    });
    renderMemos();
    showBubble("\u{1F4A1} \u7075\u611F\u5DF2\u6355\u83B7\u81F3\u6848\u5934\u624B\u8D26\uFF01", 2500);
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.AddEpisode(text, ["\u7075\u611F\u901F\u8BB0"]);
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
      showBubble("\u2705 \u6A21\u578B\u4E0E\u7AEF\u4FA7\u914D\u7F6E\u5DF2\u4FDD\u5B58\uFF01", 2500);
    });
  }
  if (orbContainer) {
    orbContainer.addEventListener("click", () => {
      if (window.go && window.go.main && window.go.main.App) {
        window.go.main.App.Interrupt();
      }
      applyState("idle");
      showBubble("\u5DF2\u91CD\u7F6E\u5E76\u5F85\u547D~", 2e3);
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
        window.go.main.App.AddEpisode(val, ["\u5907\u5FD8"]).then(() => {
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
  var defaultPersonas = {
    active_slot_id: "whale",
    auto_schedule: true,
    slots: [
      { id: "whale", name: "\u5927\u80A5\u9C7C (\u9CB8\u9C7C\u5A18)", badge: "\u{1F40B}", tone_description: "\u6E29\u548C\u5F00\u6717\u3001\u7231\u5403\u767D\u996D\uFF0C\u50CF\u597D\u670B\u53CB\u95F2\u804A" },
      { id: "companion", name: "\u77E5\u5FC3\u4F34\u4FA3", badge: "\u{1F319}", tone_description: "\u6E29\u67D4\u5171\u60C5\u3001\u677E\u5F1B\u968F\u6027\uFF0C\u5173\u6CE8\u5BF9\u65B9\u60C5\u7EEA" },
      { id: "jarvis", name: "\u9AD8\u6548\u5E55\u50DA", badge: "\u26A1", tone_description: "\u7406\u6027\u4E25\u8C28\u3001\u7ED3\u679C\u5BFC\u5411\u3001\u6761\u7406\u5206\u660E" },
      { id: "coder", name: "\u6781\u5BA2\u642D\u5B50", badge: "\u{1F4BB}", tone_description: "\u76F4\u5207\u8981\u70B9\u3001\u4EE3\u7801\u7CBE\u70BC\u3001\u6781\u5BA2\u7F8E\u5B66" }
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
        <span class="slot-badge">${s.badge || "\u{1F3AD}"}</span>
        <div style="min-width:0;">
          <div class="slot-name">${s.name}</div>
          <div class="slot-desc">${s.tone_description || ""}</div>
        </div>
      </div>
      <button class="badge-btn">${isAct ? "\u5F53\u524D\u6FC0\u6D3B" : "\u542F\u7528"}</button>
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
      item.textContent = `${isAct ? "\u2713 " : ""}${s.badge || ""} ${s.name}`;
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
  function updateBondUI(bond) {
    if (!bond) return;
    bondLevelText.textContent = bond.level_name || `Lv.${bond.level}`;
    bondStreakText.textContent = `\u{1F525} \u5DF2\u8FDE\u7EED\u966A\u4F34 ${bond.streak_days} \u5929`;
    if (bondStreakHeader) {
      bondStreakHeader.textContent = `\u8FDE\u7EED\u966A\u4F34 ${bond.streak_days} \u5929`;
    }
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
      subtitleText.title = text;
      showBubble("\u4F60\u8BF4: " + text, 5e3);
      applyState("thinking");
    });
    window.runtime.EventsOn("avatar:mic_energy", (energy) => {
      currentRms = energy;
      updateRmsMeter(energy);
      if (currentState === "speaking") {
        if (energy < 0.05) {
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
      showBubble(sentence, Math.max(4e3, sentence.length * 300));
    });
    window.runtime.EventsOn("avatar:error", (err) => {
      clearTimeout(thinkingWatchdog);
      subtitleText.textContent = "\u8FDE\u63A5\u5F02\u5E38: " + err;
      subtitleText.title = err;
      showBubble("\u8FDE\u63A5\u5F02\u5E38: " + err, 5e3);
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
    window.runtime.EventsOn("avatar:pet_action", (act) => {
      if (!act) return;
      resetSleepTimer();
      let roamStarted = false;
      if (act.move === "right") {
        roamStarted = triggerDirectedRoam(42);
      } else if (act.move === "left") {
        roamStarted = triggerDirectedRoam(-42);
      } else if (act.action === "roam") {
        const patrolPath = roamCurrentOffset > 0 ? [-42, 42, 0] : [42, -42, 0];
        roamStarted = startRoamSequence(patrolPath, "\u{1F43E} \u5DE1\u903B\u5954\u8DD1\u4E2D...");
      }
      const cue = act.cue || "jumping";
      if (!roamStarted) {
        petRenderer.triggerCue(cue, 1400);
      }
      const xpText = act.xp > 0 ? `+${act.xp} XP` : "";
      if (act.action === "feed" || act.action === "rice" || act.action === "turbo") {
        fishFedTotal += 1;
        if (fishFeedCount) fishFeedCount.textContent = `${fishFedTotal} \u6B21`;
        petRenderer.spawnHearts(100, 70, `\u{1F41F} ${xpText}`);
      } else if (act.action === "wave") {
        petRenderer.spawnHearts(100, 70, `\u{1F43E} ${xpText}`);
      } else if (act.action === "jump") {
        petRenderer.spawnHearts(100, 70, `\u2728 ${xpText}`);
      } else if (act.action === "grass") {
        petRenderer.spawnHearts(100, 70, `\u{1F33E} ${xpText}`);
      } else if (act.action === "sleep") {
        stopRoamingMove();
        applyState("sleep");
      } else {
        petRenderer.spawnHearts(100, 70, `\u2764\uFE0F ${xpText}`);
      }
      if (act.message) {
        showBubble(act.message, 3e3);
      }
      if (act.bond) {
        updateBondUI(act.bond);
      }
      if (!roamStarted) {
        updateAuditTrack(`Voice Action: ${act.action} -> ${cue}`);
      }
    });
  }
  function initApp() {
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.GetDisplayForm().then((form) => applyDisplayForm(form));
      window.go.main.App.GetPetSkin().then((skin) => applySkin(skin, false));
      window.go.main.App.GetMode().then((m) => {
        currentMode = m;
        modeBadge.textContent = m === "hands_free" ? "\u514D\u5524\u9192" : "\u5FEB\u6377\u952E";
        if (currentState === "idle") {
          subtitleText.textContent = m === "hands_free" ? "\u968F\u65F6\u5F00\u53E3\u4E0E\u6211\u7545\u804A..." : "\u6309\u4F4F\u7A7A\u683C\u8BF4\u8BDD...";
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
})();

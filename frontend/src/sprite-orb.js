// sprite-orb.js - 精灵桌宠动画状态模型与规格计算
// 兼容 Codex Pet 图集标准 (8 列 × 9 行 / 11 行，每格 192×208)

export const DEFAULT_COLUMNS = 8;
export const DEFAULT_FRAME_WIDTH = 192;
export const DEFAULT_FRAME_HEIGHT = 208;
export const DEFAULT_ROWS = 9;
export const V2_ROWS = 11;
export const DEFAULT_FPS = 8;
export const MAX_FPS = 60;

function idleAnimation() {
  const durations = [1680, 660, 660, 840, 840, 1920];
  return {
    frames: durations.map((durationMs, spriteIndex) => ({
      spriteIndex,
      durationMs,
    })),
    loopStart: 0,
    fallback: "idle",
  };
}

function rowAnimation(
  rowIndex,
  frameCount,
  frameDurationMs,
  finalFrameDurationMs
) {
  return {
    frames: Array.from({ length: frameCount }, (_, column) => ({
      spriteIndex: rowIndex * DEFAULT_COLUMNS + column,
      durationMs:
        column === frameCount - 1 ? finalFrameDurationMs : frameDurationMs,
    })),
    loopStart: null,
    fallback: "idle",
  };
}

export function defaultAnimations() {
  return {
    idle: idleAnimation(),
    "running-right": rowAnimation(1, 8, 120, 220),
    "running-left": rowAnimation(2, 8, 120, 220),
    waving: rowAnimation(3, 4, 140, 280),
    jumping: rowAnimation(4, 5, 140, 280),
    failed: rowAnimation(5, 8, 140, 240),
    waiting: rowAnimation(6, 6, 150, 260),
    running: rowAnimation(7, 6, 120, 220),
    review: rowAnimation(8, 6, 150, 280),
  };
}

// 业务表现状态映射到 Codex Pet 标准动作轨道
export const PET_STATE_ANIMATION_POLICY = Object.freeze({
  idle: { name: "idle", mode: "loop" },
  sleep: { name: "idle", mode: "loop" },
  listening: { name: "waiting", mode: "loop", blocksCues: true },
  thinking: { name: "review", mode: "loop", blocksCues: true },
  speaking: { name: "waving", mode: "loop", blocksCues: true },
  error: { name: "failed", mode: "once", blocksCues: true },
});

export const PET_EVENT_ANIMATION_POLICY = Object.freeze({
  hover: "jumping",
  click: "jumping",
  wake: "jumping",
  pet: "jumping",
  feed: "jumping",
});

export function spriteAnimationForEvent(event) {
  return PET_EVENT_ANIMATION_POLICY[event] || null;
}

export function spritePlaybackSelection({
  state,
  baseWorking = false,
  dragDirection = "",
  cue = null,
} = {}) {
  const fallback = baseWorking || state === "working" ? "running" : "idle";
  const statePolicy =
    PET_STATE_ANIMATION_POLICY[state] || { name: fallback, mode: "loop" };

  if (dragDirection === "left" || dragDirection === "right") {
    return {
      name: `running-${dragDirection}`,
      key: `drag:${dragDirection}`,
      loop: true,
      completion: "none",
      fallback,
    };
  }

  if (
    cue?.id &&
    cue.name &&
    (!statePolicy.blocksCues || cue.name === statePolicy.name)
  ) {
    return {
      name: cue.name,
      key: `cue:${cue.id}`,
      loop: false,
      completion: "cue",
      completionId: cue.id,
      fallback,
    };
  }

  return {
    name: statePolicy.name,
    key: `state:${state}:${statePolicy.name}`,
    loop: statePolicy.mode === "loop",
    completion: "none",
    completionId: null,
    fallback,
  };
}

export function spriteGeometry(manifest = {}) {
  const version = manifest.spriteVersionNumber ?? 1;
  const frame =
    manifest.frame && typeof manifest.frame === "object"
      ? manifest.frame
      : {
          width: DEFAULT_FRAME_WIDTH,
          height: DEFAULT_FRAME_HEIGHT,
          columns: DEFAULT_COLUMNS,
          rows: version === 2 ? V2_ROWS : DEFAULT_ROWS,
        };

  for (const key of ["width", "height", "columns", "rows"]) {
    if (!Number.isInteger(frame[key]) || frame[key] <= 0) return null;
  }

  return {
    width: frame.width,
    height: frame.height,
    columns: frame.columns,
    rows: frame.rows,
    frameCount: frame.columns * frame.rows,
  };
}

export function frameRect(geometry, spriteIndex) {
  return {
    x: (spriteIndex % geometry.columns) * geometry.width,
    y: Math.floor(spriteIndex / geometry.columns) * geometry.height,
    width: geometry.width,
    height: geometry.height,
  };
}

export function resolveAnimations(manifest = {}, frameCount = 0) {
  const animations = defaultAnimations();
  const specs = manifest.animations;
  if (specs && typeof specs === "object" && !Array.isArray(specs)) {
    for (const [name, spec] of Object.entries(specs)) {
      if (!spec || !Array.isArray(spec.frames) || spec.frames.length === 0) {
        continue;
      }
      const fps = spec.fps === undefined ? DEFAULT_FPS : spec.fps;
      if (!Number.isFinite(fps) || fps <= 0 || fps > MAX_FPS) {
        continue;
      }
      const durationMs = 1000 / fps;
      animations[name] = {
        frames: spec.frames.map((spriteIndex) => ({ spriteIndex, durationMs })),
        loopStart: name === "idle" ? 0 : null,
        fallback: "idle",
      };
    }
  }

  return animations;
}

function totalDuration(animation) {
  return animation.frames.reduce((sum, frame) => sum + frame.durationMs, 0);
}

export function frameAtElapsed(animation, elapsedMs) {
  const total = totalDuration(animation);
  if (total <= 0) return null;
  let position = Math.max(0, elapsedMs);
  if (position >= total) {
    if (animation.loopStart === null) return null;
    const introDuration = animation.frames
      .slice(0, animation.loopStart)
      .reduce((sum, frame) => sum + frame.durationMs, 0);
    const loopDuration = total - introDuration;
    if (loopDuration <= 0) return null;
    position = introDuration + ((position - total) % loopDuration);
  }
  let cursor = 0;
  for (const frame of animation.frames) {
    if (position < cursor + frame.durationMs) {
      return {
        spriteIndex: frame.spriteIndex,
        remainingMs: cursor + frame.durationMs - position,
      };
    }
    cursor += frame.durationMs;
  }
  return null;
}

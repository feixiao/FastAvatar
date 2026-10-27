// pet-renderer.js - 桌面宠物动画播放器与粒子交互引擎
import {
  defaultAnimations,
  resolveAnimations,
  spriteGeometry,
  frameRect,
  frameAtElapsed,
  spritePlaybackSelection,
  DEFAULT_FRAME_WIDTH,
  DEFAULT_FRAME_HEIGHT,
} from "./sprite-orb.js";
import { getBuiltinPetSpritesheet } from "./pet-generator.js";

export class PetRenderer {
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

    // 默认几何与动画配置
    this.geometry = spriteGeometry({});
    this.animations = defaultAnimations();

    this.loadSkin(this.skinId);
  }

  loadSkin(skinId, customSpritesheetPath = null, customManifest = null) {
    this.skinId = skinId;

    if (customSpritesheetPath && customManifest) {
      // 加载自定义外部皮肤
      const img = new Image();
      img.onload = () => {
        this.geometry = spriteGeometry(customManifest) || spriteGeometry({});
        this.animations = resolveAnimations(customManifest, this.geometry.frameCount);
        this.assets = { image: img, geometry: this.geometry, animations: this.animations };
        this.resetTrack();
      };
      img.onerror = () => {
        // 回退到内置
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
      animations: this.animations,
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
      name,
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
      cue: this.cue,
    });

    const activePlayback = this.playback;
    const selected =
      this.assets.animations[activePlayback.name] || this.assets.animations.idle;

    this.track = {
      ...selected,
      loopStart: activePlayback.loop ? 0 : null,
      fallback: activePlayback.fallback || "idle",
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
      // 动作单次播放完成，切入 fallback 轨道
      const fallbackName = this.track.fallback || "idle";
      const fallback =
        this.assets.animations[fallbackName] || this.assets.animations.idle;
      this.track = {
        ...fallback,
        loopStart: 0,
        fallback: "idle",
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

    const icons = ["❤️", "💖", "✨", "🐾", "🌸"];
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
      }, 1000);
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
}

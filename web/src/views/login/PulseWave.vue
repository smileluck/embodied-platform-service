<template>
  <svg class="pulse-wave" viewBox="0 0 560 80" preserveAspectRatio="none" aria-hidden="true">
    <line x1="0" y1="40" x2="560" y2="40" class="pulse-base" />
    <!-- 波形：进入时画一次，之后保持常驻；时长绑定 TRACE_S 单一来源 -->
    <path class="pulse-trace" :d="D" :style="{ animationDuration: `${TRACE_S}s` }" />
    <!-- 光束粒子：每条速度 ±15% 随机、初始相位随机，JS 逐帧驱动 -->
    <template v-for="(b, i) in beams" :key="i">
      <path :ref="el => setEl(softEls, i, el)" class="pulse-beam" :d="D" />
      <path :ref="el => setEl(coreEls, i, el)" class="pulse-beam pulse-beam--core" :d="D" />
    </template>
    <!-- 波峰/波谷高亮点：初始描线或光束前沿扫过时闪亮后衰减（JS 驱动 opacity） -->
    <g>
      <circle v-for="(cx, j) in DOT_CX" :key="j" :ref="el => setEl(dotEls, j, el)" :cx="cx" :cy="DOT_CY[j]" r="3"
        class="pulse-dot" />
    </g>
  </svg>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'

/* 波形几何与路径度量（viewBox 单位） */
const D = 'M0 40 H60 H80 L95 12 L110 66 L125 40 H190 H210 L225 12 L240 66 L255 40 H330 H350 L365 12 L380 66 L395 40 H470 H490 L505 12 L520 66 L535 40 H560'
const PATH_LEN = 851 // 折线累计长度（勾股近似）
const DASH = 44 // 光束柔光段长度
const CORE_GAP = 24 // 亮核相对柔光段前沿的后移量
const PERIOD = 1244 // dasharray 周期（44 + 1200），大于全线长保证每条仅一束
const TRACE_S = 1.9 // 初始描线时长；光束在描线触碰最右端后才开始发射
const SWEEP_S = 6.5 // 基准周期，实际每条 ±15% 随机
const EMISSION_S = 1.1 // 相邻光束发射间隔基准（约等于 1/4 波形长的穿行时间）
const FLASH_S = 0.5 // 转折点闪亮衰减时长
const HIDDEN_POS = 1000 // 路径外位置（未发射时光束藏于此，与 CSS 基线 offset 一致）

const DOT_CX = [95, 110, 225, 240, 365, 380, 505, 520]
const DOT_CY = [12, 66, 12, 66, 12, 66, 12, 66]
const DOT_S = [111.8, 167.8, 314.6, 370.6, 527.4, 583.4, 740.2, 796.2] // 各点沿路径累计长度

/* 光束队列：速度、发射间隔各带 ±15% 随机差；描线完成后再逐条发射 */
const beams = Array.from({ length: 4 }, (_, i) => ({
  dur: SWEEP_S * (1 + (Math.random() * 2 - 1) * 0.15),
  delay: TRACE_S + i * EMISSION_S * (1 + (Math.random() * 2 - 1) * 0.15),
}))

const softEls: (SVGPathElement | null)[] = []
const coreEls: (SVGPathElement | null)[] = []
const dotEls: (SVGCircleElement | null)[] = []
function setEl<T>(arr: T[], i: number, el: unknown) {
  arr[i] = el as T
}

let rafId = 0
let lastNow = 0
let elapsed = 0 // 自身时钟：挂载后归零推进，隐藏页期间不计时
let prevTraceFront = 0 // 初始描线前沿的上帧位置
const TRACE_RATE = PATH_LEN / TRACE_S // 描线前沿速度
const lastFlash = DOT_S.map(() => -FLASH_S * 1000)
const prevFront = beams.map(() => 0)
const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches

function frame(now: number) {
  if (!lastNow) lastNow = now
  const dt = Math.min((now - lastNow) / 1000, 0.05)
  lastNow = now
  elapsed += dt

  // 初始描线：前沿扫过转折点时点亮（仅描线阶段）
  if (elapsed < TRACE_S) {
    const front = TRACE_RATE * elapsed
    DOT_S.forEach((s, j) => {
      if (s > prevTraceFront && s <= front) lastFlash[j] = now
    })
    prevTraceFront = front
  }

  beams.forEach((b, i) => {
    // 未到发射时刻：光束藏于路径外
    const pos = elapsed >= b.delay ? (((elapsed - b.delay) / b.dur) % 1) * PERIOD : HIDDEN_POS
    const front = pos + DASH
    const soft = softEls[i]
    const core = coreEls[i]
    if (soft) soft.style.strokeDashoffset = `${PERIOD - pos}`
    if (core) core.style.strokeDashoffset = `${PERIOD - pos - CORE_GAP}`
    // 光束前沿扫过转折点则触发闪亮（front 发生回绕时分两段判断）
    if (elapsed >= b.delay) {
      DOT_S.forEach((s, j) => {
        const crossed = front >= prevFront[i]
          ? s > prevFront[i] && s <= front
          : s > prevFront[i] || s <= front
        if (crossed) lastFlash[j] = now
      })
    }
    prevFront[i] = front
  })
  dotEls.forEach((el, j) => {
    if (!el) return
    const since = (now - lastFlash[j]) / 1000
    el.style.opacity = `${Math.max(0, 1 - since / FLASH_S)}`
  })
  rafId = requestAnimationFrame(frame)
}

/* 标签页隐藏时挂起循环，回来自动续播（自身时钟不跳变） */
function onVisibility() {
  if (document.hidden) {
    cancelAnimationFrame(rafId)
    rafId = 0
  } else if (!rafId) {
    rafId = requestAnimationFrame(frame)
  }
}

onMounted(() => {
  if (reducedMotion) return // 降级：波形常显，光束与点保持隐藏（CSS 基线已藏于路径外）
  lastNow = 0
  rafId = requestAnimationFrame(frame)
  document.addEventListener('visibilitychange', onVisibility)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(rafId)
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<style scoped>
.pulse-wave {
  width: 100%;
  height: 72px;
  display: block;
}

.pulse-base {
  stroke: rgba(255, 255, 255, 0.12);
  stroke-width: 1;
}

/* 波形：进场由左往右画一次（约 1.9s）后保持常驻，不再整条重发 */
.pulse-trace {
  fill: none;
  stroke: var(--sx-accent-bright);
  stroke-width: 2.5;
  stroke-linejoin: round;
  stroke-linecap: round;
  stroke-dasharray: 1200;
  stroke-dashoffset: 349; /* 降级兜底：无动画时全线常显 */
  opacity: 0.9;
  animation: trace-once linear 1 forwards; /* 时长由模板内联绑定 TRACE_S */
}
@keyframes trace-once {
  0% { stroke-dashoffset: 1200; opacity: 0.4; }
  100% { stroke-dashoffset: 349; opacity: 0.9; }
}

/* 光束粒子：柔光段 + 亮核两层；offset 由 JS 按各自速度逐帧写入，
   基线 offset 藏于路径外（兼顾降级时不可见） */
.pulse-beam {
  fill: none;
  stroke: var(--sx-accent-bright);
  stroke-width: 5;
  stroke-linecap: round;
  stroke-dasharray: 44 1200;
  stroke-dashoffset: 244;
  opacity: 0.35;
  filter: drop-shadow(0 0 4px rgba(var(--sx-accent-bright-rgb), 0.7));
}
.pulse-beam--core {
  stroke-width: 2;
  stroke-dasharray: 20 1224;
  stroke-dashoffset: 220;
  opacity: 1;
  filter: drop-shadow(0 0 3px rgba(var(--sx-accent-bright-rgb), 0.9));
}

/* 转折点：opacity 由 JS 驱动，基线熄灭 */
.pulse-dot {
  fill: var(--sx-accent-bright);
  opacity: 0;
  filter: drop-shadow(0 0 3px rgba(var(--sx-accent-bright-rgb), 0.9));
}
</style>

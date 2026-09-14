<template>
  <canvas ref="canvasRef" class="mapflow-canvas"></canvas>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

/*
 * 伪 3D 地图发射效果：透视倾斜的点阵中国地图 + 城市呼吸光柱/上行发射 + 城市间飞线。
 * 纯 Canvas 手写透视投影，地图轮廓为硬编码经纬度多边形，无外部地图资源。
 */
/* 高亮色运行时读取 tokens 的 --sx-accent-bright（hex → RGB 三元组），保持主题单一来源 */
function readAccentBright(): string {
  const hex = getComputedStyle(document.documentElement).getPropertyValue('--sx-accent-bright').trim()
  if (/^#[0-9a-fA-F]{6}$/.test(hex)) {
    return `${parseInt(hex.slice(1, 3), 16)}, ${parseInt(hex.slice(3, 5), 16)}, ${parseInt(hex.slice(5, 7), 16)}`
  }
  return '143, 197, 232' // 兜底：浅清水蓝
}
const SKY = readAccentBright()

/* —— 地图轮廓（[经度, 纬度]，手工简化的中国形状）—— */
const CHINA: [number, number][] = [
  [73, 40], [80, 45], [83, 48], [90, 49], [95, 45], [100, 42], [105, 41], [110, 42],
  [112, 45], [115, 45], [117, 46], [121, 51], [122, 53], [125, 53], [127, 50], [131, 49],
  [134, 48], [135, 45], [133, 44], [131, 43], [128, 41], [124, 40], [122, 39], [118, 39],
  [119, 37], [122, 37], [119, 35], [121, 32], [122, 31], [120, 28], [120, 26], [117, 24],
  [114, 22], [110, 21], [110, 20], [108, 21], [104, 22], [101, 21], [99, 23], [98, 26],
  [97, 28], [92, 27], [88, 28], [85, 29], [81, 30], [78, 33], [75, 36], [73, 39],
]
const HAINAN: [number, number][] = [[108.8, 19.3], [109.8, 18.3], [110.8, 19.6], [111, 20.2], [109.6, 20.4]]
const TAIWAN: [number, number][] = [[120.1, 23.6], [121.2, 25.2], [122, 24.7], [121.6, 22.5], [120.3, 22.4]]

/* 城市：[经度, 纬度, 光柱高度（平面 z 单位）] */
const HUBS_LL: [number, number, number][] = [
  [87.6, 43.8, 40],  // 乌鲁木齐
  [116.4, 39.9, 62], // 北京
  [121.5, 31.2, 78], // 上海（主枢纽，最高）
  [104.1, 30.7, 54], // 成都
  [113.3, 23.1, 46], // 广州
]
const ARCS: [number, number][] = [[0, 1], [1, 2], [3, 2], [4, 2], [3, 4]]

interface Dot { x: number; y: number; r: number; a: number }
interface Hub { x: number; y: number; s: number; beam: number; phase: number; speed: number; activity: number; launchT: number; nextLaunch: number }
interface Arc { pts: [number, number][]; lens: number[]; total: number }
interface Packet { arc: number; d: number; dir: 1 | -1; speed: number }
interface Ripple { hub: number; t: number }

const FAR_S = 0.45 // 远端（北）缩放
const NEAR_S = 1 // 近端（南）缩放
const MAX_PACKETS = 5

const canvasRef = ref<HTMLCanvasElement | null>(null)
let ctx: CanvasRenderingContext2D | null = null
let rafId = 0
let observer: ResizeObserver | null = null
let W = 0
let H = 0
let polygons: [number, number][][] = []
let dots: Dot[] = []
let hubs: Hub[] = []
let arcs: Arc[] = []
let packets: Packet[] = []
let ripples: Ripple[] = []
let spawnTimer = 0.4
let lastTime = 0
const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches

/* 经纬度 → 平面归一化坐标：pu 自西向东，pv 自南向北（北 = 画面远端） */
const toPlane = (lon: number, lat: number): [number, number] => [(lon - 72) / 64, (lat - 17) / 37]
const scaleAt = (pv: number) => FAR_S + (NEAR_S - FAR_S) * (1 - pv)
const easeOutCubic = (t: number) => 1 - Math.pow(1 - t, 3)

/* 透视投影：z 为离地高度（平面单位），高度同样按纵深缩放 */
function project(pu: number, pv: number, z: number): [number, number] {
  const s = scaleAt(pv)
  return [W / 2 + (pu - 0.5) * W * 0.96 * s, (H - 4) + (28 - (H - 4)) * pv - z * s]
}

function inPoly(x: number, y: number, poly: [number, number][]): boolean {
  let inside = false
  for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    const [xi, yi] = poly[i]
    const [xj, yj] = poly[j]
    if ((yi > y) !== (yj > y) && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi) inside = !inside
  }
  return inside
}

/* 依据当前尺寸重建静态场景：轮廓投影、点阵、城市、飞线骨架 */
function buildScene() {
  polygons = [CHINA, HAINAN, TAIWAN].map(p => p.map(([lon, lat]) => toPlane(lon, lat)))

  dots = []
  for (let i = 0; i <= 34; i++) {
    const pu = 0.02 + (i / 34) * 0.96
    for (let j = 0; j <= 26; j++) {
      const pv = 0.03 + (j / 26) * 0.94
      if (polygons.some(p => inPoly(pu, pv, p))) {
        const [x, y] = project(pu, pv, 0)
        const s = scaleAt(pv)
        dots.push({ x, y, r: 0.9 * s + 0.35, a: 0.13 + 0.17 * s })
      }
    }
  }

  hubs = HUBS_LL.map(([lon, lat, beam]) => {
    const [pu, pv] = toPlane(lon, lat)
    const [x, y] = project(pu, pv, 0)
    return {
      x,
      y,
      s: scaleAt(pv),
      beam,
      phase: Math.random() * Math.PI * 2,
      speed: 0.5 + Math.random() * 0.4,
      activity: 0,
      launchT: -1,
      nextLaunch: 1 + Math.random() * 2.5,
    }
  })

  arcs = ARCS.map(([ai, bi]) => {
    const [u0, v0] = toPlane(HUBS_LL[ai][0], HUBS_LL[ai][1])
    const [u1, v1] = toPlane(HUBS_LL[bi][0], HUBS_LL[bi][1])
    const um = (u0 + u1) / 2
    const vm = (v0 + v1) / 2
    const arcH = 14 + Math.hypot(u1 - u0, v1 - v0) * 26 // 控制点抬升高度随距离增大
    const N = 26
    const pts: [number, number][] = []
    for (let k = 0; k <= N; k++) {
      const t = k / N
      const mt = 1 - t
      pts.push(project(
        mt * mt * u0 + 2 * mt * t * um + t * t * u1,
        mt * mt * v0 + 2 * mt * t * vm + t * t * v1,
        2 * mt * t * arcH,
      ))
    }
    const lens = [0]
    for (let k = 1; k <= N; k++) lens.push(lens[k - 1] + Math.hypot(pts[k][0] - pts[k - 1][0], pts[k][1] - pts[k - 1][1]))
    return { pts, lens, total: lens[N] }
  })
}

/* 沿飞线按已走像素距离取点（匀速） */
function arcPos(arc: Arc, d: number): [number, number] {
  const dd = Math.max(0, Math.min(d, arc.total))
  let k = 1
  while (k < arc.lens.length - 1 && arc.lens[k] < dd) k++
  const seg = (dd - arc.lens[k - 1]) / (arc.lens[k] - arc.lens[k - 1] || 1)
  const p0 = arc.pts[k - 1]
  const p1 = arc.pts[k]
  return [p0[0] + (p1[0] - p0[0]) * seg, p0[1] + (p1[1] - p0[1]) * seg]
}

function draw(time: number) {
  const g = ctx // 局部常量让 TS 的非空收窄能传入下方回调
  if (!g || W === 0 || H === 0) return
  g.clearRect(0, 0, W, H)

  // 地图轮廓（投影后的多边形）
  g.lineWidth = 1
  g.strokeStyle = `rgba(${SKY}, 0.16)`
  polygons.forEach(p => {
    g.beginPath()
    p.forEach(([pu, pv], i) => {
      const [x, y] = project(pu, pv, 0)
      if (i === 0) g.moveTo(x, y)
      else g.lineTo(x, y)
    })
    g.closePath()
    g.stroke()
  })

  // 点阵（近大远小、近亮远暗）
  dots.forEach(d => {
    g.beginPath()
    g.arc(d.x, d.y, d.r, 0, Math.PI * 2)
    g.fillStyle = `rgba(${SKY}, ${d.a})`
    g.fill()
  })

  // 飞线骨架
  g.strokeStyle = `rgba(${SKY}, 0.08)`
  arcs.forEach(a => {
    g.beginPath()
    a.pts.forEach(([x, y], i) => (i === 0 ? g.moveTo(x, y) : g.lineTo(x, y)))
    g.stroke()
  })

  // 城市光柱：呼吸高度 + 顶部渐隐，activity 时增亮；偶发上行发射光点
  hubs.forEach(h => {
    const hpx = h.beam * (0.82 + 0.18 * Math.sin(time * h.speed + h.phase)) * h.s
    const beam = g.createLinearGradient(h.x, h.y, h.x, h.y - hpx)
    beam.addColorStop(0, `rgba(${SKY}, ${0.4 + h.activity * 0.4})`)
    beam.addColorStop(1, `rgba(${SKY}, 0)`)
    g.strokeStyle = beam
    g.lineWidth = 4
    g.beginPath()
    g.moveTo(h.x, h.y)
    g.lineTo(h.x, h.y - hpx)
    g.stroke()
    g.lineWidth = 1.6
    g.stroke()

    if (h.launchT >= 0) {
      const zt = easeOutCubic(Math.min(h.launchT, 1)) * hpx
      const ly = h.y - zt
      const fade = h.launchT < 0.7 ? 1 : 1 - (h.launchT - 0.7) / 0.3
      g.strokeStyle = `rgba(${SKY}, ${0.35 * fade})`
      g.lineWidth = 1.4
      g.beginPath()
      g.moveTo(h.x, ly + 10)
      g.lineTo(h.x, ly)
      g.stroke()
      g.beginPath()
      g.arc(h.x, ly, 1.9, 0, Math.PI * 2)
      g.fillStyle = `rgba(${SKY}, ${0.95 * fade})`
      g.fill()
    }
  })

  // 飞线数据包：短尾迹 + 亮头
  g.lineWidth = 1.4
  packets.forEach(p => {
    const arc = arcs[p.arc]
    const [hx, hy] = arcPos(arc, p.d)
    const [tx, ty] = arcPos(arc, p.d - p.dir * 14)
    g.strokeStyle = `rgba(${SKY}, 0.4)`
    g.beginPath()
    g.moveTo(tx, ty)
    g.lineTo(hx, hy)
    g.stroke()
    g.beginPath()
    g.arc(hx, hy, 2, 0, Math.PI * 2)
    g.fillStyle = `rgba(${SKY}, 0.95)`
    g.fill()
  })

  // 到达涟漪（贴地的扁椭圆，强化透视感）
  ripples.forEach(rp => {
    const h = hubs[rp.hub]
    const r = 3 + rp.t * 13
    g.beginPath()
    g.ellipse(h.x, h.y, r * h.s, r * 0.5 * h.s, 0, 0, Math.PI * 2)
    g.strokeStyle = `rgba(${SKY}, ${(1 - rp.t) * 0.5})`
    g.stroke()
  })

  // 城市点：柔光 + 实心（最后画，压在光柱之上）
  hubs.forEach(h => {
    g.beginPath()
    g.arc(h.x, h.y, 3 * h.s + 1, 0, Math.PI * 2)
    g.fillStyle = `rgba(${SKY}, ${0.16 + h.activity * 0.3})`
    g.fill()
    g.beginPath()
    g.arc(h.x, h.y, 1.6 * h.s + 0.9, 0, Math.PI * 2)
    g.fillStyle = `rgba(${SKY}, ${0.85 + h.activity * 0.15})`
    g.fill()
  })
}

function frame(now: number) {
  const dt = Math.min((now - lastTime) / 1000, 0.05)
  lastTime = now
  const time = now / 1000

  // 飞线数据包：随机边、随机方向
  spawnTimer -= dt
  if (spawnTimer <= 0) {
    if (arcs.length && packets.length < MAX_PACKETS) {
      const arc = Math.floor(Math.random() * arcs.length)
      const dir: 1 | -1 = Math.random() < 0.5 ? 1 : -1
      packets.push({ arc, d: dir > 0 ? 0 : arcs[arc].total, dir, speed: 95 + Math.random() * 30 })
    }
    spawnTimer = 0.6 + Math.random() * 0.5
  }
  packets = packets.filter(p => {
    const arc = arcs[p.arc]
    p.d += p.dir * p.speed * dt
    if (p.d > arc.total || p.d < 0) {
      const dest = p.dir > 0 ? ARCS[p.arc][1] : ARCS[p.arc][0]
      hubs[dest].activity = 1
      ripples.push({ hub: dest, t: 0 })
      return false
    }
    return true
  })

  // 城市发射：待机倒计时 → 0.8s 上行（末段淡出）
  hubs.forEach(h => {
    if (h.launchT >= 0) {
      h.launchT += dt / 0.8
      if (h.launchT >= 1.15) h.launchT = -1
    } else {
      h.nextLaunch -= dt
      if (h.nextLaunch <= 0) {
        h.launchT = 0
        h.nextLaunch = 2.4 + Math.random() * 2.6
      }
    }
  })

  ripples = ripples.filter(r => (r.t += dt / 0.6) < 1)
  hubs.forEach(h => { h.activity *= Math.exp(-3 * dt) })

  draw(time)
  rafId = requestAnimationFrame(frame)
}

function resize() {
  const canvas = canvasRef.value
  if (!canvas) return
  const rect = canvas.getBoundingClientRect()
  const dpr = Math.min(window.devicePixelRatio || 1, 2)
  W = rect.width
  H = rect.height
  canvas.width = Math.round(W * dpr)
  canvas.height = Math.round(H * dpr)
  ctx?.setTransform(dpr, 0, 0, dpr, 0, 0)
  buildScene() // 投影依赖尺寸，尺寸变化即重建
}

/* 标签页隐藏时挂起循环，回来自动续播 */
function onVisibility() {
  if (document.hidden) {
    cancelAnimationFrame(rafId)
    rafId = 0
  } else if (!rafId) {
    lastTime = performance.now()
    rafId = requestAnimationFrame(frame)
  }
}

onMounted(() => {
  const canvas = canvasRef.value
  if (!canvas) return
  ctx = canvas.getContext('2d')
  resize()
  observer = new ResizeObserver(() => {
    resize()
    if (reducedMotion) draw(0) // 静态模式：尺寸变化后重绘单帧
  })
  observer.observe(canvas)
  if (reducedMotion) {
    draw(0) // 降级：仅静态地图与光柱，无发射与飞线
  } else {
    lastTime = performance.now()
    rafId = requestAnimationFrame(frame)
    document.addEventListener('visibilitychange', onVisibility)
  }
})

onBeforeUnmount(() => {
  cancelAnimationFrame(rafId)
  observer?.disconnect()
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<style scoped>
.mapflow-canvas {
  flex: 1;
  min-height: 0;
  width: 100%;
  display: block;
}
</style>

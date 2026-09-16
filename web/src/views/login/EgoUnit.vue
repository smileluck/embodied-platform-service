<!-- 具身单元示意图（登录品牌面板签名元素）：侧视线描 AMR 巡检机器人。
     线稿用 --sx-accent-bright 高亮琥珀，附工程标注引线（LIDAR / EGO-CAM / IMU / BAT / DRIVE）
     与微动效（雷达扫描、相机录制环、整机悬浮）。纯 SVG + CSS，无外部资产。 -->
<template>
  <svg class="ego-unit" viewBox="0 0 520 330" fill="none" aria-hidden="true">
    <!-- 地面与行进轨迹 -->
    <line x1="36" y1="282" x2="484" y2="282" class="ground" />
    <line x1="52" y1="270" x2="92" y2="270" class="trail" />
    <line x1="30" y1="270" x2="42" y2="270" class="trail trail--dim" />

    <!-- 机身（悬浮微动效；底盘件不参与） -->
    <g class="unit-bob">
      <!-- 激光雷达：底座 + 顶盖 + 扫描扇面 -->
      <rect x="250" y="118" width="44" height="34" rx="6" class="stroke" />
      <g class="lidar-sweep">
        <path d="M272 118 L272 92 A26 26 0 0 1 290.4 99.6 Z" class="sweep-wedge" />
        <line x1="272" y1="118" x2="272" y2="92" class="sweep-line" />
      </g>
      <ellipse cx="272" cy="118" rx="22" ry="6.5" class="stroke lidar-top" />

      <!-- EGO 相机：支架 + 镜头 + 录制环 -->
      <line x1="379" y1="146" x2="379" y2="152" class="stroke" />
      <rect x="366" y="126" width="26" height="22" rx="5" class="stroke" />
      <circle cx="379" cy="137" r="7.5" class="stroke" />
      <circle cx="379" cy="137" r="3" class="pupil" />
      <circle cx="379" cy="137" r="10.5" class="cam-ring" />

      <!-- 车身 -->
      <rect x="168" y="150" width="224" height="96" rx="18" class="panel" />
      <line x1="188" y1="178" x2="360" y2="178" class="seam" />
      <!-- 状态灯条：运行 / 就绪 / 备用 -->
      <circle cx="200" cy="206" r="3" class="led led--ok" />
      <circle cx="214" cy="206" r="3" class="led led--amber" />
      <circle cx="228" cy="206" r="3" class="led led--dim" />
      <text x="200" y="232" class="body-tag">SMILEX · UNIT-01</text>
    </g>

    <!-- 底盘件（接地，不悬浮） -->
    <circle cx="214" cy="260" r="20" class="wheel" />
    <circle cx="214" cy="260" r="7" class="stroke" />
    <circle cx="214" cy="260" r="2" class="pupil" />
    <circle cx="346" cy="260" r="20" class="wheel" />
    <circle cx="346" cy="260" r="7" class="stroke" />
    <circle cx="346" cy="260" r="2" class="pupil" />
    <circle cx="280" cy="271" r="7" class="caster" />
    <ellipse cx="280" cy="284" rx="150" ry="3.5" class="shadow" />

    <!-- 工程标注引线 -->
    <!-- LIDAR 360° -->
    <circle cx="288" cy="116" r="2.2" class="dot" />
    <path d="M288 116 L318 92 L336 92" class="leader" />
    <text x="342" y="96" class="lbl">LIDAR <tspan class="dim">360°</tspan></text>
    <!-- EGO-CAM 01 -->
    <circle cx="390" cy="136" r="2.2" class="dot" />
    <path d="M390 136 L420 118 L438 118" class="leader" />
    <text x="442" y="122" class="lbl">EGO-CAM <tspan class="dim">01</tspan></text>
    <!-- IMU 9-AXIS -->
    <circle cx="222" cy="164" r="2.2" class="dot" />
    <path d="M222 164 L180 120 L162 120" class="leader" />
    <text x="156" y="124" text-anchor="end" class="lbl">IMU <tspan class="dim">9-AXIS</tspan></text>
    <!-- BAT 78% -->
    <circle cx="196" cy="224" r="2.2" class="dot" />
    <path d="M196 224 L156 236 L138 236" class="leader" />
    <text x="132" y="240" text-anchor="end" class="lbl">BAT <tspan class="dim">78%</tspan></text>
    <!-- DRIVE J0·J1 -->
    <circle cx="210" cy="276" r="2.2" class="dot" />
    <path d="M210 276 L176 300 L158 300" class="leader" />
    <text x="152" y="304" text-anchor="end" class="lbl">DRIVE <tspan class="dim">J0·J1</tspan></text>
  </svg>
</template>

<style scoped>
.ego-unit {
  display: block;
  width: 100%;
  height: auto;
}

/* 线稿基色：高亮琥珀（随主题 token） */
.stroke {
  stroke: var(--sx-accent-bright);
  stroke-width: 1.6;
  fill: rgba(23, 25, 30, 0.65);
}
.panel {
  stroke: var(--sx-accent-bright);
  stroke-width: 1.8;
  fill: rgba(255, 178, 36, 0.05);
}
.lidar-top {
  fill: rgba(23, 25, 30, 0.92);
}
.seam {
  stroke: rgba(255, 178, 36, 0.35);
  stroke-width: 1;
}
.pupil {
  fill: var(--sx-accent-bright);
}
.ground {
  stroke: rgba(233, 231, 226, 0.18);
  stroke-width: 1;
}
.trail {
  stroke: rgba(255, 178, 36, 0.35);
  stroke-width: 2;
  stroke-linecap: round;
}
.trail--dim {
  opacity: 0.45;
}
.wheel {
  stroke: var(--sx-accent-bright);
  stroke-width: 1.6;
  fill: rgba(23, 25, 30, 0.92);
}
.caster {
  stroke: rgba(255, 178, 36, 0.7);
  stroke-width: 1.2;
  fill: rgba(23, 25, 30, 0.92);
}
.shadow {
  fill: rgba(0, 0, 0, 0.32);
}
.led--ok { fill: var(--sx-ok-bright); }
.led--amber { fill: var(--sx-accent-bright); }
.led--dim { fill: rgba(233, 231, 226, 0.25); }
.body-tag {
  font-family: var(--sx-font-mono);
  font-size: 9px;
  letter-spacing: 0.14em;
  fill: rgba(233, 231, 226, 0.4);
}

/* 标注引线与标签 */
.leader {
  stroke: rgba(255, 178, 36, 0.55);
  stroke-width: 1;
  fill: none;
}
.dot {
  fill: var(--sx-accent-bright);
}
.lbl {
  font-family: var(--sx-font-mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.08em;
  fill: rgba(233, 231, 226, 0.85);
}
.lbl .dim {
  fill: rgba(233, 231, 226, 0.45);
  font-weight: 400;
}

/* 微动效：雷达扫描 / 相机录制环 / 整机悬浮 */
.lidar-sweep {
  transform-box: view-box;
  transform-origin: 272px 118px;
  animation: ego-sweep 3.6s linear infinite;
}
@keyframes ego-sweep {
  to { transform: rotate(360deg); }
}
.sweep-wedge {
  fill: rgba(255, 178, 36, 0.16);
}
.sweep-line {
  stroke: rgba(255, 178, 36, 0.75);
  stroke-width: 1.2;
}
.cam-ring {
  stroke: var(--sx-ok-bright);
  stroke-width: 1;
  fill: none;
  animation: ego-rec 2.4s ease-in-out infinite;
}
@keyframes ego-rec {
  0%, 100% { opacity: 0.85; }
  50% { opacity: 0.15; }
}
.unit-bob {
  transform-box: view-box;
  transform-origin: 280px 200px;
  animation: ego-bob 6s ease-in-out infinite;
}
@keyframes ego-bob {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(3px); }
}
</style>

<template>
  <div ref="wrap" class="trend" @mousemove="onMove" @mouseleave="hover = -1">
    <svg :viewBox="`0 0 ${W} ${H}`" preserveAspectRatio="none" class="svg">
      <defs>
        <linearGradient id="trendFill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="#ff7a18" stop-opacity=".38" />
          <stop offset="60%" stop-color="#ff3d71" stop-opacity=".1" />
          <stop offset="100%" stop-color="#ff3d71" stop-opacity="0" />
        </linearGradient>
        <linearGradient id="trendLine" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stop-color="#ffc542" />
          <stop offset="55%" stop-color="#ff7a18" />
          <stop offset="100%" stop-color="#ff3d71" />
        </linearGradient>
      </defs>
      <g>
        <line v-for="i in 4" :key="i" :x1="PAD" :x2="W - PAD" :y1="(H - PAD) * i / 4 + PAD / 2" :y2="(H - PAD) * i / 4 + PAD / 2" stroke="var(--chart-grid)" stroke-dasharray="3 5" />
      </g>
      <path :d="areaPath" fill="url(#trendFill)" />
      <path :d="linePath" fill="none" stroke="url(#trendLine)" stroke-width="2.2" stroke-linejoin="round" stroke-linecap="round" />
      <circle v-for="(p, i) in points" :key="i" :cx="p[0]" :cy="p[1]" r="0" fill="#fff" />
      <g v-if="hover >= 0 && points[hover]">
        <line :x1="points[hover][0]" :x2="points[hover][0]" :y1="PAD" :y2="H - PAD" stroke="var(--line-strong)" stroke-dasharray="3 4" />
        <circle :cx="points[hover][0]" :cy="points[hover][1]" r="5" fill="#fff" stroke="#ff7a18" stroke-width="3" />
      </g>
    </svg>
    <div class="axis">
      <span v-for="(label, i) in labels" :key="i">{{ label }}</span>
    </div>
    <div v-if="hover >= 0 && data[hover]" class="tip" :style="{ left: tipLeft + 'px' }">
      <div class="tip-date">{{ data[hover].date }}</div>
      <div class="tip-row"><i class="dot ok" />成功 <b>{{ data[hover].success }}</b></div>
      <div class="tip-row"><i class="dot bad" />失败 <b>{{ data[hover].failed }}</b></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { TrendPoint } from '@/api/types'

const props = withDefaults(defineProps<{ data: TrendPoint[]; height?: number }>(), { height: 220 })
const W = 760
const H = computed(() => props.height)
const PAD = 18
const hover = ref(-1)
const wrap = ref<HTMLElement>()
const tipLeft = ref(0)

const max = computed(() => Math.max(1, ...props.data.map((d) => d.success + d.failed)))
const points = computed(() => props.data.map((d, i) => {
  const x = PAD + (i / Math.max(1, props.data.length - 1)) * (W - PAD * 2)
  const total = d.success + d.failed
  const y = H.value - PAD - (total / max.value) * (H.value - PAD * 2)
  return [x, y] as [number, number]
}))

const linePath = computed(() => smooth(points.value))
const areaPath = computed(() => `${linePath.value} L${W - PAD},${H.value - PAD} L${PAD},${H.value - PAD} Z`)

const labels = computed(() => {
  const len = props.data.length
  if (!len) return []
  const step = Math.max(1, Math.ceil(len / 7))
  return props.data.filter((_, i) => i % step === 0 || i === len - 1).map((d) => d.date)
})

function smooth(pts: [number, number][]) {
  if (pts.length < 2) return ''
  let d = `M${pts[0][0]},${pts[0][1]}`
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i - 1] || pts[i]
    const p1 = pts[i]
    const p2 = pts[i + 1]
    const p3 = pts[i + 2] || p2
    const c1x = p1[0] + (p2[0] - p0[0]) / 6
    const c1y = p1[1] + (p2[1] - p0[1]) / 6
    const c2x = p2[0] - (p3[0] - p1[0]) / 6
    const c2y = p2[1] - (p3[1] - p1[1]) / 6
    d += ` C${c1x},${c1y} ${c2x},${c2y} ${p2[0]},${p2[1]}`
  }
  return d
}

function onMove(event: MouseEvent) {
  const rect = wrap.value?.getBoundingClientRect()
  if (!rect || !props.data.length) return
  const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width))
  hover.value = Math.round(ratio * (props.data.length - 1))
  tipLeft.value = Math.max(10, Math.min(rect.width - 130, ratio * rect.width - 60))
}
</script>

<style scoped>
.trend { position: relative; width: 100%; }
.svg { width: 100%; display: block; overflow: visible; }
.axis { display: flex; justify-content: space-between; margin-top: 8px; font-size: 10.5px; color: var(--muted-2); font-variant-numeric: tabular-nums; }
.tip {
  position: absolute; top: 6px; min-width: 118px; padding: 9px 11px;
  border-radius: 12px; border: 1px solid var(--line-strong);
  background: color-mix(in srgb, var(--bg-elev) 94%, transparent);
  box-shadow: var(--shadow-pop); font-size: 11.5px; pointer-events: none;
  backdrop-filter: blur(14px);
}
.tip-date { color: var(--muted); margin-bottom: 5px; }
.tip-row { display: flex; align-items: center; gap: 6px; color: var(--text-2); line-height: 1.7; }
.tip-row b { margin-left: auto; color: var(--text); font-variant-numeric: tabular-nums; }
.dot { width: 6px; height: 6px; border-radius: 50%; display: inline-block; }
.dot.ok { background: var(--safe); }
.dot.bad { background: var(--urgent); }
</style>

<template>
  <div class="stat card card-hover">
    <div class="top">
      <span class="label">{{ label }}</span>
      <span class="icon" :style="{ color: accent }">
        <slot name="icon" />
      </span>
    </div>
    <div class="value num">
      {{ value }}<small v-if="unit">{{ unit }}</small>
    </div>
    <div class="bottom">
      <span v-if="hint" class="hint">{{ hint }}</span>
      <span v-if="badge" class="badge" :class="badgeClass">{{ badge }}</span>
    </div>
    <div v-if="spark.length > 1" class="spark">
      <svg viewBox="0 0 100 26" preserveAspectRatio="none">
        <defs>
          <linearGradient :id="gid" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" :stop-color="accent" stop-opacity=".45" />
            <stop offset="100%" :stop-color="accent" stop-opacity="0" />
          </linearGradient>
        </defs>
        <path :d="areaPath" :fill="`url(#${gid})`" />
        <path :d="linePath" fill="none" :stroke="accent" stroke-width="1.6" stroke-linejoin="round" stroke-linecap="round" />
      </svg>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  label: string
  value: string | number
  unit?: string
  hint?: string
  badge?: string
  badgeClass?: string
  accent?: string
  spark?: number[]
}>(), { accent: '#ff7a18', spark: () => [], badgeClass: '', hint: '', badge: '', unit: '' })

const gid = `stat-${Math.random().toString(36).slice(2, 8)}`

const points = computed(() => {
  const data = props.spark.length > 1 ? props.spark : [0, 0]
  const max = Math.max(...data, 1)
  const min = Math.min(...data, 0)
  const span = max - min || 1
  return data.map((v, i) => [i / (data.length - 1) * 100, 24 - ((v - min) / span) * 20])
})
const linePath = computed(() => points.value.map((p, i) => `${i === 0 ? 'M' : 'L'}${p[0].toFixed(1)},${p[1].toFixed(1)}`).join(' '))
const areaPath = computed(() => `${linePath.value} L100,26 L0,26 Z`)
</script>

<style scoped>
.stat { padding: 18px 20px 16px; overflow: hidden; display: flex; flex-direction: column; gap: 8px; min-height: 132px; }
.top { display: flex; align-items: center; justify-content: space-between; }
.label { font-size: 12.5px; color: var(--muted); letter-spacing: .02em; }
.icon { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 10px; background: var(--surface-2); }
.icon :deep(svg) { width: 16px; height: 16px; }
.value { font-size: 30px; font-weight: 720; letter-spacing: -.03em; line-height: 1.1; }
.value small { font-size: 13px; font-weight: 600; color: var(--muted); margin-left: 3px; }
.bottom { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-height: 22px; }
.hint { font-size: 11.5px; color: var(--muted-2); }
.spark { margin: 2px -20px -16px; height: 34px; opacity: .9; }
.spark svg { width: 100%; height: 100%; }
</style>

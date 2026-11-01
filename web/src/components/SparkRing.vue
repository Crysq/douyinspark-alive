<template>
  <div class="ring" :style="{ width: size + 'px', height: size + 'px' }">
    <svg :viewBox="`0 0 ${size} ${size}`" :width="size" :height="size">
      <defs>
        <linearGradient :id="gradId" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" :stop-color="colors[0]" />
          <stop offset="100%" :stop-color="colors[1]" />
        </linearGradient>
      </defs>
      <circle :cx="size / 2" :cy="size / 2" :r="radius" fill="none" stroke="var(--line)" :stroke-width="stroke" />
      <circle
        :cx="size / 2" :cy="size / 2" :r="radius" fill="none"
        :stroke="`url(#${gradId})`" :stroke-width="stroke" stroke-linecap="round"
        :stroke-dasharray="circumference" :stroke-dashoffset="offset"
        :transform="`rotate(-90 ${size / 2} ${size / 2})`"
        class="progress"
      />
    </svg>
    <div class="inner">
      <span class="value num" :style="{ color: colors[0] }">{{ label }}</span>
      <span v-if="caption" class="caption">{{ caption }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { compactCountdown } from '@/utils/format'

const props = withDefaults(defineProps<{
  remainingSec: number
  progress: number
  level?: string
  size?: number
  stroke?: number
  caption?: string
  label?: string
}>(), { level: 'safe', size: 74, stroke: 6, caption: '', label: '' })

const radius = computed(() => props.size / 2 - props.stroke / 2 - 1)
const circumference = computed(() => 2 * Math.PI * radius.value)
const offset = computed(() => circumference.value * (1 - Math.max(0, Math.min(1, props.progress))))
const gradId = `ring-${Math.random().toString(36).slice(2, 9)}`

const palette: Record<string, string[]> = {
  safe: ['#2dd4a7', '#4ade80'],
  warn: ['#f5b544', '#ffb020'],
  urgent: ['#ff7a5c', '#ff3d71'],
  broken: ['#7b7b90', '#5b5b70'],
}
const colors = computed(() => palette[props.level] || palette.safe)
const label = computed(() => props.label || compactCountdown(props.remainingSec))
</script>

<style scoped>
.ring { position: relative; flex: none; display: grid; place-items: center; }
.progress { transition: stroke-dashoffset .6s var(--ease), stroke .3s; filter: drop-shadow(0 0 6px rgba(255, 122, 24, .28)); }
.inner { position: absolute; inset: 0; display: grid; place-content: center; text-align: center; gap: 1px; }
.value { font-size: 12.5px; font-weight: 700; letter-spacing: -.02em; }
.caption { font-size: 9px; color: var(--muted-2); letter-spacing: .06em; }
</style>

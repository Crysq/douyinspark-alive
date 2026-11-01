<template>
  <div class="donut">
    <svg :viewBox="`0 0 ${size} ${size}`" :width="size" :height="size" style="display:block">
      <circle :cx="size / 2" :cy="size / 2" :r="radius" fill="none" stroke="var(--line)" :stroke-width="thickness" />
      <circle
        v-for="(seg, i) in segments" :key="i"
        :cx="size / 2" :cy="size / 2" :r="radius" fill="none"
        :stroke="seg.color" :stroke-width="thickness" stroke-linecap="butt"
        :stroke-dasharray="`${seg.length} ${circumference - seg.length}`"
        :stroke-dashoffset="-seg.offset"
        :transform="`rotate(-90 ${size / 2} ${size / 2})`"
        class="seg"
      />
    </svg>
    <div class="center">
      <strong class="num">{{ total }}</strong>
      <span>{{ label }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  slices: { label: string; value: number; color: string }[]
  size?: number
  thickness?: number
  label?: string
}>(), { size: 168, thickness: 16, label: '好友' })

const radius = computed(() => props.size / 2 - props.thickness / 2 - 2)
const circumference = computed(() => 2 * Math.PI * radius.value)
const total = computed(() => props.slices.reduce((sum, s) => sum + s.value, 0))

const segments = computed(() => {
  let offset = 0
  return props.slices.filter((s) => s.value > 0).map((slice) => {
    const length = total.value ? (slice.value / total.value) * circumference.value : 0
    const seg = { ...slice, length: Math.max(0, length - 3), offset }
    offset += length
    return seg
  })
})
</script>

<style scoped>
.donut { position: relative; display: grid; place-items: center; }
.seg { transition: stroke-dasharray .6s var(--ease), stroke-dashoffset .6s var(--ease); }
.center { position: absolute; inset: 0; display: grid; place-content: center; text-align: center; gap: 2px; }
.center strong { font-size: 26px; font-weight: 700; letter-spacing: -.03em; }
.center span { font-size: 10.5px; color: var(--muted); letter-spacing: .08em; }
</style>

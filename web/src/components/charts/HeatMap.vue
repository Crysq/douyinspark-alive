<template>
  <div class="heat">
    <div class="rows">
      <div v-for="(row, w) in grid" :key="w" class="row-cells">
        <span class="wd">{{ weekdays[w] }}</span>
        <span
          v-for="(count, h) in row" :key="h"
          class="cell" :style="{ background: color(count) }"
          :title="`${weekdays[w]} ${h}:00 · ${count} 次`"
        />
      </div>
    </div>
    <div class="hours">
      <span v-for="h in hours" :key="h">{{ h }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { HeatPoint } from '@/api/types'

const props = defineProps<{ data: HeatPoint[] }>()
const weekdays = ['日', '一', '二', '三', '四', '五', '六']
const hours = [0, 6, 12, 18, 23]

const grid = computed(() => {
  const rows = Array.from({ length: 7 }, () => Array(24).fill(0))
  for (const point of props.data) {
    if (rows[point.weekday]) rows[point.weekday][point.hour] = point.count
  }
  return rows
})
const max = computed(() => Math.max(1, ...props.data.map((p) => p.count)))

function color(count: number) {
  if (!count) return 'var(--surface)'
  const ratio = Math.min(1, Math.sqrt(count / max.value))
  return `rgba(255, 122, 24, ${0.14 + ratio * 0.78})`
}
</script>

<style scoped>
.heat { display: flex; flex-direction: column; gap: 8px; }
.row-cells { display: flex; align-items: center; gap: 4px; }
.wd { width: 16px; font-size: 10px; color: var(--muted-2); flex: none; text-align: center; }
.cell { flex: 1; height: 15px; border-radius: 4px; transition: transform .18s var(--ease), background .3s; }
.cell:hover { transform: scaleY(1.18); }
.hours { display: flex; justify-content: space-between; padding-left: 20px; font-size: 10px; color: var(--muted-2); }
</style>

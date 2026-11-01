<template>
  <div class="timeline">
    <div class="track">
      <div class="window" :style="{ left: startPct + '%', width: spanPct + '%' }" />
      <div class="now" :style="{ left: nowPct + '%' }"><span /></div>
    </div>
    <div class="marks">
      <span v-for="h in marks" :key="h" :style="{ left: (h / 24) * 100 + '%' }">{{ String(h).padStart(2, '0') }}:00</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ start: string; end: string; now: Date }>()
const marks = [0, 6, 12, 18, 24]

function minutes(value: string) {
  const [h, m] = value.split(':').map(Number)
  return (h || 0) * 60 + (m || 0)
}
const startMin = computed(() => minutes(props.start))
const endMin = computed(() => minutes(props.end))
const startPct = computed(() => (startMin.value / 1440) * 100)
const spanPct = computed(() => {
  const span = endMin.value >= startMin.value ? endMin.value - startMin.value : 1440 - startMin.value + endMin.value
  return (span / 1440) * 100
})
const nowPct = computed(() => ((props.now.getHours() * 60 + props.now.getMinutes()) / 1440) * 100)
</script>

<style scoped>
.timeline { padding-top: 6px; }
.track { position: relative; height: 22px; border-radius: 8px; background: var(--surface); border: 1px solid var(--line); overflow: hidden; }
.window {
  position: absolute; top: 0; bottom: 0;
  background: linear-gradient(90deg, rgba(255, 176, 32, .35), rgba(255, 61, 113, .4));
  border-left: 1px solid rgba(255, 176, 32, .6);
  border-right: 1px solid rgba(255, 61, 113, .6);
}
.now { position: absolute; top: -4px; bottom: -4px; width: 2px; background: #fff; box-shadow: 0 0 10px rgba(255, 255, 255, .8); }
.now span { position: absolute; top: -3px; left: -3px; width: 8px; height: 8px; border-radius: 50%; background: #fff; }
.marks { position: relative; height: 16px; margin-top: 6px; font-size: 9.5px; color: var(--muted-2); }
.marks span { position: absolute; transform: translateX(-50%); }
.marks span:first-child { transform: none; }
.marks span:last-child { transform: translateX(-100%); }
</style>

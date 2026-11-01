<template>
  <span class="avatar" :style="{ width: size + 'px', height: size + 'px', fontSize: size * 0.38 + 'px', background: gradient }">
    <img v-if="src" :src="src" :alt="name" @error="broken = true" v-show="!broken" />
    <span v-if="!src || broken">{{ initial }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = withDefaults(defineProps<{ name?: string; src?: string; size?: number }>(), {
  name: '火', src: '', size: 40,
})

const broken = ref(false)
watch(() => props.src, () => { broken.value = false })

const initial = computed(() => (props.name || '火').trim().slice(0, 1))

const palettes = [
  ['#ff9a3c', '#ff3d71'], ['#7c5cff', '#3d8bff'], ['#2dd4a7', '#1e9e8f'],
  ['#ffb020', '#ff6b35'], ['#ff6b9d', '#c445ff'], ['#4facfe', '#00f2fe'],
]
const gradient = computed(() => {
  let hash = 0
  for (const ch of props.name || '') hash = (hash * 31 + ch.charCodeAt(0)) % 997
  const [a, b] = palettes[hash % palettes.length]
  return `linear-gradient(135deg, ${a}, ${b})`
})
</script>

<style scoped>
.avatar {
  display: inline-grid; place-items: center; flex: none;
  border-radius: 50%; color: #fff; font-weight: 650;
  overflow: hidden; position: relative;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, .16), 0 6px 18px -8px rgba(0, 0, 0, .6);
  user-select: none;
}
.avatar img { width: 100%; height: 100%; object-fit: cover; }
</style>

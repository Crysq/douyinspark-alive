<template>
  <div class="empty">
    <slot name="icon">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round">
        <path d="M12 3c2.5 4-3 5.5-3 9a3 3 0 0 0 6 0c0-1.8-1-2.8-1-4.2 3 2.2 4.5 5 4.5 8.2a6.5 6.5 0 1 1-13 0C5.5 11 9 7.5 12 3Z" />
      </svg>
    </slot>
    <strong>{{ title }}</strong>
    <span class="desc">{{ desc }}</span>
    <div v-if="$slots.action" class="mt-8"><slot name="action" /></div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; desc?: string }>()
</script>

<style scoped>
strong { font-size: 14px; color: var(--text-2); font-weight: 600; }
.desc { font-size: 12.5px; max-width: 320px; line-height: 1.6; }
</style>

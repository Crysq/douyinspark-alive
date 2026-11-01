<template>
  <router-view v-slot="{ Component }">
    <transition name="page-fade" mode="out-in">
      <component :is="Component" />
    </transition>
  </router-view>
  <ToastHost />
</template>

<script setup lang="ts">
import ToastHost from '@/components/ToastHost.vue'
</script>

<style>
.page-fade-enter-active, .page-fade-leave-active { transition: opacity .18s var(--ease), transform .18s var(--ease); }
.page-fade-enter-from { opacity: 0; transform: translateY(6px); }
.page-fade-leave-to { opacity: 0; }
</style>

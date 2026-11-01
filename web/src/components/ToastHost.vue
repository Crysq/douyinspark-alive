<template>
  <Teleport to="body">
    <div class="toasts">
      <TransitionGroup name="toast">
        <div v-for="item in ui.toasts" :key="item.id" class="toast" :class="item.kind">
          <span class="dot" />
          <span class="msg">{{ item.message }}</span>
          <button class="close" @click="ui.dismiss(item.id)" aria-label="关闭">×</button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useUiStore } from '@/stores/ui'
const ui = useUiStore()
</script>

<style scoped>
.toasts { position: fixed; right: 22px; bottom: 22px; z-index: 120; display: flex; flex-direction: column; gap: 10px; pointer-events: none; }
.toast {
  pointer-events: auto; display: flex; align-items: center; gap: 10px;
  min-width: 220px; max-width: 380px; padding: 12px 14px;
  border-radius: 14px; border: 1px solid var(--line-strong);
  background: color-mix(in srgb, var(--bg-elev) 92%, transparent);
  backdrop-filter: blur(18px);
  box-shadow: var(--shadow-pop);
  font-size: 13px; color: var(--text);
}
.toast .dot { width: 8px; height: 8px; border-radius: 50%; flex: none; background: var(--muted); }
.toast.success .dot { background: var(--safe); box-shadow: 0 0 10px var(--safe); }
.toast.error .dot { background: var(--urgent); box-shadow: 0 0 10px var(--urgent); }
.toast.info .dot { background: #6aa8ff; box-shadow: 0 0 10px #6aa8ff; }
.msg { flex: 1; line-height: 1.45; }
.close { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 16px; line-height: 1; padding: 2px; }
.toast-enter-active, .toast-leave-active { transition: all .26s var(--ease-out); }
.toast-enter-from { opacity: 0; transform: translateX(18px) scale(.96); }
.toast-leave-to { opacity: 0; transform: translateX(18px) scale(.96); }
</style>

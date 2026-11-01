<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="modelValue" class="overlay" @click.self="close">
        <div class="modal" :style="{ width: width }" role="dialog" aria-modal="true">
          <div class="modal-head">
            <div>
              <h3 class="modal-title">{{ title }}</h3>
              <p v-if="subtitle" class="modal-sub">{{ subtitle }}</p>
            </div>
            <button class="btn btn-icon btn-sm btn-ghost" @click="close" aria-label="关闭">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12" /></svg>
            </button>
          </div>
          <div class="modal-body">
            <slot />
          </div>
          <div v-if="$slots.footer" class="modal-foot">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ modelValue: boolean; title: string; subtitle?: string; width?: string }>(), { width: '560px', subtitle: '' })
const emit = defineEmits<{ 'update:modelValue': [boolean] }>()
const close = () => emit('update:modelValue', false)
</script>

<style scoped>
.modal-title { font-size: 17px; font-weight: 650; letter-spacing: -.01em; }
.modal-sub { margin-top: 4px; font-size: 12.5px; color: var(--muted); }
.modal-enter-active, .modal-leave-active { transition: opacity .2s var(--ease); }
.modal-enter-active .modal, .modal-leave-active .modal { transition: transform .24s var(--ease-out), opacity .2s; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-from .modal, .modal-leave-to .modal { transform: translateY(12px) scale(.985); opacity: 0; }
</style>

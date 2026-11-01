import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastKind = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  kind: ToastKind
  message: string
}

const THEME_KEY = 'doyinspark.theme'

export const useUiStore = defineStore('ui', () => {
  const theme = ref<'dark' | 'light'>((localStorage.getItem(THEME_KEY) as 'dark' | 'light') || 'dark')
  const toasts = ref<Toast[]>([])
  let seq = 0

  function applyTheme() {
    document.documentElement.dataset.theme = theme.value
    localStorage.setItem(THEME_KEY, theme.value)
  }

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    applyTheme()
  }

  function toast(kind: ToastKind, message: string, duration = 3200) {
    const id = ++seq
    toasts.value.push({ id, kind, message })
    window.setTimeout(() => dismiss(id), duration)
  }

  function dismiss(id: number) {
    toasts.value = toasts.value.filter((item) => item.id !== id)
  }

  return { theme, toasts, applyTheme, toggleTheme, toast, dismiss }
})

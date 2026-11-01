import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { StreamEvent } from '@/api/types'

export const useRealtimeStore = defineStore('realtime', () => {
  const events = ref<StreamEvent[]>([])
  const connected = ref(false)
  let source: EventSource | null = null
  let retry = 0
  let timer: number | undefined

  function connect() {
    const token = localStorage.getItem('doyinspark.token')
    if (!token || source) return
    source = new EventSource(`/api/events/stream?token=${encodeURIComponent(token)}`)
    source.onopen = () => { connected.value = true; retry = 0 }
    source.addEventListener('spark', (event) => {
      try {
        const payload = JSON.parse((event as MessageEvent).data) as StreamEvent
        events.value = [payload, ...events.value].slice(0, 40)
      } catch { /* 忽略非法事件 */ }
    })
    source.onerror = () => {
      connected.value = false
      source?.close()
      source = null
      retry += 1
      window.clearTimeout(timer)
      timer = window.setTimeout(connect, Math.min(15000, 1500 * retry))
    }
  }

  function disconnect() {
    source?.close()
    source = null
    connected.value = false
  }

  return { events, connected, connect, disconnect }
})

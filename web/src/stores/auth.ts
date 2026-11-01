import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { authApi, membershipApi } from '@/api'
import type { User } from '@/api/types'

const TOKEN_KEY = 'doyinspark.token'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(TOKEN_KEY) || '')
  const user = ref<User | null>(null)
  const loading = ref(false)
  // 待激活：会员过期/未开通时只能兑换卡密
  const pending = ref<{ user: User; activationToken: string } | null>(null)

  const authed = computed(() => Boolean(token.value && user.value))
  const isAdmin = computed(() => user.value?.role === 'admin')
  const isMember = computed(() => Boolean(user.value?.isMember))
  const daysLeft = computed(() => user.value?.daysLeft ?? 0)

  function saveSession(nextToken: string, nextUser: User) {
    token.value = nextToken
    user.value = nextUser
    pending.value = null
    localStorage.setItem(TOKEN_KEY, nextToken)
  }

  async function login(username: string, password: string) {
    loading.value = true
    try {
      const data = await authApi.login(username, password)
      if (data.needActivation || !data.user.isMember) {
        pending.value = { user: data.user, activationToken: data.activationToken || data.token }
        return { needActivation: true as const }
      }
      saveSession(data.token, data.user)
      return { needActivation: false as const }
    } finally {
      loading.value = false
    }
  }

  async function register(payload: { username: string; password: string; nickname?: string; cardCode: string }) {
    loading.value = true
    try {
      const data = await authApi.register(payload)
      saveSession(data.token, data.user)
      return data
    } finally {
      loading.value = false
    }
  }

  async function redeem(code: string) {
    const activationToken = pending.value?.activationToken || token.value
    const data = await membershipApi.redeem(code, activationToken)
    saveSession(data.token, data.user)
    return data
  }

  async function loadMe() {
    if (!token.value) return
    try {
      user.value = await authApi.me()
    } catch {
      clear()
    }
  }

  async function refresh() {
    if (!token.value) return
    try {
      user.value = await authApi.me()
    } catch { /* 保持旧状态 */ }
  }

  async function logout() {
    try { await authApi.logout() } catch { /* 忽略网络错误 */ }
    clear()
  }

  function clear() {
    token.value = ''
    user.value = null
    pending.value = null
    localStorage.removeItem(TOKEN_KEY)
  }

  return { token, user, loading, pending, authed, isAdmin, isMember, daysLeft, login, register, redeem, loadMe, refresh, logout, clear }
})

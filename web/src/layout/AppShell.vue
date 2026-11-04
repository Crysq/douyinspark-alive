<template>
  <div class="shell">
    <aside class="sidebar">
      <router-link to="/dashboard" class="brand">
        <span class="mark">
          <svg viewBox="0 0 48 48" aria-hidden="true">
            <defs>
              <linearGradient id="brandFlame" x1="0" y1="0" x2="1" y2="1">
                <stop offset="0" stop-color="#ffc542" /><stop offset=".55" stop-color="#ff7a18" /><stop offset="1" stop-color="#ff3d71" />
              </linearGradient>
            </defs>
            <path d="M24 5c4 7-5 9.5-5 17a5 5 0 0 0 10 0c0-3.4-1.6-5-1.6-7.4C32.8 18.5 36 24 36 30a12 12 0 1 1-24 0C12 20.5 19.5 15.5 24 5Z" fill="url(#brandFlame)" />
          </svg>
        </span>
        <span class="wordmark">
          <strong>DoyinSpark</strong>
          <small>抖音火花守护</small>
        </span>
      </router-link>

      <nav class="nav">
        <router-link v-for="item in navItems" :key="item.to" :to="item.to" class="nav-item" active-class="active">
          <span class="ico" v-html="item.icon" />
          <span class="txt">{{ item.label }}</span>
        </router-link>
      </nav>

      <div class="side-foot">
        <div class="channel" :class="channel">
          <span class="pulse" />
          <div class="channel-text">
            <strong>{{ channel === 'ready' ? '真实发送通道' : '通道未就绪' }}</strong>
            <small>{{ channel === 'ready' ? '签名服务已连接' : '请检查 sign-service' }}</small>
          </div>
        </div>
        <div class="meta">
          <span>v2.0.0</span>
          <span class="dot" />
          <span>{{ health?.redis === 'up' ? 'Redis 已连接' : 'Redis 断开' }}</span>
        </div>
      </div>
    </aside>

    <div class="main">
      <header class="topbar">
        <div class="title-wrap">
          <h1 class="top-title">{{ route.meta.title }}</h1>
          <p class="top-desc hide-sm">{{ route.meta.desc }}</p>
        </div>
        <div class="top-actions">
          <router-link v-if="auth.isSaas" to="/membership" class="member-chip" :class="memberTone">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m3 8 4 3 5-6 5 6 4-3-2 10H5L3 8Z"/></svg>
            <span>{{ memberText }}</span>
          </router-link>
          <div class="clock num hide-sm">{{ clock }}</div>
          <button class="btn btn-icon btn-ghost" :title="ui.theme === 'dark' ? '切换亮色' : '切换暗色'" @click="ui.toggleTheme()">
            <svg v-if="ui.theme === 'dark'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
              <circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5 19 19M19 5l-1.5 1.5M6.5 17.5 5 19" />
            </svg>
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
              <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z" />
            </svg>
          </button>
          <div class="bell-wrap">
            <button class="btn btn-icon btn-ghost" @click="bellOpen = !bellOpen" aria-label="通知">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
                <path d="M18 8a6 6 0 1 0-12 0c0 7-3 8-3 8h18s-3-1-3-8" /><path d="M10.3 21a1.9 1.9 0 0 0 3.4 0" />
              </svg>
              <span v-if="unread" class="dot-badge">{{ unread > 9 ? '9+' : unread }}</span>
            </button>
            <Transition name="drop">
              <div v-if="bellOpen" class="bell-panel card">
                <div class="bell-head">
                  <strong>通知</strong>
                  <button class="link" @click="readAll">全部已读</button>
                </div>
                <div v-if="!notifications.length" class="bell-empty">暂无通知，火花一切正常</div>
                <ul v-else class="bell-list">
                  <li v-for="n in notifications.slice(0, 6)" :key="n.id" :class="n.level">
                    <span class="dot" />
                    <div>
                      <strong>{{ n.title }}</strong>
                      <p>{{ n.body }}</p>
                      <small>{{ timeAgo(n.createdAt) }}</small>
                    </div>
                  </li>
                </ul>
              </div>
            </Transition>
          </div>
          <div class="user-wrap">
            <button class="user" @click="menuOpen = !menuOpen">
              <Avatar :name="auth.user?.nickname || auth.user?.username || '火'" :size="34" />
              <span class="user-meta hide-sm">
                <strong>{{ auth.user?.nickname || auth.user?.username || '用户' }}</strong>
                <small>{{ roleText }}</small>
              </span>
            </button>
            <Transition name="drop">
              <div v-if="menuOpen" class="user-menu card">
                <router-link v-if="auth.isSaas" to="/membership" class="menu-item" @click="menuOpen = false">会员中心</router-link>
                <router-link v-if="auth.isAdmin && auth.isSaas" to="/admin/cards" class="menu-item" @click="menuOpen = false">卡密管理</router-link>
                <router-link to="/settings" class="menu-item" @click="menuOpen = false">系统设置</router-link>
                <button class="menu-item danger" @click="logout">退出登录</button>
              </div>
            </Transition>
          </div>
        </div>
      </header>

      <main class="content">
        <router-view v-slot="{ Component }">
          <transition name="view" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>

      <nav class="mobile-nav">
        <router-link v-for="item in mobileItems" :key="item.to" :to="item.to" class="m-item" active-class="active">
          <span class="ico" v-html="item.icon" />
          <span>{{ item.label }}</span>
        </router-link>
      </nav>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Avatar from '@/components/Avatar.vue'
import { useAuthStore } from '@/stores/auth'
import { useRealtimeStore } from '@/stores/realtime'
import { useUiStore } from '@/stores/ui'
import { overviewApi, systemApi } from '@/api'
import type { Health, Notification } from '@/api/types'
import { timeAgo } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const ui = useUiStore()
const realtime = useRealtimeStore()

const health = ref<Health | null>(null)
const notifications = ref<Notification[]>([])
const unread = ref(0)
const bellOpen = ref(false)
const menuOpen = ref(false)
const clock = ref('')
let timer: number | undefined

const icons: Record<string, string> = {
  dashboard: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="9" rx="2"/><rect x="14" y="3" width="7" height="5" rx="2"/><rect x="14" y="12" width="7" height="9" rx="2"/><rect x="3" y="16" width="7" height="5" rx="2"/></svg>',
  friends: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3c2 3.4-2.6 4.8-2.6 8a2.6 2.6 0 0 0 5.2 0c0-1.7-.8-2.6-.8-3.9 2.4 1.8 3.6 4.2 3.6 6.9a5.4 5.4 0 1 1-10.8 0C6.6 10 9.7 6.6 12 3Z"/></svg>',
  accounts: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/></svg>',
  tasks: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>',
  templates: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4 5h16v14H4z"/><path d="M8 9h8M8 13h5"/></svg>',
  records: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6h16M4 12h16M4 18h10"/></svg>',
  crown: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><path d="m3 8 4 3 5-6 5 6 4-3-2 10H5L3 8Z"/><path d="M5 21h14"/></svg>',
  users: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="9" cy="8" r="3.2"/><path d="M3 20a6 6 0 0 1 12 0"/><path d="M16 5.5a3 3 0 0 1 0 5.8"/><path d="M17.5 20a5.5 5.5 0 0 0-2-4.3"/></svg>',
  cards: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="6" width="19" height="12" rx="3"/><path d="M2.5 10h19"/><path d="M6 14.5h4"/></svg>',
  settings: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.9 1.2 2 2 0 1 1-4 0 1.7 1.7 0 0 0-2.9-1.2l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1A1.7 1.7 0 0 0 4.6 15a2 2 0 1 1 0-4 1.7 1.7 0 0 0 1.2-2.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1A1.7 1.7 0 0 0 11.5 4a2 2 0 1 1 4 0 1.7 1.7 0 0 0 2.9 1.2l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1A1.7 1.7 0 0 0 22.4 11a2 2 0 1 1 0 4Z"/></svg>',
}

const baseNav = [
  { to: '/dashboard', label: '总览', icon: icons.dashboard },
  { to: '/friends', label: '火花好友', icon: icons.friends },
  { to: '/accounts', label: '账号管理', icon: icons.accounts },
  { to: '/tasks', label: '定时任务', icon: icons.tasks },
  { to: '/templates', label: '消息模板', icon: icons.templates },
  { to: '/records', label: '发送记录', icon: icons.records },
]

const navItems = computed(() => {
  const items = [...baseNav]
  if (auth.isSaas) {
    items.push({ to: '/membership', label: '会员中心', icon: icons.crown })
    if (auth.isAdmin) {
      items.push({ to: '/admin/users', label: '用户管理', icon: icons.users })
      items.push({ to: '/admin/cards', label: '卡密管理', icon: icons.cards })
    }
  }
  items.push({ to: '/settings', label: '系统设置', icon: icons.settings })
  return items
})

const mobileItems = [
  { to: '/dashboard', label: '总览', icon: icons.dashboard },
  { to: '/friends', label: '好友', icon: icons.friends },
  { to: '/tasks', label: '任务', icon: icons.tasks },
  { to: '/records', label: '记录', icon: icons.records },
  { to: '/settings', label: '设置', icon: icons.settings },
]

const channel = computed(() => health.value?.sign === 'ready' ? 'ready' : 'offline')
const roleText = computed(() => (auth.isAdmin ? '管理员' : '会员'))
const memberText = computed(() => {
  if (!auth.user) return '会员'
  if (!auth.user.isMember) return '会员已过期'
  if (auth.user.daysLeft < 0) return '永久会员'
  return `剩 ${auth.user.daysLeft} 天`
})
const memberTone = computed(() => {
  if (!auth.user?.isMember) return 'danger'
  if (auth.user.daysLeft < 0) return 'forever'
  return auth.user.daysLeft <= 7 ? 'warn' : 'ok'
})

async function refresh() {
  try {
    health.value = await systemApi.health()
    const data = await overviewApi.notifications()
    notifications.value = data.items
    unread.value = data.unread
  } catch { /* 静默失败，下一次轮询重试 */ }
}

async function readAll() {
  await overviewApi.readNotifications()
  notifications.value = notifications.value.map((n) => ({ ...n, read: true }))
  unread.value = 0
}

async function logout() {
  await auth.logout()
  realtime.disconnect()
  router.push('/login')
}

function tickClock() {
  const now = new Date()
  clock.value = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`
}

onMounted(async () => {
  ui.applyTheme()
  tickClock()
  timer = window.setInterval(tickClock, 15000)
  await auth.loadMode()
  await auth.loadMe()
  await refresh()
  window.setInterval(refresh, 45000)
  realtime.connect()
  document.addEventListener('click', closePanels)
})

function closePanels(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.bell-wrap')) bellOpen.value = false
  if (!target.closest('.user-wrap')) menuOpen.value = false
}

onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  document.removeEventListener('click', closePanels)
  realtime.disconnect()
})
</script>

<style scoped>
.shell { display: flex; min-height: 100vh; }

.sidebar {
  width: 248px; flex: none; position: sticky; top: 0; height: 100vh;
  display: flex; flex-direction: column; gap: 22px;
  padding: 22px 16px 18px;
  border-right: 1px solid var(--line);
  background: linear-gradient(180deg, rgba(255, 255, 255, .022), transparent 55%);
  backdrop-filter: blur(18px);
}
.brand { display: flex; align-items: center; gap: 11px; padding: 4px 8px; }
.mark { width: 38px; height: 38px; display: grid; place-items: center; border-radius: 13px; background: var(--flame-soft); box-shadow: inset 0 0 0 1px rgba(255, 138, 60, .22); }
.mark svg { width: 26px; height: 26px; animation: flicker 3.6s ease-in-out infinite; transform-origin: center bottom; }
.wordmark { display: flex; flex-direction: column; line-height: 1.15; }
.wordmark strong { font-size: 15.5px; font-weight: 700; letter-spacing: -.015em; }
.wordmark small { font-size: 10.5px; color: var(--muted-2); letter-spacing: .14em; }

.nav { display: flex; flex-direction: column; gap: 4px; }
.nav-item {
  position: relative; display: flex; align-items: center; gap: 11px;
  height: 42px; padding: 0 12px; border-radius: 12px;
  color: var(--muted); font-size: 13.5px; font-weight: 540;
  transition: color var(--dur), background var(--dur), transform var(--dur);
}
.nav-item:hover { color: var(--text); background: var(--surface); }
.nav-item.active {
  color: var(--text);
  background: linear-gradient(90deg, rgba(255, 122, 24, .14), rgba(255, 61, 113, .05));
  box-shadow: inset 0 0 0 1px rgba(255, 122, 24, .16);
}
.nav-item.active::before {
  content: ""; position: absolute; left: -16px; top: 11px; bottom: 11px; width: 3px;
  border-radius: 0 3px 3px 0; background: var(--flame-grad); box-shadow: 0 0 14px rgba(255, 122, 24, .7);
}
.nav-item .ico { width: 18px; height: 18px; display: grid; place-items: center; }
.nav-item .ico :deep(svg) { width: 18px; height: 18px; }
.nav-item.active .ico { color: var(--flame-2); }

.side-foot { margin-top: auto; display: flex; flex-direction: column; gap: 12px; }
.channel {
  display: flex; align-items: center; gap: 10px; padding: 12px;
  border-radius: 14px; border: 1px solid var(--line); background: var(--surface);
}
.channel .pulse { width: 8px; height: 8px; border-radius: 50%; background: var(--muted); flex: none; }
.channel.ready .pulse { background: var(--safe); box-shadow: 0 0 0 0 rgba(45, 212, 167, .6); animation: pulseGlow 2.4s infinite; }
.channel-text { display: flex; flex-direction: column; line-height: 1.3; }
.channel-text strong { font-size: 12px; font-weight: 600; }
.channel-text small { font-size: 10.5px; color: var(--muted-2); }
.meta { display: flex; align-items: center; gap: 8px; padding: 0 6px; font-size: 10.5px; color: var(--muted-2); }
.meta .dot { width: 3px; height: 3px; border-radius: 50%; background: var(--muted-2); }

.main { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.topbar {
  position: sticky; top: 0; z-index: 30;
  display: flex; align-items: center; justify-content: space-between; gap: 18px;
  padding: 16px 30px;
  border-bottom: 1px solid var(--line);
  background: color-mix(in srgb, var(--bg) 78%, transparent);
  backdrop-filter: blur(20px) saturate(1.2);
}
.top-title { font-size: 18px; font-weight: 680; letter-spacing: -.02em; }
.top-desc { margin-top: 2px; font-size: 11.5px; color: var(--muted-2); }
.top-actions { display: flex; align-items: center; gap: 6px; }
.clock { font-size: 12.5px; color: var(--muted); padding: 0 10px; letter-spacing: .04em; }
.member-chip {
  display: inline-flex; align-items: center; gap: 6px;
  height: 30px; padding: 0 11px; border-radius: 999px;
  font-size: 11.5px; font-weight: 600; letter-spacing: .02em;
  border: 1px solid var(--line); background: var(--surface); color: var(--muted);
  transition: all var(--dur);
}
.member-chip svg { width: 14px; height: 14px; }
.member-chip:hover { border-color: var(--line-strong); color: var(--text); }
.member-chip.ok { color: var(--safe); border-color: rgba(45, 212, 167, .28); background: rgba(45, 212, 167, .08); }
.member-chip.warn { color: var(--warn); border-color: rgba(245, 181, 68, .3); background: rgba(245, 181, 68, .1); }
.member-chip.danger { color: #ff8fa3; border-color: rgba(255, 90, 120, .3); background: rgba(255, 90, 120, .1); }
@media (max-width: 640px) { .member-chip span { display: none } .member-chip { padding: 0 8px } }
.member-chip.forever { color: var(--flame-1); border-color: rgba(255, 176, 32, .32); background: rgba(255, 176, 32, .1); }

.bell-wrap, .user-wrap { position: relative; }
.dot-badge {
  position: absolute; top: 2px; right: 2px; min-width: 15px; height: 15px; padding: 0 4px;
  border-radius: 999px; background: var(--flame-3); color: #fff; font-size: 9px; font-weight: 700;
  display: grid; place-items: center; border: 2px solid var(--bg);
}
.bell-panel { position: absolute; right: 0; top: 46px; width: 320px; padding: 12px; z-index: 60; }
.bell-head { display: flex; align-items: center; justify-content: space-between; padding: 4px 6px 10px; font-size: 13px; }
.bell-head .link { background: none; border: none; color: var(--flame-2); font-size: 11.5px; cursor: pointer; }
.bell-empty { padding: 22px 10px; text-align: center; font-size: 12px; color: var(--muted-2); }
.bell-list { display: flex; flex-direction: column; gap: 4px; max-height: 320px; overflow: auto; }
.bell-list li { display: flex; gap: 9px; padding: 9px; border-radius: 10px; }
.bell-list li:hover { background: var(--surface); }
.bell-list .dot { width: 6px; height: 6px; border-radius: 50%; margin-top: 6px; flex: none; background: var(--muted); }
.bell-list li.danger .dot { background: var(--urgent); }
.bell-list li.warn .dot { background: var(--warn); }
.bell-list strong { font-size: 12px; font-weight: 600; }
.bell-list p { margin-top: 2px; font-size: 11px; color: var(--muted); line-height: 1.5; }
.bell-list small { font-size: 10px; color: var(--muted-2); }

.user { display: flex; align-items: center; gap: 9px; padding: 4px 8px 4px 4px; border-radius: 999px; border: 1px solid transparent; background: none; cursor: pointer; }
.user:hover { background: var(--surface); border-color: var(--line); }
.user-meta { display: flex; flex-direction: column; line-height: 1.2; text-align: left; }
.user-meta strong { font-size: 12.5px; font-weight: 600; }
.user-meta small { font-size: 10px; color: var(--muted-2); }
.user-menu { position: absolute; right: 0; top: 48px; width: 156px; padding: 6px; z-index: 60; }
.menu-item { display: block; width: 100%; text-align: left; padding: 9px 10px; border: none; background: none; border-radius: 9px; font-size: 12.5px; color: var(--text-2); cursor: pointer; }
.menu-item:hover { background: var(--surface-2); color: var(--text); }
.menu-item.danger { color: #ff8fa3; }

.content { padding: 26px 30px 48px; max-width: 1440px; width: 100%; margin: 0 auto; }

.mobile-nav { display: none; }

.drop-enter-active, .drop-leave-active { transition: opacity .16s var(--ease), transform .16s var(--ease); }
.drop-enter-from, .drop-leave-to { opacity: 0; transform: translateY(-6px) scale(.98); }
.view-enter-active, .view-leave-active { transition: opacity .2s var(--ease), transform .2s var(--ease); }
.view-enter-from { opacity: 0; transform: translateY(8px); }
.view-leave-to { opacity: 0; }

@media (max-width: 900px) {
  .sidebar { display: none; }
  .topbar { padding: 12px 16px; }
  .content { padding: 18px 14px 92px; }
  .mobile-nav {
    display: flex; position: fixed; left: 12px; right: 12px; bottom: 12px; z-index: 50;
    padding: 8px; border-radius: 20px; border: 1px solid var(--line-strong);
    background: color-mix(in srgb, var(--bg-elev) 88%, transparent);
    backdrop-filter: blur(20px); box-shadow: var(--shadow-pop);
  }
  .m-item { flex: 1; display: flex; flex-direction: column; align-items: center; gap: 3px; padding: 6px 0; border-radius: 12px; font-size: 10px; color: var(--muted); }
  .m-item.active { color: var(--text); background: var(--surface-2); }
  .m-item .ico :deep(svg) { width: 18px; height: 18px; }
  .m-item.active .ico { color: var(--flame-2); }
}
</style>

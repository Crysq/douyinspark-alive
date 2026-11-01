<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2 class="page-title">系统设置</h2>
        <p class="page-desc">策略、通知与数据维护。改动会立即生效。</p>
      </div>
      <button class="btn btn-primary" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存设置' }}</button>
    </div>

    <div class="settings-grid">
      <section class="card card-pad">
        <h3 class="card-title">发送策略</h3>
        <p class="card-sub">控制失败重试与同步行为</p>
        <div class="rows">
          <div class="row-item">
            <div><strong>失败重试次数</strong><span>发送失败后最多重试几次</span></div>
            <input v-model.number="form.retryTimes" type="number" min="0" max="5" class="input tiny" />
          </div>
          <div class="row-item">
            <div><strong>自动同步资料</strong><span>登录成功后拉取昵称与头像</span></div>
            <Toggle v-model="form.autoSync" />
          </div>
          <div class="row-item">
            <div><strong>保持会话活跃</strong><span>定期刷新浏览器会话，降低掉线概率</span></div>
            <Toggle v-model="form.keepAlive" />
          </div>
        </div>
      </section>

      <section class="card card-pad">
        <h3 class="card-title">静默时段</h3>
        <p class="card-sub">该时间段内调度器不会发送任何消息</p>
        <div class="grid cols-2 mt-16">
          <div class="field">
            <label>开始</label>
            <input v-model="form.quietStart" type="time" class="input" />
          </div>
          <div class="field">
            <label>结束</label>
            <input v-model="form.quietEnd" type="time" class="input" />
          </div>
        </div>
        <p class="tip">跨零点也支持，例如 23:30 – 07:00。</p>
      </section>

      <section class="card card-pad">
        <h3 class="card-title">断火通知</h3>
        <p class="card-sub">超过 24 小时未互动时提醒</p>
        <div class="rows">
          <div class="row-item">
            <div><strong>站内通知</strong><span>写入通知中心并在顶栏提醒</span></div>
            <Toggle v-model="form.notifyBreak" />
          </div>
        </div>
        <div class="field mt-16">
          <label>Webhook 地址（可选）</label>
          <input v-model.trim="form.notifyWebhook" class="input" placeholder="https://example.com/hooks/spark" />
        </div>
        <p class="tip">断火时会向该地址 POST JSON：title / body / level / at。</p>
      </section>

      <section class="card card-pad">
        <h3 class="card-title">外观</h3>
        <p class="card-sub">主题会保存在本地浏览器</p>
        <div class="theme-row">
          <button class="theme-card" :class="{ active: ui.theme === 'dark' }" @click="setTheme('dark')">
            <span class="swatch dark" /><span>暗色 · 余烬</span>
          </button>
          <button class="theme-card" :class="{ active: ui.theme === 'light' }" @click="setTheme('light')">
            <span class="swatch light" /><span>亮色 · 暖砂</span>
          </button>
        </div>
      </section>

      <section class="card card-pad">
        <h3 class="card-title">数据维护</h3>
        <p class="card-sub">演示数据明确标记，可随时清除</p>
        <div class="actions">
          <button class="btn" :disabled="working" @click="seedDemo">载入演示数据</button>
          <button class="btn" :disabled="working" @click="clearDemo">清除演示数据</button>
          <button class="btn btn-ghost" :disabled="working" @click="prune">清理 30 天前记录</button>
        </div>
      </section>

      <section class="card card-pad">
        <h3 class="card-title">账号安全</h3>
        <p class="card-sub">修改控制台登录密码</p>
        <div class="grid cols-2 mt-16">
          <div class="field">
            <label>原密码</label>
            <input v-model="password.old" type="password" class="input" autocomplete="current-password" />
          </div>
          <div class="field">
            <label>新密码（≥ 6 位）</label>
            <input v-model="password.next" type="password" class="input" autocomplete="new-password" />
          </div>
        </div>
        <button class="btn mt-16" :disabled="changing" @click="changePassword">{{ changing ? '提交中…' : '更新密码' }}</button>
      </section>
    </div>

    <section class="card card-pad about">
      <div class="about-head">
        <div>
          <h3 class="card-title">火花规则</h3>
          <p class="card-sub">与抖音 24 小时窗口一致</p>
        </div>
        <span class="badge flame">v{{ health?.version || '1.0.0' }}</span>
      </div>
      <div class="rules">
        <div v-for="rule in rules?.levels || []" :key="rule.level" class="rule">
          <span class="dot" :style="{ background: levelColor(rule.level) }" />
          <strong>{{ rule.text }}</strong>
          <span>{{ rule.rule }}</span>
        </div>
      </div>
      <ul class="streak-rules">
        <li v-for="(line, i) in rules?.streak || []" :key="i">{{ line }}</li>
      </ul>
      <div class="health">
        <span class="pill" :class="health?.mysql === 'up' ? 'ok' : 'bad'">MySQL {{ health?.mysql === 'up' ? '正常' : '异常' }}</span>
        <span class="pill" :class="health?.redis === 'up' ? 'ok' : 'bad'">Redis {{ health?.redis === 'up' ? '正常' : '异常' }}</span>
        <span class="pill" :class="health?.sign === 'ready' ? 'ok' : 'bad'">签名服务 {{ health?.sign === 'ready' ? '就绪' : '未就绪' }}</span>
        <span class="muted">已运行 {{ uptime }}</span>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import Toggle from '@/components/Toggle.vue'
import { authApi, recordApi, settingApi, systemApi } from '@/api'
import type { Health, Setting } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()
const auth = useAuthStore()
const health = ref<Health | null>(null)
const rules = ref<{ windowHours: number; levels: { level: string; text: string; rule: string }[]; streak: string[] } | null>(null)
const saving = ref(false)
const working = ref(false)
const changing = ref(false)
const password = reactive({ old: '', next: '' })

const form = reactive<Setting>({
  id: 1, retryTimes: 2, quietStart: '00:30', quietEnd: '07:00',
  notifyBreak: true, notifyWebhook: '', autoSync: true, keepAlive: true, theme: 'dark', updatedAt: '',
})

const uptime = computed(() => {
  const sec = health.value?.uptimeSec ?? 0
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  return h > 0 ? `${h} 小时 ${m} 分` : `${m} 分钟`
})

function levelColor(level: string) {
  return ({ safe: '#2dd4a7', warn: '#f5b544', urgent: '#ff7a5c', broken: '#7b7b90' } as Record<string, string>)[level] || '#ff7a18'
}

function setTheme(value: 'dark' | 'light') {
  ui.theme = value
  ui.applyTheme()
}

async function save() {
  saving.value = true
  try {
    await settingApi.save({ ...form, theme: ui.theme })
    ui.toast('success', '设置已保存')
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    saving.value = false
  }
}

async function seedDemo() {
  working.value = true
  try {
    const result = await systemApi.demo()
    ui.toast('success', `演示数据已生成：${result.friends} 位好友，${result.records} 条记录`)
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    working.value = false
  }
}

async function clearDemo() {
  if (!window.confirm('清除所有标记为演示数据的账号、好友与记录？')) return
  working.value = true
  try {
    await systemApi.clearDemo()
    ui.toast('success', '演示数据已清除')
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    working.value = false
  }
}

async function prune() {
  if (!window.confirm('清理 30 天之前的发送记录？')) return
  working.value = true
  try {
    const result = await recordApi.prune(30)
    ui.toast('success', `已清理 ${result.deleted} 条记录`)
  } finally {
    working.value = false
  }
}

async function changePassword() {
  if (password.next.length < 6) { ui.toast('error', '新密码至少 6 位'); return }
  changing.value = true
  try {
    await authApi.password(password.old, password.next)
    ui.toast('success', '密码已更新，请重新登录')
    password.old = ''; password.next = ''
    await auth.logout()
    location.href = '/login'
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    changing.value = false
  }
}

onMounted(async () => {
  const [setting, healthData, ruleData] = await Promise.all([settingApi.get(), systemApi.health(), systemApi.rules()])
  Object.assign(form, setting)
  health.value = healthData
  rules.value = ruleData
  if (setting.theme === 'light' || setting.theme === 'dark') {
    ui.theme = setting.theme
    ui.applyTheme()
  }
})
</script>

<style scoped>
.settings-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
.rows { display: flex; flex-direction: column; gap: 4px; margin-top: 14px; }
.row-item { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 11px 0; border-bottom: 1px solid var(--line); }
.row-item:last-child { border-bottom: none; }
.row-item div { display: flex; flex-direction: column; gap: 3px; }
.row-item strong { font-size: 12.8px; font-weight: 600; }
.row-item span { font-size: 11px; color: var(--muted-2); }
.input.tiny { width: 74px; height: 34px; text-align: center; }
.tip { margin-top: 10px; font-size: 11px; color: var(--muted-2); }

.theme-row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 16px; }
.theme-card { display: flex; flex-direction: column; gap: 8px; padding: 12px; border-radius: 14px; border: 1px solid var(--line); background: var(--surface); cursor: pointer; font-size: 12px; color: var(--text-2); text-align: left; }
.theme-card.active { border-color: rgba(255, 122, 24, .5); box-shadow: 0 0 0 1px rgba(255, 122, 24, .2); }
.swatch { height: 46px; border-radius: 9px; display: block; }
.swatch.dark { background: linear-gradient(135deg, #101018, #26263a); }
.swatch.light { background: linear-gradient(135deg, #fffdf9, #ece4d8); }

.actions { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 16px; }

.about { display: flex; flex-direction: column; gap: 16px; }
.about-head { display: flex; align-items: flex-start; justify-content: space-between; }
.rules { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.rule { display: flex; flex-direction: column; gap: 5px; padding: 12px; border-radius: 12px; background: var(--surface); border: 1px solid var(--line); font-size: 11.5px; color: var(--muted); }
.rule .dot { width: 8px; height: 8px; border-radius: 50%; }
.rule strong { font-size: 12.5px; color: var(--text-2); }
.streak-rules { display: flex; flex-direction: column; gap: 6px; font-size: 12px; color: var(--muted); }
.streak-rules li::before { content: "·"; margin-right: 8px; color: var(--flame-2); }
.health { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; font-size: 11px; }
.pill { padding: 4px 9px; border-radius: 999px; border: 1px solid var(--line); }
.pill.ok { color: var(--safe); border-color: rgba(45, 212, 167, .3); background: rgba(45, 212, 167, .08); }
.pill.bad { color: var(--urgent); border-color: rgba(255, 122, 92, .3); background: rgba(255, 122, 92, .08); }

@media (max-width: 980px) {
  .settings-grid { grid-template-columns: minmax(0, 1fr); }
  .rules { grid-template-columns: repeat(2, 1fr); }
}
</style>

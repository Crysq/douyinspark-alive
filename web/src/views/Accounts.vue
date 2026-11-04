<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2 class="page-title">账号管理</h2>
        <p class="page-desc">每个账号拥有独立的浏览器会话与设备指纹，互不影响。</p>
      </div>
      <button class="btn btn-primary" @click="openCreate">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14" /></svg>
        添加账号
      </button>
    </div>

    <div v-if="loading" class="account-grid">
      <div v-for="i in 3" :key="i" class="card skeleton" style="height: 220px" />
    </div>

    <div v-else-if="accounts.length" class="account-grid">
      <article v-for="account in accounts" :key="account.id" class="account card card-hover">
        <header class="a-head">
          <Avatar :name="account.nickname" :src="account.avatarUrl" :size="48" />
          <div class="who">
            <div class="row gap-8">
              <strong class="ellipsis">{{ account.nickname }}</strong>
              <span class="badge" :class="statusClass(account.status)">
                <i class="dot" />{{ statusText(account.status) }}
              </span>
            </div>
            <small class="ellipsis">{{ account.uniqueId ? '@' + account.uniqueId : '尚未同步资料' }}</small>
            <span v-if="inRisk(account)" class="risk-tag" :title="account.riskReason">
              风控冷却至 {{ formatDay(account.riskUntil) }}
            </span>
          </div>
          <button class="btn btn-icon btn-sm btn-ghost" title="更多" @click="menuFor = menuFor === account.id ? 0 : account.id">
            <svg viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.6" /><circle cx="12" cy="12" r="1.6" /><circle cx="19" cy="12" r="1.6" /></svg>
          </button>
        </header>

        <div class="stats">
          <div><b class="num">{{ account.friendCount }}</b><span>好友</span></div>
          <div><b class="num">{{ account.sparkCount }}</b><span>火花</span></div>
          <div><b class="num danger">{{ account.brokenCount }}</b><span>待续火</span></div>
        </div>

        <p class="sync">最近同步：{{ timeAgo(account.lastSyncAt) }}</p>

        <footer class="a-foot">
          <button v-if="inRisk(account)" class="btn btn-sm" @click="clearRisk(account)">解除冷却</button>
          <button class="btn btn-sm" @click="openQR(account)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="3" y="3" width="7" height="7" rx="1.5" /><rect x="14" y="3" width="7" height="7" rx="1.5" /><rect x="3" y="14" width="7" height="7" rx="1.5" /><path d="M14 14h3v3h-3zM20 20h1M14 20h2M20 14h1" /></svg>
            扫码登录
          </button>
          <button class="btn btn-sm" @click="syncAll(account)">同步</button>
          <button v-if="account.status === 'online'" class="btn btn-sm btn-ghost" @click="logout(account)">下线</button>
          <button v-else class="btn btn-sm btn-ghost" @click="openCookie(account)">导入 Cookie</button>
        </footer>

        <Transition name="drop">
          <div v-if="menuFor === account.id" class="menu card">
            <button @click="openCookie(account); menuFor = 0">导入 Cookie</button>
            <button @click="syncAll(account); menuFor = 0">同步资料与好友</button>
            <button class="danger" @click="remove(account); menuFor = 0">删除账号</button>
          </div>
        </Transition>
      </article>
    </div>

    <EmptyState v-else title="还没有托管账号" desc="添加一个抖音账号，扫码登录后即可同步好友并开始自动续火。">
      <template #action><button class="btn btn-primary btn-sm" @click="openCreate">添加第一个账号</button></template>
    </EmptyState>

    <Modal v-model="createOpen" title="添加托管账号" subtitle="先起个名字，登录后再自动同步抖音资料">
      <div class="field">
        <label>账号名称</label>
        <input v-model.trim="form.nickname" class="input" placeholder="例如：小满" />
      </div>
      <div class="field">
        <label>备注（可选）</label>
        <input v-model.trim="form.remark" class="input" placeholder="例如：主号 / 备用号" />
      </div>
      <template #footer>
        <button class="btn" @click="createOpen = false">取消</button>
        <button class="btn btn-primary" :disabled="saving" @click="submitCreate">{{ saving ? '创建中…' : '创建' }}</button>
      </template>
    </Modal>

    <Modal v-model="qrOpen" title="扫码登录" :subtitle="qrAccount?.nickname" width="440px">
      <div class="qr-box">
        <div v-if="qrLogged" class="qr-done">
          <span class="ok">✓</span>
          <strong>登录成功</strong>
          <p>账号已上线，正在同步资料…</p>
        </div>
        <template v-else>
          <img v-if="qrData" :src="qrData" alt="抖音登录二维码" class="qr-img" />
          <div v-else class="qr-loading skeleton" />
          <p class="qr-tip">打开抖音 App → 右上角扫一扫 → 确认登录</p>
          <p class="qr-hint">二维码约 60 秒刷新一次，登录后窗口会自动关闭</p>
        </template>
      </div>
      <template #footer>
        <button class="btn" @click="closeQR">{{ qrLogged ? '完成' : '关闭' }}</button>
      </template>
    </Modal>

    <Modal v-model="cookieOpen" title="导入 Cookie" subtitle="从浏览器复制完整 Cookie，需包含 sessionid" width="620px">
      <div class="field">
        <label>Cookie 字符串</label>
        <textarea v-model.trim="cookieValue" class="textarea" rows="7" placeholder="sessionid=...; passport_csrf_token=...; ttwid=..."></textarea>
      </div>
      <p class="hint-line">导入后会立即校验 sessionid 是否存在，并同步账号资料。</p>
      <template #footer>
        <button class="btn" @click="cookieOpen = false">取消</button>
        <button class="btn btn-primary" :disabled="importing" @click="submitCookie">{{ importing ? '导入中…' : '导入并校验' }}</button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import Avatar from '@/components/Avatar.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import { accountApi } from '@/api'
import type { Account } from '@/api/types'
import { useUiStore } from '@/stores/ui'
import { formatDay, timeAgo } from '@/utils/format'

const ui = useUiStore()
const accounts = ref<Account[]>([])
const loading = ref(true)
const menuFor = ref(0)

const createOpen = ref(false)
const saving = ref(false)
const form = reactive({ nickname: '', remark: '' })

const qrOpen = ref(false)
const qrData = ref('')
const qrLogged = ref(false)
const qrAccount = ref<Account | null>(null)
let poll: number | undefined

const cookieOpen = ref(false)
const cookieValue = ref('')
const cookieAccount = ref<Account | null>(null)
const importing = ref(false)

function statusText(status: string) {
  return ({ online: '在线', offline: '离线', expired: '已过期' } as Record<string, string>)[status] || status
}
function statusClass(status: string) {
  return status === 'online' ? 'safe' : status === 'expired' ? 'urgent' : 'broken'
}

async function load() {
  loading.value = true
  try { accounts.value = await accountApi.list() } finally { loading.value = false }
}

function openCreate() {
  form.nickname = ''; form.remark = ''
  createOpen.value = true
}

async function submitCreate() {
  if (!form.nickname) { ui.toast('error', '请填写账号名称'); return }
  saving.value = true
  try {
    await accountApi.create({ ...form })
    ui.toast('success', '账号已创建，请扫码登录')
    createOpen.value = false
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    saving.value = false
  }
}

async function openQR(account: Account) {
  qrAccount.value = account
  qrData.value = ''
  qrLogged.value = false
  qrOpen.value = true
  try {
    const result = await accountApi.loginQR(account.id)
    if (result.logged) {
      qrLogged.value = true
      await load()
      return
    }
    qrData.value = result.dataUrl
    startPolling(account.id)
  } catch (error) {
    ui.toast('error', (error as Error).message)
    qrOpen.value = false
  }
}

function startPolling(id: number) {
  window.clearInterval(poll)
  poll = window.setInterval(async () => {
    try {
      const status = await accountApi.loginStatus(id)
      if (status.logged) {
        qrLogged.value = true
        window.clearInterval(poll)
        ui.toast('success', '登录成功，账号已上线')
        await load()
      }
    } catch { /* 继续轮询 */ }
  }, 2200)
}

function closeQR() {
  window.clearInterval(poll)
  qrOpen.value = false
}

function openCookie(account: Account) {
  cookieAccount.value = account
  cookieValue.value = ''
  cookieOpen.value = true
}

async function submitCookie() {
  if (!cookieAccount.value || !cookieValue.value) return
  importing.value = true
  try {
    await accountApi.cookie(cookieAccount.value.id, cookieValue.value)
    ui.toast('success', 'Cookie 已导入，账号在线')
    cookieOpen.value = false
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    importing.value = false
  }
}

async function syncAll(account: Account) {
  try {
    await accountApi.sync(account.id)
    const result = await accountApi.syncFriends(account.id)
    ui.toast('success', `已同步：好友新增 ${result.created}，更新 ${result.updated}`)
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  }
}

async function logout(account: Account) {
  if (!window.confirm(`确定让「${account.nickname}」下线？浏览器会话会被清除。`)) return
  try {
    await accountApi.logout(account.id)
    ui.toast('success', '账号已下线')
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  }
}

function inRisk(account: Account) {
  return Boolean(account.riskUntil && new Date(account.riskUntil).getTime() > Date.now())
}

async function clearRisk(account: Account) {
  try {
    await accountApi.update(account.id, { clearRisk: true })
    ui.toast('success', '已解除风控冷却')
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  }
}

async function remove(account: Account) {
  if (!window.confirm(`删除「${account.nickname}」会同时移除其好友，确定？`)) return
  try {
    await accountApi.remove(account.id)
    ui.toast('success', '账号已删除')
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  }
}

onMounted(load)
onBeforeUnmount(() => window.clearInterval(poll))
</script>

<style scoped>
.account-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 18px; }
.account { padding: 18px 20px; display: flex; flex-direction: column; gap: 14px; position: relative; }
.a-head { display: flex; align-items: center; gap: 12px; }
.who { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; line-height: 1.25; }
.who strong { font-size: 15px; font-weight: 650; }
.who small { font-size: 11.5px; color: var(--muted-2); }
.stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; padding: 12px; border-radius: 14px; background: var(--surface); border: 1px solid var(--line); }
.stats div { display: flex; flex-direction: column; align-items: center; gap: 2px; }
.stats b { font-size: 19px; font-weight: 700; }
.stats b.danger { color: var(--urgent); }
.stats span { font-size: 10.5px; color: var(--muted-2); }
.sync { font-size: 11px; color: var(--muted-2); }
.risk-tag { margin-top: 2px; font-size: 10px; color: #ff8fa3; background: rgba(255, 90, 120, .12); border: 1px solid rgba(255, 90, 120, .25); border-radius: 6px; padding: 1px 6px; width: fit-content; }
.a-foot { display: flex; gap: 6px; flex-wrap: wrap; padding-top: 12px; border-top: 1px solid var(--line); }

.menu { position: absolute; right: 14px; top: 58px; z-index: 20; padding: 6px; width: 168px; }
.menu button { display: block; width: 100%; text-align: left; padding: 8px 10px; border: none; background: none; border-radius: 9px; font-size: 12.5px; color: var(--text-2); cursor: pointer; }
.menu button:hover { background: var(--surface-2); color: var(--text); }
.menu button.danger { color: #ff8fa3; }

.qr-box { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 8px 0 4px; }
.qr-img { width: 232px; height: 232px; border-radius: 16px; background: #fff; padding: 10px; object-fit: contain; }
.qr-loading { width: 232px; height: 232px; border-radius: 16px; }
.qr-tip { font-size: 12.5px; color: var(--text-2); }
.qr-hint { font-size: 11px; color: var(--muted-2); }
.qr-done { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 40px 0; }
.qr-done .ok { width: 56px; height: 56px; border-radius: 50%; display: grid; place-items: center; font-size: 26px; color: #fff; background: linear-gradient(135deg, #2dd4a7, #12a37f); }
.hint-line { font-size: 11.5px; color: var(--muted-2); }

@media (max-width: 1180px) { .account-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 720px) { .account-grid { grid-template-columns: minmax(0, 1fr); } }
</style>

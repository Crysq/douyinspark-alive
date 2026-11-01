<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2 class="page-title">火花好友</h2>
        <p class="page-desc">倒计时环显示距离断火还剩多久，紧急的好友优先处理。</p>
      </div>
      <div class="actions">
        <button class="btn" :disabled="syncing" :title="accounts.length ? '从私信会话列表同步好友' : '还没有托管账号，先去账号管理添加'" @click="syncFriends">
          <svg v-if="!syncing" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-2.6-6.4" /><path d="M21 3v6h-6" /></svg>
          <span v-else class="spin mini" />
          {{ syncing ? '同步中…' : '同步会话' }}
        </button>
        <button class="btn btn-primary" @click="openCreate">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14" /></svg>
          添加好友
        </button>
      </div>
    </div>

    <section class="filters card card-pad">
      <div class="level-tabs">
        <button v-for="tab in levelTabs" :key="tab.value" :class="{ active: level === tab.value }" @click="setLevel(tab.value)">
          {{ tab.label }}
          <span v-if="tab.value === 'all'" class="count">{{ total }}</span>
        </button>
      </div>
      <div class="filter-right">
        <select v-model="accountId" class="select" @change="reload">
          <option value="0">全部账号</option>
          <option v-for="account in accounts" :key="account.id" :value="String(account.id)">{{ account.nickname }}</option>
        </select>
        <div class="input-icon search">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.2-3.2" /></svg>
          <input v-model="keyword" class="input" placeholder="搜索昵称 / 抖音号" @input="debouncedReload" />
        </div>
        <select v-model="sort" class="select sort" @change="reload">
          <option value="streak">按连续天数</option>
          <option value="recent">按最近互动</option>
          <option value="name">按昵称</option>
          <option value="created">按添加时间</option>
        </select>
      </div>
    </section>

    <Transition name="bar">
      <div v-if="selected.size" class="batch-bar">
        <span>已选 <b>{{ selected.size }}</b> 位好友</span>
        <div class="row gap-10">
          <button class="btn btn-sm" @click="selected.clear()">取消</button>
          <button class="btn btn-sm btn-primary" :disabled="batching" @click="batchKeep">
            {{ batching ? '发送中…' : '批量续火' }}
          </button>
        </div>
      </div>
    </Transition>

    <div v-if="loading" class="friend-grid">
      <div v-for="i in 6" :key="i" class="card skeleton" style="height: 186px" />
    </div>

    <div v-else-if="friends.length" class="friend-grid">
      <article v-for="friend in friends" :key="friend.id" class="friend card card-hover" :class="{ picked: selected.has(friend.id) }">
        <header class="f-head">
          <label class="pick">
            <input type="checkbox" :checked="selected.has(friend.id)" @change="togglePick(friend.id)" />
            <span />
          </label>
          <Avatar :name="friend.name" :src="friend.avatarUrl" :size="40" />
          <div class="who">
            <strong class="ellipsis">{{ friend.name }}</strong>
            <small class="ellipsis">{{ friend.uniqueId ? '@' + friend.uniqueId : friend.accountName }}</small>
          </div>
          <span class="badge" :class="friend.state.level">{{ friend.state.levelText }}</span>
        </header>

        <div class="f-body">
          <SparkRing :remaining-sec="friend.state.remainingSec" :progress="friend.state.progress" :level="friend.state.level" :size="78" caption="剩余" />
          <div class="metrics">
            <div class="metric">
              <b class="num">{{ friend.streakDays }}</b>
              <span>连续天数</span>
            </div>
            <div class="metric">
              <b class="num">{{ friend.maxStreakDays }}</b>
              <span>历史最高</span>
            </div>
            <div class="metric wide">
              <b class="small">{{ timeAgo(friend.lastInteractAt) }}</b>
              <span>上次互动</span>
            </div>
          </div>
        </div>

        <footer class="f-foot">
          <div class="auto">
            <Toggle :model-value="friend.autoKeep" @update:model-value="(v) => toggleAuto(friend, v)" />
            <span>自动续火</span>
          </div>
          <div class="row gap-6">
            <button class="btn btn-sm btn-ghost" @click="remove(friend)" title="删除">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M4 7h16M9 7V5h6v2M6 7l1 13h10l1-13" /></svg>
            </button>
            <button class="btn btn-sm btn-primary" :disabled="keeping === friend.id" @click="keep(friend)">
              {{ keeping === friend.id ? '发送中' : '立即续火' }}
            </button>
          </div>
        </footer>
      </article>
    </div>

    <EmptyState v-else title="没有匹配的好友" desc="调整筛选条件，或从账号页同步私信会话，也可以手动添加好友。">
      <template #action><button class="btn btn-sm" @click="openCreate">添加好友</button></template>
    </EmptyState>

    <div v-if="total > pageSize" class="pager">
      <button class="btn btn-sm" :disabled="page === 1" @click="go(page - 1)">上一页</button>
      <span class="num">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
      <button class="btn btn-sm" :disabled="page * pageSize >= total" @click="go(page + 1)">下一页</button>
    </div>

    <Modal v-model="createOpen" title="添加火花好友" subtitle="需要对方的数字 user_id 才能发送私信">
      <div class="grid cols-2">
        <div class="field">
          <label>所属账号</label>
          <select v-model.number="form.accountId" class="select">
            <option v-for="account in accounts" :key="account.id" :value="account.id">{{ account.nickname }}</option>
          </select>
        </div>
        <div class="field">
          <label>好友昵称</label>
          <input v-model.trim="form.name" class="input" placeholder="例如：橘子汽水" />
        </div>
      </div>
      <div class="field">
        <label>user_id（数字，私信接口使用）</label>
        <input v-model.trim="form.userId" class="input" placeholder="例如：1234567890123" />
      </div>
      <div class="grid cols-2">
        <div class="field">
          <label>抖音号（可选）</label>
          <input v-model.trim="form.uniqueId" class="input" placeholder="unique_id" />
        </div>
        <div class="field">
          <label>当前连续天数（可选）</label>
          <input v-model.number="form.streakDays" type="number" class="input" placeholder="0" />
        </div>
      </div>
      <div class="field">
        <label>备注</label>
        <input v-model.trim="form.remark" class="input" placeholder="例如：大学同学" />
      </div>
      <template #footer>
        <button class="btn" @click="createOpen = false">取消</button>
        <button class="btn btn-primary" :disabled="saving" @click="submitCreate">{{ saving ? '保存中…' : '添加' }}</button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import Avatar from '@/components/Avatar.vue'
import SparkRing from '@/components/SparkRing.vue'
import Toggle from '@/components/Toggle.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import { accountApi, friendApi } from '@/api'
import type { Account, Friend } from '@/api/types'
import { useUiStore } from '@/stores/ui'
import { timeAgo } from '@/utils/format'

const ui = useUiStore()
const router = useRouter()
const accounts = ref<Account[]>([])
const friends = ref<Friend[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 9
const level = ref('all')
const keyword = ref('')
const accountId = ref('0')
const sort = ref('streak')
const loading = ref(true)
const keeping = ref(0)
const syncing = ref(false)
const batching = ref(false)
const selected = reactive(new Set<number>())
const createOpen = ref(false)
const saving = ref(false)

const levelTabs = [
  { value: 'all', label: '全部' },
  { value: 'safe', label: '安全' },
  { value: 'warn', label: '预警' },
  { value: 'urgent', label: '紧急' },
  { value: 'broken', label: '已断火' },
]

const form = reactive({ accountId: 0, name: '', userId: '', uniqueId: '', streakDays: 0, remark: '' })

async function load() {
  loading.value = true
  try {
    const data = await friendApi.list({
      page: page.value, pageSize, level: level.value, keyword: keyword.value,
      accountId: accountId.value, sort: sort.value,
    })
    friends.value = data.items
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function reload() { page.value = 1; load() }
function go(next: number) { page.value = next; load() }
function setLevel(value: string) { level.value = value; reload() }

let timer: number | undefined
function debouncedReload() {
  window.clearTimeout(timer)
  timer = window.setTimeout(reload, 320)
}

async function toggleAuto(friend: Friend, value: boolean) {
  friend.autoKeep = value
  try {
    await friendApi.update(friend.id, { autoKeep: value })
    ui.toast('success', value ? '已开启自动续火' : '已关闭自动续火')
  } catch (error) {
    friend.autoKeep = !value
    ui.toast('error', (error as Error).message)
  }
}

async function keep(friend: Friend) {
  keeping.value = friend.id
  try {
    const result = await friendApi.keep(friend.id)
    ui.toast('success', `已发给 ${friend.name}：${result.record.content}`)
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    keeping.value = 0
  }
}

async function batchKeep() {
  batching.value = true
  try {
    const result = await friendApi.batchKeep([...selected])
    ui.toast(result.failed ? 'info' : 'success', `成功 ${result.success} 位，失败 ${result.failed} 位`)
    selected.clear()
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    batching.value = false
  }
}

async function remove(friend: Friend) {
  if (!window.confirm(`确定删除好友「${friend.name}」？历史记录会保留。`)) return
  await friendApi.remove(friend.id)
  ui.toast('success', '已删除')
  await load()
}

function togglePick(id: number) {
  if (selected.has(id)) selected.delete(id)
  else selected.add(id)
}

function openCreate() {
  form.accountId = accounts.value[0]?.id || 0
  createOpen.value = true
}

async function submitCreate() {
  if (!form.name || !form.accountId) {
    ui.toast('error', '请填写账号与昵称')
    return
  }
  saving.value = true
  try {
    await friendApi.create({ ...form })
    ui.toast('success', '好友已添加')
    createOpen.value = false
    form.name = ''; form.userId = ''; form.uniqueId = ''; form.streakDays = 0; form.remark = ''
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    saving.value = false
  }
}

async function syncFriends() {
  if (!accounts.value.length) {
    ui.toast('info', '还没有托管账号，先去账号管理添加并扫码登录')
    router.push('/accounts')
    return
  }
  const targets = accountId.value !== '0'
    ? accounts.value.filter((item) => item.id === Number(accountId.value))
    : accounts.value
  const online = targets.filter((item) => item.status === 'online')
  if (!online.length) {
    ui.toast('error', '所选账号还未登录，请先扫码登录后再同步')
    router.push('/accounts')
    return
  }

  syncing.value = true
  let created = 0, updated = 0
  const failures: string[] = []
  try {
    for (const account of online) {
      try {
        const result = await accountApi.syncFriends(account.id)
        created += result.created
        updated += result.updated
      } catch (error) {
        failures.push(`${account.nickname}：${(error as Error).message}`)
      }
    }
    if (created || updated) {
      ui.toast('success', `同步完成：新增 ${created} 位，更新 ${updated} 位`)
    }
    if (failures.length) {
      ui.toast('error', failures[0])
    }
    if (!created && !updated && !failures.length) {
      ui.toast('info', '会话列表为空，或接口未返回可识别的数据')
    }
    await load()
  } finally {
    syncing.value = false
  }
}

onMounted(async () => {
  accounts.value = await accountApi.list()
  await load()
})
</script>

<style scoped>
.actions { display: flex; gap: 10px; }
.spin.mini { width: 13px; height: 13px; border-radius: 50%; border: 2px solid var(--line-strong); border-top-color: var(--flame-2); display: inline-block; }

.filters { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; padding: 14px 18px; }
.level-tabs { display: flex; gap: 4px; background: var(--surface); padding: 3px; border-radius: 12px; border: 1px solid var(--line); }
.level-tabs button { display: flex; align-items: center; gap: 6px; padding: 7px 13px; border: none; background: none; border-radius: 9px; font-size: 12.5px; color: var(--muted); cursor: pointer; transition: all var(--dur); }
.level-tabs button:hover { color: var(--text); }
.level-tabs button.active { background: var(--surface-3); color: var(--text); box-shadow: inset 0 0 0 1px var(--line-strong); }
.level-tabs .count { font-size: 10px; padding: 1px 6px; border-radius: 999px; background: var(--surface-3); color: var(--muted); }
.filter-right { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.filter-right .select { width: 132px; }
.search { width: 210px; }
.sort { width: 140px; }

.batch-bar {
  display: flex; align-items: center; justify-content: space-between; gap: 12px;
  padding: 10px 16px; border-radius: 14px;
  border: 1px solid rgba(255, 122, 24, .28);
  background: linear-gradient(90deg, rgba(255, 122, 24, .12), rgba(255, 61, 113, .06));
  font-size: 12.5px; color: var(--text-2);
}
.batch-bar b { color: var(--flame-1); }
.bar-enter-active, .bar-leave-active { transition: all .2s var(--ease); }
.bar-enter-from, .bar-leave-to { opacity: 0; transform: translateY(-6px); }

.friend-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.friend { padding: 16px 18px; display: flex; flex-direction: column; gap: 14px; transition: border-color var(--dur), box-shadow var(--dur), transform var(--dur); }
.friend.picked { border-color: rgba(255, 122, 24, .45); box-shadow: 0 0 0 1px rgba(255, 122, 24, .22), var(--shadow-card); }
.f-head { display: flex; align-items: center; gap: 10px; }
.pick { position: relative; width: 16px; height: 16px; flex: none; cursor: pointer; }
.pick input { position: absolute; opacity: 0; }
.pick span { display: block; width: 16px; height: 16px; border-radius: 5px; border: 1px solid var(--line-strong); background: var(--surface); transition: all var(--dur); }
.pick input:checked + span { background: var(--flame-grad); border-color: transparent; }
.pick input:checked + span::after { content: "✓"; display: grid; place-items: center; font-size: 10px; color: #fff; }
.who { flex: 1; min-width: 0; display: flex; flex-direction: column; line-height: 1.3; }
.who strong { font-size: 13.5px; font-weight: 620; }
.who small { font-size: 11px; color: var(--muted-2); }

.f-body { display: flex; align-items: center; gap: 18px; }
.metrics { flex: 1; display: grid; grid-template-columns: 1fr 1fr; gap: 8px 10px; }
.metric { display: flex; flex-direction: column; }
.metric b { font-size: 19px; font-weight: 700; letter-spacing: -.02em; }
.metric b.small { font-size: 12px; font-weight: 600; color: var(--text-2); }
.metric span { font-size: 10px; color: var(--muted-2); }
.metric.wide { grid-column: span 2; }

.f-foot { display: flex; align-items: center; justify-content: space-between; padding-top: 12px; border-top: 1px solid var(--line); }
.auto { display: flex; align-items: center; gap: 8px; font-size: 11.5px; color: var(--muted); }

.pager { display: flex; align-items: center; justify-content: center; gap: 14px; font-size: 12px; color: var(--muted); }

@media (max-width: 1180px) { .friend-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 760px) {
  .friend-grid { grid-template-columns: minmax(0, 1fr); }
  .filters { padding: 12px; }
  .filter-right, .search, .filter-right .select, .sort { width: 100%; }
  .level-tabs { overflow-x: auto; width: 100%; }
}
</style>

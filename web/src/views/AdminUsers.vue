<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2 class="page-title">用户管理</h2>
        <p class="page-desc">客户列表、会员时长与账号状态，都在这里。</p>
      </div>
      <router-link to="/admin/cards" class="btn">去卡密管理</router-link>
    </div>

    <div class="stats">
      <div class="card"><span>注册用户</span><b class="num">{{ stats.users }}</b></div>
      <div class="card"><span>有效会员</span><b class="num ok">{{ stats.members }}</b></div>
      <div class="card"><span>7 日活跃</span><b class="num">{{ stats.activeUsers }}</b></div>
      <div class="card"><span>未使用卡密</span><b class="num warn">{{ stats.cardsLeft }}</b></div>
      <div class="card"><span>托管账号</span><b class="num">{{ stats.accounts }}</b></div>
      <div class="card"><span>累计发送</span><b class="num">{{ stats.records }}</b></div>
      <div class="card revenue"><span>卡密营收（元）</span><b class="num">¥{{ stats.revenue }}</b></div>
    </div>

    <section class="card card-pad distribution" v-if="stats.planDistribution?.length">
      <h3 class="card-title">卡种分布</h3>
      <div class="dist-row">
        <div v-for="item in stats.planDistribution" :key="item.plan" class="dist-item">
          <strong>{{ item.name }}</strong><span>{{ item.count }} 张</span>
        </div>
      </div>
    </section>

    <section class="card">
      <div class="filters">
        <div class="seg">
          <button v-for="tab in tabs" :key="tab.value" :class="{ active: status === tab.value }" @click="setStatus(tab.value)">{{ tab.label }}</button>
        </div>
        <div class="input-icon search">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.2-3.2" /></svg>
          <input v-model="keyword" class="input" placeholder="搜索用户名 / 昵称" @input="debounced" />
        </div>
      </div>

      <div v-if="loading" class="loading">
        <div v-for="i in 5" :key="i" class="skeleton" style="height: 48px" />
      </div>

      <div v-else-if="rows.length" class="table-wrap">
        <table class="table">
          <thead>
            <tr><th>用户</th><th>会员</th><th>到期</th><th>账号 / 好友</th><th>最近登录</th><th>状态</th><th>操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.user.id">
              <td>
                <div class="who">
                  <Avatar :name="row.user.nickname || row.user.username" :size="32" />
                  <div>
                    <strong>{{ row.user.nickname || row.user.username }}</strong>
                    <small>@{{ row.user.username }}</small>
                  </div>
                </div>
              </td>
              <td>
                <span class="badge" :class="row.user.isMember ? (row.user.daysLeft < 0 ? 'flame' : 'safe') : 'broken'">{{ row.user.planLabel }}</span>
              </td>
              <td class="num">
                {{ row.user.daysLeft < 0 ? '永久' : (row.user.memberExpireAt ? formatDay(row.user.memberExpireAt) : '—') }}
                <small v-if="row.user.isMember && row.user.daysLeft >= 0" class="left">剩 {{ row.user.daysLeft }} 天</small>
              </td>
              <td class="num">{{ row.accountCount }} / {{ row.friendCount }}</td>
              <td class="num muted">{{ timeAgo(row.user.lastLoginAt) }}</td>
              <td><span class="badge" :class="row.user.status === 'banned' ? 'urgent' : 'blue'">{{ row.user.status === 'banned' ? '已禁用' : '正常' }}</span></td>
              <td>
                <div class="row gap-6">
                  <button class="btn btn-sm" @click="openGrant(row)">赠送会员</button>
                  <button class="btn btn-sm btn-ghost" :class="{ danger: row.user.status !== 'banned' }" @click="toggleBan(row)">
                    {{ row.user.status === 'banned' ? '恢复' : '禁用' }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <EmptyState v-else title="暂无用户" desc="客户通过注册页使用卡密开通后，会出现在这里。" />
    </section>

    <div v-if="total > pageSize" class="pager">
      <button class="btn btn-sm" :disabled="page === 1" @click="go(page - 1)">上一页</button>
      <span class="num">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
      <button class="btn btn-sm" :disabled="page * pageSize >= total" @click="go(page + 1)">下一页</button>
    </div>

    <Modal v-model="grantOpen" title="赠送会员" :subtitle="grantTarget?.user.username" width="440px">
      <div class="field">
        <label>选择卡种</label>
        <select v-model="grantPlan" class="select">
          <option v-for="plan in plans" :key="plan.code" :value="plan.code">{{ plan.name }} · {{ plan.days ? plan.days + ' 天' : '永久' }}</option>
        </select>
      </div>
      <p class="hint">天数会叠加在对方当前到期时间之后。</p>
      <template #footer>
        <button class="btn" @click="grantOpen = false">取消</button>
        <button class="btn btn-primary" :disabled="granting" @click="grant">{{ granting ? '处理中…' : '确认赠送' }}</button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Avatar from '@/components/Avatar.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import { adminApi, membershipApi } from '@/api'
import type { AdminStats, AdminUserRow, Plan } from '@/api/types'
import { useUiStore } from '@/stores/ui'
import { formatDay, timeAgo } from '@/utils/format'

const ui = useUiStore()
const rows = ref<AdminUserRow[]>([])
const stats = ref<AdminStats>({ users: 0, members: 0, activeUsers: 0, cards: 0, cardsUsed: 0, cardsLeft: 0, accounts: 0, records: 0, revenue: 0, planDistribution: [] })
const plans = ref<Plan[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 15
const status = ref('all')
const keyword = ref('')
const loading = ref(true)

const grantOpen = ref(false)
const grantTarget = ref<AdminUserRow | null>(null)
const grantPlan = ref('month')
const granting = ref(false)

const tabs = [
  { value: 'all', label: '全部' },
  { value: 'active', label: '有效会员' },
  { value: 'expired', label: '已过期' },
  { value: 'banned', label: '已禁用' },
]

async function load() {
  loading.value = true
  try {
    const [list, stat] = await Promise.all([
      adminApi.users({ page: page.value, pageSize, status: status.value, keyword: keyword.value }),
      adminApi.stats(),
    ])
    rows.value = list.items
    total.value = list.total
    stats.value = stat
  } finally {
    loading.value = false
  }
}

function reload() { page.value = 1; load() }
function go(next: number) { page.value = next; load() }
function setStatus(value: string) { status.value = value; reload() }

let timer: number | undefined
function debounced() {
  window.clearTimeout(timer)
  timer = window.setTimeout(reload, 320)
}

function openGrant(row: AdminUserRow) {
  grantTarget.value = row
  grantPlan.value = 'month'
  grantOpen.value = true
}

async function grant() {
  if (!grantTarget.value) return
  granting.value = true
  try {
    await adminApi.grant(grantTarget.value.user.id, grantPlan.value)
    ui.toast('success', '已赠送会员')
    grantOpen.value = false
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    granting.value = false
  }
}

async function toggleBan(row: AdminUserRow) {
  const next = row.user.status === 'banned' ? 'active' : 'banned'
  if (next === 'banned' && !window.confirm(`禁用用户「${row.user.username}」？其将无法登录。`)) return
  try {
    await adminApi.updateUser(row.user.id, { status: next })
    ui.toast('success', next === 'banned' ? '已禁用' : '已恢复')
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  }
}

onMounted(async () => {
  const data = await membershipApi.plans()
  plans.value = data.plans
  await load()
})
</script>

<style scoped>
.stats { display: grid; grid-template-columns: repeat(7, 1fr); gap: 12px; }
.stats .card { padding: 14px 16px; display: flex; flex-direction: column; gap: 4px; }
.stats span { font-size: 11px; color: var(--muted-2); }
.stats b { font-size: 21px; font-weight: 700; letter-spacing: -.02em; }
.stats b.ok { color: var(--safe); }
.stats b.warn { color: var(--warn); }
.revenue b { color: var(--flame-1); }
.distribution { display: flex; flex-direction: column; gap: 12px; }
.dist-row { display: flex; gap: 10px; flex-wrap: wrap; }
.dist-item { display: flex; align-items: center; gap: 8px; padding: 7px 12px; border-radius: 10px; background: var(--surface); border: 1px solid var(--line); font-size: 12px; }
.dist-item strong { font-weight: 620; }
.dist-item span { color: var(--muted-2); font-size: 11px; }

.filters { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 14px 18px; border-bottom: 1px solid var(--line); flex-wrap: wrap; }
.seg { display: flex; gap: 3px; padding: 3px; background: var(--surface); border: 1px solid var(--line); border-radius: 11px; }
.seg button { padding: 6px 13px; border: none; background: none; border-radius: 8px; font-size: 12.5px; color: var(--muted); cursor: pointer; }
.seg button.active { background: var(--surface-3); color: var(--text); }
.search { width: 220px; }

.who { display: flex; align-items: center; gap: 9px; }
.who div { display: flex; flex-direction: column; line-height: 1.3; }
.who strong { font-size: 13px; font-weight: 600; }
.who small { font-size: 10.5px; color: var(--muted-2); }
.left { display: block; font-size: 10px; color: var(--muted-2); }
.loading { display: flex; flex-direction: column; gap: 8px; padding: 18px; }
.pager { display: flex; align-items: center; justify-content: center; gap: 14px; font-size: 12px; color: var(--muted); }
.danger { color: #ff8fa3; }
.hint { font-size: 11.5px; color: var(--muted-2); }

@media (max-width: 1100px) { .stats { grid-template-columns: repeat(3, 1fr); } }
@media (max-width: 700px) { .stats { grid-template-columns: repeat(2, 1fr); } .search { width: 100%; } }
</style>

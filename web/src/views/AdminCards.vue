<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2 class="page-title">卡密管理</h2>
        <p class="page-desc">批量生成卡密，发给客户后在会员中心兑换。</p>
      </div>
      <div class="row gap-10">
        <a class="btn" :href="adminApi.exportUrl(exportParams)" target="_blank" rel="noopener">导出 CSV</a>
        <button class="btn btn-primary" @click="genOpen = true">批量生成</button>
      </div>
    </div>

    <div class="stats">
      <div class="card"><span>卡密总数</span><b class="num">{{ stats.cards }}</b></div>
      <div class="card"><span>已使用</span><b class="num ok">{{ stats.cardsUsed }}</b></div>
      <div class="card"><span>未使用</span><b class="num warn">{{ stats.cards - stats.cardsUsed }}</b></div>
      <div class="card"><span>有效会员</span><b class="num">{{ stats.members }}</b></div>
    </div>

    <section class="card">
      <div class="filters">
        <div class="seg">
          <button v-for="tab in tabs" :key="tab.value" :class="{ active: status === tab.value }" @click="setStatus(tab.value)">{{ tab.label }}</button>
        </div>
        <div class="filter-right">
          <select v-model="plan" class="select" @change="reload">
            <option value="all">全部卡种</option>
            <option v-for="item in plans" :key="item.code" :value="item.code">{{ item.name }}</option>
          </select>
          <div class="input-icon search">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.2-3.2" /></svg>
            <input v-model="keyword" class="input" placeholder="搜索卡密 / 批次" @input="debounced" />
          </div>
        </div>
      </div>

      <div v-if="loading" class="loading">
        <div v-for="i in 5" :key="i" class="skeleton" style="height: 44px" />
      </div>

      <div v-else-if="cards.length" class="table-wrap">
        <table class="table">
          <thead><tr><th>卡密</th><th>卡种</th><th>状态</th><th>批次</th><th>有效期至</th><th>使用时间</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="card in cards" :key="card.id">
              <td>
                <button class="code" @click="copy(card.code)" :title="'点击复制'">{{ card.code }}</button>
              </td>
              <td>{{ planName(card.plan) }}</td>
              <td><span class="badge" :class="statusClass(card.status)">{{ statusText(card.status) }}</span></td>
              <td class="mono muted">{{ card.batch }}</td>
              <td class="num">{{ card.expireAt ? formatDay(card.expireAt) : '—' }}</td>
              <td class="num muted">{{ card.usedAt ? formatDateTime(card.usedAt) : '—' }}</td>
              <td>
                <button v-if="card.status === 'unused'" class="btn btn-sm btn-ghost danger" @click="voidCard(card)">作废</button>
                <span v-else class="muted">—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <EmptyState v-else title="还没有卡密" desc="点击「批量生成」创建一批卡密，发给客户即可开通会员。" />
    </section>

    <div v-if="total > pageSize" class="pager">
      <button class="btn btn-sm" :disabled="page === 1" @click="go(page - 1)">上一页</button>
      <span class="num">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
      <button class="btn btn-sm" :disabled="page * pageSize >= total" @click="go(page + 1)">下一页</button>
    </div>

    <Modal v-model="genOpen" title="批量生成卡密" subtitle="生成后可复制或导出，一卡一码">
      <div class="grid cols-2">
        <div class="field">
          <label>卡种</label>
          <select v-model="form.plan" class="select">
            <option v-for="item in plans" :key="item.code" :value="item.code">{{ item.name }} · {{ item.days ? item.days + ' 天' : '永久' }}</option>
          </select>
        </div>
        <div class="field">
          <label>数量（1-200）</label>
          <input v-model.number="form.count" type="number" min="1" max="200" class="input" />
        </div>
      </div>
      <div class="grid cols-2">
        <div class="field">
          <label>卡密有效期（天）</label>
          <input v-model.number="form.validDays" type="number" min="1" class="input" />
        </div>
        <div class="field">
          <label>备注</label>
          <input v-model.trim="form.note" class="input" placeholder="例如：双十一活动" />
        </div>
      </div>
      <template #footer>
        <button class="btn" @click="genOpen = false">取消</button>
        <button class="btn btn-primary" :disabled="generating" @click="generate">{{ generating ? '生成中…' : '生成' }}</button>
      </template>
    </Modal>

    <Modal v-model="resultOpen" title="生成完成" :subtitle="`批次 ${batch}`" width="620px">
      <textarea class="textarea codes" readonly :value="generated.join('\n')" rows="10" />
      <template #footer>
        <button class="btn" @click="copyAll">复制全部</button>
        <a class="btn btn-primary" :href="adminApi.exportUrl({ plan: form.plan })" target="_blank" rel="noopener">导出 CSV</a>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import { adminApi, membershipApi } from '@/api'
import type { AdminStats, CardKey, Plan } from '@/api/types'
import { useUiStore } from '@/stores/ui'
import { formatDay, formatDateTime } from '@/utils/format'

const ui = useUiStore()
const cards = ref<CardKey[]>([])
const plans = ref<Plan[]>([])
const stats = ref<AdminStats>({ users: 0, members: 0, activeUsers: 0, cards: 0, cardsUsed: 0, cardsLeft: 0, accounts: 0, records: 0, revenue: 0, planDistribution: [] })
const total = ref(0)
const page = ref(1)
const pageSize = 20
const status = ref('all')
const plan = ref('all')
const keyword = ref('')
const loading = ref(true)

const genOpen = ref(false)
const generating = ref(false)
const resultOpen = ref(false)
const generated = ref<string[]>([])
const batch = ref('')
const form = reactive({ plan: 'month', count: 10, validDays: 365, note: '' })

const tabs = [
  { value: 'all', label: '全部' },
  { value: 'unused', label: '未使用' },
  { value: 'used', label: '已使用' },
  { value: 'void', label: '已作废' },
]

const exportParams = computed(() => {
  const params: Record<string, string> = {}
  if (status.value !== 'all') params.status = status.value
  if (plan.value !== 'all') params.plan = plan.value
  return params
})

function planName(code: string) {
  return plans.value.find((p) => p.code === code)?.name || code
}
function statusText(value: string) {
  return ({ unused: '未使用', used: '已使用', void: '已作废' } as Record<string, string>)[value] || value
}
function statusClass(value: string) {
  return ({ unused: 'warn', used: 'safe', void: 'broken' } as Record<string, string>)[value] || ''
}

async function load() {
  loading.value = true
  try {
    const [data, stat] = await Promise.all([
      adminApi.cards({ page: page.value, pageSize, status: status.value, plan: plan.value, keyword: keyword.value }),
      adminApi.stats(),
    ])
    cards.value = data.items
    total.value = data.total
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

async function generate() {
  generating.value = true
  try {
    const data = await adminApi.generate({ ...form })
    generated.value = data.cards.map((c) => c.code)
    batch.value = data.batch
    genOpen.value = false
    resultOpen.value = true
    ui.toast('success', `已生成 ${data.cards.length} 张卡密`)
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    generating.value = false
  }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    ui.toast('success', '已复制卡密')
  } catch {
    ui.toast('error', '复制失败，请手动选择')
  }
}

async function copyAll() {
  await copy(generated.value.join('\n'))
}

async function voidCard(card: CardKey) {
  if (!window.confirm(`作废卡密 ${card.code}？`)) return
  try {
    await adminApi.voidCard(card.id)
    ui.toast('success', '已作废')
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  }
}

onMounted(async () => {
  const data = await membershipApi.plans()
  plans.value = data.plans
  form.plan = 'month'
  await load()
})
</script>

<style scoped>
.stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.stats .card { padding: 14px 16px; display: flex; flex-direction: column; gap: 4px; }
.stats span { font-size: 11px; color: var(--muted-2); }
.stats b { font-size: 21px; font-weight: 700; }
.stats b.ok { color: var(--safe); }
.stats b.warn { color: var(--warn); }

.filters { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 14px 18px; border-bottom: 1px solid var(--line); flex-wrap: wrap; }
.seg { display: flex; gap: 3px; padding: 3px; background: var(--surface); border: 1px solid var(--line); border-radius: 11px; }
.seg button { padding: 6px 13px; border: none; background: none; border-radius: 8px; font-size: 12.5px; color: var(--muted); cursor: pointer; }
.seg button.active { background: var(--surface-3); color: var(--text); }
.filter-right { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
.filter-right .select { width: 132px; }
.search { width: 210px; }

.code { font-family: var(--font-mono); font-size: 12px; letter-spacing: .04em; color: var(--text-2); background: none; border: none; cursor: pointer; padding: 2px 0; }
.code:hover { color: var(--flame-1); }
.mono { font-family: var(--font-mono); font-size: 11.5px; }
.loading { display: flex; flex-direction: column; gap: 8px; padding: 18px; }
.pager { display: flex; align-items: center; justify-content: center; gap: 14px; font-size: 12px; color: var(--muted); }
.danger { color: #ff8fa3; }
.codes { font-family: var(--font-mono); font-size: 12px; line-height: 1.8; }

@media (max-width: 900px) {
  .stats { grid-template-columns: repeat(2, 1fr); }
  .filter-right, .search, .filter-right .select { width: 100%; }
}
</style>

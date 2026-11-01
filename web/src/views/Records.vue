<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2 class="page-title">发送记录</h2>
        <p class="page-desc">每一次下发都会落库，失败原因原样保留，不做任何伪造。</p>
      </div>
      <div class="row gap-10">
        <a class="btn" :href="recordApi.exportUrl()" target="_blank" rel="noopener">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12" /><path d="m7 11 5 5 5-5" /><path d="M4 20h16" /></svg>
          导出 CSV
        </a>
        <button class="btn" @click="prune">清理旧记录</button>
      </div>
    </div>

    <div class="stats-grid">
      <div class="mini card"><span>总发送</span><b class="num">{{ stats.total }}</b></div>
      <div class="mini card"><span>成功</span><b class="num ok">{{ stats.success }}</b></div>
      <div class="mini card"><span>失败</span><b class="num bad">{{ stats.failed }}</b></div>
      <div class="mini card"><span>今日</span><b class="num">{{ stats.today }}</b></div>
      <div class="mini card"><span>成功率</span><b class="num">{{ percent(stats.successRate, 1) }}</b></div>
    </div>

    <section class="card">
      <div class="filters">
        <div class="seg">
          <button v-for="opt in statusTabs" :key="opt.value" :class="{ active: status === opt.value }" @click="setStatus(opt.value)">{{ opt.label }}</button>
        </div>
        <div class="filter-right">
          <select v-model="source" class="select" @change="reload">
            <option value="all">全部来源</option>
            <option value="auto">自动任务</option>
            <option value="manual">手动续火</option>
          </select>
          <select v-model="accountId" class="select" @change="reload">
            <option value="0">全部账号</option>
            <option v-for="account in accounts" :key="account.id" :value="String(account.id)">{{ account.nickname }}</option>
          </select>
          <div class="input-icon search">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.2-3.2" /></svg>
            <input v-model="keyword" class="input" placeholder="搜索好友或内容" @input="debounced" />
          </div>
        </div>
      </div>

      <div v-if="loading" class="loading">
        <div v-for="i in 6" :key="i" class="skeleton" style="height: 46px" />
      </div>

      <div v-else-if="records.length" class="table-wrap">
        <table class="table">
          <thead>
            <tr><th>时间</th><th>好友</th><th>内容</th><th>状态</th><th>来源</th><th>耗时</th></tr>
          </thead>
          <tbody>
            <tr v-for="item in records" :key="item.id">
              <td class="num time">{{ formatDateTime(item.createdAt) }}</td>
              <td>
                <div class="friend">
                  <Avatar :name="item.friendName" :size="26" />
                  <span class="ellipsis">{{ item.friendName }}</span>
                </div>
              </td>
              <td class="content">
                <span class="ellipsis">{{ item.content }}</span>
                <small v-if="item.error" class="error">{{ item.error }}</small>
              </td>
              <td><span class="badge" :class="item.status === 'success' ? 'safe' : 'urgent'">{{ item.status === 'success' ? '成功' : '失败' }}</span></td>
              <td><span class="src">{{ item.source === 'auto' ? '自动' : '手动' }}</span></td>
              <td class="num muted">{{ item.latencyMs }} ms</td>
            </tr>
          </tbody>
        </table>
      </div>

      <EmptyState v-else title="暂无发送记录" desc="执行一次任务或手动续火后，记录会出现在这里。" />
    </section>

    <div v-if="total > pageSize" class="pager">
      <button class="btn btn-sm" :disabled="page === 1" @click="go(page - 1)">上一页</button>
      <span class="num">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
      <button class="btn btn-sm" :disabled="page * pageSize >= total" @click="go(page + 1)">下一页</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Avatar from '@/components/Avatar.vue'
import EmptyState from '@/components/EmptyState.vue'
import { accountApi, recordApi } from '@/api'
import type { Account, RecordItem } from '@/api/types'
import { useUiStore } from '@/stores/ui'
import { formatDateTime, percent } from '@/utils/format'

const ui = useUiStore()
const records = ref<RecordItem[]>([])
const accounts = ref<Account[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 15
const status = ref('all')
const source = ref('all')
const accountId = ref('0')
const keyword = ref('')
const loading = ref(true)
const stats = ref({ total: 0, success: 0, failed: 0, today: 0, successRate: 0 })

const statusTabs = [
  { value: 'all', label: '全部' },
  { value: 'success', label: '成功' },
  { value: 'failed', label: '失败' },
]

async function load() {
  loading.value = true
  try {
    const [data, stat] = await Promise.all([
      recordApi.list({ page: page.value, pageSize, status: status.value, source: source.value, accountId: accountId.value, keyword: keyword.value }),
      recordApi.stats(),
    ])
    records.value = data.items
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

async function prune() {
  if (!window.confirm('清理 30 天之前的发送记录？')) return
  const result = await recordApi.prune(30)
  ui.toast('success', `已清理 ${result.deleted} 条记录`)
  await load()
}

onMounted(async () => {
  accounts.value = await accountApi.list()
  await load()
})
</script>

<style scoped>
.stats-grid { display: grid; grid-template-columns: repeat(5, 1fr); gap: 12px; }
.mini { padding: 14px 16px; display: flex; flex-direction: column; gap: 4px; }
.mini span { font-size: 11px; color: var(--muted-2); }
.mini b { font-size: 21px; font-weight: 700; letter-spacing: -.02em; }
.mini b.ok { color: var(--safe); }
.mini b.bad { color: var(--urgent); }

.filters { display: flex; align-items: center; justify-content: space-between; gap: 14px; padding: 14px 18px; border-bottom: 1px solid var(--line); flex-wrap: wrap; }
.seg { display: flex; gap: 3px; padding: 3px; background: var(--surface); border: 1px solid var(--line); border-radius: 11px; }
.seg button { padding: 6px 13px; border: none; background: none; border-radius: 8px; font-size: 12.5px; color: var(--muted); cursor: pointer; }
.seg button.active { background: var(--surface-3); color: var(--text); }
.filter-right { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
.filter-right .select { width: 128px; }
.search { width: 200px; }

.loading { display: flex; flex-direction: column; gap: 8px; padding: 18px; }
.time { font-size: 12px; color: var(--muted); white-space: nowrap; }
.friend { display: flex; align-items: center; gap: 8px; max-width: 160px; }
.content { max-width: 380px; }
.content small.error { display: block; margin-top: 3px; font-size: 10.5px; color: #ff8fa3; }
.src { font-size: 11.5px; color: var(--muted); }
.pager { display: flex; align-items: center; justify-content: center; gap: 14px; font-size: 12px; color: var(--muted); }

@media (max-width: 900px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .filter-right, .search, .filter-right .select { width: 100%; }
}
</style>

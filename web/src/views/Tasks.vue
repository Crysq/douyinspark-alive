<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2 class="page-title">定时任务</h2>
        <p class="page-desc">只在时间窗口内补火，间隔与单轮上限让发送节奏更像真人。</p>
      </div>
      <button class="btn btn-primary" @click="openCreate">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14" /></svg>
        新建任务
      </button>
    </div>

    <div class="overview card card-pad">
      <div class="ov-item"><b class="num">{{ tasks.filter(t => t.enabled).length }}</b><span>运行中任务</span></div>
      <div class="divider-v" />
      <div class="ov-item"><b class="num">{{ tasks.filter(t => t.inWindow).length }}</b><span>当前在窗口内</span></div>
      <div class="divider-v" />
      <div class="ov-item"><b class="num">{{ totalRuns }}</b><span>累计执行轮次</span></div>
      <div class="ov-note">
        <span class="badge flame">调度心跳 20s</span>
        <span class="muted">多实例部署时通过 Redis 分布式锁保证只有一个实例发送。</span>
      </div>
    </div>

    <div v-if="loading" class="grid cols-2">
      <div v-for="i in 2" :key="i" class="card skeleton" style="height: 230px" />
    </div>

    <div v-else-if="tasks.length" class="grid cols-2">
      <article v-for="task in tasks" :key="task.id" class="task card card-hover">
        <header class="t-head">
          <div class="t-title">
            <strong>{{ task.name }}</strong>
            <div class="row gap-6 wrap">
              <span class="badge blue">{{ task.accountName }}</span>
              <span class="badge">{{ task.templateName || '随机模板' }}</span>
            </div>
          </div>
          <Toggle :model-value="task.enabled" @update:model-value="(v) => toggle(task, v)" />
        </header>

        <TimelineBar :start="task.windowStart" :end="task.windowEnd" :now="now" />

        <div class="t-meta">
          <div><span>时间窗口</span><b class="num">{{ task.windowStart }} – {{ task.windowEnd }}</b></div>
          <div><span>轮询间隔</span><b class="num">{{ task.intervalMin }} 分钟</b></div>
          <div><span>单轮上限</span><b class="num">{{ task.maxPerRound }} 位</b></div>
          <div><span>下次执行</span><b class="num">{{ task.nextRunAt ? formatClock(task.nextRunAt) : '—' }}</b></div>
        </div>

        <footer class="t-foot">
          <span class="status" :class="{ on: task.enabled && task.inWindow }">
            <i />{{ task.enabled ? (task.inWindow ? '窗口内运行' : '等待窗口开启') : '已暂停' }}
          </span>
          <div class="row gap-6">
            <button class="btn btn-sm btn-ghost" @click="openEdit(task)">编辑</button>
            <button class="btn btn-sm btn-ghost danger-text" @click="remove(task)">删除</button>
            <button class="btn btn-sm btn-primary" :disabled="running === task.id" @click="run(task)">
              {{ running === task.id ? '执行中…' : '立即执行' }}
            </button>
          </div>
        </footer>
      </article>
    </div>

    <EmptyState v-else title="还没有定时任务" desc="创建任务后，系统会在时间窗口内自动为今天还没互动的好友补发消息。">
      <template #action><button class="btn btn-primary btn-sm" @click="openCreate">新建任务</button></template>
    </EmptyState>

    <Modal v-model="modalOpen" :title="editing ? '编辑任务' : '新建任务'" subtitle="任务只对「今天还没互动」的好友生效">
      <div class="grid cols-2">
        <div class="field">
          <label>所属账号</label>
          <select v-model.number="form.accountId" class="select">
            <option v-for="account in accounts" :key="account.id" :value="account.id">{{ account.nickname }}</option>
          </select>
        </div>
        <div class="field">
          <label>任务名称</label>
          <input v-model.trim="form.name" class="input" placeholder="例如：每日续火" />
        </div>
      </div>
      <div class="field">
        <label>消息模板</label>
        <select v-model.number="form.templateId" class="select">
          <option :value="0">随机使用启用中的模板</option>
          <option v-for="tpl in templates" :key="tpl.id" :value="tpl.id">{{ tpl.name }} · {{ tpl.content.slice(0, 18) }}</option>
        </select>
      </div>
      <div class="grid cols-2">
        <div class="field">
          <label>窗口开始</label>
          <input v-model="form.windowStart" type="time" class="input" />
        </div>
        <div class="field">
          <label>窗口结束</label>
          <input v-model="form.windowEnd" type="time" class="input" />
        </div>
      </div>
      <div class="grid cols-2">
        <div class="field">
          <label>轮询间隔（分钟）</label>
          <input v-model.number="form.intervalMin" type="number" min="15" class="input" />
        </div>
        <div class="field">
          <label>单轮最多发送</label>
          <input v-model.number="form.maxPerRound" type="number" min="1" max="20" class="input" />
        </div>
      </div>
      <template #footer>
        <button class="btn" @click="modalOpen = false">取消</button>
        <button class="btn btn-primary" :disabled="saving" @click="submit">{{ saving ? '保存中…' : '保存' }}</button>
      </template>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import Toggle from '@/components/Toggle.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import TimelineBar from '@/components/charts/TimelineBar.vue'
import { accountApi, taskApi, templateApi } from '@/api'
import type { Account, Task, Template } from '@/api/types'
import { useUiStore } from '@/stores/ui'
import { formatClock } from '@/utils/format'

const ui = useUiStore()
const tasks = ref<Task[]>([])
const accounts = ref<Account[]>([])
const templates = ref<Template[]>([])
const loading = ref(true)
const running = ref(0)
const modalOpen = ref(false)
const editing = ref<Task | null>(null)
const saving = ref(false)
const now = ref(new Date())
let clock: number | undefined

const totalRuns = computed(() => tasks.value.reduce((sum, t) => sum + t.runCount, 0))

const form = reactive({
  accountId: 0, name: '', templateId: 0,
  windowStart: '08:00', windowEnd: '23:00', intervalMin: 120, maxPerRound: 5,
})

async function load() {
  loading.value = true
  try {
    const [taskList, accountList, tplList] = await Promise.all([taskApi.list(), accountApi.list(), templateApi.list()])
    tasks.value = taskList
    accounts.value = accountList
    templates.value = tplList
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  Object.assign(form, { accountId: accounts.value[0]?.id || 0, name: '', templateId: 0, windowStart: '08:00', windowEnd: '23:00', intervalMin: 120, maxPerRound: 5 })
  modalOpen.value = true
}

function openEdit(task: Task) {
  editing.value = task
  Object.assign(form, {
    accountId: task.accountId, name: task.name, templateId: task.templateId,
    windowStart: task.windowStart, windowEnd: task.windowEnd,
    intervalMin: task.intervalMin, maxPerRound: task.maxPerRound,
  })
  modalOpen.value = true
}

async function submit() {
  if (!form.accountId) { ui.toast('error', '请选择账号'); return }
  saving.value = true
  try {
    if (editing.value) {
      await taskApi.update(editing.value.id, { ...form })
      ui.toast('success', '任务已更新')
    } else {
      await taskApi.create({ ...form })
      ui.toast('success', '任务已创建')
    }
    modalOpen.value = false
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    saving.value = false
  }
}

async function toggle(task: Task, value: boolean) {
  task.enabled = value
  try {
    await taskApi.update(task.id, { enabled: value })
    ui.toast('success', value ? '任务已启用' : '任务已暂停')
  } catch (error) {
    task.enabled = !value
    ui.toast('error', (error as Error).message)
  }
}

async function run(task: Task) {
  running.value = task.id
  try {
    const result = await taskApi.run(task.id)
    ui.toast('success', `执行完成，发送 ${result.sent} 条`)
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    running.value = 0
  }
}

async function remove(task: Task) {
  if (!window.confirm(`删除任务「${task.name}」？`)) return
  await taskApi.remove(task.id)
  ui.toast('success', '已删除')
  await load()
}

onMounted(async () => {
  await load()
  clock = window.setInterval(() => { now.value = new Date() }, 30000)
})
onBeforeUnmount(() => window.clearInterval(clock))
</script>

<style scoped>
.overview { display: flex; align-items: center; gap: 26px; flex-wrap: wrap; padding: 16px 22px; }
.ov-item { display: flex; flex-direction: column; gap: 2px; }
.ov-item b { font-size: 22px; font-weight: 720; letter-spacing: -.02em; }
.ov-item span { font-size: 11px; color: var(--muted-2); }
.divider-v { width: 1px; height: 32px; background: var(--line); }
.ov-note { margin-left: auto; display: flex; align-items: center; gap: 10px; font-size: 11.5px; }

.task { padding: 20px 22px; display: flex; flex-direction: column; gap: 16px; }
.t-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.t-title { display: flex; flex-direction: column; gap: 8px; }
.t-title strong { font-size: 15.5px; font-weight: 650; }
.t-meta { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.t-meta div { display: flex; flex-direction: column; gap: 3px; }
.t-meta span { font-size: 10.5px; color: var(--muted-2); }
.t-meta b { font-size: 12.5px; font-weight: 600; color: var(--text-2); }
.t-foot { display: flex; align-items: center; justify-content: space-between; padding-top: 14px; border-top: 1px solid var(--line); gap: 10px; flex-wrap: wrap; }
.status { display: inline-flex; align-items: center; gap: 7px; font-size: 11.5px; color: var(--muted); }
.status i { width: 6px; height: 6px; border-radius: 50%; background: var(--muted-2); }
.status.on i { background: var(--safe); box-shadow: 0 0 8px var(--safe); animation: pulseGlow 2s infinite; }
.danger-text { color: #ff8fa3; }

@media (max-width: 640px) {
  .t-meta { grid-template-columns: repeat(2, 1fr); }
  .overview { gap: 16px; }
  .ov-note { margin-left: 0; width: 100%; }
}
</style>

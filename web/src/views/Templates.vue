<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2 class="page-title">消息模板</h2>
        <p class="page-desc">支持 <code v-for="token in tokens" :key="token" class="inline-token">{{ token }}</code> 占位符。</p>
      </div>
      <button class="btn btn-primary" @click="openCreate">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14" /></svg>
        新建模板
      </button>
    </div>

    <div class="tabs">
      <button v-for="tab in tabs" :key="tab.value" :class="{ active: category === tab.value }" @click="switchCategory(tab.value)">
        {{ tab.label }}<span class="count">{{ countOf(tab.value) }}</span>
      </button>
    </div>

    <div v-if="loading" class="grid cols-3">
      <div v-for="i in 3" :key="i" class="card skeleton" style="height: 196px" />
    </div>

    <div v-else-if="templates.length" class="grid cols-3">
      <article v-for="tpl in templates" :key="tpl.id" class="tpl card card-hover" :class="{ disabled: !tpl.enabled }">
        <header>
          <div class="t-head">
            <strong>{{ tpl.name }}</strong>
            <span class="badge" :class="tpl.category === 'festival' ? 'flame' : tpl.category === 'custom' ? 'blue' : 'safe'">{{ categoryText(tpl.category) }}</span>
          </div>
          <Toggle :model-value="tpl.enabled" @update:model-value="(v) => toggle(tpl, v)" />
        </header>
        <p class="content">{{ tpl.content }}</p>
        <footer>
          <span class="uses"><b class="num">{{ tpl.useCount }}</b> 次使用</span>
          <div class="row gap-6">
            <button class="btn btn-sm btn-ghost" @click="openEdit(tpl)">编辑</button>
            <button class="btn btn-sm btn-ghost danger-text" @click="remove(tpl)">删除</button>
          </div>
        </footer>
      </article>
    </div>

    <EmptyState v-else title="还没有模板" desc="创建几条日常问候，让自动续火不显得机械。">
      <template #action><button class="btn btn-primary btn-sm" @click="openCreate">新建模板</button></template>
    </EmptyState>

    <Modal v-model="modalOpen" :title="editing ? '编辑模板' : '新建模板'" width="600px">
      <div class="grid cols-2">
        <div class="field">
          <label>模板名称</label>
          <input v-model.trim="form.name" class="input" placeholder="例如：早安打卡" />
        </div>
        <div class="field">
          <label>分类</label>
          <select v-model="form.category" class="select">
            <option value="daily">日常</option>
            <option value="festival">节日</option>
            <option value="custom">自定义</option>
          </select>
        </div>
      </div>
      <div class="field">
        <label>消息内容</label>
        <textarea v-model="form.content" class="textarea" rows="5" placeholder="早呀～今天也要开开心心 {{date}}"></textarea>
        <div class="chips">
          <button v-for="token in tokens" :key="token" type="button" class="chip" @click="insert(token)">{{ token }}</button>
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
import { onMounted, reactive, ref } from 'vue'
import Toggle from '@/components/Toggle.vue'
import Modal from '@/components/Modal.vue'
import EmptyState from '@/components/EmptyState.vue'
import { templateApi } from '@/api'
import type { Template } from '@/api/types'
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()
const templates = ref<Template[]>([])
const all = ref<Template[]>([])
const loading = ref(true)
const category = ref('all')
const modalOpen = ref(false)
const editing = ref<Template | null>(null)
const saving = ref(false)

const tabs = [
  { value: 'all', label: '全部' },
  { value: 'daily', label: '日常' },
  { value: 'festival', label: '节日' },
  { value: 'custom', label: '自定义' },
]
const tokens = ['{{name}}', '{{days}}', '{{date}}', '{{time}}', '{{weekday}}']

const form = reactive({ name: '', category: 'daily', content: '' })

function categoryText(value: string) {
  return ({ daily: '日常', festival: '节日', custom: '自定义' } as Record<string, string>)[value] || value
}
function countOf(value: string) {
  return value === 'all' ? all.value.length : all.value.filter((t) => t.category === value).length
}

async function load() {
  loading.value = true
  try {
    all.value = await templateApi.list('all')
    templates.value = category.value === 'all' ? all.value : all.value.filter((t) => t.category === category.value)
  } finally {
    loading.value = false
  }
}

function switchCategory(value: string) {
  category.value = value
  templates.value = value === 'all' ? all.value : all.value.filter((t) => t.category === value)
}

function openCreate() {
  editing.value = null
  Object.assign(form, { name: '', category: 'daily', content: '' })
  modalOpen.value = true
}

function openEdit(tpl: Template) {
  editing.value = tpl
  Object.assign(form, { name: tpl.name, category: tpl.category, content: tpl.content })
  modalOpen.value = true
}

function insert(token: string) {
  form.content = `${form.content}${form.content && !form.content.endsWith(' ') ? ' ' : ''}${token}`
}

async function submit() {
  if (!form.name || !form.content) { ui.toast('error', '请填写名称与内容'); return }
  saving.value = true
  try {
    if (editing.value) {
      await templateApi.update(editing.value.id, { ...form })
      ui.toast('success', '模板已更新')
    } else {
      await templateApi.create({ ...form })
      ui.toast('success', '模板已创建')
    }
    modalOpen.value = false
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    saving.value = false
  }
}

async function toggle(tpl: Template, value: boolean) {
  tpl.enabled = value
  try {
    await templateApi.update(tpl.id, { enabled: value })
  } catch (error) {
    tpl.enabled = !value
    ui.toast('error', (error as Error).message)
  }
}

async function remove(tpl: Template) {
  if (!window.confirm(`删除模板「${tpl.name}」？`)) return
  await templateApi.remove(tpl.id)
  ui.toast('success', '已删除')
  await load()
}

onMounted(load)
</script>

<style scoped>
.tabs { display: flex; gap: 4px; padding: 3px; background: var(--surface); border: 1px solid var(--line); border-radius: 12px; width: fit-content; }
.tabs button { display: flex; align-items: center; gap: 7px; padding: 7px 14px; border: none; background: none; border-radius: 9px; font-size: 12.5px; color: var(--muted); cursor: pointer; }
.tabs button.active { background: var(--surface-3); color: var(--text); box-shadow: inset 0 0 0 1px var(--line-strong); }
.count { font-size: 10px; color: var(--muted-2); }

.tpl { padding: 18px 20px; display: flex; flex-direction: column; gap: 14px; min-height: 186px; }
.tpl.disabled { opacity: .62; }
.tpl header { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }
.t-head { display: flex; flex-direction: column; gap: 7px; }
.t-head strong { font-size: 14.5px; font-weight: 650; }
.content { flex: 1; font-size: 13px; line-height: 1.7; color: var(--text-2); white-space: pre-wrap; word-break: break-word; }
.tpl footer { display: flex; align-items: center; justify-content: space-between; padding-top: 12px; border-top: 1px solid var(--line); }
.uses { font-size: 11px; color: var(--muted-2); }
.uses b { color: var(--flame-2); font-size: 13px; }
.danger-text { color: #ff8fa3; }
.inline-token { font-family: var(--font-mono); font-size: 11px; padding: 1px 5px; border-radius: 5px; background: var(--surface-2); color: var(--flame-2); margin: 0 1px; }

.chips { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 4px; }
.chip { padding: 4px 8px; border-radius: 7px; border: 1px solid var(--line); background: var(--surface); color: var(--muted); font-size: 11px; font-family: var(--font-mono); cursor: pointer; }
.chip:hover { color: var(--flame-2); border-color: rgba(255, 122, 24, .35); }
</style>

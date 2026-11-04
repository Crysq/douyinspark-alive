<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h2 class="page-title">会员中心</h2>
        <p class="page-desc">兑换卡密即可续期，天数在到期时间基础上累加。</p>
      </div>
      <span class="badge" :class="statusClass">{{ statusText }}</span>
    </div>

    <div class="top-grid">
      <section class="card card-pad hero">
        <div class="hero-main">
          <div>
            <p class="label">当前卡种</p>
            <h3 class="plan">{{ info?.user.planLabel || '未开通' }}</h3>
            <p class="expire">
              到期时间：<b>{{ isForever ? '永久有效' : (info?.user.memberExpireAt ? formatDay(info.user.memberExpireAt) : '—') }}</b>
            </p>
          </div>
          <div class="days">
            <b class="num">{{ isForever ? '∞' : info?.user.daysLeft ?? 0 }}</b>
            <span>{{ isForever ? '永久' : '天剩余' }}</span>
          </div>
        </div>
        <div class="progress"><span :style="{ width: progress + '%' }" /></div>
        <p class="tip" v-if="!isForever">剩余不足 7 天时会提醒，建议提前续期，避免火花中断。</p>
        <p class="tip" v-else>永久卡无需续期，所有功能长期可用。</p>
      </section>

      <section class="card card-pad redeem">
        <h3 class="card-title">兑换卡密</h3>
        <p class="card-sub">输入 DYSP 开头的 16 位卡密，立即生效</p>
        <div class="redeem-row">
          <input v-model.trim="code" class="input card-input" placeholder="DYSP-XXXX-XXXX-XXXX" @keyup.enter="redeem" />
          <button class="btn btn-primary" :disabled="redeeming" @click="redeem">{{ redeeming ? '兑换中…' : '兑换' }}</button>
        </div>
        <p class="hint">卡密一次性使用；叠加在当前到期时间之后。</p>
      </section>
    </div>

    <section class="card card-pad">
      <h3 class="card-title">套餐配额</h3>
      <p class="card-sub">当前用量与上限（永久卡不限量）</p>
      <div class="quota-grid">
        <div v-for="row in quotaRows" :key="row.key" class="quota-item">
          <div class="quota-top"><span>{{ row.label }}</span><b>{{ row.used }}<small v-if="row.limit"> / {{ row.limit }}</small></b></div>
          <div class="quota-bar"><span :style="{ width: row.pct + '%' }" :class="{ over: row.pct >= 100 }" /></div>
          <small class="quota-note">{{ row.limit ? (row.pct >= 100 ? '已达上限，请升级套餐' : `剩余 ${Math.max(0, row.limit - row.used)}`) : '不限量' }}</small>
        </div>
      </div>
    </section>

    <section class="card card-pad">
      <div class="card-head">
        <div>
          <h3 class="card-title">卡种与价格</h3>
          <p class="card-sub">向管理员或经销商购买卡密后在此兑换</p>
        </div>
      </div>
      <div class="plan-grid">
        <article v-for="plan in plans" :key="plan.code" class="plan-card" :class="{ current: info?.user.plan === plan.code }">
          <div class="p-head">
            <strong>{{ plan.name }}</strong>
            <span v-if="info?.user.plan === plan.code" class="badge flame">当前</span>
          </div>
          <div class="p-price">¥{{ plan.price }}</div>
          <p class="p-days">{{ plan.days ? plan.days + ' 天' : '永久有效' }}</p>
          <p class="p-desc">{{ plan.desc }}</p>
        </article>
      </div>
    </section>

    <section class="card card-pad">
      <h3 class="card-title">兑换记录</h3>
      <p class="card-sub">最近 20 条会员流水</p>
      <div v-if="history.length" class="table-wrap mt-16">
        <table class="table">
          <thead><tr><th>时间</th><th>类型</th><th>卡种</th><th>卡密</th><th>到期时间</th></tr></thead>
          <tbody>
            <tr v-for="item in history" :key="item.id">
              <td class="num">{{ formatDateTime(item.createdAt) }}</td>
              <td><span class="badge" :class="actionClass(item.action)">{{ actionText(item.action) }}</span></td>
              <td>{{ planLabel(item.plan) }}</td>
              <td class="mono code">{{ item.cardCode || '—' }}</td>
              <td>{{ item.afterExpire ? formatDay(item.afterExpire) : '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState v-else title="还没有兑换记录" desc="输入卡密兑换后，这里会记录每一次开通与续期。" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import EmptyState from '@/components/EmptyState.vue'
import { membershipApi } from '@/api'
import type { MembershipInfo, PlanCode } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import { formatDateTime, formatDay } from '@/utils/format'

const auth = useAuthStore()
const ui = useUiStore()
const info = ref<MembershipInfo | null>(null)
const code = ref('')
const redeeming = ref(false)

const plans = computed(() => info.value?.plans || [])
const quotaRows = computed(() => {
  const q = (info.value as any)?.quota || {}
  const u = (info.value as any)?.usage || {}
  const rows = [
    { key: 'accounts', label: '托管账号', used: u.accounts || 0, limit: q.maxAccounts || 0 },
    { key: 'friends', label: '火花好友', used: u.friends || 0, limit: q.maxFriends || 0 },
    { key: 'tasks', label: '定时任务', used: u.tasks || 0, limit: q.maxTasks || 0 },
    { key: 'daily', label: '今日发送', used: u.todaySent || 0, limit: q.maxPerDay || 0 },
  ]
  return rows.map((r) => ({ ...r, pct: r.limit ? Math.min(100, Math.round((r.used / r.limit) * 100)) : 0 }))
})
const history = computed(() => info.value?.history || [])
const isForever = computed(() => (info.value?.user.daysLeft ?? 0) < 0)
const statusText = computed(() => {
  if (!info.value) return '加载中'
  return info.value.user.isMember ? (isForever.value ? '永久会员' : '会员有效') : '已过期'
})
const statusClass = computed(() => (info.value?.user.isMember ? (isForever.value ? 'flame' : 'safe') : 'urgent'))
const progress = computed(() => {
  if (!info.value || isForever.value) return 100
  const total = 30
  return Math.max(4, Math.min(100, Math.round(((info.value.user.daysLeft || 0) / total) * 100)))
})

function planLabel(plan: PlanCode | string) {
  return plans.value.find((p) => p.code === plan)?.name || String(plan)
}
function actionText(action: string) {
  return ({ register: '注册开通', redeem: '卡密兑换', grant: '管理员赠送' } as Record<string, string>)[action] || action
}
function actionClass(action: string) {
  return ({ register: 'blue', redeem: 'safe', grant: 'flame' } as Record<string, string>)[action] || ''
}

async function load() {
  info.value = await membershipApi.mine()
}

async function redeem() {
  if (!code.value) {
    ui.toast('error', '请输入卡密')
    return
  }
  redeeming.value = true
  try {
    const data = await auth.redeem(code.value)
    ui.toast('success', `${data.card.name}兑换成功`)
    code.value = ''
    await load()
  } catch (error) {
    ui.toast('error', (error as Error).message)
  } finally {
    redeeming.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.top-grid { display: grid; grid-template-columns: 1.4fr 1fr; gap: 18px; }
.hero { display: flex; flex-direction: column; gap: 16px; background: linear-gradient(135deg, rgba(255, 122, 24, .1), rgba(255, 61, 113, .05)), var(--surface); }
.hero-main { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; }
.label { font-size: 11px; color: var(--muted-2); letter-spacing: .08em; }
.plan { margin-top: 6px; font-size: 30px; font-weight: 740; letter-spacing: -.03em; }
.expire { margin-top: 8px; font-size: 12.5px; color: var(--muted); }
.expire b { color: var(--text-2); }
.days { text-align: right; display: flex; flex-direction: column; }
.days b { font-size: 42px; font-weight: 760; letter-spacing: -.04em; color: var(--flame-1); line-height: 1; }
.days span { font-size: 10.5px; color: var(--muted-2); margin-top: 4px; }
.progress { height: 8px; border-radius: 999px; background: var(--surface-3); overflow: hidden; }
.progress span { display: block; height: 100%; border-radius: 999px; background: var(--flame-grad); transition: width .5s var(--ease); }
.tip { font-size: 11.5px; color: var(--muted-2); }

.redeem { display: flex; flex-direction: column; gap: 12px; }
.redeem-row { display: flex; gap: 8px; }
.card-input { font-family: var(--font-mono); letter-spacing: .06em; text-transform: uppercase; }
.hint { font-size: 11px; color: var(--muted-2); }

.plan-grid { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 12px; margin-top: 16px; }
.plan-card { padding: 16px; border-radius: 16px; border: 1px solid var(--line); background: var(--surface); display: flex; flex-direction: column; gap: 6px; transition: border-color var(--dur), transform var(--dur); }
.plan-card:hover { transform: translateY(-2px); border-color: var(--line-strong); }
.plan-card.current { border-color: rgba(255, 122, 24, .45); box-shadow: 0 0 0 1px rgba(255, 122, 24, .18); }
.p-head { display: flex; align-items: center; justify-content: space-between; gap: 6px; }
.p-head strong { font-size: 14px; font-weight: 650; }
.p-price { font-size: 22px; font-weight: 720; letter-spacing: -.03em; color: var(--flame-1); }
.p-days { font-size: 11.5px; color: var(--text-2); }
.p-desc { font-size: 10.5px; color: var(--muted-2); line-height: 1.5; }
.code { font-size: 11.5px; color: var(--muted); }

.quota-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin-top: 16px; }
.quota-item { display: flex; flex-direction: column; gap: 6px; padding: 12px 14px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface); }
.quota-top { display: flex; align-items: center; justify-content: space-between; font-size: 11.5px; color: var(--muted); }
.quota-top b { color: var(--text); font-size: 15px; font-variant-numeric: tabular-nums; }
.quota-top small { color: var(--muted-2); font-size: 11px; }
.quota-bar { height: 6px; border-radius: 999px; background: var(--surface-3); overflow: hidden; }
.quota-bar span { display: block; height: 100%; border-radius: 999px; background: var(--flame-grad); transition: width .5s var(--ease); }
.quota-bar span.over { background: linear-gradient(90deg, #ff7a5c, #ff3d71); }
.quota-note { font-size: 10px; color: var(--muted-2); }

@media (max-width: 900px) { .quota-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }

@media (max-width: 1180px) { .plan-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 900px) {
  .top-grid { grid-template-columns: minmax(0, 1fr); }
  .plan-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>

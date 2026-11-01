<template>
  <div class="page dashboard">
    <div class="hello">
      <div>
        <h2 class="hello-title">{{ greeting() }}，{{ auth.user?.nickname || '火花管理员' }}</h2>
        <p class="hello-sub">
          今天已续火 <b class="grad-text">{{ data?.todaySuccess ?? 0 }}</b> 次，
          <template v-if="urgentCount">还有 <b class="urgent-text">{{ urgentCount }}</b> 位好友即将断火，建议优先处理。</template>
          <template v-else>所有火花都在安全区间内。</template>
        </p>
      </div>
      <div class="hello-actions">
        <router-link to="/friends" class="btn btn-primary">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2 4.5 13.5H11l-1 8.5 9-12h-6.5L13 2Z" /></svg>
          去续火
        </router-link>
        <router-link to="/tasks" class="btn">任务编排</router-link>
      </div>
    </div>

    <div v-if="loading" class="grid cols-4">
      <div v-for="i in 4" :key="i" class="card skeleton" style="height: 132px" />
    </div>
    <div v-else class="grid cols-4">
      <StatCard label="火花好友" :value="data?.friends ?? 0" unit="位" :hint="`${data?.autoKeep ?? 0} 位开启自动维护`" :spark="sparkFriends" accent="#ff7a18">
        <template #icon><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M12 3c2 3.4-2.6 4.8-2.6 8a2.6 2.6 0 0 0 5.2 0c0-1.7-.8-2.6-.8-3.9 2.4 1.8 3.6 4.2 3.6 6.9a5.4 5.4 0 1 1-10.8 0C6.6 10 9.7 6.6 12 3Z" /></svg></template>
      </StatCard>
      <StatCard label="今日续火" :value="data?.todaySent ?? 0" unit="次" :hint="`成功 ${data?.todaySuccess ?? 0} 次`" :badge="data?.channel === 'ready' ? '通道正常' : '通道离线'" :badge-class="data?.channel === 'ready' ? 'safe' : 'broken'" :spark="sparkSent" accent="#ff3d71">
        <template #icon><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M22 2 11 13" /><path d="M22 2 15 22l-4-9-9-4 20-7Z" /></svg></template>
      </StatCard>
      <StatCard label="发送成功率" :value="percent(data?.successRate ?? 0, 1)" hint="近 14 天累计" :badge="rateBadge" :badge-class="rateBadgeClass" :spark="sparkRate" accent="#2dd4a7">
        <template #icon><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="m3 17 6-6 4 4 8-8" /><path d="M15 7h6v6" /></svg></template>
      </StatCard>
      <StatCard label="在线账号" :value="data?.onlineAccounts ?? 0" :unit="`/ ${data?.accounts ?? 0}`" :hint="`累计连续 ${data?.totalStreak ?? 0} 天`" :badge="`${data?.activeTasks ?? 0} 个任务运行`" badge-class="flame" :spark="sparkAccounts" accent="#8b7cff">
        <template #icon><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="12" cy="8" r="3.6" /><path d="M4.5 20a7.5 7.5 0 0 1 15 0" /></svg></template>
      </StatCard>
    </div>

    <div class="main-grid">
      <div class="col-main">
        <section class="card card-pad">
          <div class="card-head">
            <div>
              <h3 class="card-title">续火趋势</h3>
              <p class="card-sub">每日成功与失败的下发次数</p>
            </div>
            <div class="seg">
              <button v-for="opt in [14, 30]" :key="opt" :class="{ active: days === opt }" @click="switchDays(opt)">{{ opt }} 天</button>
            </div>
          </div>
          <div v-if="loading" class="skeleton" style="height: 232px; margin-top: 18px" />
          <TrendChart v-else :data="trend" :height="228" class="mt-16" />
        </section>

        <section class="card card-pad">
          <div class="card-head">
            <div>
              <h3 class="card-title">活跃热力</h3>
              <p class="card-sub">近 14 天按星期 × 小时的发送分布</p>
            </div>
            <span class="badge flame">调度时段</span>
          </div>
          <div v-if="loading" class="skeleton" style="height: 150px; margin-top: 16px" />
          <HeatMap v-else :data="heatmap" class="mt-16" />
        </section>
      </div>

      <div class="col-side">
        <section class="card card-pad">
          <h3 class="card-title">火花健康度</h3>
          <p class="card-sub">按剩余时间分级</p>
          <div class="donut-wrap">
            <DonutChart :slices="donutSlices" :size="176" label="位好友" />
            <ul class="legend">
              <li v-for="item in data?.levels || []" :key="item.level">
                <span class="dot" :style="{ background: levelColor(item.level) }" />
                <span class="name">{{ item.text }}</span>
                <b class="num">{{ item.count }}</b>
              </li>
            </ul>
          </div>
        </section>

        <section class="card card-pad">
          <div class="card-head">
            <div>
              <h3 class="card-title">连续火花榜</h3>
              <p class="card-sub">最长的陪伴</p>
            </div>
            <router-link to="/friends" class="link">全部</router-link>
          </div>
          <ul v-if="data?.top?.length" class="rank">
            <li v-for="(item, index) in data.top" :key="item.id">
              <span class="index" :class="{ top: index < 3 }">{{ index + 1 }}</span>
              <Avatar :name="item.name" :src="item.avatarUrl" :size="32" />
              <span class="name ellipsis">{{ item.name }}</span>
              <span class="days num">{{ item.streakDays }}<small>天</small></span>
            </li>
          </ul>
          <EmptyState v-else title="还没有火花好友" desc="添加好友或同步会话后，这里会出现连续天数排行" />
        </section>

        <section class="card card-pad live">
          <div class="card-head">
            <div>
              <h3 class="card-title">
                实时动态
                <span class="live-dot" :class="{ on: realtime.connected }" />
              </h3>
              <p class="card-sub">续火与断火事件即时推送</p>
            </div>
          </div>
          <ul v-if="feed.length" class="feed">
            <li v-for="(item, i) in feed" :key="i" :class="item.level">
              <span class="mark" />
              <div class="body">
                <strong>{{ item.title }}</strong>
                <p>{{ item.body }}</p>
                <small>{{ item.time }}</small>
              </div>
            </li>
          </ul>
          <EmptyState v-else title="暂无动态" desc="任务执行或手动续火后，事件会实时出现在这里" />
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import StatCard from '@/components/StatCard.vue'
import Avatar from '@/components/Avatar.vue'
import EmptyState from '@/components/EmptyState.vue'
import TrendChart from '@/components/charts/TrendChart.vue'
import DonutChart from '@/components/charts/DonutChart.vue'
import HeatMap from '@/components/charts/HeatMap.vue'
import { overviewApi } from '@/api'
import type { HeatPoint, Overview, TrendPoint } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useRealtimeStore } from '@/stores/realtime'
import { formatClock, greeting, percent, timeAgo } from '@/utils/format'

const auth = useAuthStore()
const realtime = useRealtimeStore()
const data = ref<Overview | null>(null)
const trend = ref<TrendPoint[]>([])
const heatmap = ref<HeatPoint[]>([])
const loading = ref(true)
const days = ref(14)

const urgentCount = computed(() => {
  const level = data.value?.levels?.find((l) => l.level === 'urgent')
  return level?.count ?? 0
})

const rateBadge = computed(() => {
  const rate = data.value?.successRate ?? 0
  if (rate >= 0.95) return '优秀'
  if (rate >= 0.85) return '正常'
  return '需关注'
})
const rateBadgeClass = computed(() => (rateBadge.value === '优秀' ? 'safe' : rateBadge.value === '正常' ? 'warn' : 'urgent'))

const donutSlices = computed(() => (data.value?.levels || []).map((item) => ({
  label: item.text, value: item.count, color: levelColor(item.level),
})))

const sparkFriends = computed(() => (data.value?.trend || []).map((p) => p.success + p.failed))
const sparkSent = computed(() => (data.value?.trend || []).map((p) => p.success))
const sparkRate = computed(() => (data.value?.trend || []).map((p) => (p.success + p.failed ? p.success / (p.success + p.failed) : 0)))
const sparkAccounts = computed(() => (data.value?.trend || []).map((p) => p.success))

const feed = computed(() => {
  const live = realtime.events.slice(0, 8).map((evt) => ({
    title: evt.title, body: evt.body, level: evt.level, time: formatClock(evt.at),
  }))
  if (live.length) return live
  return (data.value?.recent || []).map((item) => ({
    title: `${item.status === 'success' ? '续火成功' : '发送失败'} · ${item.friendName}`,
    body: item.status === 'success' ? item.content : item.error,
    level: item.status === 'success' ? 'success' : 'danger',
    time: timeAgo(item.createdAt),
  }))
})

function levelColor(level: string) {
  return ({ safe: '#2dd4a7', warn: '#f5b544', urgent: '#ff7a5c', broken: '#7b7b90' } as Record<string, string>)[level] || '#ff7a18'
}

async function load() {
  loading.value = true
  try {
    const [overview, trendData, heat] = await Promise.all([
      overviewApi.overview(), overviewApi.trend(days.value), overviewApi.heatmap(),
    ])
    data.value = overview
    trend.value = trendData
    heatmap.value = heat
  } finally {
    loading.value = false
  }
}

async function switchDays(value: number) {
  days.value = value
  trend.value = await overviewApi.trend(value)
}

onMounted(load)
</script>

<style scoped>
.hello { display: flex; align-items: flex-end; justify-content: space-between; gap: 18px; flex-wrap: wrap; }
.hello-title { font-size: 26px; font-weight: 720; letter-spacing: -.03em; }
.hello-sub { margin-top: 7px; font-size: 13px; color: var(--muted); }
.hello-sub b { font-weight: 700; }
.urgent-text { color: var(--urgent); }
.hello-actions { display: flex; gap: 10px; }

.main-grid { display: grid; grid-template-columns: minmax(0, 1.62fr) minmax(0, 1fr); gap: 18px; }
.col-main, .col-side { display: flex; flex-direction: column; gap: 18px; min-width: 0; }
.card-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.seg { display: flex; padding: 3px; gap: 2px; background: var(--surface); border: 1px solid var(--line); border-radius: 10px; }
.seg button { padding: 5px 10px; font-size: 11.5px; color: var(--muted); background: none; border: none; border-radius: 8px; cursor: pointer; }
.seg button.active { background: var(--surface-3); color: var(--text); }

.donut-wrap { display: flex; align-items: center; gap: 18px; margin-top: 14px; }
.legend { flex: 1; display: flex; flex-direction: column; gap: 9px; }
.legend li { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--muted); }
.legend .dot { width: 8px; height: 8px; border-radius: 3px; }
.legend .name { flex: 1; }
.legend b { color: var(--text); font-weight: 650; }

.rank { margin-top: 14px; display: flex; flex-direction: column; gap: 3px; }
.rank li { display: flex; align-items: center; gap: 10px; padding: 7px 6px; border-radius: 10px; }
.rank li:hover { background: var(--surface); }
.index { width: 18px; font-size: 11px; color: var(--muted-2); text-align: center; font-variant-numeric: tabular-nums; }
.index.top { color: var(--flame-1); font-weight: 700; }
.rank .name { flex: 1; font-size: 12.5px; color: var(--text-2); }
.days { font-size: 14px; font-weight: 700; color: var(--flame-2); }
.days small { font-size: 10px; color: var(--muted-2); margin-left: 2px; font-weight: 500; }
.link { font-size: 11.5px; color: var(--muted); }
.link:hover { color: var(--flame-2); }

.live-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--muted-2); display: inline-block; }
.live-dot.on { background: var(--safe); box-shadow: 0 0 8px var(--safe); animation: pulseGlow 2s infinite; }
.feed { margin-top: 14px; display: flex; flex-direction: column; gap: 2px; max-height: 330px; overflow: auto; }
.feed li { display: flex; gap: 10px; padding: 9px 6px; border-radius: 10px; }
.feed li:hover { background: var(--surface); }
.feed .mark { width: 6px; height: 6px; border-radius: 50%; margin-top: 6px; flex: none; background: var(--muted); }
.feed li.success .mark { background: var(--safe); }
.feed li.danger .mark { background: var(--urgent); }
.feed li.warn .mark { background: var(--warn); }
.feed .body { min-width: 0; }
.feed strong { display: block; font-size: 12px; font-weight: 600; }
.feed p { margin-top: 2px; font-size: 11.5px; color: var(--muted); line-height: 1.5; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
.feed small { font-size: 10px; color: var(--muted-2); }

@media (max-width: 1180px) {
  .main-grid { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 640px) {
  .hello-title { font-size: 21px; }
  .donut-wrap { flex-direction: column; }
}
</style>

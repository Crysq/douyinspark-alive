import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/Login.vue'), meta: { public: true, title: '登录' } },
    { path: '/register', name: 'register', component: () => import('@/views/Register.vue'), meta: { public: true, title: '注册开通' } },
    {
      path: '/',
      component: () => import('@/layout/AppShell.vue'),
      children: [
        { path: '', redirect: '/dashboard' },
        { path: 'dashboard', name: 'dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: '总览', desc: '火花健康度与续火动态' } },
        { path: 'friends', name: 'friends', component: () => import('@/views/Friends.vue'), meta: { title: '火花好友', desc: '每个好友的倒计时与自动维护' } },
        { path: 'accounts', name: 'accounts', component: () => import('@/views/Accounts.vue'), meta: { title: '账号管理', desc: '多账号托管与登录态' } },
        { path: 'tasks', name: 'tasks', component: () => import('@/views/Tasks.vue'), meta: { title: '定时任务', desc: '时间窗口内自动补火' } },
        { path: 'templates', name: 'templates', component: () => import('@/views/Templates.vue'), meta: { title: '消息模板', desc: '让每次续火都不一样' } },
        { path: 'records', name: 'records', component: () => import('@/views/Records.vue'), meta: { title: '发送记录', desc: '每一次下发的真实结果' } },
        { path: 'membership', name: 'membership', component: () => import('@/views/Membership.vue'), meta: { title: '会员中心', desc: '卡密兑换与到期时间' } },
        { path: 'settings', name: 'settings', component: () => import('@/views/Settings.vue'), meta: { title: '系统设置', desc: '策略、通知与数据维护' } },
        { path: 'admin/users', name: 'admin-users', component: () => import('@/views/AdminUsers.vue'), meta: { title: '用户管理', desc: '客户列表、会员与状态', admin: true } },
        { path: 'admin/cards', name: 'admin-cards', component: () => import('@/views/AdminCards.vue'), meta: { title: '卡密管理', desc: '批量生成与核销记录', admin: true } },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.loadMode()
  // 原登录逻辑下不开放注册与会员入口
  if (auth.mode === 'simple' && (to.name === 'register' || to.name === 'membership' || to.meta.admin)) {
    return { name: 'dashboard' }
  }
  const hasToken = Boolean(localStorage.getItem('doyinspark.token'))
  if (!to.meta.public && !hasToken && !auth.pending) return { name: 'login', query: { redirect: to.fullPath } }
  if (to.meta.public && hasToken) return { name: 'dashboard' }
  if (to.meta.admin) {
    if (!auth.user) await auth.loadMe()
    if (auth.user?.role !== 'admin') return { name: 'dashboard' }
  }
  document.title = `${(to.meta.title as string) || '控制台'} · DoyinSpark`
  return true
})

export default router

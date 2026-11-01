import { del, get, patch, post, put, request } from './client'
import type {
  Account, AdminStats, AdminUserRow, AuthPayload, CardKey, Friend, Health, HeatPoint,
  MembershipInfo, Notification, Overview, Page, Plan, RecordItem, SendResult, Setting,
  StreamEvent, Task, Template, TrendPoint, User,
} from './types'

export const authApi = {
  login: (username: string, password: string) => post<AuthPayload>('/auth/login', { username, password }),
  register: (payload: { username: string; password: string; nickname?: string; cardCode: string }) =>
    post<AuthPayload>('/auth/register', payload),
  me: () => get<User>('/auth/me'),
  logout: () => post<null>('/auth/logout'),
  password: (oldPassword: string, newPassword: string) => put<null>('/auth/password', { oldPassword, newPassword }),
}

export const membershipApi = {
  plans: () => get<{ plans: Plan[]; windowHours: number }>('/membership/plans'),
  mine: () => get<MembershipInfo>('/membership'),
  redeem: (code: string, activationToken?: string) =>
    request<{ token: string; expiresAt: string; user: User; card: { code: string; plan: string; name: string; days: number } }>({
      url: '/membership/redeem',
      method: 'POST',
      data: { code },
      headers: activationToken ? { Authorization: `Bearer ${activationToken}` } : undefined,
    }),
}

export const adminApi = {
  stats: () => get<AdminStats>('/admin/stats'),
  users: (params: Record<string, unknown>) => get<Page<AdminUserRow>>('/admin/users', params),
  updateUser: (id: number, payload: Record<string, unknown>) => patch<null>(`/admin/users/${id}`, payload),
  grant: (id: number, plan: string) => post<User>(`/admin/users/${id}/grant`, { plan }),
  cards: (params: Record<string, unknown>) => get<Page<CardKey>>('/admin/cards', params),
  generate: (payload: { plan: string; count: number; note?: string; validDays?: number }) =>
    post<{ batch: string; cards: CardKey[] }>('/admin/cards/generate', payload),
  voidCard: (id: number) => post<null>(`/admin/cards/${id}/void`),
  exportUrl: (params: Record<string, string> = {}) => {
    const query = new URLSearchParams({ ...params, token: localStorage.getItem('doyinspark.token') || '' })
    return `/api/admin/cards/export?${query.toString()}`
  },
}

export const overviewApi = {
  overview: () => get<Overview>('/overview'),
  trend: (days = 30) => get<TrendPoint[]>('/overview/trend', { days }),
  heatmap: () => get<HeatPoint[]>('/overview/heatmap'),
  notifications: () => get<{ items: Notification[]; unread: number }>('/notifications'),
  readNotifications: () => post<null>('/notifications/read'),
  audit: (page = 1) => get<Page<{ id: number; action: string; target: string; detail: string; ip: string; createdAt: string }>>('/audit', { page }),
}

export const accountApi = {
  list: () => get<Account[]>('/accounts'),
  create: (payload: { nickname: string; remark?: string; cookie?: string }) => post<Account>('/accounts', payload),
  update: (id: number, payload: { nickname?: string; remark?: string }) => patch<null>(`/accounts/${id}`, payload),
  remove: (id: number) => del<null>(`/accounts/${id}`),
  loginQR: (id: number) => post<{ logged: boolean; dataUrl: string; fallback: boolean }>(`/accounts/${id}/login`),
  loginStatus: (id: number) => get<{ logged: boolean; cookieCount: number }>(`/accounts/${id}/login/status`),
  cookie: (id: number, cookie: string) => post<null>(`/accounts/${id}/cookie`, { cookie }),
  sync: (id: number) => post<null>(`/accounts/${id}/sync`),
  syncFriends: (id: number) => post<{ created: number; updated: number; total: number }>(`/accounts/${id}/sync/friends`),
  logout: (id: number) => post<null>(`/accounts/${id}/logout`),
}

export const friendApi = {
  list: (params: Record<string, unknown>) => get<Page<Friend>>('/friends', params),
  create: (payload: Record<string, unknown>) => post<Friend>('/friends', payload),
  update: (id: number, payload: Record<string, unknown>) => patch<null>(`/friends/${id}`, payload),
  remove: (id: number) => del<null>(`/friends/${id}`),
  keep: (id: number) => post<SendResult>(`/friends/${id}/keep`),
  batchKeep: (ids: number[]) => post<{ success: number; failed: number; messages: string[] }>('/friends/batch-keep', { ids }),
}

export const taskApi = {
  list: () => get<Task[]>('/tasks'),
  create: (payload: Record<string, unknown>) => post<Task>('/tasks', payload),
  update: (id: number, payload: Record<string, unknown>) => patch<null>(`/tasks/${id}`, payload),
  remove: (id: number) => del<null>(`/tasks/${id}`),
  run: (id: number) => post<{ sent: number }>(`/tasks/${id}/run`),
}

export const templateApi = {
  list: (category = 'all') => get<Template[]>('/templates', { category }),
  create: (payload: Record<string, unknown>) => post<Template>('/templates', payload),
  update: (id: number, payload: Record<string, unknown>) => patch<null>(`/templates/${id}`, payload),
  remove: (id: number) => del<null>(`/templates/${id}`),
}

export const recordApi = {
  list: (params: Record<string, unknown>) => get<Page<RecordItem>>('/records', params),
  stats: () => get<{ total: number; success: number; failed: number; today: number; successRate: number }>('/records/stats'),
  prune: (days = 30) => del<{ deleted: number }>('/records/prune', { days }),
  exportUrl: () => `/api/records/export?token=${encodeURIComponent(localStorage.getItem('doyinspark.token') || '')}`,
}

export const settingApi = {
  get: () => get<Setting>('/settings'),
  save: (payload: Setting) => put<Setting>('/settings', payload),
}

export const systemApi = {
  health: () => get<Health>('/system/health'),
  rules: () => get<{ windowHours: number; levels: { level: string; text: string; rule: string }[]; streak: string[] }>('/system/rules'),
  demo: () => post<{ accounts: number; friends: number; records: number }>('/system/demo'),
  clearDemo: () => del<null>('/system/demo'),
}

export type { StreamEvent }

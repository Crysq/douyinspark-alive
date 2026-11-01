export type Role = 'admin' | 'user'
export type PlanCode = 'none' | 'day' | 'week' | 'month' | 'quarter' | 'year' | 'forever'

export interface User {
  id: number
  username: string
  nickname: string
  avatar: string
  role: Role
  status: 'active' | 'pending' | 'banned'
  plan: PlanCode
  memberExpireAt?: string | null
  lastLoginAt?: string | null
  createdAt: string
  daysLeft: number
  planLabel: string
  isMember: boolean
}

export interface Plan {
  code: PlanCode
  name: string
  days: number
  price: number
  desc: string
}

export interface MembershipLog {
  id: number
  action: 'register' | 'redeem' | 'grant'
  plan: PlanCode
  days: number
  cardCode: string
  beforeExpire?: string | null
  afterExpire?: string | null
  remark: string
  createdAt: string
}

export interface MembershipInfo {
  user: User
  plans: Plan[]
  history: MembershipLog[]
}

export interface AuthPayload {
  token: string
  expiresAt: string
  user: User
  needActivation?: boolean
  activationToken?: string
}

export interface CardKey {
  id: number
  code: string
  plan: PlanCode
  days: number
  status: 'unused' | 'used' | 'void'
  batch: string
  note: string
  usedBy: number
  usedAt?: string | null
  expireAt?: string | null
  createdAt: string
}

export interface AdminUserRow {
  user: User
  accountCount: number
  friendCount: number
}

export interface AdminStats {
  users: number
  members: number
  activeUsers: number
  cards: number
  cardsUsed: number
  cardsLeft: number
  accounts: number
  records: number
}

export interface Account {
  id: number
  nickname: string
  douyinUid: string
  uniqueId: string
  secUid: string
  avatarUrl: string
  status: 'online' | 'offline' | 'expired'
  remark: string
  lastSyncAt?: string | null
  createdAt: string
  friendCount: number
  sparkCount: number
  brokenCount: number
}

export interface SparkState {
  level: 'safe' | 'warn' | 'urgent' | 'broken'
  levelText: string
  remainingSec: number
  progress: number
  streakDays: number
  maxStreakDays: number
  lastInteractAt?: string | null
  due: boolean
}

export interface Friend {
  id: number
  accountId: number
  name: string
  userId: string
  secUid: string
  uniqueId: string
  avatarUrl: string
  streakDays: number
  maxStreakDays: number
  lastInteractAt?: string | null
  autoKeep: boolean
  remark: string
  createdAt: string
  state: SparkState
  accountName: string
}

export interface Task {
  id: number
  accountId: number
  name: string
  templateId: number
  windowStart: string
  windowEnd: string
  intervalMin: number
  maxPerRound: number
  enabled: boolean
  lastRunAt?: string | null
  runCount: number
  accountName: string
  templateName: string
  nextRunAt?: string | null
  inWindow: boolean
}

export interface Template {
  id: number
  name: string
  category: 'daily' | 'festival' | 'custom'
  content: string
  useCount: number
  enabled: boolean
  createdAt: string
}

export interface RecordItem {
  id: number
  accountId: number
  friendId: number
  friendName: string
  taskId: number
  templateId: number
  content: string
  status: 'success' | 'failed' | 'skipped'
  source: 'auto' | 'manual'
  error: string
  latencyMs: number
  createdAt: string
}

export interface TrendPoint { date: string; success: number; failed: number }
export interface LevelSlice { level: string; text: string; count: number }
export interface HeatPoint { weekday: number; hour: number; count: number }

export interface FriendCard {
  id: number
  name: string
  avatarUrl: string
  streakDays: number
  accountId: number
  level: string
}

export interface Overview {
  accounts: number
  onlineAccounts: number
  friends: number
  autoKeep: number
  todaySent: number
  todaySuccess: number
  successRate: number
  totalStreak: number
  avgStreak: number
  activeTasks: number
  levels: LevelSlice[]
  trend: TrendPoint[]
  top: FriendCard[]
  recent: RecordItem[]
  channel: 'ready' | 'offline'
  updatedAt: string
}

export interface Notification {
  id: number
  level: 'info' | 'warn' | 'danger'
  title: string
  body: string
  read: boolean
  createdAt: string
}

export interface Setting {
  id: number
  retryTimes: number
  quietStart: string
  quietEnd: string
  notifyBreak: boolean
  notifyWebhook: string
  autoSync: boolean
  keepAlive: boolean
  theme: string
  updatedAt: string
}

export interface Health {
  status: string
  mysql: string
  redis: string
  sign: string
  version: string
  uptimeSec: number
  now: string
}

export interface Page<T> { items: T[]; total: number; page: number; pageSize: number }

export interface SendResult {
  record: RecordItem
  friend: Friend
  success: boolean
}

export interface StreamEvent {
  type: 'send' | 'break' | 'account' | 'system'
  level: 'info' | 'success' | 'warn' | 'danger'
  title: string
  body: string
  at: string
  payload?: Record<string, unknown>
}

// Package model 定义持久化实体。所有租户数据都带 UserID 做隔离。
package model

import "time"

// 会员卡类型。
const (
	PlanNone    = "none"    // 未激活
	PlanDay     = "day"     // 日卡
	PlanWeek    = "week"    // 周卡
	PlanMonth   = "month"   // 月卡
	PlanQuarter = "quarter" // 季卡
	PlanYear    = "year"    // 年卡
	PlanForever = "forever" // 永久卡
)

// User 是控制台用户：管理员与普通客户共用一张表，用 Role 区分。
type User struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Username       string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash   string     `gorm:"size:120;not null" json:"-"`
	Nickname       string     `gorm:"size:64" json:"nickname"`
	Avatar         string     `gorm:"size:255" json:"avatar"`
	Role           string     `gorm:"size:16;default:user;index" json:"role"`   // admin | user
	Status         string     `gorm:"size:16;default:active;index" json:"status"` // active | pending | banned
	Plan           string     `gorm:"size:16;default:none" json:"plan"`
	MemberExpireAt *time.Time `gorm:"index" json:"memberExpireAt"`
	LastLoginAt    *time.Time `json:"lastLoginAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// CardKey 卡密：一次性使用，按卡种兑换会员时长。
type CardKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Code       string     `gorm:"size:40;uniqueIndex;not null" json:"code"`
	Plan       string     `gorm:"size:16;index" json:"plan"`
	Days       int        `json:"days"`
	Status     string     `gorm:"size:16;default:unused;index" json:"status"` // unused | used | void
	Batch      string     `gorm:"size:32;index" json:"batch"`
	Note       string     `gorm:"size:128" json:"note"`
	CreatedBy  uint       `json:"createdBy"`
	UsedBy     uint       `gorm:"index" json:"usedBy"`
	UsedAt     *time.Time `json:"usedAt"`
	ExpireAt   *time.Time `json:"expireAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// MembershipLog 会员流水。
type MembershipLog struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"index;not null" json:"userId"`
	Action       string     `gorm:"size:16;index" json:"action"` // register | redeem | grant
	Plan         string     `gorm:"size:16" json:"plan"`
	Days         int        `json:"days"`
	CardCode     string     `gorm:"size:40" json:"cardCode"`
	BeforeExpire *time.Time `json:"beforeExpire"`
	AfterExpire  *time.Time `json:"afterExpire"`
	Remark       string     `gorm:"size:128" json:"remark"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// Account 被托管的抖音账号。
type Account struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"index;not null" json:"userId"`
	Nickname   string     `gorm:"size:64;not null" json:"nickname"`
	DouyinUID  string     `gorm:"size:64;index" json:"douyinUid"`
	UniqueID   string     `gorm:"size:64" json:"uniqueId"`
	SecUID     string     `gorm:"size:128" json:"secUid"`
	AvatarURL  string     `gorm:"size:512" json:"avatarUrl"`
	Cookie     string     `gorm:"type:text" json:"-"`
	Status     string     `gorm:"size:16;default:offline;index" json:"status"` // online | offline | expired
	Remark     string     `gorm:"size:255" json:"remark"`
	LastSyncAt *time.Time `json:"lastSyncAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// Friend 火花好友。
type Friend struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"index;not null" json:"-"`
	AccountID      uint       `gorm:"index;not null" json:"accountId"`
	Name           string     `gorm:"size:64;not null" json:"name"`
	DouyinUserID   string     `gorm:"column:friend_user_id;size:64;index" json:"userId"`
	SecUID         string     `gorm:"size:128" json:"secUid"`
	UniqueID       string     `gorm:"size:64" json:"uniqueId"`
	AvatarURL      string     `gorm:"size:512" json:"avatarUrl"`
	StreakDays     int        `gorm:"default:0" json:"streakDays"`
	MaxStreakDays  int        `gorm:"default:0" json:"maxStreakDays"`
	LastInteractAt *time.Time `gorm:"index" json:"lastInteractAt"`
	AutoKeep       bool       `gorm:"default:true" json:"autoKeep"`
	Remark         string     `gorm:"size:255" json:"remark"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// Task 定时续火任务。
type Task struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"index;not null" json:"userId"`
	AccountID   uint       `gorm:"index;not null" json:"accountId"`
	Name        string     `gorm:"size:64;not null" json:"name"`
	TemplateID  uint       `json:"templateId"`
	WindowStart string     `gorm:"size:5;default:08:00" json:"windowStart"`
	WindowEnd   string     `gorm:"size:5;default:23:00" json:"windowEnd"`
	IntervalMin int        `gorm:"default:120" json:"intervalMin"`
	MaxPerRound int        `gorm:"default:5" json:"maxPerRound"`
	Enabled     bool       `gorm:"default:true;index" json:"enabled"`
	LastRunAt   *time.Time `json:"lastRunAt"`
	RunCount    int        `gorm:"default:0" json:"runCount"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Template 消息模板（UserID=0 为系统模板库）。
type Template struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"userId"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Category  string    `gorm:"size:16;default:daily;index" json:"category"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	UseCount  int       `gorm:"default:0" json:"useCount"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Record 一次发送记录。
type Record struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"userId"`
	AccountID  uint      `gorm:"index" json:"accountId"`
	FriendID   uint      `gorm:"index" json:"friendId"`
	FriendName string    `gorm:"size:64" json:"friendName"`
	TaskID     uint      `gorm:"index" json:"taskId"`
	TemplateID uint      `json:"templateId"`
	Content    string    `gorm:"type:text" json:"content"`
	Status     string    `gorm:"size:16;index" json:"status"`
	Source     string    `gorm:"size:16;index" json:"source"`
	Error      string    `gorm:"size:512" json:"error"`
	LatencyMs  int       `json:"latencyMs"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
}

// Setting 每个用户一份配置。
type Setting struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"uniqueIndex;not null" json:"userId"`
	RetryTimes    int       `gorm:"default:2" json:"retryTimes"`
	QuietStart    string    `gorm:"size:5;default:00:30" json:"quietStart"`
	QuietEnd      string    `gorm:"size:5;default:07:00" json:"quietEnd"`
	NotifyBreak   bool      `gorm:"default:true" json:"notifyBreak"`
	NotifyWebhook string    `gorm:"size:512" json:"notifyWebhook"`
	AutoSync      bool      `gorm:"default:true" json:"autoSync"`
	KeepAlive     bool      `gorm:"default:true" json:"keepAlive"`
	Theme         string    `gorm:"size:16;default:dark" json:"theme"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// AuditLog 操作审计。
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"userId"`
	Action    string    `gorm:"size:64;index" json:"action"`
	Target    string    `gorm:"size:128" json:"target"`
	Detail    string    `gorm:"size:512" json:"detail"`
	IP        string    `gorm:"size:64" json:"ip"`
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

// Notification 站内通知。
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"userId"`
	Level     string    `gorm:"size:16;default:info" json:"level"`
	Title     string    `gorm:"size:128" json:"title"`
	Body      string    `gorm:"size:512" json:"body"`
	Read      bool      `gorm:"default:false;index" json:"read"`
	CreatedAt time.Time `json:"createdAt"`
}

// All 返回迁移所需的全部实体。
func All() []any {
	return []any{&User{}, &CardKey{}, &MembershipLog{}, &Account{}, &Friend{}, &Task{}, &Template{}, &Record{}, &Setting{}, &AuditLog{}, &Notification{}}
}

package service

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"doyinspark/internal/model"
)

// Usage 一个用户当前的用量。
type Usage struct {
	Accounts  int64 `json:"accounts"`
	Friends   int64 `json:"friends"`
	Tasks     int64 `json:"tasks"`
	TodaySent int64 `json:"todaySent"`
}

// Entitlement 套餐权限评估结果。
type Entitlement struct {
	Plan     string   `json:"plan"`
	Valid    bool     `json:"valid"`
	DaysLeft int      `json:"daysLeft"`
	ExpireAt *time.Time `json:"expireAt"`
	Quota    Quota    `json:"quota"`
	Usage    Usage    `json:"usage"`
}

// EntitlementOf 读取用户套餐与用量，统一在内存里计算。
func EntitlementOf(db *gorm.DB, user *model.User, now time.Time) *Entitlement {
	e := &Entitlement{
		Plan:     user.Plan,
		Valid:    IsMember(user.MemberExpireAt, now),
		DaysLeft: DaysLeft(user.MemberExpireAt, now),
		ExpireAt: user.MemberExpireAt,
		Quota:    QuotaOf(user.Plan),
	}
	e.Quota = QuotaOf(user.Plan)
	db.Model(&model.Account{}).Where("user_id = ?", user.ID).Count(&e.Usage.Accounts)
	db.Model(&model.Friend{}).Where("user_id = ?", user.ID).Count(&e.Usage.Friends)
	db.Model(&model.Task{}).Where("user_id = ?", user.ID).Count(&e.Usage.Tasks)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	db.Model(&model.Record{}).Where("user_id = ? AND created_at >= ? AND status = ?", user.ID, dayStart, "success").Count(&e.Usage.TodaySent)
	return e
}

// CheckLimit 校验某项是否超限，返回可读错误；limit<=0 视为不限量。
func (e *Entitlement) CheckLimit(resource string, current int64) error {
	var limit int
	var name string
	switch resource {
	case "accounts":
		limit, name = e.Quota.MaxAccounts, "托管账号"
	case "friends":
		limit, name = e.Quota.MaxFriends, "火花好友"
	case "tasks":
		limit, name = e.Quota.MaxTasks, "定时任务"
	case "daily_send":
		limit, name = e.Quota.MaxPerDay, "今日发送"
	default:
		return fmt.Errorf("未知资源")
	}
	if limit <= 0 {
		return nil
	}
	if current >= int64(limit) {
		return fmt.Errorf("%s已达套餐上限（%s：%d/%d），请升级套餐或续期", name, PlanLabel(e.Plan), current, limit)
	}
	return nil
}

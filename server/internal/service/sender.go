package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"

	"doyinspark/internal/cache"
	"doyinspark/internal/model"
	"doyinspark/internal/signer"
)

// Sender 负责一次真实的私信下发与落库。
type Sender struct {
	db   *gorm.DB
	rdb  *cache.Cache
	sign *signer.Client
	bus  *EventBus
	loc  *time.Location
}

// NewSender 装配发送器。
func NewSender(db *gorm.DB, rdb *cache.Cache, sign *signer.Client, bus *EventBus, loc *time.Location) *Sender {
	return &Sender{db: db, rdb: rdb, sign: sign, bus: bus, loc: loc}
}

// 风控信号关键词：命中即熔断，避免"越试越糟"。
var riskSignals = []string{"风控", "频繁", "验证码", "captcha", "too many", "403", "账号异常", "请稍后再试", "拒绝", "blocked"}

func isRiskSignal(msg string) bool {
	lower := strings.ToLower(msg)
	for _, sig := range riskSignals {
		if strings.Contains(lower, strings.ToLower(sig)) {
			return true
		}
	}
	return false
}

// SendResult 发送结果，包含落库后的记录。
type SendResult struct {
	Record  model.Record `json:"record"`
	Friend  model.Friend `json:"friend"`
	Success bool         `json:"success"`
}

// SendForFriend 给指定好友发一条续火消息。
//
// source: auto（调度器）| manual（手动）
// templateID 为 0 时随机挑选一条启用中的模板。
func (s *Sender) SendForFriend(ctx context.Context, accountID, friendID, templateID, taskID uint, source string) (*SendResult, error) {
	if !s.sign.Configured() {
		return nil, errors.New("发送通道未配置：请设置 SIGN_SERVICE_URL 并启动签名服务")
	}

	var account model.Account
	if err := s.db.First(&account, accountID).Error; err != nil {
		return nil, errors.New("账号不存在")
	}
	if account.Status != "online" {
		return nil, errors.New("账号当前不在线，请先完成扫码登录")
	}
	var friend model.Friend
	if err := s.db.First(&friend, friendID).Error; err != nil {
		return nil, errors.New("好友不存在")
	}
	if friend.AccountID != account.ID || friend.UserID != account.UserID {
		return nil, errors.New("好友不属于该账号")
	}

	var owner model.User
	if err := s.db.First(&owner, account.UserID).Error; err != nil {
		return nil, errors.New("账号归属用户不存在")
	}
	ent := EntitlementOf(s.db, &owner, time.Now().In(s.loc))
	if err := ent.CheckLimit("daily_send", ent.Usage.TodaySent); err != nil {
		return nil, err
	}
	if friend.DouyinUserID == "" {
		return nil, errors.New("缺少对方 user_id，无法发送")
	}

	nowCheck := time.Now().In(s.loc)
	if account.RiskUntil != nil && account.RiskUntil.After(nowCheck) {
		return nil, fmt.Errorf("账号处于风控冷却中（%s 解除），已跳过发送", account.RiskUntil.In(s.loc).Format("01-02 15:04"))
	}
	// 新账号冷启动：注册后 N 天内每天最多 M 条（可在系统设置里调整，0=关闭）
	warmupDays, warmupDaily := 3, 5
	var setting model.Setting
	if err := s.db.Where("user_id = ?", account.UserID).First(&setting).Error; err == nil {
		if setting.WarmupDaily > 0 {
			warmupDays, warmupDaily = setting.WarmupDays, setting.WarmupDaily
		}
	}
	if warmupDaily > 0 && nowCheck.Sub(account.CreatedAt) < time.Duration(warmupDays)*24*time.Hour {
		dayStart := time.Date(nowCheck.Year(), nowCheck.Month(), nowCheck.Day(), 0, 0, 0, 0, s.loc)
		var todayCount int64
		s.db.Model(&model.Record{}).Where("account_id = ? AND created_at >= ? AND status = ?", account.ID, dayStart, "success").Count(&todayCount)
		if todayCount >= int64(warmupDaily) {
			return nil, fmt.Errorf("新账号冷启动期：今日已发 %d 条（上限 %d），明天再继续", todayCount, warmupDaily)
		}
	}

	tpl, err := s.pickTemplate(account.ID, templateID)
	if err != nil {
		return nil, err
	}

	now := time.Now().In(s.loc)
	streak := NextStreak(friend.LastInteractAt, friend.StreakDays, now)
	content := RenderTemplate(tpl.Content, friend.Name, streak, now)

	start := time.Now()
	sendErr := s.sign.SendIM(account.ID, account.Cookie, friend.DouyinUserID, friend.Name, content)
	latency := int(time.Since(start).Milliseconds())

	record := model.Record{
		UserID:     account.UserID,
		AccountID:  account.ID,
		FriendID:   friend.ID,
		FriendName: friend.Name,
		TaskID:     taskID,
		TemplateID: tpl.ID,
		Content:    content,
		Status:     "success",
		Source:     source,
		LatencyMs:  latency,
		CreatedAt:  now,
	}

	if sendErr != nil {
		record.Status = "failed"
		record.Error = truncate(sendErr.Error(), 500)
		s.db.Create(&record)
		s.bus.Publish(Event{UserID: account.UserID, Type: "send", Level: "danger", Title: "发送失败 · " + friend.Name, Body: record.Error, Payload: map[string]any{"accountId": account.ID, "friendId": friend.ID}})
		s.invalidate(ctx, account.UserID, account.ID)
		s.markExpiredIfNeeded(&account, sendErr)
		if isRiskSignal(sendErr.Error()) {
			s.circuitBreak(&account, sendErr.Error())
		}
		return &SendResult{Record: record, Friend: friend, Success: false}, sendErr
	}

	// 发送成功：推进连续天数与上次互动时间
	oldLast := friend.LastInteractAt
	friend.LastInteractAt = &now
	friend.StreakDays = streak
	if friend.StreakDays > friend.MaxStreakDays {
		friend.MaxStreakDays = friend.StreakDays
	}
	s.db.Model(&friend).Updates(map[string]any{
		"last_interact_at": friend.LastInteractAt,
		"streak_days":      friend.StreakDays,
		"max_streak_days":  friend.MaxStreakDays,
	})
	s.db.Model(&model.Template{}).Where("id = ?", tpl.ID).UpdateColumn("use_count", gorm.Expr("use_count + 1"))
	s.db.Create(&record)

	gained := oldLast == nil || !sameDay(*oldLast, now)
	title := "续火成功 · " + friend.Name
	body := fmt.Sprintf("%s · 连续 %d 天", content, friend.StreakDays)
	if !gained {
		body = fmt.Sprintf("%s · 今日已互动，天数保持 %d", content, friend.StreakDays)
	}
	s.bus.Publish(Event{UserID: account.UserID, Type: "send", Level: "success", Title: title, Body: body, Payload: map[string]any{"accountId": account.ID, "friendId": friend.ID, "recordId": record.ID}})
	s.invalidate(ctx, account.UserID, account.ID)

	return &SendResult{Record: record, Friend: friend, Success: true}, nil
}

func (s *Sender) pickTemplate(accountID, templateID uint) (*model.Template, error) {
	if templateID > 0 {
		var tpl model.Template
		if err := s.db.First(&tpl, templateID).Error; err != nil {
			return nil, errors.New("消息模板不存在")
		}
		return &tpl, nil
	}
	var list []model.Template
	if err := s.db.Where("enabled = ?", true).Find(&list).Error; err != nil || len(list) == 0 {
		return nil, errors.New("没有可用的消息模板，请先在模板页创建")
	}
	// 同一账号同一天不重复使用同一模板，避免"每天同一句话"
	ctx := context.Background()
	day := time.Now().In(s.loc).Format("20060102")
	key := fmt.Sprintf("doyinspark:tmpl:%d:%s", accountID, day)
	used := s.rdb.GetList(ctx, key)
	usedSet := map[uint]bool{}
	for _, id := range used {
		usedSet[id] = true
	}
	candidates := make([]model.Template, 0, len(list))
	for _, item := range list {
		if !usedSet[item.ID] {
			candidates = append(candidates, item)
		}
	}
	if len(candidates) == 0 { // 都用过了，重置本轮
		used = nil
		candidates = list
	}
	chosen := candidates[rand.Intn(len(candidates))]
	used = append(used, chosen.ID)
	s.rdb.SetList(ctx, key, used, 36*time.Hour)
	return &chosen, nil
}

func (s *Sender) invalidate(ctx context.Context, userID, accountID uint) {
	if s.rdb == nil {
		return
	}
	s.rdb.Del(ctx, fmt.Sprintf("doyinspark:overview:%d", userID))
	s.rdb.Del(ctx, fmt.Sprintf("doyinspark:account:%d:stat", accountID))
}

func (s *Sender) markExpiredIfNeeded(account *model.Account, err error) {
	msg := err.Error()
	if strings.Contains(msg, "登录态") || strings.Contains(msg, "401") || strings.Contains(msg, "csrf") {
		s.db.Model(account).Update("status", "expired")
		s.bus.Publish(Event{UserID: account.UserID, Type: "account", Level: "warn", Title: "登录态失效 · " + account.Nickname, Body: "请重新扫码登录该账号"})
	}
}

// circuitBreak 命中风控：该账号冷却到次日 06:00，并通知用户。
func (s *Sender) circuitBreak(account *model.Account, reason string) {
	now := time.Now().In(s.loc)
	until := time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, s.loc).AddDate(0, 0, 1)
	s.db.Model(account).Updates(map[string]any{"risk_until": until, "risk_reason": truncate(reason, 240)})
	s.db.Create(&model.Notification{
		UserID: account.UserID, Level: "danger",
		Title: "账号触发风控 · " + account.Nickname,
		Body:  fmt.Sprintf("已自动停止该账号发送，冷却至 %s。原因：%s", until.Format("01-02 15:04"), truncate(reason, 120)),
	})
	s.bus.Publish(Event{UserID: account.UserID, Type: "account", Level: "danger", Title: "风控熔断 · " + account.Nickname,
		Body: "已停止发送，冷却至 " + until.Format("01-02 15:04")})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

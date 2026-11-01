package handler

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"doyinspark/internal/model"
	"doyinspark/internal/service"
	"doyinspark/internal/util"
)

// TrendPoint 单日续火趋势。
type TrendPoint struct {
	Date    string `json:"date"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

// LevelSlice 火花状态分布。
type LevelSlice struct {
	Level string `json:"level"`
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// FriendCard 看板里的好友卡片。
type FriendCard struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	AvatarURL  string `json:"avatarUrl"`
	StreakDays int    `json:"streakDays"`
	AccountID  uint   `json:"accountId"`
	Level      string `json:"level"`
}

// Overview 看板聚合数据。
type Overview struct {
	Accounts       int64          `json:"accounts"`
	OnlineAccounts int64          `json:"onlineAccounts"`
	Friends        int64          `json:"friends"`
	AutoKeep       int64          `json:"autoKeep"`
	TodaySent      int64          `json:"todaySent"`
	TodaySuccess   int64          `json:"todaySuccess"`
	SuccessRate    float64        `json:"successRate"`
	TotalStreak    int64          `json:"totalStreak"`
	AvgStreak      float64        `json:"avgStreak"`
	ActiveTasks    int64          `json:"activeTasks"`
	Levels         []LevelSlice   `json:"levels"`
	Trend          []TrendPoint   `json:"trend"`
	Top            []FriendCard   `json:"top"`
	Recent         []model.Record `json:"recent"`
	Channel        string         `json:"channel"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// Overview 返回当前用户的总览看板（Redis 缓存 20 秒）。
func (h *Handler) Overview(c *gin.Context) {
	ctx := c.Request.Context()
	userID := uid(c)
	cacheKey := fmt.Sprintf("doyinspark:overview:%d", userID)
	var cached Overview
	if h.Cache.GetJSON(ctx, cacheKey, &cached) {
		util.OK(c, cached)
		return
	}

	loc := h.Cfg.Location
	now := time.Now().In(loc)
	var out Overview

	h.DB.Model(&model.Account{}).Where("user_id = ?", userID).Count(&out.Accounts)
	h.DB.Model(&model.Account{}).Where("user_id = ? AND status = ?", userID, "online").Count(&out.OnlineAccounts)
	h.DB.Model(&model.Friend{}).Where("user_id = ?", userID).Count(&out.Friends)
	h.DB.Model(&model.Friend{}).Where("user_id = ? AND auto_keep = ?", userID, true).Count(&out.AutoKeep)
	h.DB.Model(&model.Task{}).Where("user_id = ? AND enabled = ?", userID, true).Count(&out.ActiveTasks)

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	h.DB.Model(&model.Record{}).Where("user_id = ? AND created_at >= ?", userID, dayStart).Count(&out.TodaySent)
	h.DB.Model(&model.Record{}).Where("user_id = ? AND created_at >= ? AND status = ?", userID, dayStart, "success").Count(&out.TodaySuccess)
	if out.TodaySent > 0 {
		out.SuccessRate = float64(out.TodaySuccess) / float64(out.TodaySent)
	}

	var friends []model.Friend
	h.DB.Where("user_id = ?", userID).Order("streak_days desc").Find(&friends)
	levels := map[string]int{}
	for i := range friends {
		friend := &friends[i]
		state := service.Evaluate(friend.LastInteractAt, friend.StreakDays, friend.MaxStreakDays, now)
		levels[state.Level]++
		out.TotalStreak += int64(friend.StreakDays)
		if len(out.Top) < 8 {
			out.Top = append(out.Top, FriendCard{ID: friend.ID, Name: friend.Name, AvatarURL: friend.AvatarURL, StreakDays: friend.StreakDays, AccountID: friend.AccountID, Level: state.Level})
		}
	}
	if len(friends) > 0 {
		out.AvgStreak = float64(out.TotalStreak) / float64(len(friends))
	}
	labels := map[string]string{service.LevelSafe: "安全", service.LevelWarn: "预警", service.LevelUrgent: "紧急", service.LevelBroken: "已断火"}
	for _, level := range []string{service.LevelSafe, service.LevelWarn, service.LevelUrgent, service.LevelBroken} {
		out.Levels = append(out.Levels, LevelSlice{Level: level, Text: labels[level], Count: levels[level]})
	}

	trendStart := dayStart.AddDate(0, 0, -13)
	var records []model.Record
	h.DB.Select("status", "created_at").Where("user_id = ? AND created_at >= ?", userID, trendStart).Find(&records)
	buckets := map[string]*TrendPoint{}
	for i := 0; i < 14; i++ {
		key := trendStart.AddDate(0, 0, i).Format("01-02")
		buckets[key] = &TrendPoint{Date: key}
	}
	for i := range records {
		key := records[i].CreatedAt.In(loc).Format("01-02")
		point, ok := buckets[key]
		if !ok {
			continue
		}
		if records[i].Status == "success" {
			point.Success++
		} else {
			point.Failed++
		}
	}
	for i := 0; i < 14; i++ {
		key := trendStart.AddDate(0, 0, i).Format("01-02")
		out.Trend = append(out.Trend, *buckets[key])
	}

	h.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(8).Find(&out.Recent)
	out.Channel = h.channelState(c)
	out.UpdatedAt = now
	h.Cache.SetJSON(ctx, cacheKey, out, 20*time.Second)
	util.OK(c, out)
}

// Trend 近 N 天成功 / 失败曲线。
func (h *Handler) Trend(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 7 || days > 90 {
		days = 30
	}
	loc := h.Cfg.Location
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(days - 1))

	var records []model.Record
	h.DB.Select("status", "created_at").Where("user_id = ? AND created_at >= ?", uid(c), start).Find(&records)
	buckets := make(map[string]*TrendPoint, days)
	out := make([]TrendPoint, 0, days)
	for i := 0; i < days; i++ {
		key := start.AddDate(0, 0, i).Format("01-02")
		point := &TrendPoint{Date: key}
		buckets[key] = point
		out = append(out, *point)
	}
	for i := range records {
		key := records[i].CreatedAt.In(loc).Format("01-02")
		point, ok := buckets[key]
		if !ok {
			continue
		}
		if records[i].Status == "success" {
			point.Success++
		} else {
			point.Failed++
		}
	}
	for i := range out {
		out[i] = *buckets[out[i].Date]
	}
	util.OK(c, out)
}

// HeatmapPoint 24 小时活跃热力图。
type HeatmapPoint struct {
	Weekday int `json:"weekday"`
	Hour    int `json:"hour"`
	Count   int `json:"count"`
}

// Heatmap 近 14 天按星期 / 小时聚合。
func (h *Handler) Heatmap(c *gin.Context) {
	loc := h.Cfg.Location
	now := time.Now().In(loc)
	start := now.AddDate(0, 0, -14)
	var records []model.Record
	h.DB.Select("created_at").Where("user_id = ? AND created_at >= ?", uid(c), start).Find(&records)
	grid := make([][]int, 7)
	for i := range grid {
		grid[i] = make([]int, 24)
	}
	for i := range records {
		t := records[i].CreatedAt.In(loc)
		grid[int(t.Weekday())][t.Hour()]++
	}
	out := make([]HeatmapPoint, 0, 168)
	for w := 0; w < 7; w++ {
		for hour := 0; hour < 24; hour++ {
			out = append(out, HeatmapPoint{Weekday: w, Hour: hour, Count: grid[w][hour]})
		}
	}
	util.OK(c, out)
}

// Notifications 站内通知。
func (h *Handler) Notifications(c *gin.Context) {
	var items []model.Notification
	h.DB.Where("user_id = ?", uid(c)).Order("created_at desc").Limit(30).Find(&items)
	var unread int64
	h.DB.Model(&model.Notification{}).Where("user_id = ? AND read = ?", uid(c), false).Count(&unread)
	util.OK(c, gin.H{"items": items, "unread": unread})
}

// ReadNotifications 全部标记已读。
func (h *Handler) ReadNotifications(c *gin.Context) {
	h.DB.Model(&model.Notification{}).Where("user_id = ? AND read = ?", uid(c), false).Update("read", true)
	util.OKMsg(c, "已全部标记为已读", nil)
}

// AuditLogs 审计日志。
func (h *Handler) AuditLogs(c *gin.Context) {
	page, size := pageParams(atoi(c.DefaultQuery("page", "1")), atoi(c.DefaultQuery("pageSize", "20")))
	var items []model.AuditLog
	var total int64
	h.DB.Model(&model.AuditLog{}).Where("user_id = ?", uid(c)).Count(&total)
	h.DB.Where("user_id = ?", uid(c)).Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&items)
	util.OK(c, util.Page{Items: items, Total: total, Page: page, PageSize: size})
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func (h *Handler) channelState(c *gin.Context) string {
	if !h.Sign.Configured() {
		return "offline"
	}
	if err := h.Sign.Health(); err != nil {
		return "offline"
	}
	return "ready"
}

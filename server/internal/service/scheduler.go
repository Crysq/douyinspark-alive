package service

import (
	"bytes"
	"errors"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"doyinspark/internal/cache"
	"doyinspark/internal/model"
	"doyinspark/internal/signer"
)

// Scheduler 周期扫描任务，为「今天还没互动」的好友补发消息。
type Scheduler struct {
	db       *gorm.DB
	rdb      *cache.Cache
	sender   *Sender
	bus      *EventBus
	loc      *time.Location
	interval time.Duration
	stop     chan struct{}
}

// NewScheduler 装配调度器。
func NewScheduler(db *gorm.DB, rdb *cache.Cache, sender *Sender, bus *EventBus, loc *time.Location, interval time.Duration) *Scheduler {
	return &Scheduler{db: db, rdb: rdb, sender: sender, bus: bus, loc: loc, interval: interval, stop: make(chan struct{})}
}

// Start 启动后台循环。
func (s *Scheduler) Start() {
	log.Printf("调度器已启动，心跳 %s", s.interval)
	go s.loop()
}

// Stop 停止后台循环。
func (s *Scheduler) Stop() { close(s.stop) }

func (s *Scheduler) loop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.tick(context.Background())
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick(context.Background())
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	// 多实例部署时只允许一个实例执行，避免重复发送
	locked, release := s.rdb.Lock(ctx, "doyinspark:lock:scheduler", 45*time.Second)
	if !locked {
		return
	}
	defer release()

	now := time.Now().In(s.loc)
	setting := s.setting()

	if inQuiet(now, setting.QuietStart, setting.QuietEnd) {
		s.checkBreaks(ctx, now, setting)
		return
	}

	var tasks []model.Task
	if err := s.db.Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		log.Println("读取任务失败:", err)
		return
	}
	for i := range tasks {
		task := &tasks[i]
		if !WithinWindow(now, task.WindowStart, task.WindowEnd) {
			continue
		}
		if task.LastRunAt != nil && now.Sub(task.LastRunAt.In(s.loc)) < time.Duration(task.IntervalMin)*time.Minute {
			continue
		}
		if sent := s.runTask(ctx, task, setting.RetryTimes, false); sent > 0 {
			log.Printf("任务 %s 发送 %d 条", task.Name, sent)
		}
	}
	s.checkBreaks(ctx, now, setting)
}

// RunNow 手动执行一次任务（忽略时间窗口与间隔）。
func (s *Scheduler) RunNow(ctx context.Context, taskID uint) (int, error) {
	var task model.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		return 0, fmt.Errorf("任务不存在")
	}
	setting := s.setting()
	return s.runTask(ctx, &task, setting.RetryTimes, true), nil
}

func (s *Scheduler) runTask(ctx context.Context, task *model.Task, retries int, manual bool) int {
	now := time.Now().In(s.loc)
	cutoff := startOfDay(now)

	var account model.Account
	if err := s.db.First(&account, task.AccountID).Error; err != nil {
		return 0
	}
	if account.Status != "online" {
		return 0 // 账号不在线时跳过，避免制造无意义的失败记录
	}

	limit := task.MaxPerRound
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	var friends []model.Friend
	err := s.db.Where("account_id = ? AND auto_keep = ? AND (last_interact_at IS NULL OR last_interact_at < ?)", task.AccountID, true, cutoff).
		Order("last_interact_at asc").Limit(limit).Find(&friends).Error
	if err != nil {
		log.Println("读取待续火好友失败:", err)
		return 0
	}

	sent := 0
	for i := range friends {
		friend := friends[i]
		onceKey := fmt.Sprintf("doyinspark:once:%d:%s", friend.ID, now.Format("20060102"))
		if !s.rdb.MarkOnce(ctx, onceKey, 40*time.Hour) {
			continue // 今天已经处理过
		}
		if manual {
			// 手动触发不受幂等限制影响，但仍记录一次尝试
			s.rdb.Del(ctx, onceKey)
		}
		for attempt := 0; attempt <= retries; attempt++ {
			result, err := s.sender.SendForFriend(ctx, task.AccountID, friend.ID, task.TemplateID, task.ID, sourceOf(manual))
			if err == nil && result != nil && result.Success {
				sent++
				break
			}
			if errors.Is(err, signer.ErrDeliveryUncertain) {
				break // 已尝试发送但回执不确定，绝不重试，避免重复消息
			}
			if attempt < retries {
				time.Sleep(time.Duration(800*(attempt+1)) * time.Millisecond)
			}
		}
	}

	updates := map[string]any{"last_run_at": now, "run_count": gorm.Expr("run_count + ?", 1)}
	s.db.Model(&model.Task{}).Where("id = ?", task.ID).Updates(updates)
	task.LastRunAt = &now
	return sent
}

func sourceOf(manual bool) string {
	if manual {
		return "manual"
	}
	return "auto"
}

func (s *Scheduler) setting() model.Setting {
	var setting model.Setting
	if err := s.db.First(&setting, 1).Error; err != nil {
		return model.Setting{ID: 1, RetryTimes: 2, QuietStart: "00:30", QuietEnd: "07:00", NotifyBreak: true}
	}
	return setting
}

// checkBreaks 断火检测：超窗且当天未提醒过的好友写入通知。
func (s *Scheduler) checkBreaks(ctx context.Context, now time.Time, setting model.Setting) {
	if !setting.NotifyBreak {
		return
	}
	deadline := now.Add(-SparkWindow)
	var friends []model.Friend
	if err := s.db.Where("auto_keep = ? AND last_interact_at IS NOT NULL AND last_interact_at < ?", true, deadline).Limit(20).Find(&friends).Error; err != nil {
		return
	}
	for i := range friends {
		friend := friends[i]
		key := fmt.Sprintf("doyinspark:break:%d:%s", friend.ID, now.Format("20060102"))
		if !s.rdb.MarkOnce(ctx, key, 26*time.Hour) {
			continue
		}
		note := model.Notification{UserID: friend.UserID, Level: "danger", Title: "火花已断 · " + friend.Name, Body: fmt.Sprintf("已超过 24 小时没有互动，连续 %d 天的火花中断。", friend.StreakDays), CreatedAt: now}
		s.db.Create(&note)
		s.bus.Publish(Event{UserID: friend.UserID, Type: "break", Level: "danger", Title: note.Title, Body: note.Body, Payload: map[string]any{"friendId": friend.ID}})
		s.notifyWebhook(setting.NotifyWebhook, note)
	}
}

func (s *Scheduler) notifyWebhook(url string, note model.Notification) {
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	payload, _ := json.Marshal(map[string]any{"title": note.Title, "body": note.Body, "level": note.Level, "at": note.CreatedAt})
	go func() {
		client := &http.Client{Timeout: 6 * time.Second}
		resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
		if err == nil {
			resp.Body.Close()
		}
	}()
}

func WithinWindow(now time.Time, start, end string) bool {
	s, ok1 := ParseClock(start)
	e, ok2 := ParseClock(end)
	if !ok1 || !ok2 {
		return true
	}
	minutes := now.Hour()*60 + now.Minute()
	if s == e {
		return true
	}
	if s < e {
		return minutes >= s && minutes < e
	}
	return minutes >= s || minutes < e // 跨零点窗口
}

func inQuiet(now time.Time, start, end string) bool {
	s, ok1 := ParseClock(start)
	e, ok2 := ParseClock(end)
	if !ok1 || !ok2 || s == e {
		return false
	}
	minutes := now.Hour()*60 + now.Minute()
	if s < e {
		return minutes >= s && minutes < e
	}
	return minutes >= s || minutes < e
}

func ParseClock(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Package service 承载火花状态机、发送通道与调度器。
package service

import (
	"fmt"
	"strings"
	"time"
)

// SparkWindow 是火花保鲜窗口：双方 24 小时内需要一次互动。
const SparkWindow = 24 * time.Hour

// 火花状态分级。
const (
	LevelSafe   = "safe"   // 安全
	LevelWarn   = "warn"   // 预警
	LevelUrgent = "urgent" // 紧急
	LevelBroken = "broken" // 已断火
)

// State 是某个好友当前的火花健康度快照。
type State struct {
	Level          string  `json:"level"`
	LevelText      string  `json:"levelText"`
	RemainingSec   int64   `json:"remainingSec"`
	Progress       float64 `json:"progress"` // 0~1，剩余窗口占比
	StreakDays     int     `json:"streakDays"`
	MaxStreakDays  int     `json:"maxStreakDays"`
	LastInteractAt *time.Time `json:"lastInteractAt"`
	Due            bool    `json:"due"` // 今天还没互动
}

// Evaluate 根据上次互动时间计算火花状态。
func Evaluate(last *time.Time, streak, maxStreak int, now time.Time) State {
	state := State{StreakDays: streak, MaxStreakDays: maxStreak, LastInteractAt: last, Due: true}
	if last == nil {
		state.Level = LevelUrgent
		state.LevelText = "待互动"
		state.Progress = 1
		return state
	}
	elapsed := now.Sub(*last)
	remaining := SparkWindow - elapsed
	state.RemainingSec = int64(remaining.Seconds())
	state.Progress = clamp01(float64(remaining) / float64(SparkWindow))
	state.Due = !sameDay(*last, now)

	switch {
	case remaining <= 0:
		state.Level, state.LevelText = LevelBroken, "已断火"
	case remaining < 3*time.Hour:
		state.Level, state.LevelText = LevelUrgent, "紧急"
	case remaining < 8*time.Hour:
		state.Level, state.LevelText = LevelWarn, "预警"
	default:
		state.Level, state.LevelText = LevelSafe, "安全"
	}
	return state
}

// NextStreak 计算一次成功互动后的连续天数。
//   - 同一天重复互动：天数不变（刷不出天数）
//   - 距上次 ≤ 24h：+1
//   - 超过 24h：从 1 重新开始
func NextStreak(last *time.Time, streak int, now time.Time) int {
	if last == nil {
		// 手动导入的好友可能带着历史天数但没有互动时间，视为今天继续累计
		if streak > 0 {
			return streak + 1
		}
		return 1
	}
	if sameDay(*last, now) {
		if streak < 1 {
			return 1
		}
		return streak
	}
	if now.Sub(*last) <= SparkWindow {
		if streak < 1 {
			return 1
		}
		return streak + 1
	}
	return 1
}

// RenderTemplate 渲染消息模板占位符。
func RenderTemplate(content, friendName string, streak int, now time.Time) string {
	weekdays := [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	replacer := map[string]string{
		"{{name}}":    friendName,
		"{{days}}":    fmt.Sprintf("%d", streak),
		"{{date}}":    now.Format("1月2日"),
		"{{time}}":    now.Format("15:04"),
		"{{weekday}}": weekdays[int(now.Weekday())],
	}
	out := content
	for key, value := range replacer {
		out = strings.ReplaceAll(out, key, value)
	}
	return out
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.In(time.Local).Date()
	by, bm, bd := b.In(time.Local).Date()
	return ay == by && am == bm && ad == bd
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

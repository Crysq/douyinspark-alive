package service

import (
	"testing"
	"time"
)

func at(t time.Time, d time.Duration) *time.Time {
	v := t.Add(-d)
	return &v
}

func TestNextStreak(t *testing.T) {
	now := time.Date(2026, 11, 1, 18, 0, 0, 0, time.Local)
	cases := []struct {
		name   string
		last   *time.Time
		streak int
		want   int
	}{
		{"首次互动", nil, 0, 1},
		{"导入历史天数后首次发送", nil, 7, 8},
		{"同一天重复发送不加天数", at(now, 2*time.Hour), 5, 5},
		{"24 小时内跨天 +1", at(now, 20*time.Hour), 5, 6},
		{"超过 24 小时归 1", at(now, 30*time.Hour), 5, 1},
	}
	for _, c := range cases {
		if got := NextStreak(c.last, c.streak, now); got != c.want {
			t.Fatalf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	now := time.Date(2026, 11, 1, 18, 0, 0, 0, time.Local)
	cases := []struct {
		name  string
		last  *time.Time
		level string
	}{
		{"从未互动", nil, LevelUrgent},
		{"安全区间", at(now, 2*time.Hour), LevelSafe},
		{"预警区间", at(now, 18*time.Hour), LevelWarn},
		{"紧急区间", at(now, 22*time.Hour), LevelUrgent},
		{"已断火", at(now, 25*time.Hour), LevelBroken},
	}
	for _, c := range cases {
		state := Evaluate(c.last, 3, 9, now)
		if state.Level != c.level {
			t.Fatalf("%s: got %s want %s", c.name, state.Level, c.level)
		}
	}
}

func TestRenderTemplate(t *testing.T) {
	now := time.Date(2026, 11, 1, 18, 5, 0, 0, time.Local)
	got := RenderTemplate("{{name}} 火花 {{days}} 天 · {{date}} {{time}} {{weekday}}", "橘子", 12, now)
	want := "橘子 火花 12 天 · 11月1日 18:05 周日"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

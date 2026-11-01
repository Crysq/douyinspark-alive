package service

import (
	"strings"
	"testing"
	"time"
)

func TestExtendAndDaysLeft(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.Local)
	day, _ := PlanOf("day")
	month, _ := PlanOf("month")

	// 未过期：从原到期时间续
	future := now.AddDate(0, 0, 5)
	next := Extend(&future, day, now)
	if got := DaysLeft(next, now); got != 6 {
		t.Fatalf("续期后应有 6 天，得到 %d", got)
	}

	// 已过期：从现在算
	past := now.AddDate(0, 0, -3)
	next = Extend(&past, month, now)
	if got := DaysLeft(next, now); got != 30 {
		t.Fatalf("过期后续月卡应为 30 天，得到 %d", got)
	}

	// 永久卡
	forever, _ := PlanOf("forever")
	next = Extend(nil, forever, now)
	if got := DaysLeft(next, now); got != -1 {
		t.Fatalf("永久卡应为 -1，得到 %d", got)
	}
	if !IsMember(next, now) || IsMember(&past, now) {
		t.Fatal("会员有效性判断错误")
	}
}

func TestCardCode(t *testing.T) {
	code, err := NewCardCode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, "DYSP-") || len(code) != 19 {
		t.Fatalf("卡密格式错误: %s", code)
	}
	if strings.ContainsAny(code, "IO01") {
		t.Fatalf("卡密不应包含易混字符: %s", code)
	}
	if got := NormalizeCardCode(strings.ToLower(strings.ReplaceAll(code, "-", ""))); got != code {
		t.Fatalf("规范化失败: %s -> %s", code, got)
	}
}

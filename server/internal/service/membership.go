package service

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"time"
)

// Plan 会员卡种定义。
type Plan struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Days  int    `json:"days"`
	Price int    `json:"price"` // 参考售价（元），仅用于展示
	Desc  string `json:"desc"`
}

// Plans 全部卡种，顺序即前端展示顺序。
var Plans = []Plan{
	{Code: "day", Name: "日卡", Days: 1, Price: 2, Desc: "1 天体验，适合先试试"},
	{Code: "week", Name: "周卡", Days: 7, Price: 9, Desc: "7 天，短期托管"},
	{Code: "month", Name: "月卡", Days: 30, Price: 29, Desc: "30 天，最常用"},
	{Code: "quarter", Name: "季卡", Days: 90, Price: 69, Desc: "90 天，性价比高"},
	{Code: "year", Name: "年卡", Days: 365, Price: 199, Desc: "365 天，一次到位"},
	{Code: "forever", Name: "永久卡", Days: 0, Price: 399, Desc: "永久有效，含后续更新"},
}

// Quota 套餐配额：0 表示不限量。
type Quota struct {
	MaxAccounts int `json:"maxAccounts"` // 托管抖音账号上限
	MaxFriends  int `json:"maxFriends"`  // 火花好友上限
	MaxTasks    int `json:"maxTasks"`    // 定时任务上限
	MaxPerDay   int `json:"maxPerDay"`   // 每日续火发送上限
}

// Quotas 各套餐配额，与 Plans 的卡种一一对应。
var Quotas = map[string]Quota{
	"day":     {MaxAccounts: 1, MaxFriends: 3, MaxTasks: 1, MaxPerDay: 10},
	"week":    {MaxAccounts: 2, MaxFriends: 10, MaxTasks: 2, MaxPerDay: 50},
	"month":   {MaxAccounts: 3, MaxFriends: 30, MaxTasks: 5, MaxPerDay: 200},
	"quarter": {MaxAccounts: 5, MaxFriends: 100, MaxTasks: 10, MaxPerDay: 500},
	"year":    {MaxAccounts: 10, MaxFriends: 300, MaxTasks: 30, MaxPerDay: 2000},
	"forever": {MaxAccounts: 0, MaxFriends: 0, MaxTasks: 0, MaxPerDay: 0},
}

// QuotaOf 返回套餐配额，未知套餐按日卡兜底（最小权限）。
func QuotaOf(plan string) Quota {
	if quota, ok := Quotas[plan]; ok {
		return quota
	}
	return Quotas["day"]
}

// ForeverYears 永久卡的到期时间（实际等于永久）。
const ForeverYears = 100

// PlanOf 按 code 查卡种。
func PlanOf(code string) (Plan, bool) {
	for _, plan := range Plans {
		if plan.Code == code {
			return plan, true
		}
	}
	return Plan{}, false
}

// Extend 在现有会员基础上叠加时长，返回新的到期时间与卡种。
// 未过期则从原到期时间续，已过期则从现在开始算。
func Extend(current *time.Time, plan Plan, now time.Time) *time.Time {
	if plan.Code == "forever" {
		forever := now.AddDate(ForeverYears, 0, 0)
		return &forever
	}
	base := now
	if current != nil && current.After(now) {
		base = *current
	}
	next := base.AddDate(0, 0, plan.Days)
	return &next
}

// DaysLeft 剩余天数（向上取整），永久卡返回 -1。
func DaysLeft(expire *time.Time, now time.Time) int {
	if expire == nil {
		return 0
	}
	if expire.Year() >= now.Year()+ForeverYears-1 {
		return -1
	}
	diff := expire.Sub(now)
	if diff <= 0 {
		return 0
	}
	return int((diff + 24*time.Hour - time.Second) / (24 * time.Hour))
}

// IsMember 会员是否有效（永久卡或未过期）。
func IsMember(expire *time.Time, now time.Time) bool {
	if expire == nil {
		return false
	}
	return expire.After(now)
}

var codeEncoding = base32.NewEncoding("ABCDEFGHJKLMNPQRSTUVWXYZ23456789").WithPadding(base32.NoPadding)

// NewCardCode 生成易读卡密：DYSP-XXXX-XXXX-XXXX（去掉 I/O/0/1）。
func NewCardCode() (string, error) {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	raw := codeEncoding.EncodeToString(buf)[:12]
	parts := []string{raw[0:4], raw[4:8], raw[8:12]}
	return "DYSP-" + strings.Join(parts, "-"), nil
}

// NormalizeCardCode 统一大小写与分隔符，容错用户输入（粘贴空格、缺少横线都能识别）。
func NormalizeCardCode(input string) string {
	clean := strings.ToUpper(strings.TrimSpace(input))
	clean = strings.ReplaceAll(clean, " ", "")
	clean = strings.ReplaceAll(clean, "—", "-")
	compact := strings.ReplaceAll(clean, "-", "")
	compact = strings.TrimPrefix(compact, "DYSP")
	if len(compact) == 12 {
		return fmt.Sprintf("DYSP-%s-%s-%s", compact[0:4], compact[4:8], compact[8:12])
	}
	return clean
}

// PlanLabel 卡种中文名。
func PlanLabel(code string) string {
	if plan, ok := PlanOf(code); ok {
		return plan.Name
	}
	if code == "" || code == "none" {
		return "未激活"
	}
	return code
}

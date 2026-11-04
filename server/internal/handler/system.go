package handler

import (
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"

	"doyinspark/internal/model"
	"doyinspark/internal/service"
	"doyinspark/internal/util"
)

var startedAt = time.Now()

// Health 依赖健康检查。
func (h *Handler) Health(c *gin.Context) {
	mysqlOK := "down"
	if sqlDB, err := h.DB.DB(); err == nil && sqlDB.Ping() == nil {
		mysqlOK = "up"
	}
	redisOK := "down"
	if err := h.Cache.Ping(c.Request.Context()); err == nil {
		redisOK = "up"
	}
	signState := "offline"
	if h.Sign.Configured() {
		if err := h.Sign.Health(); err == nil {
			signState = "ready"
		}
	}
	status := "ok"
	if mysqlOK != "up" || redisOK != "up" {
		status = "degraded"
	}
	util.OK(c, gin.H{
		"status": status, "mysql": mysqlOK, "redis": redisOK, "sign": signState,
		"version": "2.0.0", "uptimeSec": int(time.Since(startedAt).Seconds()),
		"now": time.Now().In(h.Cfg.Location),
	})
}

// SeedDemo 为当前用户生成演示数据（标记为演示，可一键清除）。
func (h *Handler) SeedDemo(c *gin.Context) {
	userID := uid(c)
	var count int64
	h.DB.Model(&model.Account{}).Where("user_id = ? AND remark = ?", userID, "演示数据").Count(&count)
	if count > 0 {
		util.Fail(c, 400, "演示数据已存在，可先清除再重新生成")
		return
	}
	loc := h.Cfg.Location
	now := time.Now().In(loc)

	accounts := []model.Account{
		{UserID: userID, Nickname: "小满", UniqueID: "xiaoman_01", Status: "offline", Remark: "演示数据"},
		{UserID: userID, Nickname: "阿黎", UniqueID: "ali_night", Status: "offline", Remark: "演示数据"},
	}
	h.DB.Create(&accounts)

	names := []string{"橘子汽水", "一颗小行星", "晚安电台", "芝士就是力量", "雾岛听风", "阿柚", "西瓜啵啵", "北岛不冷", "云朵收藏家", "晚风急"}
	streaks := []int{128, 96, 77, 65, 42, 33, 21, 14, 9, 3}
	var friends []model.Friend
	for i, name := range names {
		account := accounts[0]
		if i%4 == 3 {
			account = accounts[1]
		}
		offset := time.Duration(0)
		switch i % 5 {
		case 0:
			offset = 30 * time.Minute
		case 1:
			offset = 5 * time.Hour
		case 2:
			offset = 21 * time.Hour
		case 3:
			offset = 26 * time.Hour
		case 4:
			offset = 9 * time.Hour
		}
		last := now.Add(-offset)
		friends = append(friends, model.Friend{
			UserID: userID, AccountID: account.ID, Name: name,
			DouyinUserID: "1000" + string(rune('0'+i)),
			UniqueID:     "dy_" + name, StreakDays: streaks[i], MaxStreakDays: streaks[i] + 12,
			LastInteractAt: &last, AutoKeep: i == 0,
		})
	}
	h.DB.Create(&friends)
	for i := 1; i < len(friends); i++ {
		h.DB.Model(&model.Friend{}).Where("id = ?", friends[i].ID).Update("auto_keep", false)
	}

	var templates []model.Template
	h.DB.Where("user_id = ?", userID).Limit(3).Find(&templates)
	tplID := uint(0)
	if len(templates) > 0 {
		tplID = templates[0].ID
	}
	tasks := []model.Task{
		{UserID: userID, AccountID: accounts[0].ID, Name: "每日续火 · 小满", TemplateID: tplID, WindowStart: "08:00", WindowEnd: "23:30", IntervalMin: 90, MaxPerRound: 5, Enabled: true},
		{UserID: userID, AccountID: accounts[1].ID, Name: "夜间补火 · 阿黎", TemplateID: tplID, WindowStart: "19:00", WindowEnd: "23:59", IntervalMin: 60, MaxPerRound: 3},
	}
	h.DB.Create(&tasks)
	if len(tasks) > 1 {
		h.DB.Model(&model.Task{}).Where("id = ?", tasks[1].ID).Update("enabled", false)
	}

	contents := []string{"早呀～今天也要开开心心", "晚安啦，火花不能断 🔥", "在忙吗？来续个火花", "今天也要记得回我一句哦"}
	var records []model.Record
	for day := 13; day >= 0; day-- {
		perDay := 3 + rand.Intn(6)
		for i := 0; i < perDay; i++ {
			friend := friends[rand.Intn(len(friends))]
			status := "success"
			errText := ""
			if rand.Intn(100) < 7 {
				status = "failed"
				errText = "私信接口返回 403: 风控校验未通过"
			}
			at := now.AddDate(0, 0, -day).Add(-time.Duration(rand.Intn(10)) * time.Hour).Add(-time.Duration(rand.Intn(60)) * time.Minute)
			records = append(records, model.Record{
				UserID: userID, AccountID: friend.AccountID, FriendID: friend.ID, FriendName: friend.Name,
				TaskID: tasks[0].ID, TemplateID: tplID, Content: contents[rand.Intn(len(contents))],
				Status: status, Source: "auto", Error: errText, LatencyMs: 120 + rand.Intn(600), CreatedAt: at,
			})
		}
	}
	h.DB.CreateInBatches(&records, 100)

	h.Cache.Del(c.Request.Context(), "doyinspark:overview:"+itoa(int(userID)))
	h.audit(c, "system.demo", "demo", "生成演示数据")
	util.OKMsg(c, "演示数据已生成", gin.H{"accounts": len(accounts), "friends": len(friends), "records": len(records)})
}

// ClearDemo 清除当前用户的演示数据。
func (h *Handler) ClearDemo(c *gin.Context) {
	userID := uid(c)
	var accounts []model.Account
	h.DB.Where("user_id = ? AND remark = ?", userID, "演示数据").Find(&accounts)
	ids := make([]uint, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.ID)
	}
	if len(ids) > 0 {
		h.DB.Where("account_id IN ?", ids).Delete(&model.Friend{})
		h.DB.Where("account_id IN ?", ids).Delete(&model.Task{})
		h.DB.Where("account_id IN ?", ids).Delete(&model.Record{})
		h.DB.Where("id IN ?", ids).Delete(&model.Account{})
	}
	h.Cache.Del(c.Request.Context(), "doyinspark:overview:"+itoa(int(userID)))
	util.OKMsg(c, "演示数据已清除", nil)
}

// SparkRules 火花规则说明。
func (h *Handler) SparkRules(c *gin.Context) {
	util.OK(c, gin.H{
		"windowHours": 24,
		"levels": []gin.H{
			{"level": service.LevelSafe, "text": "安全", "rule": "剩余 > 8 小时"},
			{"level": service.LevelWarn, "text": "预警", "rule": "剩余 3 ~ 8 小时"},
			{"level": service.LevelUrgent, "text": "紧急", "rule": "剩余 < 3 小时或从未互动"},
			{"level": service.LevelBroken, "text": "已断火", "rule": "超过 24 小时未互动"},
		},
		"streak": []string{"同一天重复互动不增加天数", "距上次 ≤ 24 小时 +1 天", "超过 24 小时从 1 重新开始"},
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

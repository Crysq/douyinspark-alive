package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"doyinspark/internal/model"
	"doyinspark/internal/service"
	"doyinspark/internal/util"
)

// TaskView 任务列表项。
type TaskView struct {
	model.Task
	AccountName  string     `json:"accountName"`
	TemplateName string     `json:"templateName"`
	NextRunAt    *time.Time `json:"nextRunAt"`
	InWindow     bool       `json:"inWindow"`
}

// Tasks 任务列表。
func (h *Handler) Tasks(c *gin.Context) {
	userID := uid(c)
	var tasks []model.Task
	h.DB.Where("user_id = ?", userID).Order("id asc").Find(&tasks)

	var accounts []model.Account
	h.DB.Select("id", "nickname").Where("user_id = ?", userID).Find(&accounts)
	names := map[uint]string{}
	for _, a := range accounts {
		names[a.ID] = a.Nickname
	}
	var templates []model.Template
	h.DB.Select("id", "name").Where("user_id = ?", userID).Find(&templates)
	tplNames := map[uint]string{}
	for _, t := range templates {
		tplNames[t.ID] = t.Name
	}

	now := time.Now().In(h.Cfg.Location)
	out := make([]TaskView, 0, len(tasks))
	for _, task := range tasks {
		view := TaskView{Task: task, AccountName: names[task.AccountID], TemplateName: tplNames[task.TemplateID]}
		view.NextRunAt = nextRun(task, now)
		view.InWindow = service.WithinWindow(now, task.WindowStart, task.WindowEnd)
		out = append(out, view)
	}
	util.OK(c, out)
}

type taskReq struct {
	AccountID   uint   `json:"accountId"`
	Name        string `json:"name"`
	TemplateID  uint   `json:"templateId"`
	WindowStart string `json:"windowStart"`
	WindowEnd   string `json:"windowEnd"`
	IntervalMin int    `json:"intervalMin"`
	MaxPerRound int    `json:"maxPerRound"`
	Enabled     *bool  `json:"enabled"`
}

// CreateTask 新建定时任务。
func (h *Handler) CreateTask(c *gin.Context) {
	var req taskReq
	if err := c.ShouldBindJSON(&req); err != nil || req.AccountID == 0 {
		util.Fail(c, 400, "请选择账号")
		return
	}
	var account model.Account
	if err := h.DB.Where("id = ? AND user_id = ?", req.AccountID, uid(c)).First(&account).Error; err != nil {
		util.Fail(c, 404, "账号不存在")
		return
	}
	ent, err := h.entitlement(c)
	if err != nil {
		util.Fail(c, 401, "登录态已失效")
		return
	}
	if err := ent.CheckLimit("tasks", ent.Usage.Tasks); err != nil {
		util.Fail(c, 403, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	task := model.Task{
		UserID: uid(c), AccountID: account.ID, Name: req.Name, TemplateID: req.TemplateID,
		WindowStart: def(req.WindowStart, "08:00"), WindowEnd: def(req.WindowEnd, "23:00"),
		IntervalMin: defInt(req.IntervalMin, 120), MaxPerRound: defInt(req.MaxPerRound, 5), Enabled: enabled,
	}
	if task.Name == "" {
		task.Name = account.Nickname + " 的续火任务"
	}
	if err := h.DB.Create(&task).Error; err != nil {
		util.Fail(c, 500, "创建失败")
		return
	}
	if !enabled {
		h.DB.Model(&task).Update("enabled", false)
	}
	h.audit(c, "task.create", task.Name, "新建定时任务")
	util.OK(c, task)
}

// UpdateTask 更新任务。
func (h *Handler) UpdateTask(c *gin.Context) {
	var req taskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.AccountID != 0 {
		var account model.Account
		if err := h.DB.Where("id = ? AND user_id = ?", req.AccountID, uid(c)).First(&account).Error; err != nil {
			util.Fail(c, 404, "账号不存在")
			return
		}
		updates["account_id"] = req.AccountID
	}
	if req.TemplateID != 0 {
		updates["template_id"] = req.TemplateID
	}
	if req.WindowStart != "" {
		updates["window_start"] = req.WindowStart
	}
	if req.WindowEnd != "" {
		updates["window_end"] = req.WindowEnd
	}
	if req.IntervalMin > 0 {
		updates["interval_min"] = req.IntervalMin
	}
	if req.MaxPerRound > 0 {
		updates["max_per_round"] = req.MaxPerRound
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	res := h.DB.Model(&model.Task{}).Where("id = ? AND user_id = ?", c.Param("id"), uid(c)).Updates(updates)
	if res.RowsAffected == 0 {
		util.Fail(c, 404, "任务不存在")
		return
	}
	util.OKMsg(c, "已保存", nil)
}

// DeleteTask 删除任务。
func (h *Handler) DeleteTask(c *gin.Context) {
	res := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), uid(c)).Delete(&model.Task{})
	if res.RowsAffected == 0 {
		util.Fail(c, 404, "任务不存在")
		return
	}
	util.OKMsg(c, "已删除", nil)
}

// RunTask 立即执行一次任务。
func (h *Handler) RunTask(c *gin.Context) {
	var task model.Task
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), uid(c)).First(&task).Error; err != nil {
		util.Fail(c, 404, "任务不存在")
		return
	}
	sent, err := h.Sched.RunNow(c.Request.Context(), task.ID)
	if err != nil {
		util.Fail(c, 400, err.Error())
		return
	}
	h.audit(c, "task.run", task.Name, "手动执行任务")
	util.OK(c, gin.H{"sent": sent})
}

func nextRun(task model.Task, now time.Time) *time.Time {
	if !task.Enabled {
		return nil
	}
	start, ok1 := service.ParseClock(task.WindowStart)
	end, ok2 := service.ParseClock(task.WindowEnd)
	if !ok1 || !ok2 {
		return nil
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startAt := day.Add(time.Duration(start) * time.Minute)
	endAt := day.Add(time.Duration(end) * time.Minute)
	interval := time.Duration(task.IntervalMin) * time.Minute
	if interval <= 0 {
		interval = 2 * time.Hour
	}

	candidate := startAt
	if task.LastRunAt != nil {
		candidate = task.LastRunAt.In(now.Location()).Add(interval)
	}
	if now.Before(startAt) {
		candidate = startAt
	}
	if now.After(endAt) || candidate.After(endAt) {
		candidate = startAt.AddDate(0, 0, 1)
	}
	if candidate.Before(now) {
		candidate = now
	}
	return &candidate
}

func def(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

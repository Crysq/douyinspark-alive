// Package handler 提供 HTTP 接口层。
package handler

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"doyinspark/internal/cache"
	"doyinspark/internal/config"
	"doyinspark/internal/middleware"
	"doyinspark/internal/model"
	"doyinspark/internal/service"
	"doyinspark/internal/signer"
	"doyinspark/internal/util"
)

// Deps 是接口层依赖集合。
type Deps struct {
	DB     *gorm.DB
	Cache  *cache.Cache
	Sign   *signer.Client
	Sender *service.Sender
	Sched  *service.Scheduler
	Bus    *service.EventBus
	Cfg    *config.Config
}

// Handler 聚合所有接口。
type Handler struct {
	Deps
}

// New 构造接口层。
func New(d Deps) *Handler { return &Handler{Deps: d} }

// UserView 用户对外视图，带会员信息。
type UserView struct {
	model.User
	DaysLeft  int    `json:"daysLeft"`
	PlanLabel string `json:"planLabel"`
	IsMember  bool   `json:"isMember"`
}

// View 组装用户视图。
func (h *Handler) View(user *model.User) UserView {
	now := time.Now().In(h.Cfg.Location)
	return UserView{
		User:      *user,
		DaysLeft:  service.DaysLeft(user.MemberExpireAt, now),
		PlanLabel: service.PlanLabel(user.Plan),
		IsMember:  service.IsMember(user.MemberExpireAt, now),
	}
}

func uid(c *gin.Context) uint { return c.GetUint(middleware.CtxUserID) }

func pageParams(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}

func (h *Handler) audit(c *gin.Context, action, target, detail string) {
	entry := model.AuditLog{UserID: uid(c), Action: action, Target: target, Detail: detail, IP: c.ClientIP(), CreatedAt: time.Now()}
	h.DB.Create(&entry)
}

func (h *Handler) currentUser(c *gin.Context) (*model.User, bool) {
	var user model.User
	if err := h.DB.First(&user, uid(c)).Error; err != nil {
		util.Fail(c, 401, "登录态已失效")
		return nil, false
	}
	return &user, true
}

// entitlement 读取当前用户的套餐权限与用量。
func (h *Handler) entitlement(c *gin.Context) (*service.Entitlement, error) {
	user, ok := h.currentUser(c)
	if !ok {
		return nil, fmt.Errorf("用户不存在")
	}
	return service.EntitlementOf(h.DB, user, time.Now().In(h.Cfg.Location)), nil
}

// setting 读取当前用户设置，不存在则创建默认值。
func (h *Handler) setting(userID uint) model.Setting {
	var setting model.Setting
	if err := h.DB.Where("user_id = ?", userID).First(&setting).Error; err == nil {
		return setting
	}
	setting = model.Setting{UserID: userID, RetryTimes: 2, QuietStart: "00:30", QuietEnd: "07:00", NotifyBreak: true, AutoSync: true, KeepAlive: true, Theme: "dark"}
	h.DB.Create(&setting)
	return setting
}

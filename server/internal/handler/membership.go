package handler

import (
	"errors"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"doyinspark/internal/model"
	"doyinspark/internal/service"
	"doyinspark/internal/util"
)

// errInvalidCard 兑换失败（对用户统一文案，内部保留原因）。
var errInvalidCard = errors.New("invalid card")

// maskCode 日志与审计里脱敏卡密：只保留前后各 4 位。
func maskCode(code string) string {
	if len(code) <= 10 {
		return "****"
	}
	return code[:9] + "****" + code[len(code)-4:]
}

// Plans 公开卡种列表（登录页 / 注册页要用）。
func (h *Handler) Plans(c *gin.Context) {
	util.OK(c, gin.H{"plans": service.Plans, "windowHours": 24})
}

// Membership 当前会员信息 + 兑换流水。
func (h *Handler) Membership(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	var history []model.MembershipLog
	h.DB.Where("user_id = ?", user.ID).Order("created_at desc").Limit(20).Find(&history)
	now := time.Now().In(h.Cfg.Location)
	ent := service.EntitlementOf(h.DB, user, now)
	util.OK(c, gin.H{
		"user":    h.View(user),
		"plans":   service.Plans,
		"history": history,
		"now":     now,
		"quota":   ent.Quota,
		"usage":   ent.Usage,
	})
}

// Redeem 兑换卡密：待激活令牌与完整令牌都可以调用。
func (h *Handler) Redeem(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "请输入卡密")
		return
	}
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	if user.Status == "banned" {
		util.Fail(c, 403, "账号已被禁用，请联系客服")
		return
	}
	code := service.NormalizeCardCode(req.Code)

	var card model.CardKey
	var plan service.Plan
	var before, after *time.Time
	reason := ""
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("code = ?", code).First(&card).Error; err != nil {
			reason = "卡密不存在"
			return errInvalidCard
		}
		if card.Status == "used" {
			reason = "卡密已被使用"
			return errInvalidCard
		}
		if card.Status == "void" {
			reason = "卡密已作废"
			return errInvalidCard
		}
		now := time.Now()
		if card.ExpireAt != nil && card.ExpireAt.Before(now) {
			reason = "卡密已过期"
			return errInvalidCard
		}
		var found bool
		plan, found = service.PlanOf(card.Plan)
		if !found {
			reason = "卡种异常"
			return errInvalidCard
		}
		before = user.MemberExpireAt
		after = service.Extend(user.MemberExpireAt, plan, now)

		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"plan": plan.Code, "member_expire_at": after, "status": "active",
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CardKey{}).Where("id = ? AND status = ?", card.ID, "unused").Updates(map[string]any{
			"status": "used", "used_by": user.ID, "used_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.MembershipLog{
			UserID: user.ID, Action: "redeem", Plan: plan.Code, Days: plan.Days,
			CardCode: card.Code, BeforeExpire: before, AfterExpire: after,
			Remark: plan.Name, CreatedAt: now,
		}).Error
	})
	if err != nil {
		// 统一文案，避免用不同报错探测卡密是否真实存在
		h.audit(c, "membership.redeem.failed", maskCode(code), "兑换失败："+reason)
		log.Printf("兑换失败 user=%d code=%s reason=%s ip=%s", user.ID, maskCode(code), reason, c.ClientIP())
		util.Fail(c, 400, "卡密无效或已被使用，请检查后重试")
		return
	}

	var fresh model.User
	if err := h.DB.First(&fresh, user.ID).Error; err != nil {
		util.Fail(c, 500, "读取会员信息失败")
		return
	}
	token, expires, err := h.issue(c, &fresh, util.ScopeFull, h.Cfg.TokenTTL)
	if err != nil {
		util.Fail(c, 500, "令牌签发失败")
		return
	}
	h.audit(c, "membership.redeem", plan.Name, "兑换卡密 "+card.Code)
	util.OKMsg(c, plan.Name+" 兑换成功", gin.H{
		"token": token, "expiresAt": expires, "user": h.View(&fresh),
		"card": gin.H{"code": card.Code, "plan": plan.Code, "name": plan.Name, "days": plan.Days},
	})
}

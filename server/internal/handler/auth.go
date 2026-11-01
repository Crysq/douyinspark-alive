package handler

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"doyinspark/internal/model"
	"doyinspark/internal/service"
	"doyinspark/internal/store"
	"doyinspark/internal/util"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

type authPayload struct {
	Token         string   `json:"token"`
	ExpiresAt     time.Time `json:"expiresAt"`
	User          UserView `json:"user"`
	NeedActivation bool    `json:"needActivation,omitempty"`
	ActivationToken string `json:"activationToken,omitempty"`
}

// Login 登录：会员无效时只发「待激活」令牌，必须先兑换卡密。
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "请输入用户名和密码")
		return
	}
	var user model.User
	if err := h.DB.Where("username = ?", strings.ToLower(strings.TrimSpace(req.Username))).First(&user).Error; err != nil {
		util.Fail(c, 401, "用户名或密码错误")
		return
	}
	if !util.CheckPassword(user.PasswordHash, req.Password) {
		util.Fail(c, 401, "用户名或密码错误")
		return
	}
	if user.Status == "banned" {
		util.Fail(c, 403, "账号已被禁用，请联系客服")
		return
	}

	now := time.Now().In(h.Cfg.Location)
	h.DB.Model(&user).Update("last_login_at", now)

	if !service.IsMember(user.MemberExpireAt, now) {
		token, expires, err := util.SignToken(h.Cfg.JWTSecret, user.ID, user.Role, util.ScopeActivate, 2*time.Hour)
		if err != nil {
			util.Fail(c, 500, "令牌签发失败")
			return
		}
		c.AbortWithStatusJSON(403, util.Body{Code: 403, Message: "会员未激活或已过期，请先兑换卡密", Data: authPayload{
			Token: token, ExpiresAt: expires, User: h.View(&user), NeedActivation: true, ActivationToken: token,
		}})
		return
	}

	token, expires, err := h.issue(c, &user, util.ScopeFull, h.Cfg.TokenTTL)
	if err != nil {
		util.Fail(c, 500, "令牌签发失败")
		return
	}
	h.audit(c, "auth.login", user.Username, "登录控制台")
	util.OK(c, authPayload{Token: token, ExpiresAt: expires, User: h.View(&user)})
}

// Register 注册：必须携带未使用的卡密，兑换成功后才能登录。
func (h *Handler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Nickname string `json:"nickname"`
		CardCode string `json:"cardCode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "请填写用户名、密码（至少 6 位）与卡密")
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if !usernamePattern.MatchString(username) {
		util.Fail(c, 400, "用户名需为 3-32 位字母、数字或下划线")
		return
	}
	code := service.NormalizeCardCode(req.CardCode)
	if len(code) < 12 {
		util.Fail(c, 400, "卡密格式不正确")
		return
	}

	var created model.User
	var card model.CardKey
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&model.User{}).Where("username = ?", username).Count(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			return errors.New("用户名已被占用")
		}
		if err := tx.Where("code = ?", code).First(&card).Error; err != nil {
			return errors.New("卡密不存在")
		}
		if card.Status != "unused" {
			return errors.New("卡密已被使用或已作废")
		}
		now := time.Now()
		if card.ExpireAt != nil && card.ExpireAt.Before(now) {
			return errors.New("卡密已过期")
		}
		plan, ok := service.PlanOf(card.Plan)
		if !ok {
			return errors.New("卡密卡种异常")
		}
		hash, err := util.HashPassword(req.Password)
		if err != nil {
			return err
		}
		nickname := strings.TrimSpace(req.Nickname)
		if nickname == "" {
			nickname = username
		}
		created = model.User{
			Username: username, PasswordHash: hash, Nickname: nickname,
			Role: "user", Status: "active", Plan: plan.Code,
			MemberExpireAt: service.Extend(nil, plan, now),
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CardKey{}).Where("id = ? AND status = ?", card.ID, "unused").Updates(map[string]any{
			"status": "used", "used_by": created.ID, "used_at": now,
		}).Error; err != nil {
			return err
		}
		log := model.MembershipLog{
			UserID: created.ID, Action: "register", Plan: plan.Code, Days: plan.Days,
			CardCode: card.Code, AfterExpire: created.MemberExpireAt, Remark: "注册兑换",
			CreatedAt: now,
		}
		return tx.Create(&log).Error
	})
	if err != nil {
		util.Fail(c, 400, err.Error())
		return
	}

	if err := store.SeedUserDefaults(h.DB, created.ID); err != nil {
		// 模板复制失败不影响注册，记录日志即可
		h.DB.Create(&model.AuditLog{UserID: created.ID, Action: "user.seed.failed", Detail: err.Error(), CreatedAt: time.Now()})
	}

	token, expires, err := h.issue(c, &created, util.ScopeFull, h.Cfg.TokenTTL)
	if err != nil {
		util.Fail(c, 500, "令牌签发失败")
		return
	}
	h.audit(c, "auth.register", created.Username, "注册并兑换 "+card.Code)
	util.OKMsg(c, "注册成功，会员已开通", authPayload{Token: token, ExpiresAt: expires, User: h.View(&created)})
}

func (h *Handler) issue(c *gin.Context, user *model.User, scope string, ttl time.Duration) (string, time.Time, error) {
	token, expires, err := util.SignToken(h.Cfg.JWTSecret, user.ID, user.Role, scope, ttl)
	if err == nil {
		h.Cache.Touch(c.Request.Context(), "doyinspark:user:"+user.Username)
	}
	return token, expires, err
}

// Me 当前登录用户（含会员信息）。
func (h *Handler) Me(c *gin.Context) {
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	util.OK(c, h.View(user))
}

// Logout 登出。
func (h *Handler) Logout(c *gin.Context) {
	if jti := c.GetString("jti"); jti != "" {
		h.Cache.Blacklist(c.Request.Context(), jti, h.Cfg.TokenTTL)
	}
	util.OKMsg(c, "已退出登录", nil)
}

// ChangePassword 修改密码。
func (h *Handler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "新密码至少 6 位")
		return
	}
	user, ok := h.currentUser(c)
	if !ok {
		return
	}
	if !util.CheckPassword(user.PasswordHash, req.OldPassword) {
		util.Fail(c, 400, "原密码不正确")
		return
	}
	hash, err := util.HashPassword(req.NewPassword)
	if err != nil {
		util.Fail(c, 500, "密码加密失败")
		return
	}
	h.DB.Model(user).Update("password_hash", hash)
	h.audit(c, "auth.password", user.Username, "修改登录密码")
	util.OKMsg(c, "密码已更新", nil)
}

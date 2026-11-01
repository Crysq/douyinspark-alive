package handler

import (
	"github.com/gin-gonic/gin"

	"doyinspark/internal/model"
	"doyinspark/internal/util"
)

// Settings 读取当前用户设置。
func (h *Handler) Settings(c *gin.Context) {
	util.OK(c, h.setting(uid(c)))
}

// UpdateSettings 保存当前用户设置。
func (h *Handler) UpdateSettings(c *gin.Context) {
	var req model.Setting
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "参数错误")
		return
	}
	if req.RetryTimes < 0 || req.RetryTimes > 5 {
		req.RetryTimes = 2
	}
	if req.QuietStart == "" {
		req.QuietStart = "00:30"
	}
	if req.QuietEnd == "" {
		req.QuietEnd = "07:00"
	}
	userID := uid(c)
	current := h.setting(userID)
	req.ID = current.ID
	req.UserID = userID
	if err := h.DB.Save(&req).Error; err != nil {
		util.Fail(c, 500, "保存失败")
		return
	}
	h.audit(c, "setting.update", "settings", "更新系统设置")
	util.OKMsg(c, "设置已保存", req)
}

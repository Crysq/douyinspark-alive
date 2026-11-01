package handler

import (
	"github.com/gin-gonic/gin"

	"doyinspark/internal/model"
	"doyinspark/internal/util"
)

// Templates 当前用户的模板。
func (h *Handler) Templates(c *gin.Context) {
	var items []model.Template
	query := h.DB.Model(&model.Template{}).Where("user_id = ?", uid(c))
	if category := c.Query("category"); category != "" && category != "all" {
		query = query.Where("category = ?", category)
	}
	query.Order("use_count desc, id asc").Find(&items)
	util.OK(c, items)
}

type templateReq struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Content  string `json:"content"`
	Enabled  *bool  `json:"enabled"`
}

// CreateTemplate 新建模板。
func (h *Handler) CreateTemplate(c *gin.Context) {
	var req templateReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.Content == "" {
		util.Fail(c, 400, "请填写模板名称与内容")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	tpl := model.Template{UserID: uid(c), Name: req.Name, Category: def(req.Category, "daily"), Content: req.Content, Enabled: enabled}
	if err := h.DB.Create(&tpl).Error; err != nil {
		util.Fail(c, 500, "创建失败")
		return
	}
	if !enabled {
		h.DB.Model(&tpl).Update("enabled", false)
	}
	h.audit(c, "template.create", tpl.Name, "新建模板")
	util.OK(c, tpl)
}

// UpdateTemplate 更新模板。
func (h *Handler) UpdateTemplate(c *gin.Context) {
	var req templateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	res := h.DB.Model(&model.Template{}).Where("id = ? AND user_id = ?", c.Param("id"), uid(c)).Updates(updates)
	if res.RowsAffected == 0 {
		util.Fail(c, 404, "模板不存在")
		return
	}
	util.OKMsg(c, "已保存", nil)
}

// DeleteTemplate 删除模板。
func (h *Handler) DeleteTemplate(c *gin.Context) {
	res := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), uid(c)).Delete(&model.Template{})
	if res.RowsAffected == 0 {
		util.Fail(c, 404, "模板不存在")
		return
	}
	util.OKMsg(c, "已删除", nil)
}

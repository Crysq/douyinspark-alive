package handler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"doyinspark/internal/model"
	"doyinspark/internal/service"
	"doyinspark/internal/util"
)

// FriendView 好友列表项，附带实时火花状态。
type FriendView struct {
	model.Friend
	State       service.State `json:"state"`
	AccountName string        `json:"accountName"`
}

// Friends 好友列表。
func (h *Handler) Friends(c *gin.Context) {
	userID := uid(c)
	page, size := pageParams(atoi(c.DefaultQuery("page", "1")), atoi(c.DefaultQuery("pageSize", "12")))
	query := h.DB.Model(&model.Friend{}).Where("user_id = ?", userID)
	if accountID := c.Query("accountId"); accountID != "" && accountID != "0" {
		query = query.Where("account_id = ?", accountID)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR unique_id LIKE ? OR remark LIKE ?", like, like, like)
	}
	if level := c.Query("level"); level != "" && level != "all" {
		now := time.Now().In(h.Cfg.Location)
		switch level {
		case service.LevelBroken:
			query = query.Where("last_interact_at IS NOT NULL AND last_interact_at < ?", now.Add(-service.SparkWindow))
		case service.LevelUrgent:
			query = query.Where("(last_interact_at IS NULL OR last_interact_at < ?) AND (last_interact_at IS NULL OR last_interact_at >= ?)", now.Add(-3*time.Hour), now.Add(-service.SparkWindow))
		case service.LevelWarn:
			query = query.Where("last_interact_at < ? AND last_interact_at >= ?", now.Add(-3*time.Hour), now.Add(-8*time.Hour))
		case service.LevelSafe:
			query = query.Where("last_interact_at >= ?", now.Add(-8*time.Hour))
		}
	}

	var total int64
	query.Count(&total)
	order := "streak_days desc"
	switch c.DefaultQuery("sort", "streak") {
	case "recent":
		order = "last_interact_at desc"
	case "name":
		order = "name asc"
	case "created":
		order = "created_at desc"
	}

	var friends []model.Friend
	if err := query.Order(order).Offset((page - 1) * size).Limit(size).Find(&friends).Error; err != nil {
		util.Fail(c, 500, "读取好友失败")
		return
	}

	accountNames := h.accountNameMap(userID)
	now := time.Now().In(h.Cfg.Location)
	items := make([]FriendView, 0, len(friends))
	for _, friend := range friends {
		items = append(items, FriendView{
			Friend:      friend,
			State:       service.Evaluate(friend.LastInteractAt, friend.StreakDays, friend.MaxStreakDays, now),
			AccountName: accountNames[friend.AccountID],
		})
	}
	util.OK(c, util.Page{Items: items, Total: total, Page: page, PageSize: size})
}

type friendReq struct {
	AccountID uint   `json:"accountId"`
	Name      string `json:"name"`
	UserID    string `json:"userId"`
	UniqueID  string `json:"uniqueId"`
	SecUID    string `json:"secUid"`
	AvatarURL string `json:"avatarUrl"`
	Remark    string `json:"remark"`
	AutoKeep  *bool  `json:"autoKeep"`
	Streak    int    `json:"streakDays"`
}

// CreateFriend 手动添加火花好友。
func (h *Handler) CreateFriend(c *gin.Context) {
	var req friendReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.AccountID == 0 {
		util.Fail(c, 400, "请填写账号与好友昵称")
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
	if err := ent.CheckLimit("friends", ent.Usage.Friends); err != nil {
		util.Fail(c, 403, err.Error())
		return
	}
	autoKeep := true
	if req.AutoKeep != nil {
		autoKeep = *req.AutoKeep
	}
	friend := model.Friend{
		UserID: uid(c), AccountID: account.ID, Name: req.Name, DouyinUserID: req.UserID,
		UniqueID: req.UniqueID, SecUID: req.SecUID, AvatarURL: req.AvatarURL,
		Remark: req.Remark, AutoKeep: autoKeep, StreakDays: req.Streak, MaxStreakDays: req.Streak,
	}
	if err := h.DB.Create(&friend).Error; err != nil {
		util.Fail(c, 500, "添加失败")
		return
	}
	if !autoKeep {
		h.DB.Model(&friend).Update("auto_keep", false)
	}
	h.audit(c, "friend.create", friend.Name, "添加火花好友")
	util.OK(c, friend)
}

// UpdateFriend 更新好友设置。
func (h *Handler) UpdateFriend(c *gin.Context) {
	var req friendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.UserID != "" {
		updates["friend_user_id"] = req.UserID
	}
	if req.UniqueID != "" {
		updates["unique_id"] = req.UniqueID
	}
	if req.Remark != "" || c.Query("clear") == "1" {
		updates["remark"] = req.Remark
	}
	if req.AutoKeep != nil {
		updates["auto_keep"] = *req.AutoKeep
	}
	if req.Streak > 0 {
		updates["streak_days"] = req.Streak
		updates["max_streak_days"] = req.Streak
	}
	res := h.DB.Model(&model.Friend{}).Where("id = ? AND user_id = ?", c.Param("id"), uid(c)).Updates(updates)
	if res.RowsAffected == 0 {
		util.Fail(c, 404, "好友不存在")
		return
	}
	util.OKMsg(c, "已保存", nil)
}

// DeleteFriend 删除好友。
func (h *Handler) DeleteFriend(c *gin.Context) {
	res := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), uid(c)).Delete(&model.Friend{})
	if res.RowsAffected == 0 {
		util.Fail(c, 404, "好友不存在")
		return
	}
	util.OKMsg(c, "已删除", nil)
}

// KeepFriend 手动续火。
func (h *Handler) KeepFriend(c *gin.Context) {
	friendID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		util.Fail(c, 400, "参数错误")
		return
	}
	var friend model.Friend
	if err := h.DB.Where("id = ? AND user_id = ?", friendID, uid(c)).First(&friend).Error; err != nil {
		util.Fail(c, 404, "好友不存在")
		return
	}
	result, err := h.Sender.SendForFriend(c.Request.Context(), friend.AccountID, friend.ID, 0, 0, "manual")
	if err != nil {
		h.audit(c, "friend.keep.failed", friend.Name, err.Error())
		util.Fail(c, 502, err.Error())
		return
	}
	h.audit(c, "friend.keep", friend.Name, result.Record.Content)
	util.OK(c, result)
}

// BatchKeep 批量续火。
func (h *Handler) BatchKeep(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		util.Fail(c, 400, "请选择好友")
		return
	}
	success, failed := 0, 0
	messages := make([]string, 0, 4)
	for _, id := range req.IDs {
		var friend model.Friend
		if err := h.DB.Where("id = ? AND user_id = ?", id, uid(c)).First(&friend).Error; err != nil {
			failed++
			continue
		}
		if _, err := h.Sender.SendForFriend(c.Request.Context(), friend.AccountID, friend.ID, 0, 0, "manual"); err != nil {
			failed++
			if len(messages) < 3 {
				messages = append(messages, fmt.Sprintf("%s：%s", friend.Name, err.Error()))
			}
			continue
		}
		success++
	}
	util.OK(c, gin.H{"success": success, "failed": failed, "messages": messages})
}

func (h *Handler) accountNameMap(userID uint) map[uint]string {
	var accounts []model.Account
	h.DB.Select("id", "nickname").Where("user_id = ?", userID).Find(&accounts)
	out := make(map[uint]string, len(accounts))
	for _, account := range accounts {
		out[account.ID] = account.Nickname
	}
	return out
}

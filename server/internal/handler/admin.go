package handler

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"doyinspark/internal/model"
	"doyinspark/internal/service"
	"doyinspark/internal/util"
)

// AdminStats 运营概览。
func (h *Handler) AdminStats(c *gin.Context) {
	now := time.Now().In(h.Cfg.Location)
	var users, members, activeUsers, cards, cardsUsed int64
	h.DB.Model(&model.User{}).Where("role = ?", "user").Count(&users)
	h.DB.Model(&model.User{}).Where("role = ? AND member_expire_at > ?", "user", now).Count(&members)
	h.DB.Model(&model.User{}).Where("role = ? AND last_login_at > ?", "user", now.AddDate(0, 0, -7)).Count(&activeUsers)
	h.DB.Model(&model.CardKey{}).Count(&cards)
	h.DB.Model(&model.CardKey{}).Where("status = ?", "used").Count(&cardsUsed)
	var accounts, records int64
	h.DB.Model(&model.Account{}).Count(&accounts)
	h.DB.Model(&model.Record{}).Count(&records)
	util.OK(c, gin.H{
		"users": users, "members": members, "activeUsers": activeUsers,
		"cards": cards, "cardsUsed": cardsUsed, "cardsLeft": cards - cardsUsed,
		"accounts": accounts, "records": records,
	})
}

// AdminUsers 用户列表。
func (h *Handler) AdminUsers(c *gin.Context) {
	page, size := pageParams(atoi(c.DefaultQuery("page", "1")), atoi(c.DefaultQuery("pageSize", "15")))
	query := h.DB.Model(&model.User{}).Where("role = ?", "user")
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR nickname LIKE ?", like, like)
	}
	switch c.Query("status") {
	case "active":
		query = query.Where("member_expire_at > ?", time.Now())
	case "expired":
		query = query.Where("member_expire_at IS NULL OR member_expire_at <= ?", time.Now())
	case "banned":
		query = query.Where("status = ?", "banned")
	}
	var total int64
	query.Count(&total)
	var users []model.User
	query.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&users)

	type row struct {
		UserID      uint
		AccountCount int64
		FriendCount  int64
	}
	var counts []row
	h.DB.Model(&model.Account{}).Select("user_id, count(*) as account_count").Group("user_id").Scan(&counts)
	friendCounts := map[uint]int64{}
	var frows []row
	h.DB.Model(&model.Friend{}).Select("user_id, count(*) as friend_count").Group("user_id").Scan(&frows)
	for _, r := range frows {
		friendCounts[r.UserID] = r.FriendCount
	}
	accountCounts := map[uint]int64{}
	for _, r := range counts {
		accountCounts[r.UserID] = r.AccountCount
	}

	items := make([]gin.H, 0, len(users))
	for i := range users {
		items = append(items, gin.H{
			"user":         h.View(&users[i]),
			"accountCount": accountCounts[users[i].ID],
			"friendCount":  friendCounts[users[i].ID],
		})
	}
	util.OK(c, util.Page{Items: items, Total: total, Page: page, PageSize: size})
}

// AdminUpdateUser 修改用户状态 / 昵称 / 角色。
func (h *Handler) AdminUpdateUser(c *gin.Context) {
	var req struct {
		Nickname string `json:"nickname"`
		Status   string `json:"status"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "参数错误")
		return
	}
	var user model.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		util.Fail(c, 404, "用户不存在")
		return
	}
	if user.ID == uid(c) {
		util.Fail(c, 400, "不能修改自己的状态或角色")
		return
	}
	updates := map[string]any{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Status != "" {
		if req.Status != "active" && req.Status != "banned" {
			util.Fail(c, 400, "状态只能是 active 或 banned")
			return
		}
		updates["status"] = req.Status
	}
	if req.Role != "" {
		if req.Role != "admin" && req.Role != "user" {
			util.Fail(c, 400, "角色只能是 admin 或 user")
			return
		}
		updates["role"] = req.Role
	}
	if len(updates) == 0 {
		util.Fail(c, 400, "没有需要修改的字段")
		return
	}
	h.DB.Model(&user).Updates(updates)
	h.audit(c, "admin.user.update", user.Username, fmt.Sprintf("%v", updates))
	util.OKMsg(c, "已更新", nil)
}

// AdminGrant 管理员直接赠送会员。
func (h *Handler) AdminGrant(c *gin.Context) {
	var req struct {
		Plan string `json:"plan" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "请选择卡种")
		return
	}
	plan, ok := service.PlanOf(req.Plan)
	if !ok {
		util.Fail(c, 400, "卡种不存在")
		return
	}
	var user model.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		util.Fail(c, 404, "用户不存在")
		return
	}
	now := time.Now()
	before := user.MemberExpireAt
	after := service.Extend(before, plan, now)
	h.DB.Model(&user).Updates(map[string]any{"plan": plan.Code, "member_expire_at": after, "status": "active"})
	h.DB.Create(&model.MembershipLog{
		UserID: user.ID, Action: "grant", Plan: plan.Code, Days: plan.Days,
		BeforeExpire: before, AfterExpire: after, Remark: "管理员赠送 " + plan.Name, CreatedAt: now,
	})
	h.audit(c, "admin.user.grant", user.Username, plan.Name)
	util.OKMsg(c, "已赠送 "+plan.Name, h.View(&user))
}

// AdminCards 卡密列表。
func (h *Handler) AdminCards(c *gin.Context) {
	page, size := pageParams(atoi(c.DefaultQuery("page", "1")), atoi(c.DefaultQuery("pageSize", "20")))
	query := h.DB.Model(&model.CardKey{})
	if status := c.Query("status"); status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if plan := c.Query("plan"); plan != "" && plan != "all" {
		query = query.Where("plan = ?", plan)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		query = query.Where("code LIKE ? OR batch LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	query.Count(&total)
	var cards []model.CardKey
	query.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&cards)
	util.OK(c, util.Page{Items: cards, Total: total, Page: page, PageSize: size})
}

// AdminGenerateCards 批量生成卡密。
func (h *Handler) AdminGenerateCards(c *gin.Context) {
	var req struct {
		Plan      string `json:"plan" binding:"required"`
		Count     int    `json:"count"`
		Note      string `json:"note"`
		ValidDays int    `json:"validDays"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "请选择卡种")
		return
	}
	plan, ok := service.PlanOf(req.Plan)
	if !ok {
		util.Fail(c, 400, "卡种不存在")
		return
	}
	count := req.Count
	if count <= 0 {
		count = 10
	}
	if count > 200 {
		count = 200
	}
	validDays := req.ValidDays
	if validDays <= 0 {
		validDays = 365
	}
	batch := "B" + time.Now().Format("20060102150405")
	expire := time.Now().AddDate(0, 0, validDays)
	cards := make([]model.CardKey, 0, count)
	for i := 0; i < count; i++ {
		code, err := service.NewCardCode()
		if err != nil {
			util.Fail(c, 500, "卡密生成失败")
			return
		}
		cards = append(cards, model.CardKey{
			Code: code, Plan: plan.Code, Days: plan.Days, Status: "unused",
			Batch: batch, Note: req.Note, CreatedBy: uid(c), ExpireAt: &expire,
		})
	}
	if err := h.DB.Create(&cards).Error; err != nil {
		util.Fail(c, 500, "卡密保存失败")
		return
	}
	h.audit(c, "admin.card.generate", plan.Name, fmt.Sprintf("批量生成 %d 张 %s", count, batch))
	util.OKMsg(c, fmt.Sprintf("已生成 %d 张%s", count, plan.Name), gin.H{"batch": batch, "cards": cards})
}

// AdminVoidCard 作废卡密。
func (h *Handler) AdminVoidCard(c *gin.Context) {
	var card model.CardKey
	if err := h.DB.First(&card, c.Param("id")).Error; err != nil {
		util.Fail(c, 404, "卡密不存在")
		return
	}
	if card.Status == "used" {
		util.Fail(c, 400, "已使用的卡密不能作废")
		return
	}
	if card.Status == "void" {
		util.Fail(c, 400, "该卡密已经是作废状态")
		return
	}
	h.DB.Model(&card).Update("status", "void")
	h.audit(c, "admin.card.void", card.Code, "作废卡密")
	util.OKMsg(c, "已作废", nil)
}

// AdminExportCards 导出当前筛选下的卡密为 CSV。
func (h *Handler) AdminExportCards(c *gin.Context) {
	query := h.DB.Model(&model.CardKey{})
	if status := c.Query("status"); status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if plan := c.Query("plan"); plan != "" && plan != "all" {
		query = query.Where("plan = ?", plan)
	}
	var cards []model.CardKey
	query.Order("id desc").Limit(5000).Find(&cards)

	filename := fmt.Sprintf("doyinspark-cards-%s.csv", time.Now().Format("20060102-1504"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+url.PathEscape(filename))
	c.Writer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	writer.Write([]string{"卡密", "卡种", "天数", "状态", "批次", "有效期至", "使用者ID", "使用时间"})
	for _, card := range cards {
		expire, usedAt := "", ""
		if card.ExpireAt != nil {
			expire = card.ExpireAt.Format("2006-01-02")
		}
		if card.UsedAt != nil {
			usedAt = card.UsedAt.Format("2006-01-02 15:04:05")
		}
		writer.Write([]string{card.Code, service.PlanLabel(card.Plan), fmt.Sprint(card.Days), card.Status, card.Batch, expire, fmt.Sprint(card.UsedBy), usedAt})
	}
}

var _ = errors.New
var _ = gorm.ErrRecordNotFound

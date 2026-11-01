package handler

import (
	"encoding/csv"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"doyinspark/internal/model"
	"doyinspark/internal/util"
)

// Records 发送记录分页。
func (h *Handler) Records(c *gin.Context) {
	page, size := pageParams(atoi(c.DefaultQuery("page", "1")), atoi(c.DefaultQuery("pageSize", "15")))
	query := h.recordQuery(c)
	var total int64
	query.Count(&total)
	var items []model.Record
	query.Order("created_at desc").Offset((page - 1) * size).Limit(size).Find(&items)
	util.OK(c, util.Page{Items: items, Total: total, Page: page, PageSize: size})
}

// RecordStats 记录页顶部统计。
func (h *Handler) RecordStats(c *gin.Context) {
	userID := uid(c)
	now := time.Now().In(h.Cfg.Location)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var total, success, failed, today int64
	h.DB.Model(&model.Record{}).Where("user_id = ?", userID).Count(&total)
	h.DB.Model(&model.Record{}).Where("user_id = ? AND status = ?", userID, "success").Count(&success)
	h.DB.Model(&model.Record{}).Where("user_id = ? AND status = ?", userID, "failed").Count(&failed)
	h.DB.Model(&model.Record{}).Where("user_id = ? AND created_at >= ?", userID, dayStart).Count(&today)
	rate := 0.0
	if total > 0 {
		rate = float64(success) / float64(total)
	}
	util.OK(c, gin.H{"total": total, "success": success, "failed": failed, "today": today, "successRate": rate})
}

// ExportRecords 导出 CSV。
func (h *Handler) ExportRecords(c *gin.Context) {
	var items []model.Record
	h.recordQuery(c).Order("created_at desc").Limit(5000).Find(&items)

	filename := fmt.Sprintf("doyinspark-records-%s.csv", time.Now().Format("20060102-1504"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+url.PathEscape(filename))
	c.Writer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	_ = writer.Write([]string{"时间", "账号ID", "好友", "内容", "状态", "来源", "耗时(ms)", "失败原因"})
	for _, item := range items {
		_ = writer.Write([]string{
			item.CreatedAt.Format("2006-01-02 15:04:05"),
			strconv.Itoa(int(item.AccountID)),
			item.FriendName,
			item.Content,
			item.Status,
			item.Source,
			strconv.Itoa(item.LatencyMs),
			item.Error,
		})
	}
}

// PruneRecords 清理 N 天之前的记录。
func (h *Handler) PruneRecords(c *gin.Context) {
	days := atoi(c.DefaultQuery("days", "30"))
	if days < 1 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	res := h.DB.Where("user_id = ? AND created_at < ?", uid(c), cutoff).Delete(&model.Record{})
	util.OK(c, gin.H{"deleted": res.RowsAffected, "before": cutoff})
}

func (h *Handler) recordQuery(c *gin.Context) *gorm.DB {
	query := h.DB.Model(&model.Record{}).Where("user_id = ?", uid(c))
	if status := c.Query("status"); status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if source := c.Query("source"); source != "" && source != "all" {
		query = query.Where("source = ?", source)
	}
	if accountID := c.Query("accountId"); accountID != "" && accountID != "0" {
		query = query.Where("account_id = ?", accountID)
	}
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("friend_name LIKE ? OR content LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if days := atoi(c.Query("days")); days > 0 {
		query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, -days))
	}
	return query
}

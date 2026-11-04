package handler

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"

	"doyinspark/internal/model"
	"doyinspark/internal/service"
	"doyinspark/internal/signer"
	"doyinspark/internal/util"
)

// AccountView 账号列表项，附带统计。
type AccountView struct {
	model.Account
	FriendCount int `json:"friendCount"`
	SparkCount  int `json:"sparkCount"`
	BrokenCount int `json:"brokenCount"`
}

// Accounts 当前用户的账号列表。
func (h *Handler) Accounts(c *gin.Context) {
	userID := uid(c)
	var accounts []model.Account
	h.DB.Where("user_id = ?", userID).Order("id asc").Find(&accounts)
	now := time.Now().In(h.Cfg.Location)
	cutoff := now.Add(-service.SparkWindow)

	out := make([]AccountView, 0, len(accounts))
	for _, account := range accounts {
		view := AccountView{Account: account}
		var total, broken int64
		h.DB.Model(&model.Friend{}).Where("account_id = ? AND user_id = ?", account.ID, userID).Count(&total)
		h.DB.Model(&model.Friend{}).Where("account_id = ? AND user_id = ? AND (last_interact_at IS NULL OR last_interact_at < ?)", account.ID, userID, cutoff).Count(&broken)
		view.FriendCount = int(total)
		view.BrokenCount = int(broken)
		view.SparkCount = int(total - broken)
		out = append(out, view)
	}
	util.OK(c, out)
}

type accountReq struct {
	Nickname  string `json:"nickname"`
	Remark    string `json:"remark"`
	Cookie    string `json:"cookie"`
	ProxyURL  string `json:"proxyUrl"`
	ClearRisk bool   `json:"clearRisk"`
}

// 一批常见桌面 UA，创建账号时随机绑定并长期固定。
var userAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.2 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Safari/537.36 Edg/155.0.0.0",
}

func pickUserAgent() string { return userAgents[rand.Intn(len(userAgents))] }

// CreateAccount 新增托管账号。
func (h *Handler) CreateAccount(c *gin.Context) {
	var req accountReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Nickname == "" {
		util.Fail(c, 400, "请填写账号名称")
		return
	}
	ent, err := h.entitlement(c)
	if err != nil {
		util.Fail(c, 401, "登录态已失效")
		return
	}
	if err := ent.CheckLimit("accounts", ent.Usage.Accounts); err != nil {
		util.Fail(c, 403, err.Error())
		return
	}
	account := model.Account{
		UserID: uid(c), Nickname: req.Nickname, Remark: req.Remark, Status: "offline",
		UserAgent: pickUserAgent(), Timezone: "Asia/Shanghai", Locale: "zh-CN", Viewport: "1400x900",
	}
	if err := h.DB.Create(&account).Error; err != nil {
		util.Fail(c, 500, "创建账号失败")
		return
	}
	if req.Cookie != "" {
		if status, err := h.Sign.ImportCookie(account.ID, req.Cookie); err == nil && status.Logged {
			h.DB.Model(&account).Updates(map[string]any{"cookie": req.Cookie, "status": "online"})
			account.Status = "online"
			_ = h.syncProfile(&account)
		}
	}
	if h.Sign.Configured() {
		_ = h.Sign.SetBrowserConfig(account.ID, signer.BrowserConfig{
			UserAgent: account.UserAgent, Timezone: account.Timezone,
			Locale: account.Locale, Viewport: account.Viewport, ProxyURL: account.ProxyURL,
		})
	}
	h.audit(c, "account.create", req.Nickname, "新增托管账号")
	util.OK(c, account)
}

// UpdateAccount 修改账号信息。
func (h *Handler) UpdateAccount(c *gin.Context) {
	var req accountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "参数错误")
		return
	}
	updates := map[string]any{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	updates["remark"] = req.Remark
	if req.ProxyURL != "" {
		updates["proxy_url"] = req.ProxyURL
	}
	if req.ClearRisk {
		updates["risk_until"] = nil
		updates["risk_reason"] = ""
	}
	res := h.DB.Model(&model.Account{}).Where("id = ? AND user_id = ?", c.Param("id"), uid(c)).Updates(updates)
	if res.RowsAffected == 0 {
		util.Fail(c, 404, "账号不存在")
		return
	}
	if req.ProxyURL != "" && h.Sign.Configured() {
		var acc model.Account
		if err := h.DB.First(&acc, c.Param("id")).Error; err == nil {
			_ = h.Sign.SetBrowserConfig(acc.ID, signer.BrowserConfig{
				UserAgent: acc.UserAgent, Timezone: acc.Timezone, Locale: acc.Locale,
				Viewport: acc.Viewport, ProxyURL: req.ProxyURL,
			})
		}
	}
	h.audit(c, "account.update", c.Param("id"), "更新账号信息")
	util.OKMsg(c, "已保存", nil)
}

// DeleteAccount 删除账号及其好友。
func (h *Handler) DeleteAccount(c *gin.Context) {
	account, ok := h.accountOf(c)
	if !ok {
		return
	}
	if h.Sign.Configured() {
		_ = h.Sign.Logout(account.ID)
	}
	h.DB.Where("account_id = ?", account.ID).Delete(&model.Friend{})
	h.DB.Where("account_id = ?", account.ID).Delete(&model.Task{})
	h.DB.Delete(&account)
	h.audit(c, "account.delete", account.Nickname, "删除账号")
	util.OKMsg(c, "账号已删除", nil)
}

// LoginQR 打开扫码登录并返回二维码。
func (h *Handler) LoginQR(c *gin.Context) {
	account, ok := h.accountOf(c)
	if !ok {
		return
	}
	if !h.Sign.Configured() {
		util.Fail(c, 503, "签名服务未配置，无法发起登录")
		return
	}
	result, err := h.Sign.OpenLogin(account.ID)
	if err != nil {
		util.Fail(c, 502, err.Error())
		return
	}
	if result.Logged {
		h.markOnline(account)
	}
	util.OK(c, gin.H{"logged": result.Logged, "dataUrl": result.DataURL, "fallback": result.Fallback})
}

// LoginStatus 轮询扫码结果。
func (h *Handler) LoginStatus(c *gin.Context) {
	account, ok := h.accountOf(c)
	if !ok {
		return
	}
	if !h.Sign.Configured() {
		util.Fail(c, 503, "签名服务未配置")
		return
	}
	status, err := h.Sign.AccountStatus(account.ID)
	if err != nil {
		util.Fail(c, 502, err.Error())
		return
	}
	if status.Logged {
		if err := h.syncProfile(account); err == nil {
			h.DB.Model(account).Update("status", "online")
			util.OK(c, gin.H{"logged": true, "cookieCount": status.CookieCount})
			return
		}
	}
	util.OK(c, gin.H{"logged": false, "cookieCount": status.CookieCount})
}

// ImportCookie 导入完整 Cookie 登录。
func (h *Handler) ImportCookie(c *gin.Context) {
	account, ok := h.accountOf(c)
	if !ok {
		return
	}
	var req struct {
		Cookie string `json:"cookie" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, 400, "请粘贴完整 Cookie")
		return
	}
	status, err := h.Sign.ImportCookie(account.ID, req.Cookie)
	if err != nil {
		util.Fail(c, 502, err.Error())
		return
	}
	if !status.Logged {
		h.DB.Model(account).Update("status", "expired")
		util.Fail(c, 400, "Cookie 已导入，但没有找到 sessionid，请确认复制完整")
		return
	}
	// 仅看 sessionid 存在并不等于登录态有效，必须用资料接口做真实校验
	h.DB.Model(account).Updates(map[string]any{"cookie": req.Cookie})
	if err := h.syncProfile(account); err != nil {
		h.DB.Model(account).Update("status", "expired")
		util.Fail(c, 400, "Cookie 无效或已失效："+err.Error())
		return
	}
	h.DB.Model(account).Update("status", "online")
	account.Status = "online"
	h.audit(c, "account.cookie", account.Nickname, "导入 Cookie")
	util.OKMsg(c, "登录态已导入", nil)
}

// SyncAccount 同步账号资料。
func (h *Handler) SyncAccount(c *gin.Context) {
	account, ok := h.accountOf(c)
	if !ok {
		return
	}
	if err := h.syncProfile(account); err != nil {
		util.Fail(c, 502, err.Error())
		return
	}
	util.OKMsg(c, "资料已同步", account)
}

// SyncFriends 同步会话好友：聊天页拿到 sec_uid，再批量换取 uid / 昵称 / 头像。
func (h *Handler) SyncFriends(c *gin.Context) {
	account, ok := h.accountOf(c)
	if !ok {
		return
	}
	if !h.Sign.Configured() {
		util.Fail(c, 503, "签名服务未配置，请先启动 sign-service")
		return
	}
	if account.Status != "online" {
		util.Fail(c, 409, "账号「"+account.Nickname+"」还未登录，请先扫码登录后再同步会话")
		return
	}

	secUIDs, err := h.Sign.FetchFriendSecUIDs(account.ID)
	if err != nil {
		util.Fail(c, 502, "同步失败："+err.Error())
		return
	}
	profiles, err := h.Sign.FetchFriendProfiles(account.ID, account.Cookie, secUIDs)
	if err != nil && len(profiles) == 0 {
		util.Fail(c, 502, "同步失败："+err.Error())
		return
	}

	created, updated := 0, 0
	for _, profile := range profiles {
		if profile.Nickname == "" {
			continue
		}
		key := profile.UID
		if key == "" {
			key = profile.SecUID
		}
		var friend model.Friend
		err := h.DB.Where("account_id = ? AND (friend_user_id = ? OR sec_uid = ?)", account.ID, key, profile.SecUID).First(&friend).Error
		if err != nil {
			h.DB.Create(&model.Friend{
				UserID: uid(c), AccountID: account.ID, Name: profile.Nickname,
				DouyinUserID: profile.UID, SecUID: profile.SecUID,
				UniqueID: profile.UniqueID, AvatarURL: profile.AvatarURL, AutoKeep: true,
			})
			created++
			continue
		}
		h.DB.Model(&friend).Updates(map[string]any{
			"name": profile.Nickname, "avatar_url": profile.AvatarURL,
			"unique_id": profile.UniqueID, "sec_uid": profile.SecUID,
			"friend_user_id": profile.UID,
		})
		updated++
	}

	now := time.Now()
	h.DB.Model(account).Update("last_sync_at", now)
	h.audit(c, "account.sync", account.Nickname, "同步会话好友")
	util.OKMsg(c, "同步完成", gin.H{"created": created, "updated": updated, "total": len(profiles)})
}

// Emojis 拉取聊天框表情面板（Redis 缓存 1 小时）。
func (h *Handler) Emojis(c *gin.Context) {
	account, ok := h.accountOf(c)
	if !ok {
		return
	}
	if !h.Sign.Configured() {
		util.Fail(c, 503, "签名服务未配置")
		return
	}
	if account.Status != "online" {
		util.Fail(c, 409, "账号「"+account.Nickname+"」未登录，无法读取表情")
		return
	}
	cacheKey := fmt.Sprintf("doyinspark:emoji:%d", account.ID)
	var cached []signer.Emoji
	if h.Cache.GetJSON(c.Request.Context(), cacheKey, &cached) && len(cached) > 0 {
		util.OK(c, gin.H{"items": cached, "cached": true})
		return
	}
	items, err := h.Sign.FetchEmojis(account.ID, account.Cookie)
	if err != nil {
		util.Fail(c, 502, "读取表情失败："+err.Error())
		return
	}
	h.Cache.SetJSON(c.Request.Context(), cacheKey, items, time.Hour)
	util.OK(c, gin.H{"items": items, "cached": false})
}

// LogoutAccount 下线账号并清除浏览器会话。
func (h *Handler) LogoutAccount(c *gin.Context) {
	account, ok := h.accountOf(c)
	if !ok {
		return
	}
	if h.Sign.Configured() {
		_ = h.Sign.Logout(account.ID)
	}
	h.DB.Model(account).Updates(map[string]any{"status": "offline", "cookie": ""})
	h.audit(c, "account.logout", account.Nickname, "下线账号")
	util.OKMsg(c, "已下线", nil)
}

func (h *Handler) accountOf(c *gin.Context) (*model.Account, bool) {
	var account model.Account
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), uid(c)).First(&account).Error; err != nil {
		util.Fail(c, 404, "账号不存在")
		return nil, false
	}
	return &account, true
}

func (h *Handler) markOnline(account *model.Account) {
	h.DB.Model(account).Update("status", "online")
	account.Status = "online"
	_ = h.syncProfile(account)
}

func (h *Handler) syncProfile(account *model.Account) error {
	if !h.Sign.Configured() {
		return nil
	}
	profile, err := h.Sign.AccountProfile(account.ID)
	if err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]any{
		"douyin_uid": profile.UID, "unique_id": profile.UniqueID,
		"sec_uid": profile.SecUID, "avatar_url": profile.AvatarURL, "last_sync_at": now,
	}
	if profile.Nickname != "" {
		updates["nickname"] = profile.Nickname
	}
	h.DB.Model(account).Updates(updates)
	if profile.Nickname != "" {
		account.Nickname = profile.Nickname
	}
	account.AvatarURL = profile.AvatarURL
	account.UniqueID = profile.UniqueID
	return nil
}

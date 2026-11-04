// Package signer 对接独立的浏览器签名服务（sign-service）。
//
// 为什么必须外置：抖音 Web 接口需要 a_bogus / X-Bogus 签名，签名由页面里的
// byted_acrawler 生成，设备指纹（ttwid / fpk1 / UIFID）也绑定在同一个浏览器实例上。
// 把「登录态 + 签名 + 设备身份」放进一个常驻浏览器，是唯一稳定的形态。
package signer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client 签名服务 HTTP 客户端。
type Client struct {
	base string
	http *http.Client
}

// New 创建客户端；base 为空时返回 nil，调用方据此判断通道未配置。
func New(base string) *Client {
	if strings.TrimSpace(base) == "" {
		return nil
	}
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 45 * time.Second}}
}

// Configured 是否已配置签名服务。
func (c *Client) Configured() bool { return c != nil }

// Status 账号登录态。
type Status struct {
	Logged      bool   `json:"logged"`
	Nickname    string `json:"nickname"`
	CookieCount int    `json:"cookieCount"`
}

// LoginResult 二维码登录结果。
type LoginResult struct {
	Logged   bool   `json:"logged"`
	DataURL  string `json:"dataUrl"`
	Fallback bool   `json:"fallback"`
	Width    int    `json:"width"`
}

// Profile 抖音账号真实资料。
type Profile struct {
	UID       string `json:"uid"`
	Nickname  string `json:"nickname"`
	UniqueID  string `json:"uniqueId"`
	SecUID    string `json:"secUid"`
	AvatarURL string `json:"avatarUrl"`
}

// FetchResult 站内请求结果。
type FetchResult struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
	Error  string `json:"error"`
}

func (c *Client) post(path string, payload any, out any) error {
	if c == nil {
		return errors.New("签名服务未配置，请设置 SIGN_SERVICE_URL")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.http.Post(c.base+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("签名服务不可达: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<21))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}
		return fmt.Errorf("签名服务返回 %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("签名服务响应异常: %s", truncate(string(body), 200))
	}
	return nil
}

// Health 探测签名服务与浏览器是否就绪。
func (c *Client) Health() error {
	var out struct {
		OK bool `json:"ok"`
	}
	return c.post("/health", map[string]any{}, &out)
}

// AccountStatus 查询指定账号登录态。
func (c *Client) AccountStatus(accountID uint) (*Status, error) {
	var out Status
	if err := c.post("/login/status", map[string]uint{"accountId": accountID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OpenLogin 打开登录页并返回二维码。
func (c *Client) OpenLogin(accountID uint) (*LoginResult, error) {
	var out LoginResult
	if err := c.post("/login/open", map[string]uint{"accountId": accountID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ImportCookie 导入控制台 Cookie 到该账号的浏览器会话。
func (c *Client) ImportCookie(accountID uint, cookie string) (*Status, error) {
	var out Status
	if err := c.post("/cookie/import", map[string]any{"accountId": accountID, "cookie": cookie}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AccountProfile 读取当前登录账号资料。
func (c *Client) AccountProfile(accountID uint) (*Profile, error) {
	var out Profile
	if err := c.post("/account/profile", map[string]uint{"accountId": accountID}, &out); err != nil {
		return nil, err
	}
	if out.UID == "" && out.Nickname == "" {
		return nil, errors.New("登录态无效：没有读取到账号资料")
	}
	return &out, nil
}

// Logout 关闭浏览器会话并清除 profile。
func (c *Client) Logout(accountID uint) error {
	var out map[string]any
	return c.post("/logout", map[string]uint{"accountId": accountID}, &out)
}

// BrowserConfig 账号级浏览器指纹与出口配置。
type BrowserConfig struct {
	UserAgent string `json:"userAgent"`
	Timezone  string `json:"timezone"`
	Locale    string `json:"locale"`
	Viewport  string `json:"viewport"`
	ProxyURL  string `json:"proxyUrl"`
}

// SetBrowserConfig 把指纹写入该账号的浏览器 profile，下次启动会话时生效。
func (c *Client) SetBrowserConfig(accountID uint, cfg BrowserConfig) error {
	var out map[string]any
	return c.post("/session/config", map[string]any{
		"accountId": accountID, "userAgent": cfg.UserAgent, "timezone": cfg.Timezone,
		"locale": cfg.Locale, "viewport": cfg.Viewport, "proxyUrl": cfg.ProxyURL,
	}, &out)
}

// Fetch 在指定账号的登录上下文里发起一次已签名请求。
func (c *Client) Fetch(accountID uint, cookie, url, method, body string, headers map[string]string) (*FetchResult, error) {
	var out FetchResult
	payload := map[string]any{"accountId": accountID, "cookie": cookie, "url": url, "method": method, "body": body, "headers": headers}
	if err := c.post("/fetch", payload, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return &out, errors.New(out.Error)
	}
	return &out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

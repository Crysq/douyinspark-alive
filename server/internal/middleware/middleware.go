// Package middleware 提供鉴权、限流、跨域与访问日志。
package middleware

import (
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"doyinspark/internal/cache"
	"doyinspark/internal/config"
	"doyinspark/internal/util"
)

// contextKey 统一上下文键名。
const (
	CtxUserID = "userID"
	CtxRole   = "role"
	CtxScope  = "scope"
	CtxJTI    = "jti"
)

func bearer(c *gin.Context) string {
	raw := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if raw == "" {
		raw = c.Query("token") // SSE / 下载链接用 query 传递
	}
	return raw
}

func verify(cfg *config.Config, rdb *cache.Cache, c *gin.Context, wantScope string) bool {
	raw := bearer(c)
	if raw == "" {
		util.Fail(c, 401, "请先登录")
		return false
	}
	claims, err := util.ParseToken(cfg.JWTSecret, raw)
	if err != nil {
		util.Fail(c, 401, "登录态已过期，请重新登录")
		return false
	}
	if rdb.IsBlacklisted(c.Request.Context(), claims.JTI) {
		util.Fail(c, 401, "登录态已失效，请重新登录")
		return false
	}
	if wantScope == util.ScopeFull && claims.Scope != util.ScopeFull {
		util.Fail(c, 403, "账号尚未激活，请先兑换卡密")
		return false
	}
	c.Set(CtxUserID, claims.UserID)
	c.Set(CtxRole, claims.Role)
	c.Set(CtxScope, claims.Scope)
	c.Set(CtxJTI, claims.JTI)
	return true
}

// Auth 完整权限：会员有效且已激活。
func Auth(cfg *config.Config, rdb *cache.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !verify(cfg, rdb, c, util.ScopeFull) {
			return
		}
		c.Next()
	}
}

// Activation 允许「待激活」令牌访问：仅用于兑换卡密与查询自身状态。
func Activation(cfg *config.Config, rdb *cache.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !verify(cfg, rdb, c, "") {
			return
		}
		c.Next()
	}
}

// AdminOnly 仅管理员。
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString(CtxRole) != "admin" {
			util.Fail(c, 403, "需要管理员权限")
			return
		}
		c.Next()
	}
}

// RateLimit 基于 Redis 的固定窗口限流。
func RateLimit(rdb *cache.Cache, name string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "doyinspark:rl:" + name + ":" + c.ClientIP()
		if count := rdb.IncrWindow(c.Request.Context(), key, window); count > int64(limit) {
			util.Fail(c, 429, "操作太频繁，请稍后再试")
			return
		}
		c.Next()
	}
}

// CORS 跨域处理。
func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		allow := origin
		if allow == "" || allow == "*" {
			allow = "*"
		}
		c.Header("Access-Control-Allow-Origin", allow)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
		c.Header("Access-Control-Expose-Headers", "Content-Disposition")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// AccessLog 打印慢请求与错误请求，避免刷屏。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		cost := time.Since(start)
		if cost > 800*time.Millisecond || c.Writer.Status() >= 500 {
			log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), cost)
		}
	}
}

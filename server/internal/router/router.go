// Package router 装配路由与前端静态资源。
package router

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"doyinspark/internal/cache"
	"doyinspark/internal/config"
	"doyinspark/internal/handler"
	"doyinspark/internal/middleware"
)

// New 构建 gin 引擎。
func New(h *handler.Handler, cfg *config.Config, rdb *cache.Cache) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	// 默认不信任任何代理头：否则 X-Forwarded-For 可伪造，限流形同虚设。
	// 真在 Nginx / 负载均衡后面部署时，用 TRUSTED_PROXIES 显式声明网段。
	if len(cfg.TrustedProxies) == 0 {
		_ = r.SetTrustedProxies(nil)
	} else {
		_ = r.SetTrustedProxies(cfg.TrustedProxies)
	}
	r.Use(gin.Recovery(), middleware.CORS(cfg.AllowOrigin), middleware.AccessLog())

	r.GET("/health", h.Health)

	api := r.Group("/api")
	{
		// 公开接口
		api.POST("/auth/login", middleware.RateLimit(rdb, "login", 12, 5*time.Minute), h.Login)
		api.POST("/auth/register", middleware.RateLimit(rdb, "register", 10, 10*time.Minute), h.Register)
		api.GET("/membership/plans", h.Plans)

		// 待激活令牌也能访问（兑换卡密 / 查询自身会员）
		activation := api.Group("", middleware.Activation(cfg, rdb))
		{
			activation.POST("/membership/redeem", middleware.RateLimit(rdb, "redeem", 8, 5*time.Minute), h.Redeem)
			activation.GET("/membership", h.Membership)
		}

		// 需要完整会员权限
		auth := api.Group("", middleware.Auth(cfg, rdb))
		{
			auth.GET("/events/stream", h.Stream)
			auth.GET("/auth/me", h.Me)
			auth.POST("/auth/logout", h.Logout)
			auth.PUT("/auth/password", h.ChangePassword)

			auth.GET("/overview", h.Overview)
			auth.GET("/overview/trend", h.Trend)
			auth.GET("/overview/heatmap", h.Heatmap)
			auth.GET("/notifications", h.Notifications)
			auth.POST("/notifications/read", h.ReadNotifications)
			auth.GET("/audit", h.AuditLogs)

			auth.GET("/accounts", h.Accounts)
			auth.POST("/accounts", h.CreateAccount)
			auth.PATCH("/accounts/:id", h.UpdateAccount)
			auth.DELETE("/accounts/:id", h.DeleteAccount)
			auth.POST("/accounts/:id/login", middleware.RateLimit(rdb, "login-qr", 6, time.Minute), h.LoginQR)
			auth.GET("/accounts/:id/login/status", h.LoginStatus)
			auth.POST("/accounts/:id/cookie", h.ImportCookie)
			auth.POST("/accounts/:id/sync", h.SyncAccount)
			auth.POST("/accounts/:id/sync/friends", middleware.RateLimit(rdb, "sync", 6, time.Minute), h.SyncFriends)
			auth.POST("/accounts/:id/logout", h.LogoutAccount)

			auth.GET("/friends", h.Friends)
			auth.POST("/friends", h.CreateFriend)
			auth.PATCH("/friends/:id", h.UpdateFriend)
			auth.DELETE("/friends/:id", h.DeleteFriend)
			auth.POST("/friends/:id/keep", middleware.RateLimit(rdb, "keep", 30, time.Minute), h.KeepFriend)
			auth.POST("/friends/batch-keep", middleware.RateLimit(rdb, "keep-batch", 12, time.Minute), h.BatchKeep)

			auth.GET("/tasks", h.Tasks)
			auth.POST("/tasks", h.CreateTask)
			auth.PATCH("/tasks/:id", h.UpdateTask)
			auth.DELETE("/tasks/:id", h.DeleteTask)
			auth.POST("/tasks/:id/run", middleware.RateLimit(rdb, "task-run", 10, time.Minute), h.RunTask)

			auth.GET("/templates", h.Templates)
			auth.POST("/templates", h.CreateTemplate)
			auth.PATCH("/templates/:id", h.UpdateTemplate)
			auth.DELETE("/templates/:id", h.DeleteTemplate)

			auth.GET("/records", h.Records)
			auth.GET("/records/stats", h.RecordStats)
			auth.GET("/records/export", h.ExportRecords)
			auth.DELETE("/records/prune", h.PruneRecords)

			auth.GET("/settings", h.Settings)
			auth.PUT("/settings", h.UpdateSettings)

			auth.GET("/system/health", h.Health)
			auth.GET("/system/rules", h.SparkRules)
			auth.POST("/system/demo", h.SeedDemo)
			auth.DELETE("/system/demo", h.ClearDemo)

			// 管理端：运营数据、用户、卡密
			admin := auth.Group("/admin", middleware.AdminOnly())
			{
				admin.GET("/stats", h.AdminStats)
				admin.GET("/users", h.AdminUsers)
				admin.PATCH("/users/:id", h.AdminUpdateUser)
				admin.POST("/users/:id/grant", h.AdminGrant)
				admin.GET("/cards", h.AdminCards)
				admin.POST("/cards/generate", h.AdminGenerateCards)
				admin.POST("/cards/:id/void", h.AdminVoidCard)
				admin.GET("/cards/export", h.AdminExportCards)
			}
		}
	}

	serveStatic(r)
	return r
}

// serveStatic 托管前端构建产物，并为 SPA 做 history 回退。
func serveStatic(r *gin.Engine) {
	publicDir := os.Getenv("PUBLIC_DIR")
	if publicDir == "" {
		publicDir = "public"
	}
	index := filepath.Join(publicDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		return
	}
	r.Static("/assets", filepath.Join(publicDir, "assets"))
	r.StaticFile("/favicon.svg", filepath.Join(publicDir, "favicon.svg"))
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(404, gin.H{"code": 404, "message": "接口不存在"})
			return
		}
		c.File(index)
	})
}

// DoyinSpark Keeper —— 抖音火花守护系统后端入口。
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"doyinspark/internal/cache"
	"doyinspark/internal/config"
	"doyinspark/internal/handler"
	"doyinspark/internal/router"
	"doyinspark/internal/service"
	"doyinspark/internal/signer"
	"doyinspark/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置错误: %v", err)
	}
	time.Local = cfg.Location

	db, err := store.Open(cfg.DSN)
	if err != nil {
		log.Fatalf("数据库不可用: %v", err)
	}
	if err := store.Migrate(db); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	rdb, err := cache.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Redis 不可用: %v", err)
	}

	sign := signer.New(cfg.SignURL)
	bus := service.NewEventBus(rdb)
	sender := service.NewSender(db, rdb, sign, bus, cfg.Location)
	sched := service.NewScheduler(db, rdb, sender, bus, cfg.Location, cfg.TickInterval)
	sched.Start()
	defer sched.Stop()

	h := handler.New(handler.Deps{DB: db, Cache: rdb, Sign: sign, Sender: sender, Sched: sched, Bus: bus, Cfg: cfg})
	engine := router.New(h, cfg, rdb)

	server := &http.Server{Addr: ":" + cfg.Port, Handler: engine, ReadHeaderTimeout: 15 * time.Second}
	go func() {
		log.Printf("DoyinSpark 已启动 http://127.0.0.1:%s （签名通道：%s）", cfg.Port, channelLabel(sign))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在优雅停机…")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func channelLabel(sign *signer.Client) string {
	if !sign.Configured() {
		return "未配置"
	}
	return "已配置"
}

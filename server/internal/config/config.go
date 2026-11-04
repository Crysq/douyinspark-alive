// Package config 负责从环境变量装配运行参数。
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 是服务运行所需的全部配置。
type Config struct {
	Port           string
	AllowOrigin    string
	JWTSecret      string
	TokenTTL       time.Duration
	DSN            string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	SignURL        string
	AuthMode       string
	TrustedProxies []string
	TickInterval   time.Duration
	Location       *time.Location
	Env            string
}

// Load 读取 .env（若存在）并生成配置。
func Load() (*Config, error) {
	loadDotEnv(".env")
	loc, err := time.LoadLocation(env("TZ", "Asia/Shanghai"))
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}

	cfg := &Config{
		Port:          env("PORT", "8787"),
		AllowOrigin:   env("ALLOW_ORIGIN", "*"),
		JWTSecret:     env("JWT_SECRET", "doyinspark-dev-secret-change-me"),
		TokenTTL:      time.Duration(envInt("TOKEN_TTL_HOURS", 72)) * time.Hour,
		DSN:           env("DB_DSN", "root:root@tcp(127.0.0.1:3306)/doyinspark?charset=utf8mb4&parseTime=True&loc=Local"),
		RedisAddr:     env("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: env("REDIS_PASSWORD", ""),
		RedisDB:       envInt("REDIS_DB", 0),
		SignURL:       strings.TrimRight(env("SIGN_SERVICE_URL", ""), "/"),
		AuthMode:      env("AUTH_MODE", "simple"),
		TickInterval:  time.Duration(envInt("TICK_SECONDS", 20)) * time.Second,
		Location:      loc,
		Env:           env("APP_ENV", "production"),
	}
	if raw := strings.TrimSpace(env("TRUSTED_PROXIES", "")); raw != "" {
		for _, item := range strings.Split(raw, ",") {
			if item = strings.TrimSpace(item); item != "" {
				cfg.TrustedProxies = append(cfg.TrustedProxies, item)
			}
		}
	}
	if len(cfg.JWTSecret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET 至少 16 个字符")
	}
	if cfg.TickInterval < 5*time.Second {
		cfg.TickInterval = 5 * time.Second
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return fallback
}

// loadDotEnv 解析简单的 KEY=VALUE 文件，已存在的环境变量优先。
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}

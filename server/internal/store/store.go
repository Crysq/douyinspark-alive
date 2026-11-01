// Package store 负责数据库连接、迁移与初始数据。
package store

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"doyinspark/internal/model"
)

// Open 连接 MySQL，带启动重试（等待容器就绪）。
func Open(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 30; attempt++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger:  logger.Default.LogMode(logger.Warn),
			NowFunc: func() time.Time { return time.Now().Local() },
		})
		if err == nil {
			if sqlDB, e := db.DB(); e == nil && sqlDB.Ping() == nil {
				sqlDB.SetMaxOpenConns(40)
				sqlDB.SetMaxIdleConns(10)
				sqlDB.SetConnMaxLifetime(time.Hour)
				return db, nil
			} else if e != nil {
				err = e
			} else {
				err = fmt.Errorf("数据库 ping 失败")
			}
		}
		log.Printf("等待 MySQL 就绪 (%d/30): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
}

// Migrate 建表并写入初始管理员、系统模板与兼容数据。
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(model.All()...); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}
	admin, err := seedAdmin(db)
	if err != nil {
		return err
	}
	// 先认领旧版本无归属数据，再写系统模板库，避免把模板库也搬走
	if err := adoptLegacyData(db, admin.ID); err != nil {
		return err
	}
	if err := seedSettings(db, admin.ID); err != nil {
		return err
	}
	return seedTemplates(db)
}

func seedAdmin(db *gorm.DB) (*model.User, error) {
	var admin model.User
	err := db.Where("role = ?", "admin").First(&admin).Error
	if err == nil {
		return &admin, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	forever := time.Now().AddDate(serviceForeverYears(), 0, 0)
	admin = model.User{
		Username: "admin", PasswordHash: string(hash), Nickname: "火花管理员",
		Role: "admin", Status: "active", Plan: "forever", MemberExpireAt: &forever,
	}
	if err := db.Create(&admin).Error; err != nil {
		return nil, err
	}
	log.Println("已创建默认管理员 admin / admin123（请尽快修改密码）")
	return &admin, nil
}

func serviceForeverYears() int { return 100 }

func seedSettings(db *gorm.DB, userID uint) error {
	var count int64
	if err := db.Model(&model.Setting{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return db.Create(&model.Setting{UserID: userID, RetryTimes: 2, QuietStart: "00:30", QuietEnd: "07:00", NotifyBreak: true, AutoSync: true, KeepAlive: true, Theme: "dark"}).Error
}

// seedTemplates 写入系统模板库（user_id=0），新用户注册时会复制一份。
func seedTemplates(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.Template{}).Where("user_id = 0").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	items := []model.Template{
		{UserID: 0, Name: "早安打卡", Category: "daily", Content: "早呀～今天也要开开心心 {{date}} {{weekday}}", Enabled: true},
		{UserID: 0, Name: "晚安火花", Category: "daily", Content: "晚安啦，火花不能断 🔥 已经连着 {{days}} 天了", Enabled: true},
		{UserID: 0, Name: "随手一句", Category: "daily", Content: "在忙吗？来续个火花 {{time}}", Enabled: true},
		{UserID: 0, Name: "节日问候", Category: "festival", Content: "节日快乐！今天也要记得回我一句哦 🎉", Enabled: true},
	}
	return db.Create(&items).Error
}

// adoptLegacyData 把旧版本无归属的数据挂到管理员名下，避免升级后数据“消失”。
func adoptLegacyData(db *gorm.DB, adminID uint) error {
	// 旧版 friends.user_id 存的是「好友抖音 ID」，搬到 friend_user_id（幂等；租户 ID 都是小整数，不会误判）
	if err := db.Exec("UPDATE friends SET friend_user_id = CAST(user_id AS CHAR) WHERE (friend_user_id IS NULL OR friend_user_id = '') AND user_id > 1000").Error; err != nil {
		return err
	}
	// 无归属的遗留数据统一挂到管理员名下（系统模板库 user_id=0 不参与）
	tables := []any{&model.Account{}, &model.Friend{}, &model.Task{}, &model.Record{}, &model.Notification{}, &model.AuditLog{}}
	for _, item := range tables {
		if err := db.Model(item).Where("user_id NOT IN (SELECT id FROM users)").Update("user_id", adminID).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedUserDefaults 为新注册用户复制系统模板与默认设置。
func SeedUserDefaults(db *gorm.DB, userID uint) error {
	if err := seedSettings(db, userID); err != nil {
		return err
	}
	var library []model.Template
	if err := db.Where("user_id = ?", 0).Find(&library).Error; err != nil {
		return err
	}
	if len(library) == 0 {
		return nil
	}
	copies := make([]model.Template, 0, len(library))
	for _, tpl := range library {
		copies = append(copies, model.Template{UserID: userID, Name: tpl.Name, Category: tpl.Category, Content: tpl.Content, Enabled: true})
	}
	return db.Create(&copies).Error
}

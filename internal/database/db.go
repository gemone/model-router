package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gemone/model-router/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

// Init 初始化数据库
func Init(dbPath string) error {
	if dbPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dbDir := filepath.Join(homeDir, ".model-router")
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return err
		}
		dbPath = filepath.Join(dbDir, "data.db")
	}

	gormLogger := logger.Default
	if os.Getenv("DEBUG") == "true" {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	// 自动迁移
	return db.AutoMigrate(
		&model.Profile{},
		&model.Provider{},
		&model.Model{},
		&model.RouteRule{},
		&model.RequestLog{},
		&model.APIKey{},
		&model.Stats{},
		&model.Setting{},
		&model.TestResult{},
	)
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return db
}

// Close 关闭数据库连接
func Close() error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

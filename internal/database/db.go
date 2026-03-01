package database

import (
	"fmt"
	"os"
	"path/filepath"

	// "github.com/gemone/model-router/internal/model" // TODO: Re-enable for auto-migrate
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

	// 自动迁移 disabled - tables already set up manually
	// TODO: Re-enable after fixing GORM SQLite FOREIGN KEY parsing issue

	// 创建索引
	if err := createCompressionGroupIndexes(db); err != nil {
		return err
	}
	if err := createCompositeAutoModelIndexes(db); err != nil {
		return err
	}

	return nil
}

func createCompressionGroupIndexes(db *gorm.DB) error {
	// Composite lookup index
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_compression_group_profile_name
		ON compression_model_groups(profile_id, name)
	`).Error; err != nil {
		return fmt.Errorf("failed to create profile_name index: %w", err)
	}

	// Enabled groups index
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_compression_group_enabled
		ON compression_model_groups(enabled) WHERE enabled = true
	`).Error; err != nil {
		return fmt.Errorf("failed to create enabled index: %w", err)
	}

	return nil
}

func createCompositeAutoModelIndexes(db *gorm.DB) error {
	// Composite lookup index
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_composite_auto_model_profile_name
		ON composite_auto_models(profile_id, name)
	`).Error; err != nil {
		return fmt.Errorf("failed to create profile_name index: %w", err)
	}

	// Enabled models index
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_composite_auto_model_enabled
		ON composite_auto_models(enabled) WHERE enabled = true
	`).Error; err != nil {
		return fmt.Errorf("failed to create enabled index: %w", err)
	}

	return nil
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

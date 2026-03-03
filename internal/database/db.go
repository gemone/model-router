package database

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gemone/model-router/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// isValidTableName validates that a table name contains only safe characters
// to prevent SQL injection attacks. Table names must:
// - Start with a lowercase letter or underscore
// - Contain only lowercase letters, numbers, and underscores
// This is a defense-in-depth measure even though current queries use hardcoded names
func isValidTableName(name string) bool {
	// Only allow lowercase letters, numbers, and underscores
	// Must start with a letter or underscore
	matched, _ := regexp.MatchString(`^[a-z_][a-z0-9_]*$`, name)
	return matched && len(name) > 0 && len(name) < 65
}

// whitelistTableName checks if a table name is in the allowed whitelist
// This provides an additional layer of security beyond pattern matching
var allowedTableNames = map[string]bool{
	"providers":                true,
	"models":                   true,
	"profiles":                 true,
	"routes":                   true,
	"compression_model_groups": true,
	"composite_auto_models":    true,
	"request_logs":             true,
	"stats":                    true,
	"prompt_templates":         true,
}

// isTableNameAllowed verifies the table name is both valid format and in whitelist
func isTableNameAllowed(name string) bool {
	if !isValidTableName(name) {
		return false
	}
	return allowedTableNames[name]
}

var db *gorm.DB

// autoMigrateTables 自动迁移表结构
func autoMigrateTables(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Provider{},
		&model.Model{},
		&model.Profile{},
		&model.Route{},
		&model.CompressionModelGroup{},
		&model.CompositeAutoModel{},
		&model.RequestLog{},
		&model.Stats{},
		&model.PromptTemplate{},
	)
}

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

	// 自动迁移表结构
	if err := autoMigrateTables(db); err != nil {
		return fmt.Errorf("failed to auto migrate tables: %w", err)
	}

	// 创建索引（如果表存在）
	if db.Migrator().HasTable("compression_model_groups") {
		if err := createCompressionGroupIndexes(db); err != nil {
			return err
		}
	}
	if db.Migrator().HasTable("composite_auto_models") {
		if err := createCompositeAutoModelIndexes(db); err != nil {
			return err
		}
	}

	return nil
}

func createCompressionGroupIndexes(db *gorm.DB) error {
	tableName := "compression_model_groups"
	if !isTableNameAllowed(tableName) {
		return fmt.Errorf("security: invalid table name %q", tableName)
	}

	// Use GORM's Quote method to properly escape table name
	quotedTableName := db.Statement.Quote(tableName)

	// Composite lookup index
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_compression_group_profile_name
		ON ` + quotedTableName + `(profile_id, name)
	`).Error; err != nil {
		return fmt.Errorf("failed to create profile_name index: %w", err)
	}

	// Enabled groups index
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_compression_group_enabled
		ON ` + quotedTableName + `(enabled) WHERE enabled = true
	`).Error; err != nil {
		return fmt.Errorf("failed to create enabled index: %w", err)
	}

	return nil
}

func createCompositeAutoModelIndexes(db *gorm.DB) error {
	tableName := "composite_auto_models"
	if !isTableNameAllowed(tableName) {
		return fmt.Errorf("security: invalid table name %q", tableName)
	}

	// Use GORM's Quote method to properly escape table name
	quotedTableName := db.Statement.Quote(tableName)

	// Composite lookup index
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_composite_auto_model_profile_name
		ON ` + quotedTableName + `(profile_id, name)
	`).Error; err != nil {
		return fmt.Errorf("failed to create profile_name index: %w", err)
	}

	// Enabled models index
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_composite_auto_model_enabled
		ON ` + quotedTableName + `(enabled) WHERE enabled = true
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

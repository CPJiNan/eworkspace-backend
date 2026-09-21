package database

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"eworkspace/internal/config"
	"eworkspace/internal/model"
	"eworkspace/internal/pkg/password"
)

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn, err := buildDSN(cfg.Path)
	if err != nil {
		return nil, err
	}

	gormLogLevel := logger.Warn
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(gormLogLevel),
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层连接失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接不可用: %w", err)
	}
	return db, nil
}

func buildDSN(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("database.path 不能为空")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", fmt.Errorf("创建数据目录 %s 失败: %w", dir, err)
		}
	}
	return path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", nil
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.User{},
		&model.RefreshToken{},
		&model.Semester{},
		&model.Tag{},
		&model.Project{},
		&model.ProjectSemester{},
		&model.ProjectTag{},
		&model.Assignment{},
		&model.ProjectMember{},
		&model.Discussion{},
		&model.Notification{},
		&model.OperationLog{},
	); err != nil {
		return fmt.Errorf("自动迁移失败: %w", err)
	}

	if err := db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_type_name ON tags(type, name)`,
	).Error; err != nil {
		return fmt.Errorf("创建标签唯一索引失败: %w", err)
	}

	if err := db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_members_assignment_user ON project_members(assignment_id, student_id)`,
	).Error; err != nil {
		return fmt.Errorf("创建申领唯一索引失败: %w", err)
	}

	return nil
}

func Seed(db *gorm.DB, cfg config.BootstrapConfig, log *slog.Logger) error {
	studentID := cfg.SuperAdminStudentID
	if studentID == "" {
		studentID = "0"
	}

	var count int64
	if err := db.Model(&model.User{}).Where("student_id = ?", studentID).Count(&count).Error; err != nil {
		return fmt.Errorf("查询内置超管失败: %w", err)
	}
	if count > 0 {
		return nil
	}

	initialPassword := cfg.SuperAdminPassword
	if initialPassword == "" {
		initialPassword = model.DefaultPasswordFor(studentID)
	}
	hash, err := password.Default.Hash(initialPassword)
	if err != nil {
		return fmt.Errorf("生成超管密码失败: %w", err)
	}

	admin := model.User{
		StudentID:          studentID,
		Name:               cfg.SuperAdminName,
		Phone:              "",
		WeChat:             "",
		Email:              model.DefaultEmail(studentID),
		PasswordHash:       hash,
		Role:               model.RoleSuperAdmin,
		MustChangePassword: true,
	}
	if admin.Name == "" {
		admin.Name = "超级管理员"
	}
	if err := db.Create(&admin).Error; err != nil {
		return fmt.Errorf("创建内置超管失败: %w", err)
	}

	log.Warn("已创建内置超级管理员",
		"studentId", studentID,
		"initialPassword", initialPassword,
	)
	return nil
}

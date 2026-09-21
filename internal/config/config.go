package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	JWT       JWTConfig       `yaml:"jwt"`
	Log       LogConfig       `yaml:"log"`
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type JWTConfig struct {
	Secret           string `yaml:"secret"`
	AccessTTLMinutes int    `yaml:"access_ttl_minutes"`
	RefreshTTLHours  int    `yaml:"refresh_ttl_hours"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type BootstrapConfig struct {
	SuperAdminStudentID string `yaml:"super_admin_student_id"`
	SuperAdminName      string `yaml:"super_admin_name"`
	SuperAdminPassword  string `yaml:"super_admin_password"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
			Mode: "debug",
		},
		Database: DatabaseConfig{
			Path: "data/eworkspace.db",
		},
		JWT: JWTConfig{
			Secret:           "please-change-me-in-production",
			AccessTTLMinutes: 30,
			RefreshTTLHours:  168,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Bootstrap: BootstrapConfig{
			SuperAdminStudentID: "0",
			SuperAdminName:      "超级管理员",
			SuperAdminPassword:  "",
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		raw, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := yaml.Unmarshal(raw, cfg); err != nil {
				return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
			}
		case os.IsNotExist(err):
		default:
			return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
		}
	}

	applyEnv(cfg)

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("EWORKSPACE_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("EWORKSPACE_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("EWORKSPACE_SERVER_MODE"); v != "" {
		cfg.Server.Mode = v
	}
	if v := os.Getenv("EWORKSPACE_DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("EWORKSPACE_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}
	if v := os.Getenv("EWORKSPACE_JWT_ACCESS_TTL_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.JWT.AccessTTLMinutes = n
		}
	}
	if v := os.Getenv("EWORKSPACE_JWT_REFRESH_TTL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.JWT.RefreshTTLHours = n
		}
	}
	if v := os.Getenv("EWORKSPACE_SUPER_ADMIN_PASSWORD"); v != "" {
		cfg.Bootstrap.SuperAdminPassword = v
	}
	if v := os.Getenv("EWORKSPACE_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("EWORKSPACE_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
}

func (c *Config) validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 非法: %d", c.Server.Port)
	}
	if strings.TrimSpace(c.Database.Path) == "" {
		return fmt.Errorf("database.path 不能为空")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret 不能为空")
	}
	if c.Server.Mode == "release" && c.JWT.Secret == Default().JWT.Secret {
		return fmt.Errorf("生产模式必须通过 jwt.secret 或 EWORKSPACE_JWT_SECRET 设置 JWT 密钥")
	}
	if c.JWT.AccessTTLMinutes <= 0 {
		c.JWT.AccessTTLMinutes = 30
	}
	if c.JWT.RefreshTTLHours <= 0 {
		c.JWT.RefreshTTLHours = 168
	}
	if strings.TrimSpace(c.Bootstrap.SuperAdminStudentID) == "" {
		c.Bootstrap.SuperAdminStudentID = "0"
	}
	return nil
}

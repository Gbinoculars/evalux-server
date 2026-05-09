package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string

	// 默认管理员（首次启动无管理员时自动创建）
	AdminAccount  string
	AdminPassword string
	AdminNickname string

	// Cloudreve 对象存储
	CloudreveBaseURL  string
	CloudreveUser     string
	CloudrevePassword string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:password@127.0.0.1:5432/evalux?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "evalux-dev-secret-key-change-in-production"),

		// 默认管理员
		AdminAccount:  getEnv("ADMIN_ACCOUNT", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123456"),
		AdminNickname: getEnv("ADMIN_NICKNAME", "系统管理员"),

		// Cloudreve
		CloudreveBaseURL:  getEnv("CLOUDREVE_BASE_URL", "http://127.0.0.1:5212"),
		CloudreveUser:     getEnv("CLOUDREVE_USER", "root@root.com"),
		CloudrevePassword: getEnv("CLOUDREVE_PASSWORD", "123456"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

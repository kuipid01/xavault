package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName            string
	AppEnv             string
	AppPort            string
	DBHost             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBPort             string
	CORSAllowedOrigins string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		AppName:            getEnv("APP_NAME", "Salary Vault API"),
		AppEnv:             getEnv("APP_ENV", "development"),
		AppPort:            getEnv("APP_PORT", "8080"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBUser:             getEnv("DB_USER", "salary_vault"),
		DBPassword:         getEnv("DB_PASSWORD", "salary_vault_password"),
		DBName:             getEnv("DB_NAME", "salary_vault"),
		DBPort:             getEnv("DB_PORT", "5432"),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

package config

import (
	"fmt"
	"os"
	"strings"

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
	DBSSLMode          string
	AutoMigrate        bool
	CORSAllowedOrigins string
	AdminAPIKey        string
	JWTAccessSecret    string
	JWTRefreshSecret   string
	PaystackSecretKey  string
	BrevoAPIKey        string
	BrevoSenderName    string
	BrevoSenderEmail   string
	AlertEmailTo       string
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
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		AutoMigrate:        strings.EqualFold(getEnv("AUTO_MIGRATE", "false"), "true"),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"),
		AdminAPIKey:        os.Getenv("ADMIN_API_KEY"),
		JWTAccessSecret:    os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret:   os.Getenv("JWT_REFRESH_SECRET"),
		PaystackSecretKey:  os.Getenv("PAYSTACK_SECRET_KEY"),
		BrevoAPIKey:        os.Getenv("BREVO_API_KEY"),
		BrevoSenderName:    os.Getenv("BREVO_SENDER_NAME"),
		BrevoSenderEmail:   os.Getenv("BREVO_SENDER_EMAIL"),
		AlertEmailTo:       os.Getenv("ALERT_EMAIL_TO"),
	}
}

func (c *Config) Validate() error {
	if strings.EqualFold(c.AppEnv, "production") && c.AutoMigrate {
		return fmt.Errorf("AUTO_MIGRATE must be false in production")
	}
	if !strings.EqualFold(c.AppEnv, "production") {
		return nil
	}
	required := map[string]string{
		"JWT_ACCESS_SECRET":   c.JWTAccessSecret,
		"JWT_REFRESH_SECRET":  c.JWTRefreshSecret,
		"PAYSTACK_SECRET_KEY": c.PaystackSecretKey,
		"BREVO_API_KEY":       c.BrevoAPIKey,
		"BREVO_SENDER_EMAIL":  c.BrevoSenderEmail,
		"ALERT_EMAIL_TO":      c.AlertEmailTo,
		"ADMIN_API_KEY":       c.AdminAPIKey,
		"DB_PASSWORD":         c.DBPassword,
	}
	for name, value := range required {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s is required in production", name)
		}
		if (strings.Contains(name, "SECRET") || strings.Contains(name, "PASSWORD") || name == "ADMIN_API_KEY") && len(value) < 32 {
			return fmt.Errorf("%s must contain at least 32 characters in production", name)
		}
		lower := strings.ToLower(value)
		if strings.Contains(lower, "change_me") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") {
			return fmt.Errorf("%s uses an unsafe placeholder value", name)
		}
	}
	switch strings.ToLower(strings.TrimSpace(c.DBSSLMode)) {
	case "require", "verify-ca", "verify-full":
	default:
		return fmt.Errorf("DB_SSLMODE must require TLS in production")
	}
	return nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

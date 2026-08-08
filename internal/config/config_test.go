package config

import (
	"strings"
	"testing"
)

func TestProductionConfigRejectsWeakSecrets(t *testing.T) {
	cfg := &Config{AppEnv: "production"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production validation failure")
	}
}

func TestProductionConfigAcceptsStrongSecrets(t *testing.T) {
	strong := strings.Repeat("x", 40)
	cfg := &Config{
		AppEnv: "production", JWTAccessSecret: strong, JWTRefreshSecret: strong + "r",
		PaystackSecretKey: strong + "p", BrevoAPIKey: strong + "b",
		BrevoSenderEmail: "alerts@example.com", AlertEmailTo: "operator@example.com",
		AdminAPIKey: strong + "a", DBPassword: strong + "d", DBSSLMode: "verify-full",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestProductionConfigRejectsDisabledDatabaseTLS(t *testing.T) {
	strong := strings.Repeat("x", 40)
	cfg := &Config{
		AppEnv: "production", JWTAccessSecret: strong, JWTRefreshSecret: strong + "r",
		PaystackSecretKey: strong + "p", BrevoAPIKey: strong + "b",
		BrevoSenderEmail: "alerts@example.com", AlertEmailTo: "operator@example.com",
		AdminAPIKey: strong + "a", DBPassword: strong + "d", DBSSLMode: "disable",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production database TLS validation failure")
	}
}

func TestProductionConfigRejectsAutoMigrate(t *testing.T) {
	cfg := &Config{AppEnv: "production", AutoMigrate: true}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production AutoMigrate validation failure")
	}
}

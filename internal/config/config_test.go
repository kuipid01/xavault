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
		AdminAPIKey: strong + "a", DBPassword: strong + "d",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

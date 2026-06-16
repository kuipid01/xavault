package security

import (
	"os"

	"github.com/google/uuid"
)

func IsProduction() bool {
	return os.Getenv("APP_ENV") == "production"
}

func GenerateReference() string {
	return uuid.New().String()
}

func GenateUniqueID() string {
	return uuid.New().String()
}

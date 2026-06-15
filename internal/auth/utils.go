package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"unicode"
)

func generateToken() string {

	bytes := make([]byte, 32)

	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}

	return base64.RawURLEncoding.EncodeToString(bytes)

}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func mustContainSpecialCharacters(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			return true
		}
	}
	return false
}

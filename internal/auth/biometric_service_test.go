package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestVerifyRS256BiometricSignature(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseBiometricPublicKey(base64.StdEncoding.EncodeToString(der), "RS256")
	if err != nil {
		t.Fatal(err)
	}
	payload := biometricSigningPayload("challenge-id", "random-challenge")
	digest := sha256.Sum256([]byte(payload))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyBiometricSignature(parsed, "RS256", payload, base64.StdEncoding.EncodeToString(signature)); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err = verifyBiometricSignature(parsed, "RS256", payload+"tampered", base64.StdEncoding.EncodeToString(signature)); err == nil {
		t.Fatal("tampered payload was accepted")
	}
}

func TestVerifyES256BiometricSignature(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseBiometricPublicKey(base64.StdEncoding.EncodeToString(der), "ES256")
	if err != nil {
		t.Fatal(err)
	}
	payload := biometricSigningPayload("challenge-id", "random-challenge")
	digest := sha256.Sum256([]byte(payload))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if err = verifyBiometricSignature(parsed, "ES256", payload, base64.StdEncoding.EncodeToString(signature)); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestPublicKeyAlgorithmMustMatch(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = parseBiometricPublicKey(base64.StdEncoding.EncodeToString(der), "ES256"); err == nil {
		t.Fatal("RSA key accepted as ES256")
	}
}

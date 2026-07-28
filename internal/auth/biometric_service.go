package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

const biometricChallengeLifetime = 2 * time.Minute

func biometricSigningPayload(challengeID, challenge string) string {
	return "xacvault-biometric-v1:" + challengeID + ":" + challenge
}

func parseBiometricPublicKey(value, algorithm string) (crypto.PublicKey, error) {
	value = strings.TrimSpace(value)
	der := []byte(nil)
	if block, _ := pem.Decode([]byte(value)); block != nil {
		der = block.Bytes
	} else {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, errors.New("public_key must be PEM or base64-encoded SPKI")
		}
		der = decoded
	}
	key, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, errors.New("invalid public key")
	}
	switch algorithm {
	case "RS256":
		if _, ok := key.(*rsa.PublicKey); !ok {
			return nil, errors.New("public key does not match RS256")
		}
	case "ES256":
		if _, ok := key.(*ecdsa.PublicKey); !ok {
			return nil, errors.New("public key does not match ES256")
		}
	default:
		return nil, errors.New("key_algorithm must be RS256 or ES256")
	}
	return key, nil
}

func verifyBiometricSignature(key crypto.PublicKey, algorithm, payload, encodedSignature string) error {
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedSignature))
	if err != nil {
		return errors.New("invalid signature encoding")
	}
	digest := sha256.Sum256([]byte(payload))
	switch publicKey := key.(type) {
	case *rsa.PublicKey:
		if algorithm != "RS256" {
			return errors.New("signature algorithm mismatch")
		}
		return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature)
	case *ecdsa.PublicKey:
		if algorithm != "ES256" || !ecdsa.VerifyASN1(publicKey, digest[:], signature) {
			return errors.New("invalid signature")
		}
		return nil
	default:
		return errors.New("unsupported public key")
	}
}

func biometricFailure(message, code string) *response.APIResponse {
	return &response.APIResponse{Success: false, Message: message, Error: code}
}

func (s *AuthService) EnrollBiometricDevice(userID string, req EnrollBiometricDeviceRequest) *response.APIResponse {
	req.DeviceID, req.DeviceName = strings.TrimSpace(req.DeviceID), strings.TrimSpace(req.DeviceName)
	req.Platform, req.KeyAlgorithm = strings.TrimSpace(req.Platform), strings.ToUpper(strings.TrimSpace(req.KeyAlgorithm))
	if req.DeviceID == "" || len(req.DeviceID) > 128 || strings.TrimSpace(req.PublicKey) == "" {
		return biometricFailure("Device ID and public key are required", "invalid_biometric_device")
	}
	if req.DeviceName == "" {
		req.DeviceName = "My device"
	}
	if len(req.DeviceName) > 128 || len(req.Platform) > 32 {
		return biometricFailure("Invalid device metadata", "invalid_device_metadata")
	}
	if _, err := parseBiometricPublicKey(req.PublicKey, req.KeyAlgorithm); err != nil {
		return biometricFailure(err.Error(), "invalid_public_key")
	}
	device := &BiometricDevice{UserID: userID, DeviceID: req.DeviceID, DeviceName: req.DeviceName, Platform: req.Platform, PublicKey: strings.TrimSpace(req.PublicKey), KeyAlgorithm: req.KeyAlgorithm}
	if err := s.authRepo.UpsertBiometricDevice(device); err != nil {
		return biometricFailure("Could not register biometric device", err.Error())
	}
	return &response.APIResponse{Success: true, Message: "Biometric device registered", Data: fiber.Map{"device_id": req.DeviceID}}
}

func (s *AuthService) ListBiometricDevices(userID string) *response.APIResponse {
	devices, err := s.authRepo.ListBiometricDevices(userID)
	if err != nil {
		return biometricFailure("Could not retrieve biometric devices", err.Error())
	}
	return &response.APIResponse{Success: true, Message: "Biometric devices retrieved", Data: devices}
}

func (s *AuthService) RevokeBiometricDevice(userID, deviceID string) *response.APIResponse {
	revoked, err := s.authRepo.RevokeBiometricDevice(userID, strings.TrimSpace(deviceID))
	if err != nil {
		return biometricFailure("Could not revoke biometric device", err.Error())
	}
	if !revoked {
		return biometricFailure("Biometric device not found", "biometric_device_not_found")
	}
	return &response.APIResponse{Success: true, Message: "Biometric device revoked"}
}

func (s *AuthService) CreateBiometricChallenge(req BiometricChallengeRequest) *response.APIResponse {
	device, err := s.authRepo.FindActiveBiometricDevice(strings.TrimSpace(req.DeviceID))
	if err != nil || device == nil {
		return biometricFailure("Biometric login is unavailable", "biometric_login_unavailable")
	}
	randomBytes := make([]byte, 32)
	if _, err = rand.Read(randomBytes); err != nil {
		return biometricFailure("Could not create login challenge", "challenge_generation_failed")
	}
	challengeValue := base64.RawURLEncoding.EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(challengeValue))
	challenge := &BiometricChallenge{CredentialID: device.ID, ChallengeHash: hex.EncodeToString(hash[:]), ExpiresAt: time.Now().UTC().Add(biometricChallengeLifetime)}
	if err = s.authRepo.CreateBiometricChallenge(challenge); err != nil {
		return biometricFailure("Could not create login challenge", err.Error())
	}
	data := BiometricChallengeResponse{ChallengeID: challenge.ID, Challenge: challengeValue, SigningPayload: biometricSigningPayload(challenge.ID, challengeValue), ExpiresAt: challenge.ExpiresAt}
	return &response.APIResponse{Success: true, Message: "Biometric challenge created", Data: data}
}

func (s *AuthService) VerifyBiometricChallenge(req VerifyBiometricRequest) *response.APIResponse {
	device, err := s.authRepo.FindActiveBiometricDevice(strings.TrimSpace(req.DeviceID))
	if err != nil || device == nil {
		return biometricFailure("Biometric authentication failed", "invalid_biometric_challenge")
	}
	challenge, err := s.authRepo.FindBiometricChallenge(strings.TrimSpace(req.ChallengeID), device.ID)
	now := time.Now().UTC()
	if err != nil || challenge == nil || challenge.UsedAt != nil || !challenge.ExpiresAt.After(now) {
		return biometricFailure("Biometric authentication failed", "invalid_biometric_challenge")
	}
	hash := sha256.Sum256([]byte(req.Challenge))
	if hex.EncodeToString(hash[:]) != challenge.ChallengeHash {
		return biometricFailure("Biometric authentication failed", "invalid_biometric_challenge")
	}
	key, err := parseBiometricPublicKey(device.PublicKey, device.KeyAlgorithm)
	if err != nil || verifyBiometricSignature(key, device.KeyAlgorithm, biometricSigningPayload(challenge.ID, req.Challenge), req.Signature) != nil {
		return biometricFailure("Biometric authentication failed", "invalid_biometric_signature")
	}
	consumed, err := s.authRepo.ConsumeBiometricChallenge(challenge.ID, now)
	if err != nil || !consumed {
		return biometricFailure("Biometric authentication failed", "biometric_challenge_already_used")
	}
	user, err := s.repository.FindById(device.UserID)
	if err != nil || user == nil {
		return biometricFailure("Biometric authentication failed", "user_not_found")
	}
	accessToken, refreshToken, err := GenerateAllToken(user.ID, user.Email, s.tokenService)
	if err != nil {
		return biometricFailure("Could not generate tokens", "token_generation_failed")
	}
	if err = s.repository.UpdateRefreshToken(user.ID, refreshToken); err != nil {
		return biometricFailure("Could not save refresh token", err.Error())
	}
	_ = s.authRepo.TouchBiometricDevice(device.ID, now)
	return &response.APIResponse{Success: true, Message: "Biometric login successful", Data: map[string]interface{}{
		"access_token": accessToken, "refresh_token": refreshToken,
		"user": map[string]interface{}{"id": user.ID, "email": user.Email, "mail_verified": user.MailVerified, "email_verified": user.EmailVerifiedAt, "name": fmt.Sprintf("%s %s", user.FirstName, user.LastName)},
	}}
}

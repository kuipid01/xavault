package auth

import "time"

type EnrollBiometricDeviceRequest struct {
	DeviceID     string `json:"device_id"`
	DeviceName   string `json:"device_name"`
	Platform     string `json:"platform"`
	PublicKey    string `json:"public_key"`
	KeyAlgorithm string `json:"key_algorithm"`
}

type BiometricChallengeRequest struct {
	DeviceID string `json:"device_id"`
}

type VerifyBiometricRequest struct {
	DeviceID    string `json:"device_id"`
	ChallengeID string `json:"challenge_id"`
	Challenge   string `json:"challenge"`
	Signature   string `json:"signature"`
}

type BiometricChallengeResponse struct {
	ChallengeID    string    `json:"challenge_id"`
	Challenge      string    `json:"challenge"`
	SigningPayload string    `json:"signing_payload"`
	ExpiresAt      time.Time `json:"expires_at"`
}

package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *AuthRepository) UpsertBiometricDevice(device *BiometricDevice) error {
	var existing BiometricDevice
	err := r.db.Where("device_id = ?", device.DeviceID).First(&existing).Error
	if err == nil && existing.UserID != device.UserID {
		return errors.New("device is already registered to another account")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		device.ID = uuid.NewString()
		return r.db.Create(device).Error
	}
	return r.db.Model(&existing).Updates(map[string]interface{}{
		"device_name": device.DeviceName, "platform": device.Platform,
		"public_key": device.PublicKey, "key_algorithm": device.KeyAlgorithm,
		"revoked_at": nil,
	}).Error
}

func (r *AuthRepository) ListBiometricDevices(userID string) ([]BiometricDevice, error) {
	var devices []BiometricDevice
	err := r.db.Where("user_id = ? AND revoked_at IS NULL", userID).Order("created_at DESC").Find(&devices).Error
	return devices, err
}

func (r *AuthRepository) RevokeBiometricDevice(userID, deviceID string) (bool, error) {
	result := r.db.Model(&BiometricDevice{}).Where("user_id = ? AND device_id = ? AND revoked_at IS NULL", userID, deviceID).Update("revoked_at", time.Now().UTC())
	return result.RowsAffected == 1, result.Error
}

func (r *AuthRepository) FindActiveBiometricDevice(deviceID string) (*BiometricDevice, error) {
	var device BiometricDevice
	err := r.db.Where("device_id = ? AND revoked_at IS NULL", deviceID).First(&device).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &device, err
}

func (r *AuthRepository) CreateBiometricChallenge(challenge *BiometricChallenge) error {
	challenge.ID = uuid.NewString()
	return r.db.Create(challenge).Error
}

func (r *AuthRepository) FindBiometricChallenge(id, credentialID string) (*BiometricChallenge, error) {
	var challenge BiometricChallenge
	err := r.db.Where("id = ? AND credential_id = ?", id, credentialID).First(&challenge).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &challenge, err
}

func (r *AuthRepository) ConsumeBiometricChallenge(id string, usedAt time.Time) (bool, error) {
	result := r.db.Model(&BiometricChallenge{}).Clauses(clause.Returning{}).
		Where("id = ? AND used_at IS NULL AND expires_at > ?", id, usedAt).
		Update("used_at", usedAt)
	return result.RowsAffected == 1, result.Error
}

func (r *AuthRepository) TouchBiometricDevice(id string, usedAt time.Time) error {
	return r.db.Model(&BiometricDevice{}).Where("id = ?").Update("last_used_at", usedAt).Error
}

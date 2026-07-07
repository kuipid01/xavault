package users

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) UpdateUserToken(email, token string, expiresAt time.Time) error {
	return r.db.Model(&User{}).Where("email = ?", email).Updates(map[string]interface{}{
		"verification_token":            token,
		"verification_token_expires_at": expiresAt,
	}).Error
}

func (r *UserRepository) FindByEmail(email string) (*User, error) {
	var user User

	err := r.db.Where("email = ?", email).First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) FindByNickname(nickname string) (*User, error) {
	var user User
	nickname = strings.ToLower(strings.TrimSpace(nickname))

	err := r.db.Where("LOWER(nickname) = ?", nickname).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}
func (r *UserRepository) Create(user *User) (*User, error) {
	result := r.db.Create(user)
	return user, result.Error
}

func (r *UserRepository) FindVerificationToken(token string) (*User, error) {
	var user User

	err := r.db.Where("verification_token = ?", token).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) MarkEmailAsVerified(userID string) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Update("mail_verified", true).Error
}

func (r *UserRepository) DeleteVerificationToken(token string) error {
	return r.db.Model(&User{}).Where("verification_token = ?", token).Update("verification_token", nil).Error
}

func (r *UserRepository) ResetPassword(userID, hashedPassword string) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"password": hashedPassword,
	}).Error
}

func (r *UserRepository) VerifyEmail(token string) (*User, error) {
	var user User

	err := r.db.Where("verification_token = ?", token).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetUserWithToken(token string) (*User, error) {
	var user User

	err := r.db.Where("refresh_token = ?", token).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) UpdateRefreshToken(userID string, refreshToken string) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Update("refresh_token", refreshToken).Error
}

func (r *UserRepository) UpdateWithdrawalPin(userID string, pinHash string) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"withdrawal_pin_hash":             pinHash,
		"withdrawal_pin_reset_otp":        nil,
		"withdrawal_pin_reset_expires_at": nil,
	}).Error
}

func (r *UserRepository) UpdateWithdrawalPinResetOTP(userID string, otpHash string, expiresAt time.Time) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"withdrawal_pin_reset_otp":        otpHash,
		"withdrawal_pin_reset_expires_at": expiresAt,
	}).Error
}

func (r *UserRepository) UpdateProfile(userID string, updates map[string]interface{}) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Updates(updates).Error
}

func (r *UserRepository) ReplaceSecurityQuestions(userID string, questions []SecurityQuestion) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&SecurityQuestion{}).Error; err != nil {
			return err
		}
		if len(questions) == 0 {
			return nil
		}
		return tx.Create(&questions).Error
	})
}

func (r *UserRepository) ListSecurityQuestions(userID string) ([]SecurityQuestion, error) {
	var questions []SecurityQuestion
	err := r.db.Where("user_id = ?", userID).Order("created_at ASC").Find(&questions).Error
	return questions, err
}

func (r *UserRepository) ClearWithdrawalPinResetOTP(userID string) error {
	return r.db.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"withdrawal_pin_reset_otp":        nil,
		"withdrawal_pin_reset_expires_at": nil,
	}).Error
}

func (r *UserRepository) FindById(id string) (*User, error) {
	var user User

	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &user, nil
}

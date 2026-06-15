package users

import (
	"errors"
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

package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kuipid01/salary-vault/internal/platform/mail"
	"github.com/kuipid01/salary-vault/internal/shared/response"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/wallets"
)

type AuthService struct {
	repository    *users.UserRepository
	mailer        mail.Mailer
	tokenService  *TokenService
	walletService *wallets.WalletService
}

func NewAuthService(repository *users.UserRepository, mailer mail.Mailer, tokenService *TokenService, walletService *wallets.WalletService) *AuthService {
	return &AuthService{repository: repository, mailer: mailer, tokenService: tokenService, walletService: walletService}
}

func (s *AuthService) Register(req RegisterRequest) *response.APIResponse {
	validData, validationErrors := validateRegistrationData(req)
	var finalUser users.User
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	userExists, err := s.repository.FindByEmail(validData.Email)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not check email",
			Error:   err,
		}
	}

	if userExists != nil {
		if !userExists.MailVerified {
			_, err = s.userVerificationFlow(userExists.Email, "register")
			if err != nil {
				return &response.APIResponse{
					Success: false,
					Message: "We could not reverify user",
					Error:   err,
				}
			}
			return &response.APIResponse{
				Success: true,
				Message: "Resent Verifcation link",
				Data:    validData.Email,
			}
		} else {
			return &response.APIResponse{
				Success: false,
				Message: "Email already exists",
				Error:   ErrEmailAlreadyExists,
			}
		}

	}
	hashedPassword, err := hashPassword(validData.Password)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "We could not hash the password",
			Error:   err,
		}
	}
	now := time.Now()
	finalUser = users.User{
		ID:              uuid.Must(uuid.NewV7()).String(),
		Email:           validData.Email,
		Password:        hashedPassword,
		EmailVerifiedAt: &now,
		MailVerified:    false,
	}
	_, err = s.repository.Create(&finalUser)
	_, err = s.userVerificationFlow(finalUser.Email, "register")
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "We could not create the user",
			Error:   err,
		}
	}
	s.walletService.CreateWallet(finalUser.ID)
	return &response.APIResponse{
		Success: true,
		Message: "Registration successful",
		Data:    validData.Email,
	}
}

func (s *AuthService) Login(req LoginRequest) *response.APIResponse {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	validationErrors := validateLoginData(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	user, err := s.repository.FindByEmail(req.Email)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not login user",
			Error:   err,
		}
	}

	if user == nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid email or password",
			Error:   "invalid_credentials",
		}
	}

	if !user.MailVerified {
		return &response.APIResponse{
			Success: false,
			Message: "Please verify your email before logging in",
			Error:   "email_not_verified",
		}
	}

	passwordIsValid := CheckPasswordHash(req.Password, user.Password)
	if !passwordIsValid {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid email or password",
			Error:   "invalid_credentials",
		}
	}
	fmt.Println("USER FROM SERVICE", user)
	accessToken, refreshToken, err := GenerateAllToken(user.ID, user.Email, s.tokenService)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not generate tokens",
			Error:   err,
		}
	}

	if err := s.repository.UpdateRefreshToken(user.ID, refreshToken); err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not save refresh token",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Login successful",
		Data: map[string]interface{}{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"user": map[string]interface{}{
				"id":             user.ID,
				"email":          user.Email,
				"mail_verified":  user.MailVerified,
				"email_verified": user.EmailVerifiedAt,
			},
		},
	}
}

func (s *AuthService) ValidateMail(req ValidateMailRequest) *response.APIResponse {
	req.Token = strings.TrimSpace(req.Token)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Token != "" {
		return s.VerifyEmail(req.Token)
	}

	if req.Email == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Email or token is required",
			Error:   "email_or_token_required",
		}
	}

	user, err := s.repository.FindByEmail(req.Email)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not check email verification",
			Error:   err.Error(),
		}
	}

	if user == nil {
		return &response.APIResponse{
			Success: false,
			Message: "User not found",
			Error:   "user_not_found",
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Email verification status fetched",
		Data: fiber.Map{
			"email":         user.Email,
			"mail_verified": user.MailVerified,
		},
	}
}

func (s *AuthService) ForgetPassword(req ForgetPasswordRequest) *response.APIResponse {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	validationErrors := validateForgetPasswordData(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	user, err := s.repository.FindByEmail(req.Email)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not find user",
			Error:   err,
		}
	}

	if user == nil {
		return &response.APIResponse{
			Success: false,
			Message: "User not found",
			Error:   "user_not_found",
		}
	}

	_, err = s.userVerificationFlow(user.Email, "forgetPassword")
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not send forget password email",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Forget password email sent successfully",
	}
}
func (s *AuthService) ResetPassword(req ResetPasswordRequest) *response.APIResponse {
	req.Password = strings.TrimSpace(req.Password)
	req.ConfirmPassword = strings.TrimSpace(req.ConfirmPassword)

	validationErrors := validateResetPasswordData(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	hashedToken := hashToken(req.Token)

	user, err := s.CheckTokenValidity(req.Token)
	fmt.Println(err)

	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not verify token",
			Error:   err.Error(),
		}
	}

	userExists, err := s.repository.FindByEmail(user.Email)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not find user",
			Error:   err.Error(),
		}
	}

	if userExists == nil {
		return &response.APIResponse{
			Success: false,
			Message: "User not found",
			Error:   "user_not_found",
		}
	}

	if CheckPasswordHash(req.Password, userExists.Password) {
		return &response.APIResponse{
			Success: false,
			Message: "Password already exists",
			Error:   "password_already_exists",
		}
	}

	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not hash password",
			Error:   err.Error(),
		}
	}

	err = s.repository.ResetPassword(userExists.ID, hashedPassword)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not update password",
			Error:   err.Error(),
		}
	}

	err = s.repository.DeleteVerificationToken(hashedToken)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not delete verification token",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Password reset successfully",
	}
}

func (s *AuthService) Logout(req ResetPasswordRequest) *response.APIResponse {

	return &response.APIResponse{
		Success: true,
		Message: "Logout successful",
	}

}

func (s *AuthService) ResendVerificationMail(req ResetPasswordRequest) *response.APIResponse {
	return nil
}

func (s *AuthService) Me(req MeRequest) *response.APIResponse {

	user, err := s.repository.FindById(req.UserID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not find user",
			Error:   err.Error(),
		}
	}
	if user == nil {
		return &response.APIResponse{
			Success: false,
			Message: "User not found",
			Error:   "user_not_found",
		}
	}
	return &response.APIResponse{
		Success: true,
		Message: "User found",
		Data:    user,
	}
}

func (s *AuthService) RefreshToken(req RefreshTokenRequest) *response.APIResponse {
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)

	if req.RefreshToken == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Refresh token is required",
			Error:   "refresh_token_required",
		}
	}

	claims, err := s.tokenService.ValidateToken(req.RefreshToken, TokenTypeRefresh)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid refresh token",
			Error:   "invalid_refresh_token",
		}
	}

	user, err := s.repository.GetUserWithToken(req.RefreshToken)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not refresh token",
			Error:   err.Error(),
		}
	}

	if user == nil || user.ID != claims.UserID {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid refresh token",
			Error:   "invalid_refresh_token",
		}
	}

	accessToken, newRefreshToken, err := GenerateAllToken(user.ID, user.Email, s.tokenService)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not generate tokens",
			Error:   err,
		}
	}

	err = s.repository.UpdateRefreshToken(user.ID, newRefreshToken)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not update refresh token",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Token refreshed successfully",
		Data: fiber.Map{
			"access_token":  accessToken,
			"refresh_token": newRefreshToken,
		},
	}
}
func (s *AuthService) ChangePassword(req ResetPasswordRequest) *response.APIResponse {
	return nil
}

// /helpers
func (s *AuthService) userVerificationFlow(email string, flowType string) (bool, error) {
	token := generateToken()
	tokenHash := hashToken(token)
	mailExpiresAt := time.Now().Add(30 * time.Minute)
	if err := s.repository.UpdateUserToken(email, tokenHash, mailExpiresAt); err != nil {
		return false, err
	}
	if flowType == "forgetPassword" {
		if err := s.sendForgetPasswordMail(email, token); err != nil {
			return false, err
		}
	} else {
		if err := s.sendVerificationEmail(email, token); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (s *AuthService) sendVerificationEmail(email, token string) error {
	env := os.Getenv("APP_ENV")
	var verifyURL string
	if env == "production" {
		frontendURL := os.Getenv("APP_FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "https://xavault.kuipid.com"
		}
		verifyURL = fmt.Sprintf("%s/verify-email?token=%s", frontendURL, token)
	} else {
		localURL := os.Getenv("APP_LOCAL_URL")
		if localURL == "" {
			localURL = "http://localhost:3000"
		}
		verifyURL = fmt.Sprintf("%s/verify-email?token=%s", localURL, token)
	}
	fmt.Println(verifyURL)
	err := s.mailer.SendMailVerification(email, verifyURL)
	return err
}
func (s *AuthService) sendForgetPasswordMail(email, token string) error {
	env := os.Getenv("APP_ENV")
	var resetPasswordURL string
	if env == "production" {
		frontendURL := os.Getenv("APP_FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "https://xavault.kuipid.com"
		}
		resetPasswordURL = fmt.Sprintf("%s/reset-password?token=%s", frontendURL, token)
	} else {
		localURL := os.Getenv("APP_LOCAL_URL")
		if localURL == "" {
			localURL = "http://localhost:3000"
		}
		resetPasswordURL = fmt.Sprintf("%s/reset-password?token=%s", localURL, token)
	}
	fmt.Println(resetPasswordURL, "RESET PASSWORD URL")
	err := s.mailer.SendForgetPasswordMail(email, resetPasswordURL)
	return err
}

func (s *AuthService) VerifyEmail(rawToken string) *response.APIResponse {
	hashedToken := hashToken(rawToken)
	tokenRecord, err := s.CheckTokenValidity(rawToken)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not verify email",
			Error:   err,
		}
	}

	err = s.repository.MarkEmailAsVerified(tokenRecord.ID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not verify email",
			Error:   err,
		}
	}

	err = s.repository.DeleteVerificationToken(hashedToken)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not delete verification token",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Email verified successfully",
	}
}

func (s *AuthService) CheckTokenValidity(rawToken string) (*users.User, error) {
	fmt.Println("Checking token validity...")

	hashedToken := hashToken(rawToken)
	fmt.Println("Token hashed successfully:", hashedToken)

	tokenRecord, err := s.repository.FindVerificationToken(hashedToken)
	if err != nil {
		fmt.Println("Error finding verification token:", err)
		return nil, errors.New("Invalid verification token")
	}

	if tokenRecord == nil {
		fmt.Println("No token record found for hashed token:", hashedToken)
		return nil, errors.New("Invalid verification token")
	}

	fmt.Println("Token record found for user ID:", tokenRecord.ID)

	if tokenRecord.VerificationTokenExpiresAt == nil {
		fmt.Println("Verification token expiry time is nil")
		return nil, errors.New("Invalid verification token")
	}

	fmt.Println("Token expires at:", *tokenRecord.VerificationTokenExpiresAt)
	fmt.Println("Current time:", time.Now())

	if time.Now().After(*tokenRecord.VerificationTokenExpiresAt) {
		fmt.Println("Verification token has expired")
		return nil, errors.New("Verification token has expired")
	}

	fmt.Println("Verification token is valid")

	return tokenRecord, nil
}

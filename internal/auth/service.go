package auth

import (
	"errors"
	"fmt"
	"regexp"
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
	authRepo      *AuthRepository
	mailer        mail.Mailer
	tokenService  *TokenService
	walletService *wallets.WalletService
}

func NewAuthService(repository *users.UserRepository, authRepo *AuthRepository, mailer mail.Mailer, tokenService *TokenService, walletService *wallets.WalletService) *AuthService {
	return &AuthService{repository: repository, authRepo: authRepo, mailer: mailer, tokenService: tokenService, walletService: walletService}
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
		return &response.APIResponse{
			Success: false,
			Message: "Email already exists",
			Error:   ErrEmailAlreadyExists,
		}
	}

	temporaryUser, err := s.authRepo.FindTemporaryUserByEmail(validData.Email)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not check email verification",
			Error:   err.Error(),
		}
	}
	if temporaryUser == nil || !temporaryUser.Verified {
		return &response.APIResponse{
			Success: false,
			Message: "Verify your email before creating your account",
			Error:   "email_not_verified",
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
		FirstName:       validData.FirstName,
		LastName:        validData.LastName,
		EmailVerifiedAt: &now,
		MailVerified:    true,
		Country:         validData.Country,
		PhoneNumber:     validData.PhoneNumber,
	}
	nickname := normalizeNickname(req.Nickname)
	if nickname != "" {
		if !validNickname(nickname) {
			return &response.APIResponse{
				Success: false,
				Message: "Invalid username",
				Error:   "invalid_nickname",
			}
		}
		existingNickname, err := s.repository.FindByNickname(nickname)
		if err != nil {
			return &response.APIResponse{
				Success: false,
				Message: "Could not check username",
				Error:   err.Error(),
			}
		}
		if existingNickname != nil {
			return &response.APIResponse{
				Success: false,
				Message: "Username is not available",
				Error:   "nickname_unavailable",
			}
		}
		finalUser.Nickname = &nickname
	}
	_, err = s.repository.Create(&finalUser)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "We could not create the user",
			Error:   err,
		}
	}
	s.walletService.CreateWallet(finalUser.ID)
	_ = s.authRepo.DeleteTemporaryUser(finalUser.Email)
	return &response.APIResponse{
		Success: true,
		Message: "Registration successful",
		Data:    validData.Email,
	}
}

func (s *AuthService) StartRegistration(req StartRegistrationRequest) *response.APIResponse {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || !strings.Contains(email, "@") {
		return &response.APIResponse{
			Success: false,
			Message: "Enter a valid email",
			Error:   "invalid_email",
		}
	}

	userExists, err := s.repository.FindByEmail(email)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not check email",
			Error:   err.Error(),
		}
	}
	if userExists != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Email already exists",
			Error:   ErrEmailAlreadyExists,
		}
	}

	temporaryUser, err := s.authRepo.FindTemporaryUserByEmail(email)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not check registration progress",
			Error:   err.Error(),
		}
	}
	if temporaryUser != nil && temporaryUser.Verified {
		return &response.APIResponse{
			Success: true,
			Message: "Email already verified. Continue registration.",
			Data: fiber.Map{
				"email":         email,
				"mail_verified": true,
				"resume":        true,
			},
		}
	}

	otp, err := generateNumericOTP(6)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not generate verification OTP",
			Error:   err.Error(),
		}
	}

	record := &TemporaryUser{
		ID:         uuid.Must(uuid.NewV7()).String(),
		Email:      email,
		TokenHash:  hashToken(otp),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
		Verified:   false,
		VerifiedAt: nil,
	}
	if err := s.authRepo.UpsertTemporaryUser(record); err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not save verification OTP",
			Error:   err.Error(),
		}
	}

	if err := s.sendVerificationEmail(email, otp); err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not send verification email",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Verification OTP sent",
		Data: fiber.Map{
			"email": email,
		},
	}
}

func (s *AuthService) VerifyRegistrationMail(req VerifyRegistrationMailRequest) *response.APIResponse {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	otp := strings.TrimSpace(req.OTP)
	if email == "" || !strings.Contains(email, "@") {
		return &response.APIResponse{Success: false, Message: "Enter a valid email", Error: "invalid_email"}
	}
	if len(otp) != 6 {
		return &response.APIResponse{Success: false, Message: "OTP must be 6 digits", Error: "invalid_otp"}
	}

	record, err := s.authRepo.FindTemporaryUserByEmail(email)
	if err != nil {
		return &response.APIResponse{Success: false, Message: "Could not check OTP", Error: err.Error()}
	}
	if record == nil {
		return &response.APIResponse{Success: false, Message: "Start registration again", Error: "registration_not_started"}
	}
	if time.Now().After(record.ExpiresAt) {
		return &response.APIResponse{Success: false, Message: "Verification OTP has expired", Error: "otp_expired"}
	}
	if record.TokenHash != hashToken(otp) {
		return &response.APIResponse{Success: false, Message: "Invalid verification OTP", Error: "invalid_otp"}
	}

	now := time.Now()
	if err := s.authRepo.MarkTemporaryUserVerified(email, now); err != nil {
		return &response.APIResponse{Success: false, Message: "Could not verify email", Error: err.Error()}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Email verified successfully",
		Data: fiber.Map{
			"email": email,
		},
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
				"name":           user.FirstName + " " + user.LastName,
			},
		},
	}
}

func (s *AuthService) ValidateMail(req ValidateMailRequest) *response.APIResponse {
	req.Token = strings.TrimSpace(req.Token)
	req.OTP = strings.TrimSpace(req.OTP)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.OTP != "" || req.Token != "" {
		rawOTP := req.OTP
		if rawOTP == "" {
			rawOTP = req.Token
		}
		return s.VerifyEmail(rawOTP)
	}

	if req.Email == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Email or OTP is required",
			Error:   "email_or_otp_required",
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
		Message: "Password reset OTP sent successfully",
	}
}
func (s *AuthService) ResetPassword(req ResetPasswordRequest) *response.APIResponse {
	req.Password = strings.TrimSpace(req.Password)
	req.ConfirmPassword = strings.TrimSpace(req.ConfirmPassword)
	req.Token = strings.TrimSpace(req.Token)
	req.OTP = strings.TrimSpace(req.OTP)

	validationErrors := validateResetPasswordData(req)
	if len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   validationErrors,
		}
	}

	rawOTP := req.OTP
	if rawOTP == "" {
		rawOTP = req.Token
	}

	hashedToken := hashToken(rawOTP)

	user, err := s.CheckTokenValidity(rawOTP)
	fmt.Println(err)

	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not verify OTP",
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

func (s *AuthService) Logout(req RefreshTokenRequest) *response.APIResponse {
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken != "" {
		user, err := s.repository.GetUserWithToken(req.RefreshToken)
		if err != nil {
			return &response.APIResponse{Success: false, Message: "Could not log out", Error: err.Error()}
		}
		if user != nil {
			if err := s.repository.UpdateRefreshToken(user.ID, ""); err != nil {
				return &response.APIResponse{Success: false, Message: "Could not log out", Error: err.Error()}
			}
		}
	}
	return &response.APIResponse{
		Success: true,
		Message: "Logout successful",
		Data:    fiber.Map{"logged_out": true},
	}
}

func (s *AuthService) LogoutAllDevices(userID string) *response.APIResponse {
	if err := s.repository.UpdateRefreshToken(strings.TrimSpace(userID), ""); err != nil {
		return &response.APIResponse{Success: false, Message: "Could not revoke sessions", Error: err.Error()}
	}
	return &response.APIResponse{
		Success: true,
		Message: "All sessions have been revoked",
		Data:    fiber.Map{"logged_out_all_devices": true},
	}
}

func (s *AuthService) ResendVerificationMail(req ResendVerificationMailRequest) *response.APIResponse {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.Email == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Email is required",
			Error:   "email_required",
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

	if user.MailVerified {
		return &response.APIResponse{
			Success: false,
			Message: "Email is already verified",
			Error:   "email_already_verified",
		}
	}

	_, err = s.userVerificationFlow(user.Email, "register")
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not send verification email",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Verification email resent successfully",
	}
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
		Data: fiber.Map{
			"id":                 user.ID,
			"email":              user.Email,
			"first_name":         user.FirstName,
			"last_name":          user.LastName,
			"nickname":           user.Nickname,
			"date_of_birth":      user.DateOfBirth,
			"address":            user.Address,
			"profile_image_url":  user.ProfileImageURL,
			"mail_verified":      user.MailVerified,
			"email_verified":     user.EmailVerifiedAt,
			"country":            user.Country,
			"phone_number":       user.PhoneNumber,
			"has_withdrawal_pin": user.WithdrawalPinHash != nil && *user.WithdrawalPinHash != "",
			"created_at":         user.CreatedAt,
			"updated_at":         user.UpdatedAt,
		},
	}
}

func (s *AuthService) UpdateProfile(req UpdateProfileRequest) *response.APIResponse {
	user, err := s.repository.FindById(req.UserID)
	if err != nil {
		return &response.APIResponse{Success: false, Message: "Could not fetch user", Error: err.Error()}
	}
	if user == nil {
		return &response.APIResponse{Success: false, Message: "User not found", Error: "user_not_found"}
	}

	updates := map[string]interface{}{}
	if nickname := normalizeNickname(req.Nickname); nickname != "" {
		if !validNickname(nickname) {
			return &response.APIResponse{Success: false, Message: "Invalid nickname", Error: "invalid_nickname"}
		}
		existing, err := s.repository.FindByNickname(nickname)
		if err != nil {
			return &response.APIResponse{Success: false, Message: "Could not check nickname", Error: err.Error()}
		}
		if existing != nil && existing.ID != req.UserID {
			return &response.APIResponse{Success: false, Message: "Nickname is not available", Error: "nickname_unavailable"}
		}
		updates["nickname"] = nickname
	}

	if strings.TrimSpace(req.DateOfBirth) != "" {
		dob, err := time.Parse("2006-01-02", strings.TrimSpace(req.DateOfBirth))
		if err != nil {
			return &response.APIResponse{Success: false, Message: "Date of birth must be YYYY-MM-DD", Error: "invalid_date_of_birth"}
		}
		updates["date_of_birth"] = dob
	}

	if strings.TrimSpace(req.Address) != "" {
		updates["address"] = strings.TrimSpace(req.Address)
	}

	if len(updates) == 0 {
		return s.Me(MeRequest{UserID: req.UserID})
	}

	if err := s.repository.UpdateProfile(req.UserID, updates); err != nil {
		return &response.APIResponse{Success: false, Message: "Could not update profile", Error: err.Error()}
	}

	return s.Me(MeRequest{UserID: req.UserID})
}

func (s *AuthService) CheckNickname(req CheckNicknameRequest) *response.APIResponse {
	nickname := normalizeNickname(req.Nickname)
	if !validNickname(nickname) {
		return &response.APIResponse{Success: false, Message: "Invalid nickname", Error: "invalid_nickname"}
	}

	existing, err := s.repository.FindByNickname(nickname)
	if err != nil {
		return &response.APIResponse{Success: false, Message: "Could not check nickname", Error: err.Error()}
	}

	available := existing == nil || existing.ID == req.UserID
	return &response.APIResponse{
		Success: true,
		Message: "Nickname availability checked",
		Data: fiber.Map{
			"nickname":  nickname,
			"available": available,
		},
	}
}

func (s *AuthService) SaveProfileImage(userID string, imageURL string, publicID string) *response.APIResponse {
	if strings.TrimSpace(imageURL) == "" {
		return &response.APIResponse{Success: false, Message: "Image URL is required", Error: "image_url_required"}
	}

	if err := s.repository.UpdateProfile(userID, map[string]interface{}{
		"profile_image_url":       imageURL,
		"profile_image_public_id": publicID,
	}); err != nil {
		return &response.APIResponse{Success: false, Message: "Could not save profile image", Error: err.Error()}
	}

	return s.Me(MeRequest{UserID: userID})
}

func (s *AuthService) WithdrawalPinStatus(req WithdrawalPinStatusRequest) *response.APIResponse {
	user, err := s.repository.FindById(req.UserID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch withdrawal PIN status",
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
		Message: "Withdrawal PIN status fetched",
		Data: fiber.Map{
			"has_pin": user.WithdrawalPinHash != nil && *user.WithdrawalPinHash != "",
		},
	}
}

func (s *AuthService) VerifyWithdrawalPin(req VerifyWithdrawalPinRequest) *response.APIResponse {
	req.Pin = strings.TrimSpace(req.Pin)
	if validationErrors := validateWithdrawalPin(req.Pin); len(validationErrors) > 0 {
		return &response.APIResponse{Success: false, Message: "Invalid withdrawal PIN", Error: validationErrors}
	}

	err := s.repository.VerifyWithdrawalPin(req.UserID, req.Pin)
	if err != nil {
		switch {
		case errors.Is(err, users.ErrWithdrawalPinNotSet):
			return &response.APIResponse{Success: false, Message: "Payment PIN is not set", Error: "pin_not_set"}
		case errors.Is(err, users.ErrWithdrawalPinInvalid):
			return &response.APIResponse{Success: false, Message: "Incorrect payment PIN. One attempt remaining before withdrawals are locked.", Error: "invalid_pin"}
		case errors.Is(err, users.ErrWithdrawalPinLocked):
			return &response.APIResponse{Success: false, Message: "Withdrawals are locked after two incorrect PIN attempts. Reset your PIN to continue.", Error: "withdrawal_pin_locked"}
		default:
			return &response.APIResponse{Success: false, Message: "Could not verify payment PIN", Error: err.Error()}
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Payment PIN verified",
		Data: fiber.Map{
			"verified": true,
		},
	}
}

func (s *AuthService) SetWithdrawalPin(req SetWithdrawalPinRequest) *response.APIResponse {
	req.Pin = strings.TrimSpace(req.Pin)

	if validationErrors := validateWithdrawalPin(req.Pin); len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid withdrawal PIN",
			Error:   validationErrors,
		}
	}

	user, err := s.repository.FindById(req.UserID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch user",
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

	pinHash, err := hashPassword(req.Pin)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not secure withdrawal PIN",
			Error:   err.Error(),
		}
	}

	if err := s.repository.UpdateWithdrawalPin(req.UserID, pinHash); err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not save withdrawal PIN",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal PIN saved successfully",
		Data: fiber.Map{
			"has_pin": true,
		},
	}
}

func (s *AuthService) ForgotWithdrawalPin(req ForgotWithdrawalPinRequest) *response.APIResponse {
	user, err := s.repository.FindById(req.UserID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch user",
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

	otp, err := generateNumericOTP(6)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not generate reset OTP",
			Error:   err.Error(),
		}
	}

	if err := s.repository.UpdateWithdrawalPinResetOTP(user.ID, hashToken(otp), time.Now().Add(10*time.Minute)); err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not save reset OTP",
			Error:   err.Error(),
		}
	}

	if err := s.mailer.SendWithdrawalPinOTP(user.Email, otp); err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not send reset OTP",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal PIN reset OTP sent successfully",
	}
}

func (s *AuthService) ResetWithdrawalPin(req ResetWithdrawalPinRequest) *response.APIResponse {
	req.OTP = strings.TrimSpace(req.OTP)
	req.Pin = strings.TrimSpace(req.Pin)

	if validationErrors := validateWithdrawalPinOTP(req.OTP); len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid OTP",
			Error:   validationErrors,
		}
	}

	if validationErrors := validateWithdrawalPin(req.Pin); len(validationErrors) > 0 {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid withdrawal PIN",
			Error:   validationErrors,
		}
	}

	user, err := s.repository.FindById(req.UserID)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch user",
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

	if user.WithdrawalPinResetOTP == nil ||
		*user.WithdrawalPinResetOTP != hashToken(req.OTP) ||
		user.WithdrawalPinResetExpiresAt == nil ||
		time.Now().After(*user.WithdrawalPinResetExpiresAt) {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid or expired OTP",
			Error:   "invalid_or_expired_otp",
		}
	}

	pinHash, err := hashPassword(req.Pin)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not secure withdrawal PIN",
			Error:   err.Error(),
		}
	}

	if err := s.repository.UpdateWithdrawalPin(user.ID, pinHash); err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not save withdrawal PIN",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Withdrawal PIN reset successfully",
		Data: fiber.Map{
			"has_pin": true,
		},
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

func (s *AuthService) ListSecurityQuestions(userID string) *response.APIResponse {
	questions, err := s.repository.ListSecurityQuestions(userID)
	if err != nil {
		return &response.APIResponse{Success: false, Message: "Could not fetch security questions", Error: err.Error()}
	}

	items := make([]fiber.Map, 0, len(questions))
	for _, question := range questions {
		items = append(items, fiber.Map{
			"id":         question.ID,
			"question":   question.Question,
			"created_at": question.CreatedAt,
			"updated_at": question.UpdatedAt,
		})
	}

	return &response.APIResponse{Success: true, Message: "Security questions fetched", Data: fiber.Map{"items": items}}
}

func (s *AuthService) SaveSecurityQuestions(req SaveSecurityQuestionsRequest) *response.APIResponse {
	if len(req.Questions) == 0 {
		return &response.APIResponse{Success: false, Message: "At least one security question is required", Error: "questions_required"}
	}

	questions := make([]users.SecurityQuestion, 0, len(req.Questions))
	for _, input := range req.Questions {
		question := strings.TrimSpace(input.Question)
		answer := strings.ToLower(strings.TrimSpace(input.Answer))
		if question == "" || answer == "" {
			return &response.APIResponse{Success: false, Message: "Question and answer are required", Error: "invalid_security_question"}
		}

		questions = append(questions, users.SecurityQuestion{
			ID:         uuid.Must(uuid.NewV7()).String(),
			UserID:     req.UserID,
			Question:   question,
			AnswerHash: hashToken(answer),
		})
	}

	if err := s.repository.ReplaceSecurityQuestions(req.UserID, questions); err != nil {
		return &response.APIResponse{Success: false, Message: "Could not save security questions", Error: err.Error()}
	}

	return s.ListSecurityQuestions(req.UserID)
}

func (s *AuthService) ChangePassword(req ResetPasswordRequest) *response.APIResponse {
	return nil
}

func normalizeNickname(value string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "@"))
}

func validNickname(value string) bool {
	if len(value) < 3 || len(value) > 24 {
		return false
	}
	return regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(value)
}

// /helpers
func (s *AuthService) userVerificationFlow(email string, flowType string) (bool, error) {
	otp, err := generateNumericOTP(6)
	if err != nil {
		return false, err
	}
	tokenHash := hashToken(otp)
	mailExpiresAt := time.Now().Add(30 * time.Minute)
	if err := s.repository.UpdateUserToken(email, tokenHash, mailExpiresAt); err != nil {
		return false, err
	}
	if flowType == "forgetPassword" {
		if err := s.sendForgetPasswordMail(email, otp); err != nil {
			return false, err
		}
	} else {
		if err := s.sendVerificationEmail(email, otp); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (s *AuthService) sendVerificationEmail(email, otp string) error {
	return s.mailer.SendMailVerification(email, otp)
}
func (s *AuthService) sendForgetPasswordMail(email, otp string) error {
	return s.mailer.SendForgetPasswordMail(email, otp)
}

func (s *AuthService) VerifyEmail(rawToken string) *response.APIResponse {
	hashedToken := hashToken(rawToken)
	tokenRecord, err := s.CheckTokenValidity(rawToken)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not verify email OTP",
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

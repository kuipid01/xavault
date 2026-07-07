package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/platform/cloudinary"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

type AuthHandler struct {
	authService *AuthService
	cloudinary  *cloudinary.Service
}

func NewAuthHandler(authService *AuthService) *AuthHandler {
	return &AuthHandler{authService: authService, cloudinary: cloudinary.NewService()}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.Register(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) StartRegistration(c *fiber.Ctx) error {
	var req StartRegistrationRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.StartRegistration(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) VerifyRegistrationMail(c *fiber.Ctx) error {
	var req VerifyRegistrationMailRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.VerifyRegistrationMail(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.Login(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	if result.Data != nil {
		dataMap := result.Data.(map[string]interface{})
		accessToken := dataMap["access_token"].(string)
		refreshToken := dataMap["refresh_token"].(string)
		SetAuthCookies(c, accessToken, refreshToken)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ValidateMail(c *fiber.Ctx) error {
	var req ValidateMailRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ValidateMail(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ForgetPassword(c *fiber.Ctx) error {
	var req ForgetPasswordRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ForgetPassword(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ResetPassword(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var req ResetPasswordRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.Logout(req)
	if result.Data != nil {

		RemoveAuthCookies(c)
	}
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {

	userID := c.Locals("user_id").(string)

	result := h.authService.Me(MeRequest{
		UserID: userID,
	})

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	req.UserID = userID

	result := h.authService.UpdateProfile(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) CheckNickname(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	nickname := c.Query("nickname")

	result := h.authService.CheckNickname(CheckNicknameRequest{
		UserID:   userID,
		Nickname: nickname,
	})
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) UploadProfileImage(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)
	header, err := c.FormFile("image")
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Profile image is required", err.Error())
	}

	file, err := header.Open()
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Could not read profile image", err.Error())
	}
	defer file.Close()

	upload, err := h.cloudinary.UploadImage(file, header.Filename, "xavault/profile-images")
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Could not upload profile image", err.Error())
	}

	result := h.authService.SaveProfileImage(userID, upload.SecureURL, upload.PublicID)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshTokenRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	if req.RefreshToken == "" {
		req.RefreshToken = c.Cookies("refresh_token")
	}

	result := h.authService.RefreshToken(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	if result.Data != nil {
		dataMap := result.Data.(fiber.Map)
		accessToken := dataMap["access_token"].(string)
		refreshToken := dataMap["refresh_token"].(string)
		SetAuthCookies(c, accessToken, refreshToken)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ResendVerificationMail(c *fiber.Ctx) error {
	var req ResendVerificationMailRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ResendVerificationMail(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	var req ResetPasswordRequest

	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}

	result := h.authService.ChangePassword(req)

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) WithdrawalPinStatus(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	result := h.authService.WithdrawalPinStatus(WithdrawalPinStatusRequest{
		UserID: userID,
	})

	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) VerifyWithdrawalPin(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req VerifyWithdrawalPinRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	req.UserID = userID

	result := h.authService.VerifyWithdrawalPin(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) SetWithdrawalPin(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req SetWithdrawalPinRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	req.UserID = userID

	result := h.authService.SetWithdrawalPin(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ForgotWithdrawalPin(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	result := h.authService.ForgotWithdrawalPin(ForgotWithdrawalPinRequest{
		UserID: userID,
	})
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ResetWithdrawalPin(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req ResetWithdrawalPinRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	req.UserID = userID

	result := h.authService.ResetWithdrawalPin(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) ListSecurityQuestions(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	result := h.authService.ListSecurityQuestions(userID)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

func (h *AuthHandler) SaveSecurityQuestions(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req SaveSecurityQuestionsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request", err)
	}
	req.UserID = userID

	result := h.authService.SaveSecurityQuestions(req)
	if result.Error != nil {
		return response.Error(c, fiber.StatusBadRequest, result.Message, result.Error)
	}

	return response.Success(c, fiber.StatusOK, result.Message, result.Data)
}

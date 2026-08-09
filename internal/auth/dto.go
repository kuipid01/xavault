package auth

type RegisterRequest struct {
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=6,max=128"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
	FirstName       string `json:"first_name" validate:"required"`
	LastName        string `json:"last_name" validate:"required"`
	Country         string `json:"country" validate:"required"`
	PhoneNumber     string `json:"phone_number" validate:"required"`
	Nickname        string `json:"nickname"`
	Pin             string `json:"pin" validate:"required"`
}

type StartRegistrationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type VerifyRegistrationMailRequest struct {
	Email string `json:"email" validate:"required,email"`
	OTP   string `json:"otp" validate:"required"`
}

type ValidateMailRequest struct {
	Token string `json:"token"`
	OTP   string `json:"otp"`
	Email string `json:"email"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6,max=128"`
}

type ForgetPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Password        string `json:"password" validate:"required,min=6,max=128"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
	Token           string `json:"token"`
	OTP             string `json:"otp"`
}

type ResendVerificationMailRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type MeRequest struct {
	UserID string `json:"user_id" validate:"required"`
}

type UpdateProfileRequest struct {
	UserID      string `json:"user_id" validate:"required"`
	Nickname    string `json:"nickname"`
	DateOfBirth string `json:"date_of_birth"`
	Address     string `json:"address"`
}

type CheckNicknameRequest struct {
	UserID   string `json:"user_id" validate:"required"`
	Nickname string `json:"nickname" validate:"required"`
}

type WithdrawalPinStatusRequest struct {
	UserID string `json:"user_id" validate:"required"`
}

type VerifyWithdrawalPinRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Pin    string `json:"pin" validate:"required"`
}

type SetWithdrawalPinRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Pin    string `json:"pin" validate:"required"`
}

type ForgotWithdrawalPinRequest struct {
	UserID string `json:"user_id" validate:"required"`
}

type ResetWithdrawalPinRequest struct {
	UserID string `json:"user_id" validate:"required"`
	OTP    string `json:"otp" validate:"required"`
	Pin    string `json:"pin" validate:"required"`
}

type SecurityQuestionInput struct {
	Question string `json:"question" validate:"required"`
	Answer   string `json:"answer" validate:"required"`
}

type SaveSecurityQuestionsRequest struct {
	UserID    string                  `json:"user_id" validate:"required"`
	Questions []SecurityQuestionInput `json:"questions" validate:"required"`
}

package auth

type RegisterRequest struct {
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=6,max=128"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
}

type ValidateMailRequest struct {
	Token string `json:"token" validate:"required"`
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
	Token           string `json:"token" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type MeRequest struct {
	UserID string `json:"user_id" validate:"required"`
}

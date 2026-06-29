package auth

import "strings"

func validateForgetPasswordData(req ForgetPasswordRequest) map[string]string {
	errors := make(map[string]string)

	if strings.TrimSpace(req.Email) == "" {
		errors["email"] = "Email is required"
	} else if !strings.Contains(req.Email, "@") {
		errors["email"] = "Email is invalid"
	}

	return errors
}

func validateLoginData(req LoginRequest) map[string]string {
	errors := make(map[string]string)

	if strings.TrimSpace(req.Email) == "" {
		errors["email"] = "Email is required"
	} else if !strings.Contains(req.Email, "@") {
		errors["email"] = "Email is invalid"
	}

	if strings.TrimSpace(req.Password) == "" {
		errors["password"] = "Password is required"
	}

	return errors
}

func validateRegistrationData(userData RegisterRequest) (RegisterRequest, map[string]string) {
	errors := make(map[string]string)

	if len(userData.Email) == 0 {
		errors["email"] = "Email is required"
	}

	if len(userData.Password) == 0 {
		errors["password"] = "Password is required"
	}

	if len(userData.Password) < 6 {
		errors["password"] = "Password must be at least 6 characters"
	}

	if !strings.Contains(userData.Email, "@") {
		errors["email"] = "Email is invalid"
	}
	if userData.Password != userData.ConfirmPassword {
		errors["password"] = "Passwords do not match"
	}
	if !mustContainSpecialCharacters(userData.Password) {
		errors["password"] = "Password must contain at least one special character"
	}

	return RegisterRequest{
		Email:           strings.ToLower(userData.Email),
		Password:        userData.Password,
		ConfirmPassword: userData.ConfirmPassword,
	}, errors
}

func validateResetPasswordData(req ResetPasswordRequest) map[string]string {
	if strings.TrimSpace(req.Password) == "" {
		return map[string]string{"password": "Password is required"}
	}
	if strings.TrimSpace(req.Token) == "" {
		return map[string]string{"token": "Token is required"}
	}
	if len(req.Password) < 6 {
		return map[string]string{"password": "Password must be at least 6 characters"}
	}
	if req.Password != req.ConfirmPassword {
		return map[string]string{"password": "Passwords do not match"}
	}
	if !mustContainSpecialCharacters(req.Password) {
		return map[string]string{"password": "Password must contain at least one special character"}
	}
	return nil
}

func validateWithdrawalPin(pin string) map[string]string {
	pin = strings.TrimSpace(pin)
	if len(pin) != 4 {
		return map[string]string{"pin": "Withdrawal PIN must be 4 digits"}
	}

	for _, r := range pin {
		if r < '0' || r > '9' {
			return map[string]string{"pin": "Withdrawal PIN must contain only digits"}
		}
	}

	return nil
}

func validateWithdrawalPinOTP(otp string) map[string]string {
	otp = strings.TrimSpace(otp)
	if len(otp) != 6 {
		return map[string]string{"otp": "OTP must be 6 digits"}
	}

	for _, r := range otp {
		if r < '0' || r > '9' {
			return map[string]string{"otp": "OTP must contain only digits"}
		}
	}

	return nil
}

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
	userData.Email = strings.ToLower(strings.TrimSpace(userData.Email))
	userData.FirstName = strings.TrimSpace(userData.FirstName)
	userData.LastName = strings.TrimSpace(userData.LastName)
	userData.Country = strings.ToUpper(strings.TrimSpace(userData.Country))
	userData.Pin = strings.TrimSpace(userData.Pin)

	if len(userData.Email) == 0 {
		errors["email"] = "Email is required"
	}

	if len(userData.Password) == 0 {
		errors["password"] = "Password is required"
	}
	if len(userData.FirstName) == 0 {
		errors["first_name"] = "First name is required"
	}
	if len(userData.LastName) == 0 {
		errors["last_name"] = "Last name is required"
	}
	if len(userData.PhoneNumber) == 0 {
		errors["phone_number"] = "PhoneNumber is required"
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
	if pinErrors := validateWithdrawalPin(userData.Pin); len(pinErrors) > 0 {
		errors["pin"] = pinErrors["pin"]
	}
	if !mustContainSpecialCharacters(userData.Password) {
		errors["password"] = "Password must contain at least one special character"
	}
	phoneNumber, countryCode, phoneErr := normalizePhoneForCountry(userData.Country, userData.PhoneNumber)
	if phoneErr != "" {
		errors["phone_number"] = phoneErr
	}

	return RegisterRequest{
		Email:           userData.Email,
		Password:        userData.Password,
		ConfirmPassword: userData.ConfirmPassword,
		FirstName:       userData.FirstName,
		LastName:        userData.LastName,
		Country:         countryCode,
		PhoneNumber:     phoneNumber,
		Pin:             userData.Pin,
	}, errors
}

func validateResetPasswordData(req ResetPasswordRequest) map[string]string {
	if strings.TrimSpace(req.Password) == "" {
		return map[string]string{"password": "Password is required"}
	}
	if strings.TrimSpace(req.OTP) == "" && strings.TrimSpace(req.Token) == "" {
		return map[string]string{"otp": "OTP is required"}
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

func normalizePhoneForCountry(country string, phoneNumber string) (string, string, string) {
	digits := onlyDigits(phoneNumber)

	switch country {
	case "NG", "NIGERIA", "+234", "234":
		if len(digits) == 11 && strings.HasPrefix(digits, "0") {
			digits = strings.TrimPrefix(digits, "0")
		}
		if len(digits) != 10 {
			return "", "NG", "Nigeria phone number must be 10 digits, or 11 digits starting with 0"
		}
		return digits, "NG", ""
	case "GH", "GHANA", "+233", "233":
		if len(digits) == 10 && strings.HasPrefix(digits, "0") {
			digits = strings.TrimPrefix(digits, "0")
		}
		if len(digits) != 9 {
			return "", "GH", "Ghana phone number must be 9 digits, or 10 digits starting with 0"
		}
		return digits, "GH", ""
	default:
		return "", "", "Country must be Nigeria or Ghana"
	}
}

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
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

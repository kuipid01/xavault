package mail

type Mailer interface {
	SendMailVerification(toEmail string, mailVerificationURL string) error
	SendForgetPasswordMail(toEmail string, forgetPasswordURL string) error
	SendWithdrawalPinOTP(toEmail string, otp string) error
}

package mail

type Mailer interface {
	SendMailVerification(toEmail string, otp string) error
	SendForgetPasswordMail(toEmail string, otp string) error
	SendWithdrawalPinOTP(toEmail string, otp string) error
}

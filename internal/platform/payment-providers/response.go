package paymentproviders

type DepositResponse struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type WithdrawalResponse struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type DepositBody struct {
	Amount      int64  `json:"amount" validate:"required,gt=0"`
	Email       string `json:"email" validate:"required"`
	Currency    string `json:"currency" validate:"required"`
	Reference   string `json:"reference" validate:"required"`
	CallbackURL string `json:"callback_url" validate:"required"`
}
type WithdrawalBody struct {
	Amount      int64  `json:"amount" validate:"required,gt=0"`
	Email       string `json:"email" validate:"required"`
	Currency    string `json:"currency" validate:"required"`
	Reference   string `json:"reference" validate:"required"`
	CallbackURL string `json:"callback_url" validate:"required"`
}

type PaystackTransferData struct {
	ID            int64  `json:"id"`
	Reference     string `json:"reference"`
	TransferCode  string `json:"transfer_code"`
	RecipientCode string `json:"recipient"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
}

type InitiateTransferResponse struct {
	Status  bool                 `json:"status"`
	Message string               `json:"message"`
	Data    PaystackTransferData `json:"data"`
}

type InitiateTransferBody struct {
	Source    string `json:"source"`    // "balance"
	Amount    int64  `json:"amount"`    // kobo
	Recipient string `json:"recipient"` // recipient_code
	Reason    string `json:"reason"`
	Reference string `json:"reference"`
}

// Paystack expects transfer recipients before transfer initiation.
// Recipient creation can be done once when user saves bank details.
// A duplicate account number can return the existing recipient, so save recipient_code in DB.
type CreateTransferRecipientBody struct {
	Type          string `json:"type"` // "nuban" for Nigerian bank account
	Name          string `json:"name"`
	AccountNumber string `json:"account_number"`
	BankCode      string `json:"bank_code"`
	Currency      string `json:"currency"` // "NGN"
}

type PaystackTransferRecipientData struct {
	RecipientCode string `json:"recipient_code"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Currency      string `json:"currency"`
}

type CreateTransferRecipientResponse struct {
	Status  bool                          `json:"status"`
	Message string                        `json:"message"`
	Data    PaystackTransferRecipientData `json:"data"`
}

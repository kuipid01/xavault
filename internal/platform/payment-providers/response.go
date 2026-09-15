package paymentproviders

import "time"

type DepositResponse struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type VerifyTransactionData struct {
	ID              int64      `json:"id"`
	Reference       string     `json:"reference"`
	Status          string     `json:"status"`
	Amount          int64      `json:"amount"`
	// Fees is the fee Paystack actually charged (kobo). Zero on some flows.
	// Observability only — credit math stays rule-based.
	Fees            int64      `json:"fees"`
	Currency        string     `json:"currency"`
	Domain          string     `json:"domain"`
	GatewayResponse string     `json:"gateway_response"`
	PaidAt          *time.Time `json:"paid_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type VerifyTransactionResponse struct {
	Status  bool                  `json:"status"`
	Message string                `json:"message"`
	Data    VerifyTransactionData `json:"data"`
}

type DepositBody struct {
	Amount      int64                  `json:"amount" validate:"required,gt=0"`
	Email       string                 `json:"email" validate:"required,email"`
	Currency    string                 `json:"currency" validate:"required"`
	Reference   string                 `json:"reference" validate:"required"`
	CallbackURL string                 `json:"callback_url" validate:"required,url"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type PaystackBank struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Code     string `json:"code"`
	LongCode string `json:"longcode"`
	Country  string `json:"country"`
	Currency string `json:"currency"`
	Type     string `json:"type"`
	Active   bool   `json:"active"`
}

type ListBanksResponse struct {
	Status  bool           `json:"status"`
	Message string         `json:"message"`
	Data    []PaystackBank `json:"data"`
}

type ResolveBankAccountData struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	BankID        int64  `json:"bank_id"`
}

type ResolveBankAccountResponse struct {
	Status  bool                   `json:"status"`
	Message string                 `json:"message"`
	Data    ResolveBankAccountData `json:"data"`
}

type CreateCustomerBody struct {
	Email     string `json:"email" validate:"required,email"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

type PaystackCustomerData struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	CustomerCode string `json:"customer_code"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Phone        string `json:"phone"`
	RiskAction   string `json:"risk_action"`
}

type CreateCustomerResponse struct {
	Status  bool                 `json:"status"`
	Message string               `json:"message"`
	Data    PaystackCustomerData `json:"data"`
}

type CreateDedicatedAccountBody struct {
	Customer      string `json:"customer" validate:"required"`
	PreferredBank string `json:"preferred_bank,omitempty"`
}

type PaystackDedicatedBank struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Code     string `json:"code"`
	LongCode string `json:"longcode"`
}

type PaystackDedicatedAccountData struct {
	ID            int64                  `json:"id"`
	AccountName   string                 `json:"account_name"`
	AccountNumber string                 `json:"account_number"`
	Bank          PaystackDedicatedBank  `json:"bank"`
	Customer      PaystackCustomerData   `json:"customer"`
	Assignment    map[string]interface{} `json:"assignment"`
	Active        bool                   `json:"active"`
	Currency      string                 `json:"currency"`
}

type CreateDedicatedAccountResponse struct {
	Status  bool                         `json:"status"`
	Message string                       `json:"message"`
	Data    PaystackDedicatedAccountData `json:"data"`
}

type WithdrawalBody struct {
	Amount      int64  `json:"amount" validate:"required,gt=0"`
	Email       string `json:"email" validate:"required,email"`
	Currency    string `json:"currency" validate:"required"`
	Reference   string `json:"reference" validate:"required"`
	CallbackURL string `json:"callback_url" validate:"required,url"`
}

type InitiateTransferBody struct {
	Source    string `json:"source" validate:"required"`
	Amount    int64  `json:"amount" validate:"required,gt=0"`
	Recipient string `json:"recipient" validate:"required"`
	Reason    string `json:"reason,omitempty"`
	Reference string `json:"reference" validate:"required"`
}

type PaystackTransferData struct {
	ID           int64  `json:"id"`
	Integration  int64  `json:"integration"`
	Request      int64  `json:"request"`
	RecipientID  int64  `json:"recipient"`
	Reference    string `json:"reference"`
	TransferCode string `json:"transfer_code"`

	Domain   string `json:"domain"`
	Source   string `json:"source"`
	Reason   string `json:"reason"`
	Status   string `json:"status"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`

	SourceDetails interface{} `json:"source_details"`
	Failures      interface{} `json:"failures"`
	TitanCode     *string     `json:"titan_code"`
	TransferredAt *time.Time  `json:"transferred_at"`

	TransferSessionID []interface{} `json:"transfersessionid"`
	TransferTrials    []interface{} `json:"transfertrials"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type InitiateTransferResponse struct {
	Status  bool                 `json:"status"`
	Message string               `json:"message"`
	Data    PaystackTransferData `json:"data"`
}

// Paystack expects a transfer recipient to exist before transfer initiation.
// Create the recipient once when the user saves their bank details.
// Save the returned recipient_code, such as RCP_xxxxxxxxx, in the database.
type CreateTransferRecipientBody struct {
	Type          string `json:"type" validate:"required"`
	Name          string `json:"name" validate:"required"`
	AccountNumber string `json:"account_number" validate:"required"`
	BankCode      string `json:"bank_code" validate:"required"`
	Currency      string `json:"currency" validate:"required"`
}

type PaystackTransferRecipientDetails struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	BankCode      string `json:"bank_code"`
	BankName      string `json:"bank_name"`
}

type PaystackTransferRecipientData struct {
	ID            int64                            `json:"id"`
	RecipientCode string                           `json:"recipient_code"`
	Name          string                           `json:"name"`
	Type          string                           `json:"type"`
	Currency      string                           `json:"currency"`
	Active        bool                             `json:"active"`
	Domain        string                           `json:"domain"`
	Description   string                           `json:"description"`
	Details       PaystackTransferRecipientDetails `json:"details"`
	CreatedAt     time.Time                        `json:"createdAt"`
	UpdatedAt     time.Time                        `json:"updatedAt"`
}

type CreateTransferRecipientResponse struct {
	Status  bool                          `json:"status"`
	Message string                        `json:"message"`
	Data    PaystackTransferRecipientData `json:"data"`
}

type FinalizeTransferBody struct {
	TransferCode string `json:"transfer_code" validate:"required"`
	OTP          string `json:"otp" validate:"required"`
}

type FinalizeTransferData struct {
	ID           int64  `json:"id"`
	Reference    string `json:"reference"`
	TransferCode string `json:"transfer_code"`
	Status       string `json:"status"`
	Amount       int64  `json:"amount"`
	Currency     string `json:"currency"`
}

type FinalizeTransferResponse struct {
	Status  bool                 `json:"status"`
	Message string               `json:"message"`
	Data    FinalizeTransferData `json:"data"`
}

type VerifyTransferData struct {
	ID           int64  `json:"id"`
	Reference    string `json:"reference"`
	TransferCode string `json:"transfer_code"`
	Status       string `json:"status"`
	Amount       int64  `json:"amount"`
	Currency     string `json:"currency"`
	Reason       string `json:"reason"`

	RecipientID int64 `json:"recipient"`

	TransferredAt *time.Time `json:"transferred_at"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type VerifyTransferResponse struct {
	Status  bool               `json:"status"`
	Message string             `json:"message"`
	Data    VerifyTransferData `json:"data"`
}

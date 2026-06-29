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

type PaystackBank struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Code     string `json:"code"`
	LongCode string `json:"longcode"`
	Country  string `json:"country"`
	Currency string `json:"currency"`
	Type     string `json:"type"`
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
	Email     string `json:"email"`
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
}

type CreateCustomerResponse struct {
	Status  bool                 `json:"status"`
	Message string               `json:"message"`
	Data    PaystackCustomerData `json:"data"`
}

type CreateDedicatedAccountBody struct {
	Customer      string `json:"customer"`
	PreferredBank string `json:"preferred_bank,omitempty"`
}

type PaystackDedicatedBank struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	ID   int64  `json:"id"`
}

type PaystackDedicatedAccountData struct {
	ID            int64                  `json:"id"`
	AccountName   string                 `json:"account_name"`
	AccountNumber string                 `json:"account_number"`
	Bank          PaystackDedicatedBank  `json:"bank"`
	Customer      PaystackCustomerData   `json:"customer"`
	Assignment    map[string]interface{} `json:"assignment"`
}

type CreateDedicatedAccountResponse struct {
	Status  bool                         `json:"status"`
	Message string                       `json:"message"`
	Data    PaystackDedicatedAccountData `json:"data"`
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

package bankaccounts

type ResolveBankAccountRequest struct {
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
}

type CreateBankAccountRequest struct {
	BankName      string `json:"bank_name"`
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	Nickname      string `json:"nickname"`
	IsDefault     bool   `json:"is_default"`
}

type BankAccountResponse struct {
	ID            string `json:"id"`
	OwnerType     string `json:"owner_type"`
	Nickname      string `json:"nickname"`
	BankName      string `json:"bank_name"`
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	IsDefault     bool   `json:"is_default"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type ResolveBankAccountResponse struct {
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

type ListBankAccountsResponse struct {
	Items []BankAccountResponse `json:"items"`
}

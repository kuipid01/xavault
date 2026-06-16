package paymentproviders

type DepositResponse struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type WithdrawalResponse struct {
}

type DepositBody struct {
	Amount      int64  `json:"amount" validate:"required,gt=0"`
	Email       string `json:"email" validate:"required"`
	Currency    string `json:"currency" validate:"required"`
	Reference   string `json:"reference" validate:"required"`
	CallbackURL string `json:"callback_url" validate:"required"`
}

package webhooks

import "encoding/json"

// EventEnvelope extracts the generic type identifier first
type EventEnvelope struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"` // Keeps the nested data as unparsed raw bytes
}

// ChargeSuccessData represents the structural payload for a successful deposit
type ChargeSuccessData struct {
	Amount    int64  `json:"amount"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Customer  struct {
		Email string `json:"email"`
	} `json:"customer"`
	DedicatedAccount struct {
		AccountNumber string `json:"account_number"`
	} `json:"dedicated_account"`
}

type TransferEventData struct {
	Reference    string `json:"reference"`
	Status       string `json:"status"`
	Reason       string `json:"reason"`
	TransferCode string `json:"transfer_code"`
	Amount       int64  `json:"amount"`
}

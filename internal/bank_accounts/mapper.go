package bankaccounts

func mapAccountToResponse(account BankAccount) BankAccountResponse {
	return BankAccountResponse{
		ID:            account.ID,
		OwnerType:     account.OwnerType,
		Nickname:      account.Nickname,
		BankName:      account.BankName,
		BankCode:      account.BankCode,
		AccountNumber: account.AccountNumber,
		AccountName:   account.AccountName,
		IsDefault:     account.IsDefault,
		CreatedAt:     account.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     account.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func mapAccountsToResponse(accounts []BankAccount) []BankAccountResponse {
	items := make([]BankAccountResponse, 0, len(accounts))

	for _, account := range accounts {
		items = append(items, mapAccountToResponse(account))
	}

	return items
}

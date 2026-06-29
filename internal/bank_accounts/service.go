package bankaccounts

import (
	"context"
	"strings"

	paymentproviders "github.com/kuipid01/salary-vault/internal/platform/payment-providers"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

type Service struct {
	repo    *Repository
	payment paymentproviders.PaymentProvider
}

func NewService(repo *Repository, payment paymentproviders.PaymentProvider) *Service {
	return &Service{repo: repo, payment: payment}
}

func (s *Service) ListBanks(ctx context.Context) *response.APIResponse {
	banks, err := s.payment.ListBanks(ctx)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch banks",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Banks fetched successfully",
		Data:    banks.Data,
	}
}

func (s *Service) ResolveAccount(ctx context.Context, req ResolveBankAccountRequest) *response.APIResponse {
	req.BankCode = strings.TrimSpace(req.BankCode)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)

	if req.BankCode == "" || req.AccountNumber == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Bank code and account number are required",
			Error: map[string]string{
				"bank_code":      "Bank code is required",
				"account_number": "Account number is required",
			},
		}
	}

	resolved, err := s.payment.ResolveBankAccount(ctx, req.AccountNumber, req.BankCode)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not verify account",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Account verified successfully",
		Data: ResolveBankAccountResponse{
			BankCode:      req.BankCode,
			AccountNumber: resolved.Data.AccountNumber,
			AccountName:   resolved.Data.AccountName,
		},
	}
}

func (s *Service) ListAccounts(userID string, ownerType string) *response.APIResponse {
	accounts, err := s.repo.FindByUserIDAndOwnerType(userID, ownerType)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not fetch accounts",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Accounts fetched successfully",
		Data: ListBankAccountsResponse{
			Items: mapAccountsToResponse(accounts),
		},
	}
}

func (s *Service) CreateAccount(
	ctx context.Context,
	userID string,
	ownerType string,
	req CreateBankAccountRequest,
) *response.APIResponse {
	req.BankName = strings.TrimSpace(req.BankName)
	req.BankCode = strings.TrimSpace(req.BankCode)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)
	req.Nickname = strings.TrimSpace(req.Nickname)

	if req.BankName == "" || req.BankCode == "" || req.AccountNumber == "" {
		return &response.APIResponse{
			Success: false,
			Message: "Bank name, bank code and account number are required",
			Error:   "invalid_bank_account",
		}
	}

	duplicate, err := s.repo.FindDuplicate(userID, ownerType, req.BankCode, req.AccountNumber)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not check account",
			Error:   err.Error(),
		}
	}

	if duplicate != nil {
		if req.IsDefault && !duplicate.IsDefault {
			updated, err := s.repo.SetDefault(duplicate.ID, userID, ownerType)
			if err != nil {
				return &response.APIResponse{
					Success: false,
					Message: "Could not set default account",
					Error:   err.Error(),
				}
			}

			if updated != nil {
				duplicate = updated
			}
		}

		return &response.APIResponse{
			Success: true,
			Message: "Account already saved",
			Data:    mapAccountToResponse(*duplicate),
		}
	}

	resolved, err := s.payment.ResolveBankAccount(ctx, req.AccountNumber, req.BankCode)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not verify account",
			Error:   err.Error(),
		}
	}

	recipient, err := s.payment.CreateTransferRecipient(ctx, &paymentproviders.CreateTransferRecipientBody{
		Type:          "nuban",
		Name:          resolved.Data.AccountName,
		AccountNumber: resolved.Data.AccountNumber,
		BankCode:      req.BankCode,
		Currency:      "NGN",
	})
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not create transfer recipient",
			Error:   err.Error(),
		}
	}

	account, err := s.repo.Create(&BankAccount{
		UserID:        userID,
		OwnerType:     ownerType,
		Nickname:      req.Nickname,
		BankName:      req.BankName,
		BankCode:      req.BankCode,
		AccountNumber: resolved.Data.AccountNumber,
		AccountName:   resolved.Data.AccountName,
		RecipientCode: recipient.Data.RecipientCode,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not save account",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Account saved successfully",
		Data:    mapAccountToResponse(*account),
	}
}

func (s *Service) SetDefault(userID string, accountID string, ownerType string) *response.APIResponse {
	account, err := s.repo.SetDefault(accountID, userID, ownerType)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not set default account",
			Error:   err.Error(),
		}
	}

	if account == nil {
		return &response.APIResponse{
			Success: false,
			Message: "Account not found",
			Error:   "account_not_found",
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Default account updated",
		Data:    mapAccountToResponse(*account),
	}
}

func (s *Service) DeleteAccount(userID string, accountID string, ownerType string) *response.APIResponse {
	if err := s.repo.Delete(accountID, userID, ownerType); err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not delete account",
			Error:   err.Error(),
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Account deleted successfully",
	}
}

func (s *Service) GetRecipientCode(userID string, accountID string, ownerType string) (string, error) {
	account, err := s.repo.FindByIDAndUserID(accountID, userID)
	if err != nil {
		return "", err
	}

	if account == nil || account.OwnerType != ownerType {
		return "", nil
	}

	return account.RecipientCode, nil
}

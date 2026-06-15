package wallets

import "github.com/kuipid01/salary-vault/internal/shared/response"

type WalletService struct {
	repo *WalletRepository
}

func NewWalletService(repo *WalletRepository) *WalletService {
	return &WalletService{repo: repo}
}

func (svc *WalletService) GetMyWallet(userId string) *response.APIResponse {
	wallet, err := svc.repo.GetWalletByOwnerID(userId)

	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Invalid data",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: false,
		Message: "Invalid data",
		Error:   wallet,
	}
}

func (s *WalletService) CreateWallet(userId string) *response.APIResponse {
	// Check If user has wallet

	wallet, _ := s.repo.GetWalletByOwnerID(userId)
	if wallet != nil {
		return &response.APIResponse{
			Success: false,
			Message: "User already has a wallet",
			Error:   "wallet_exists",
		}
	}
	walletReq := &Wallet{
		UserID:   userId,
		Balance:  0,
		Currency: "NGN",
	}

	wallet, err := s.repo.Create(walletReq)
	if err != nil {
		return &response.APIResponse{
			Success: false,
			Message: "Could not create wallet",
			Error:   err,
		}
	}

	return &response.APIResponse{
		Success: true,
		Message: "Wallet created successfully",
		Data:    wallet,
	}

}

package wallets

import "gorm.io/gorm"

type WalletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (repo *WalletRepository) Create(wallet *Wallet) (*Wallet, error) {
	if err := repo.db.Create(wallet).Error; err != nil {
		return nil, err
	}

	return wallet, nil
}

func (repo *WalletRepository) GetWalletByOwnerID(ownerID string) (*Wallet, error) {
	var wallet Wallet

	if err := repo.db.Where("user_id = ?", ownerID).First(&wallet).Error; err != nil {
		return nil, err
	}

	return &wallet, nil
}

func (repo *WalletRepository) GetWalletByID(walletID string) (*Wallet, error) {
	var wallet Wallet

	if err := repo.db.Where("id = ?", walletID).First(&wallet).Error; err != nil {
		return nil, err
	}

	return &wallet, nil
}

func (repo *WalletRepository) Update(wallet *Wallet) (*Wallet, error) {
	if err := repo.db.Save(wallet).Error; err != nil {
		return nil, err
	}

	return wallet, nil
}

func (repo *WalletRepository) CreateDeposit(deposit *WalletDeposit) (*WalletDeposit, error) {
	if err := repo.db.Create(deposit).Error; err != nil {
		return nil, err
	}

	return deposit, nil
}

func (repo *WalletRepository) GetDepositByReference(reference string) (*WalletDeposit, error) {
	var deposit WalletDeposit

	if err := repo.db.Where("reference = ?", reference).First(&deposit).Error; err != nil {
		return nil, err
	}

	return &deposit, nil
}

func (repo *WalletRepository) GetDepositsByWalletID(walletID string) ([]WalletDeposit, error) {
	var deposits []WalletDeposit

	if err := repo.db.Where("wallet_id = ?", walletID).
		Order("created_at DESC").
		Find(&deposits).Error; err != nil {
		return nil, err
	}

	return deposits, nil
}

func (repo *WalletRepository) UpdateDepositStatus(reference string, status string) (*WalletDeposit, error) {
	var deposit WalletDeposit

	if err := repo.db.Where("reference = ?", reference).First(&deposit).Error; err != nil {
		return nil, err
	}

	deposit.Status = status

	if err := repo.db.Save(&deposit).Error; err != nil {
		return nil, err
	}

	return &deposit, nil
}

package wallets

import "gorm.io/gorm"

type WalletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (repo *WalletRepository) GetWalletByOwnerID(ownerID string) (*Wallet, error) {
	var wallet Wallet

	if err := repo.db.Where("owner_id = ?", ownerID).First(&wallet).Error; err != nil {
		return nil, err
	}

	return &wallet, nil
}

func (repo *WalletRepository) Create(wallet *Wallet) (*Wallet, error) {
	if err := repo.db.Create(wallet).Error; err != nil {
		return nil, err
	}

	return wallet, nil
}

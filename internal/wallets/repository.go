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

func (rep *WalletRepository) ExecuteInTransaction(fn func(txRepo *WalletRepository) error) error {
	// 1. Begin the GORM database transaction
	tx := rep.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// 2. Setup defer block to catch panics and force a safety rollback
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 3. Clone the wallet repository, injecting the *transactional* DB instance
	txRepository := &WalletRepository{db: tx}

	// 4. Pass the cloned repository into your business logic function closure
	if err := fn(txRepository); err != nil {
		tx.Rollback() // Rollback if the business logic returns an error
		return err
	}

	// 5. Commit if everything completes cleanly without errors
	return tx.Commit().Error
}

func (repo *WalletRepository) GetWalletDepositsByUserID(userID string) ([]WalletDeposit, error) {
	var deposits []WalletDeposit

	if err := repo.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&deposits).Error; err != nil {
		return nil, err
	}

	return deposits, nil
}

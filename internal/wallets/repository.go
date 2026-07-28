package wallets

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

	if err := repo.db.
		Preload("User").
		Where("user_id = ?", ownerID).
		First(&wallet).Error; err != nil {
		return nil, err
	}

	return &wallet, nil
}
func (repo *WalletRepository) GetWalletByOwnerIDForUpdate(ownerID string) (*Wallet, error) {
	var wallet Wallet
	if err := repo.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", ownerID).First(&wallet).Error; err != nil {
		return nil, err
	}
	return &wallet, nil
}
func (repo *WalletRepository) GetWalletByDedicatedAccountNumber(accountNumber string) (*Wallet, error) {
	var wallet Wallet

	if err := repo.db.Where("dedicated_account_number = ?", accountNumber).First(&wallet).Error; err != nil {
		return nil, err
	}

	return &wallet, nil
}
func (repo *WalletRepository) GetWalletByDedicatedAccountNumberForUpdate(accountNumber string) (*Wallet, error) {
	var wallet Wallet
	if err := repo.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("dedicated_account_number = ?", accountNumber).
		First(&wallet).Error; err != nil {
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
func (repo *WalletRepository) GetDepositByReferenceForUpdate(reference string) (*WalletDeposit, error) {
	var deposit WalletDeposit
	if err := repo.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reference = ?", reference).First(&deposit).Error; err != nil {
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
	return rep.db.Transaction(func(tx *gorm.DB) error {
		return fn(&WalletRepository{db: tx})
	})
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

func (repo *WalletRepository) LogWalletLedger(walletLedger *WalletLedger) (*WalletLedger, error) {
	if err := repo.db.Create(walletLedger).Error; err != nil {
		return nil, err
	}

	return walletLedger, nil
}

func (repo *WalletRepository) GetWalletLedgerByUserId(userId string) ([]WalletLedger, error) {
	var walletLedger []WalletLedger

	if err := repo.db.
		Where("user_id = ?", userId).
		Where("type NOT IN ?", []string{
			WalletLedgerTypeLock,
			WalletLedgerTypeUnlock,
			WalletLedgerTypeWithdrawalHold,
			WalletLedgerTypeWithdrawalReversal,
		}).
		Order("created_at DESC").Find(&walletLedger).Error; err != nil {
		return nil, err
	}

	return walletLedger, nil
}

func (repo *WalletRepository) LockFundsForSpendingPlanTx(
	tx *gorm.DB,
	userID string,
	walletID string,
	amount int64,
) (*Wallet, int64, int64, error) {
	var wallet Wallet

	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", walletID, userID).
		First(&wallet).Error

	if err != nil {
		return nil, 0, 0, err
	}

	if amount <= 0 {
		return nil, 0, 0, ErrInvalidWalletAmount
	}

	if wallet.AvailableBalance < amount {
		return nil, 0, 0, ErrInsufficientWalletBalance
	}

	balanceBefore := wallet.AvailableBalance

	wallet.AvailableBalance -= amount
	wallet.LockedBalance += amount

	balanceAfter := wallet.AvailableBalance

	err = tx.Save(&wallet).Error
	if err != nil {
		return nil, 0, 0, err
	}

	return &wallet, balanceBefore, balanceAfter, nil
}

func (repo *WalletRepository) CreateWalletLedgerTx(tx *gorm.DB, data *WalletLedger) (*WalletLedger, error) {
	if err := tx.Create(data).Error; err != nil {
		return nil, err
	}
	return data, nil
}

func (repo *WalletRepository) CancelFlexiblePlanTx(
	tx *gorm.DB,
	userID string,
	walletID string,
	planAmount int64,
	cancellationFee int64,
) (*Wallet, int64, int64, int64, int64, error) {
	var wallet Wallet

	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", walletID, userID).
		First(&wallet).Error

	if err != nil {
		return nil, 0, 0, 0, 0, err
	}

	if planAmount < 0 {
		return nil, 0, 0, 0, 0, ErrInvalidWalletAmount
	}

	if cancellationFee < 0 {
		return nil, 0, 0, 0, 0, ErrInvalidWalletAmount
	}

	if cancellationFee > planAmount {
		return nil, 0, 0, 0, 0, ErrInvalidWalletAmount
	}

	if wallet.LockedBalance < planAmount {
		return nil, 0, 0, 0, 0, ErrInsufficientLockedBalance
	}

	refundableAmount := planAmount - cancellationFee

	balanceBefore := wallet.AvailableBalance
	ledgerBefore := wallet.LedgerBalance

	wallet.LockedBalance -= planAmount
	wallet.AvailableBalance += refundableAmount
	wallet.LedgerBalance -= cancellationFee

	balanceAfter := wallet.AvailableBalance
	ledgerAfter := wallet.LedgerBalance

	err = tx.Save(&wallet).Error
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}

	return &wallet, balanceBefore, balanceAfter, ledgerBefore, ledgerAfter, nil
}

func (repo *WalletRepository) DebitLockedFundsForWithdrawalTx(
	tx *gorm.DB,
	userID string,
	walletID string,
	amount int64,
) (*Wallet, int64, int64, error) {
	var wallet Wallet

	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", walletID, userID).
		First(&wallet).Error

	if err != nil {
		return nil, 0, 0, err
	}

	if amount <= 0 {
		return nil, 0, 0, ErrInvalidWalletAmount
	}

	if wallet.LockedBalance < amount {
		return nil, 0, 0, ErrInsufficientLockedBalance
	}

	balanceBefore := wallet.LedgerBalance

	wallet.LockedBalance -= amount
	wallet.LedgerBalance -= amount

	balanceAfter := wallet.LedgerBalance

	err = tx.Save(&wallet).Error
	if err != nil {
		return nil, 0, 0, err
	}

	return &wallet, balanceBefore, balanceAfter, nil
}

// Data flow:
// 1. Lock wallet row with SELECT FOR UPDATE.
// 2. Ensure locked balance can cover this withdrawal.
// 3. Keep balances unchanged until Paystack confirms success.
func (r *WalletRepository) VerifyLockedFundsForWithdrawalTx(
	tx *gorm.DB,
	userID string,
	walletID string,
	amount int64,
) (*Wallet, error) {
	if tx == nil {
		return nil, errors.New("transaction is required")
	}

	var wallet Wallet

	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", walletID, userID).
		First(&wallet).Error
	if err != nil {
		return nil, err
	}

	if amount <= 0 {
		return nil, ErrInvalidWalletAmount
	}

	if wallet.LockedBalance < amount {
		return nil, ErrInsufficientLockedBalance
	}

	return &wallet, nil
}

// Data flow:
// 1. Lock wallet row.
// 2. Move funds from locked back to available.
// 3. Used when Paystack initiation fails or withdrawal is rejected.
func (r *WalletRepository) ReleaseWithdrawalHoldTx(
	tx *gorm.DB,
	userID string,
	walletID string,
	amount int64,
) (*Wallet, error) {
	if tx == nil {
		return nil, errors.New("transaction is required")
	}

	var wallet Wallet

	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", walletID, userID).
		First(&wallet).Error
	if err != nil {
		return nil, err
	}

	if wallet.LockedBalance < amount {
		return nil, errors.New("locked balance is less than withdrawal amount")
	}

	wallet.LockedBalance -= amount
	wallet.AvailableBalance += amount

	err = tx.Save(&wallet).Error
	if err != nil {
		return nil, err
	}

	return &wallet, nil
}

// Data flow:
// 1. Lock wallet row.
// 2. Remove funds from locked balance permanently.
// 3. Used only after Paystack webhook confirms successful transfer.
func (r *WalletRepository) CompleteWithdrawalTx(
	tx *gorm.DB,
	userID string,
	walletID string,
	amount int64,
) (int64, *Wallet, error) {
	if tx == nil {
		return 0, nil, errors.New("transaction is required")
	}

	var wallet Wallet

	err := tx.
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", walletID, userID).
		First(&wallet).Error
	if err != nil {
		return 0, nil, err
	}

	if wallet.LockedBalance < amount {
		return 0, nil, errors.New("locked balance is less than withdrawal amount")
	}

	ledgerBefore := wallet.LedgerBalance
	wallet.LockedBalance -= amount
	wallet.LedgerBalance -= amount

	err = tx.Save(&wallet).Error
	if err != nil {
		return 0, nil, err
	}

	return ledgerBefore, &wallet, nil
}

func (r *WalletRepository) ReverseCompletedWithdrawalTx(
	tx *gorm.DB,
	userID string,
	walletID string,
	amount int64,
) (int64, *Wallet, error) {
	if tx == nil {
		return 0, nil, errors.New("transaction is required")
	}
	if amount <= 0 {
		return 0, nil, ErrInvalidWalletAmount
	}

	var wallet Wallet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", walletID, userID).
		First(&wallet).Error; err != nil {
		return 0, nil, err
	}

	ledgerBefore := wallet.LedgerBalance
	wallet.LockedBalance += amount
	wallet.LedgerBalance += amount
	if err := tx.Save(&wallet).Error; err != nil {
		return 0, nil, err
	}
	return ledgerBefore, &wallet, nil
}

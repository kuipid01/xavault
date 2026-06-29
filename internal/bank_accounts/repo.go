package bankaccounts

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(account *BankAccount) (*BankAccount, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if account.IsDefault {
			if err := tx.
				Model(&BankAccount{}).
				Where("user_id = ? AND owner_type = ?", account.UserID, account.OwnerType).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}

		return tx.Create(account).Error
	})

	return account, err
}

func (r *Repository) FindByIDAndUserID(id string, userID string) (*BankAccount, error) {
	var account BankAccount

	err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}

		return nil, err
	}

	return &account, nil
}

func (r *Repository) FindByUserIDAndOwnerType(userID string, ownerType string) ([]BankAccount, error) {
	var accounts []BankAccount

	err := r.db.
		Where("user_id = ? AND owner_type = ?", userID, ownerType).
		Order("is_default DESC, created_at DESC").
		Find(&accounts).Error
	if err != nil {
		return nil, err
	}

	return accounts, nil
}

func (r *Repository) FindDuplicate(userID string, ownerType string, bankCode string, accountNumber string) (*BankAccount, error) {
	var account BankAccount

	err := r.db.
		Where(
			"user_id = ? AND owner_type = ? AND bank_code = ? AND account_number = ?",
			userID,
			ownerType,
			bankCode,
			accountNumber,
		).
		First(&account).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}

		return nil, err
	}

	return &account, nil
}

func (r *Repository) ClearDefault(userID string, ownerType string) error {
	return r.db.
		Model(&BankAccount{}).
		Where("user_id = ? AND owner_type = ?", userID, ownerType).
		Update("is_default", false).Error
}

func (r *Repository) SetDefault(id string, userID string, ownerType string) (*BankAccount, error) {
	account, err := r.FindByIDAndUserID(id, userID)
	if err != nil {
		return nil, err
	}

	if account == nil || account.OwnerType != ownerType {
		return nil, nil
	}

	err = r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Model(&BankAccount{}).
			Where("user_id = ? AND owner_type = ?", userID, ownerType).
			Update("is_default", false).Error; err != nil {
			return err
		}

		return tx.
			Model(&BankAccount{}).
			Where("id = ? AND user_id = ? AND owner_type = ?", id, userID, ownerType).
			Update("is_default", true).Error
	})
	if err != nil {
		return nil, err
	}

	return r.FindByIDAndUserID(id, userID)
}

func (r *Repository) Delete(id string, userID string, ownerType string) error {
	return r.db.
		Where("id = ? AND user_id = ? AND owner_type = ?", id, userID, ownerType).
		Delete(&BankAccount{}).Error
}

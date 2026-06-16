package client

import "gorm.io/gorm"

type IdempotentRepository struct {
	db *gorm.DB
}

func NewIdempotentRepository(db *gorm.DB) *IdempotentRepository {
	return &IdempotentRepository{db: db}
}

func (r *IdempotentRepository) Create(data *IdempotentKey) (*IdempotentKey, error) {
	// Note: Passed data instead of &data pointer-to-pointer references
	if err := r.db.Create(data).Error; err != nil {
		return nil, err
	}
	return data, nil
}

func (r *IdempotentRepository) FindByKey(key string) (*IdempotentKey, error) {
	var data IdempotentKey
	err := r.db.Where("key = ?", key).First(&data).Error
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// Added Update method to handle lifecycle tracking mutations cleanly
func (r *IdempotentRepository) Update(data *IdempotentKey) error {
	return r.db.Save(data).Error
}

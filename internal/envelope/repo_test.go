package envelope

import (
	"testing"

	"gorm.io/gorm"
)

func TestDBOrTxFallsBackToRepositoryDB(t *testing.T) {
	database := &gorm.DB{}
	repo := NewEnvelopeRepo(database)

	if got := repo.dbOrTx(nil); got != database {
		t.Fatal("expected nil transaction to use repository database")
	}

	transaction := &gorm.DB{}
	if got := repo.dbOrTx(transaction); got != transaction {
		t.Fatal("expected supplied transaction to be used")
	}
}

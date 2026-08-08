//go:build integration

package webhooks

import (
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type crashTestBalance struct {
	ID      string `gorm:"type:uuid;primaryKey"`
	Balance int64
}

func (crashTestBalance) TableName() string { return "webhook_crash_test_balances" }

func integrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL recovery integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&PaystackEvent{}, &crashTestBalance{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestFinancialMutationRollsBackWhenCompletionCrashes(t *testing.T) {
	db := integrationDB(t)
	store := NewEventStore(db)
	balanceID := uuid.NewString()
	if err := db.Create(&crashTestBalance{ID: balanceID}).Error; err != nil {
		t.Fatal(err)
	}
	event, err := NewPaystackEvent([]byte(`{"event":"charge.success","data":{"id":987654321,"reference":"crash-test","status":"success"}}`))
	if err != nil {
		t.Fatal(err)
	}
	event.ProviderEventID += ":" + uuid.NewString()
	if _, err := store.Receive(event); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Delete(&PaystackEvent{}, "id = ?", event.ID).Error
		_ = db.Delete(&crashTestBalance{}, "id = ?", balanceID).Error
	})

	claimed, ok, err := store.Claim(event.ID)
	if err != nil || !ok {
		t.Fatalf("claim failed: ok=%v err=%v", ok, err)
	}
	simulatedCrash := errors.New("simulated crash after financial mutation")
	err = store.CompleteInTransaction(claimed, func(tx *gorm.DB) error {
		if err := tx.Model(&crashTestBalance{}).Where("id = ?", balanceID).
			Update("balance", gorm.Expr("balance + 100")).Error; err != nil {
			return err
		}
		return simulatedCrash
	})
	if !errors.Is(err, simulatedCrash) {
		t.Fatalf("expected simulated crash, got %v", err)
	}
	var balance crashTestBalance
	if err := db.First(&balance, "id = ?", balanceID).Error; err != nil {
		t.Fatal(err)
	}
	if balance.Balance != 0 {
		t.Fatalf("financial mutation escaped rollback: balance=%d", balance.Balance)
	}

	if err := store.MarkFailed(claimed, simulatedCrash); err != nil {
		t.Fatal(err)
	}
	if err := store.MakeDue(event.ID); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err = store.Claim(event.ID)
	if err != nil || !ok {
		t.Fatalf("retry claim failed: ok=%v err=%v", ok, err)
	}
	if err := store.CompleteInTransaction(claimed, func(tx *gorm.DB) error {
		return tx.Model(&crashTestBalance{}).Where("id = ?", balanceID).
			Update("balance", gorm.Expr("balance + 100")).Error
	}); err != nil {
		t.Fatal(err)
	}
	_ = store.MakeDue(event.ID)
	if _, ok, err := store.Claim(event.ID); err != nil || ok {
		t.Fatalf("processed event was claimable: ok=%v err=%v", ok, err)
	}
	if err := db.First(&balance, "id = ?", balanceID).Error; err != nil {
		t.Fatal(err)
	}
	if balance.Balance != 100 {
		t.Fatalf("expected exactly one committed mutation, got %d", balance.Balance)
	}
}

func TestClaimDueBatchDoesNotDuplicateAcrossWorkers(t *testing.T) {
	db := integrationDB(t)
	store := NewEventStore(db)
	var ids []string
	for i := 0; i < 10; i++ {
		event, err := NewPaystackEvent([]byte(`{"event":"charge.success","data":{"reference":"batch-test","status":"success"}}`))
		if err != nil {
			t.Fatal(err)
		}
		event.ProviderEventID += ":" + uuid.NewString()
		if _, err := store.Receive(event); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, event.ID)
	}
	t.Cleanup(func() { _ = db.Delete(&PaystackEvent{}, "id IN ?", ids).Error })

	var wg sync.WaitGroup
	var mu sync.Mutex
	claimedIDs := map[string]bool{}
	var duplicate bool
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			events, err := store.ClaimDueBatch(10)
			if err != nil {
				t.Errorf("claim batch: %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, event := range events {
				if claimedIDs[event.ID] {
					duplicate = true
				}
				claimedIDs[event.ID] = true
			}
		}()
	}
	wg.Wait()
	if duplicate {
		t.Fatal("the same event was returned to two workers")
	}
	for _, id := range ids {
		if !claimedIDs[id] {
			t.Fatalf("event %s was not claimed", id)
		}
	}
}

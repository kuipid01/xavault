//go:build integration

package users_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kuipid01/salary-vault/internal/testsupport"
	"github.com/kuipid01/salary-vault/internal/users"
	"golang.org/x/crypto/bcrypt"
)

// pinFixture seeds a user with the given PIN already set.
func pinFixture(t *testing.T, pin string) (*users.UserRepository, string) {
	t.Helper()
	db := testsupport.FinancialDB(t)
	userID := uuid.NewString()
	user := users.User{
		ID: userID, Email: userID + "@example.test", Password: "hash",
		FirstName: "Pin", LastName: "Test", Country: "NG", PhoneNumber: "08000000000",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repo := users.NewUserRepository(db)
	if err := repo.UpdateWithdrawalPin(userID, string(hash)); err != nil {
		t.Fatal(err)
	}
	return repo, userID
}

func TestThreeWrongPinAttemptsLockAndCorrectPinStillFailsWhileLocked(t *testing.T) {
	repo, userID := pinFixture(t, "1234")

	// First two wrong attempts: invalid, not locked.
	if err := repo.VerifyWithdrawalPin(userID, "0000"); !errors.Is(err, users.ErrWithdrawalPinInvalid) {
		t.Fatalf("attempt 1: expected ErrWithdrawalPinInvalid, got %v", err)
	}
	if err := repo.VerifyWithdrawalPin(userID, "0000"); !errors.Is(err, users.ErrWithdrawalPinInvalid) {
		t.Fatalf("attempt 2: expected ErrWithdrawalPinInvalid, got %v", err)
	}

	// Third wrong attempt triggers the lock.
	if err := repo.VerifyWithdrawalPin(userID, "0000"); !errors.Is(err, users.ErrWithdrawalPinLocked) {
		t.Fatalf("attempt 3: expected ErrWithdrawalPinLocked, got %v", err)
	}

	// While locked, even the CORRECT PIN is rejected — by design.
	if err := repo.VerifyWithdrawalPin(userID, "1234"); !errors.Is(err, users.ErrWithdrawalPinLocked) {
		t.Fatalf("locked account: correct pin should still be rejected, got %v", err)
	}

	// Resetting the PIN clears the lock and the new PIN verifies.
	hash, _ := bcrypt.GenerateFromPassword([]byte("5678"), bcrypt.MinCost)
	if err := repo.UpdateWithdrawalPin(userID, string(hash)); err != nil {
		t.Fatal(err)
	}
	if err := repo.VerifyWithdrawalPin(userID, "5678"); err != nil {
		t.Fatalf("after reset, correct new pin should verify, got %v", err)
	}
}

func TestCorrectPinAfterWrongAttemptRestoresCounter(t *testing.T) {
	repo, userID := pinFixture(t, "1234")

	if err := repo.VerifyWithdrawalPin(userID, "9999"); !errors.Is(err, users.ErrWithdrawalPinInvalid) {
		t.Fatalf("attempt 1: expected ErrWithdrawalPinInvalid, got %v", err)
	}
	if err := repo.VerifyWithdrawalPin(userID, "1234"); err != nil {
		t.Fatalf("correct pin after one wrong attempt should verify, got %v", err)
	}

	// Counter reset by success: two more wrong attempts must still not lock early.
	if err := repo.VerifyWithdrawalPin(userID, "9999"); !errors.Is(err, users.ErrWithdrawalPinInvalid) {
		t.Fatalf("expected ErrWithdrawalPinInvalid, got %v", err)
	}
	if err := repo.VerifyWithdrawalPin(userID, "9999"); !errors.Is(err, users.ErrWithdrawalPinInvalid) {
		t.Fatalf("expected ErrWithdrawalPinInvalid (not locked), got %v", err)
	}
	if err := repo.VerifyWithdrawalPin(userID, "1234"); err != nil {
		t.Fatalf("counter should have been reset, correct pin must verify, got %v", err)
	}
}

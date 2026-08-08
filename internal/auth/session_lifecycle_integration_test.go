//go:build integration

package auth_test

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/kuipid01/salary-vault/internal/auth"
	"github.com/kuipid01/salary-vault/internal/subscription"
	"github.com/kuipid01/salary-vault/internal/testsupport"
	"github.com/kuipid01/salary-vault/internal/users"
	"github.com/kuipid01/salary-vault/internal/wallets"
)

func TestCompletedRegistrationCreatesWalletAndFreeSubscription(t *testing.T) {
	db := testsupport.FinancialDB(t)
	email := "new-free-user@example.test"
	authRepo := auth.NewAuthRepository(db)
	if err := authRepo.UpsertTemporaryUser(&auth.TemporaryUser{ID: "ad45ae82-663d-4f5b-940f-dbb7da8300c6", Email: email, TokenHash: "", ExpiresAt: time.Now().Add(time.Hour), Verified: true}); err != nil {
		t.Fatal(err)
	}
	userRepo := users.NewUserRepository(db)
	subRepo := subscription.NewSubscriptionRepository(db)
	walletService := wallets.NewWalletService(wallets.NewWalletRepository(db), nil, nil, subRepo)
	service := auth.NewAuthService(userRepo, authRepo, nil, &auth.TokenService{AccessSecret: []byte("test")}, walletService)
	service.SetRegistrationDB(db)
	result := service.Register(auth.RegisterRequest{Email: email, Password: "Strong!Pass1", ConfirmPassword: "Strong!Pass1", FirstName: "Free", LastName: "User", Country: "NG", PhoneNumber: "08012345678"})
	if !result.Success {
		t.Fatalf("registration failed: %+v", result)
	}
	user, err := userRepo.FindByEmail(email)
	if err != nil || user == nil {
		t.Fatalf("registered user missing: user=%v err=%v", user, err)
	}
	var walletCount int64
	if err := db.Model(&wallets.Wallet{}).Where("user_id = ?", user.ID).Count(&walletCount).Error; err != nil {
		t.Fatal(err)
	}
	userID := uuid.MustParse(user.ID)
	active, err := subRepo.GetActiveSubscription(userID)
	if err != nil {
		t.Fatal(err)
	}
	if walletCount != 1 || active.Plan.Code != subscription.PlanFree {
		t.Fatalf("wallets=%d subscription=%+v", walletCount, active.Plan)
	}
	if err := subRepo.CanAddMonthlyVolume(userID, 0, 100_000); err != nil {
		t.Fatalf("Free user could not deposit NGN 1,000: %v", err)
	}
}

func TestTemporaryRegistrationUpsertsByEmail(t *testing.T) {
	db := testsupport.FinancialDB(t)
	repo := auth.NewAuthRepository(db)
	email := "registration-upsert@example.test"
	first := &auth.TemporaryUser{ID: "6a516e8b-1548-4cd7-a0ff-d58615977be0", Email: email, TokenHash: "first", ExpiresAt: time.Now().Add(time.Minute)}
	second := &auth.TemporaryUser{ID: "942d6694-60d6-4689-b7f4-933b47d58801", Email: email, TokenHash: "second", ExpiresAt: time.Now().Add(2 * time.Minute)}
	if err := repo.UpsertTemporaryUser(first); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertTemporaryUser(second); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.FindTemporaryUserByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.TokenHash != "second" {
		t.Fatalf("temporary registration was not updated: %+v", stored)
	}
	var count int64
	if err := db.Model(&auth.TemporaryUser{}).Where("email = ?", email).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("got %d temporary registrations, want 1", count)
	}
}

func authFixture(t *testing.T) (*auth.AuthService, *users.UserRepository, users.User, string) {
	t.Helper()
	db := testsupport.FinancialDB(t)
	user, _ := testsupport.SeedUserWallet(t, db, 0, 0, 0)
	repo := users.NewUserRepository(db)
	tokens := &auth.TokenService{AccessSecret: []byte("financial-test-secret-at-least-32-bytes"), Issuer: "xavault-test"}
	service := auth.NewAuthService(repo, nil, nil, tokens, nil)
	_, refresh, err := auth.GenerateAllToken(user.ID, user.Email, tokens)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateRefreshToken(user.ID, refresh); err != nil {
		t.Fatal(err)
	}
	return service, repo, user, refresh
}

func TestRefreshRotatesTokenAndRejectsReplay(t *testing.T) {
	service, repo, user, original := authFixture(t)
	result := service.RefreshToken(auth.RefreshTokenRequest{RefreshToken: original})
	if !result.Success {
		t.Fatalf("refresh failed: %+v", result)
	}
	data, ok := result.Data.(fiber.Map)
	if !ok {
		t.Fatalf("unexpected response data %T", result.Data)
	}
	rotated, _ := data["refresh_token"].(string)
	if rotated == "" || rotated == original {
		t.Fatal("refresh token was not rotated")
	}
	if found, err := repo.GetUserWithToken(original); err != nil || found != nil {
		t.Fatalf("original token remained usable: user=%v err=%v", found, err)
	}
	if replay := service.RefreshToken(auth.RefreshTokenRequest{RefreshToken: original}); replay.Success {
		t.Fatal("rotated refresh token was replayed successfully")
	}
	if found, err := repo.GetUserWithToken(rotated); err != nil || found == nil || found.ID != user.ID {
		t.Fatalf("rotated token was not persisted: user=%v err=%v", found, err)
	}
}

func TestLogoutAndLogoutAllRevokeRefreshSessions(t *testing.T) {
	service, repo, user, refresh := authFixture(t)
	if result := service.Logout(auth.RefreshTokenRequest{RefreshToken: refresh}); !result.Success {
		t.Fatalf("logout failed: %+v", result)
	}
	if result := service.RefreshToken(auth.RefreshTokenRequest{RefreshToken: refresh}); result.Success {
		t.Fatal("logged-out token refreshed successfully")
	}
	_, second, err := auth.GenerateAllToken(user.ID, user.Email, &auth.TokenService{AccessSecret: []byte("financial-test-secret-at-least-32-bytes"), Issuer: "xavault-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateRefreshToken(user.ID, second); err != nil {
		t.Fatal(err)
	}
	if result := service.LogoutAllDevices(user.ID); !result.Success {
		t.Fatalf("logout-all failed: %+v", result)
	}
	if result := service.RefreshToken(auth.RefreshTokenRequest{RefreshToken: second}); result.Success {
		t.Fatal("logout-all token refreshed successfully")
	}
}

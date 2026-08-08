//go:build integration

package auth_test

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/auth"
	"github.com/kuipid01/salary-vault/internal/testsupport"
	"github.com/kuipid01/salary-vault/internal/users"
)

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

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/platform/db"
)

func TestAuthServiceFlow(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	svc := auth.NewService(sqlDB, "test-jwt-secret")

	// Register
	u, err := svc.Register(ctx, "qa@kiwit.local", "qa", "supersecret123")
	require.NoError(t, err)
	require.Equal(t, "qa@kiwit.local", u.Email)

	// Wrong password fails
	_, _, err = svc.Login(ctx, "qa@kiwit.local", "wrongpass")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)

	// Login issues JWT
	jwtToken, loggedIn, err := svc.Login(ctx, "qa@kiwit.local", "supersecret123")
	require.NoError(t, err)
	require.NotEmpty(t, jwtToken)
	require.Equal(t, u.ID, loggedIn.ID)

	// AuthenticateBearer with JWT
	authUser, err := svc.AuthenticateBearer(ctx, jwtToken)
	require.NoError(t, err)
	require.Equal(t, u.ID, authUser.ID)

	// Create CI/CD API token and authenticate with it
	apiToken, err := svc.CreateAPIToken(ctx, u.ID, "github-actions")
	require.NoError(t, err)
	require.Contains(t, apiToken, "kiwit_")

	tokenUser, err := svc.AuthenticateBearer(ctx, apiToken)
	require.NoError(t, err)
	require.Equal(t, u.ID, tokenUser.ID)
}

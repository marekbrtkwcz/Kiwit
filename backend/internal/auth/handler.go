package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type RegisterInput struct {
	Body struct {
		Email    string `json:"email" format:"email"`
		Username string `json:"username" minLength:"2" maxLength:"64"`
		Password string `json:"password" minLength:"8"`
	}
}

type UserOutput struct {
	Body User
}

type LoginInput struct {
	Body struct {
		Email    string `json:"email" format:"email" doc:"User email"`
		Password string `json:"password" minLength:"8" doc:"User password"`
	}
}

type LoginOutput struct {
	Body struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		User        User   `json:"user"`
	}
}

type CreateTokenInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"80"`
	}
}

type CreateTokenOutput struct {
	Body struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}
}

// RegisterRoutes registers authentication OpenAPI endpoints.
func RegisterRoutes(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-register",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/register",
		Summary:     "Register a new user",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *RegisterInput) (*UserOutput, error) {
		u, err := svc.Register(ctx, input.Body.Email, input.Body.Username, input.Body.Password)
		if err != nil {
			if errors.Is(err, ErrRegistrationClosed) {
				return nil, huma.Error403Forbidden("registration is disabled")
			}
			return nil, huma.Error409Conflict("failed to register user", err)
		}
		return &UserOutput{Body: u}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/login",
		Summary:     "Authenticate user and issue JWT",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
		token, u, err := svc.Login(ctx, input.Body.Email, input.Body.Password)
		if err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				return nil, huma.Error401Unauthorized("invalid email or password")
			}
			return nil, huma.Error500InternalServerError("login failed", err)
		}
		out := &LoginOutput{}
		out.Body.TokenType = "Bearer"
		out.Body.AccessToken = token
		out.Body.User = u
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-create-token",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/tokens",
		Summary:     "Create CI/CD API token",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *CreateTokenInput) (*CreateTokenOutput, error) {
		u, ok := UserFromContext(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}
		if IsAPIToken(ctx) {
			return nil, huma.Error403Forbidden("API tokens cannot create other API tokens")
		}
		plain, err := svc.CreateAPIToken(ctx, u.ID, input.Body.Name)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to create token", err)
		}
		out := &CreateTokenOutput{}
		out.Body.Token = plain
		out.Body.Name = input.Body.Name
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-change-password",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/change-password",
		Summary:     "Change password for authenticated user",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			CurrentPassword string `json:"current_password" minLength:"8"`
			NewPassword     string `json:"new_password" minLength:"8"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		u, ok := UserFromContext(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}
		if IsAPIToken(ctx) {
			return nil, huma.Error403Forbidden("API tokens cannot change account password")
		}
		if err := svc.ChangePassword(ctx, u.ID, input.Body.CurrentPassword, input.Body.NewPassword); err != nil {
			if errors.Is(err, ErrInvalidCredentials) {
				return nil, huma.Error401Unauthorized("current password is incorrect")
			}
			return nil, huma.Error500InternalServerError("failed to change password", err)
		}
		out := &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{}
		out.Body.OK = true
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-list-tokens",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/tokens",
		Summary:     "List API tokens for authenticated user",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []APITokenInfo `json:"items"`
		}
	}, error) {
		u, ok := UserFromContext(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}
		if IsAPIToken(ctx) {
			return nil, huma.Error403Forbidden("API tokens cannot manage other API tokens")
		}
		items, err := svc.ListAPITokens(ctx, u.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list tokens", err)
		}
		out := &struct {
			Body struct {
				Items []APITokenInfo `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-revoke-token",
		Method:      http.MethodDelete,
		Path:        "/api/v1/auth/tokens/{id}",
		Summary:     "Revoke an API token owned by authenticated user",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		u, ok := UserFromContext(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}
		if IsAPIToken(ctx) {
			return nil, huma.Error403Forbidden("API tokens cannot revoke API tokens")
		}
		if err := svc.RevokeAPIToken(ctx, u.ID, input.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("token not found")
			}
			return nil, huma.Error500InternalServerError("failed to revoke token", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-request-password-reset",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/password-reset",
		Summary:     "Request a password reset link (always succeeds; prevents account enumeration)",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Email string `json:"email" format:"email"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := svc.RequestPasswordReset(ctx, input.Body.Email); err != nil {
			return nil, huma.Error500InternalServerError("failed to request password reset", err)
		}
		out := &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{}
		out.Body.OK = true
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "auth-confirm-password-reset",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/password-reset/confirm",
		Summary:     "Set a new password with a one-time reset token (HTTP 400 for unknown, expired or used tokens)",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Token       string `json:"token" minLength:"1"`
			NewPassword string `json:"new_password" minLength:"8"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if IsAPIToken(ctx) {
			return nil, huma.Error403Forbidden("API tokens cannot reset account passwords")
		}
		if err := svc.ConfirmPasswordReset(ctx, input.Body.Token, input.Body.NewPassword); err != nil {
			if errors.Is(err, ErrResetTokenInvalid) {
				return nil, huma.Error400BadRequest("invalid, expired or used password reset token")
			}
			return nil, huma.Error500InternalServerError("failed to confirm password reset", err)
		}
		out := &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{}
		out.Body.OK = true
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-request-password-reset",
		Method:      http.MethodPost,
		Path:        "/api/v1/users/{id}/password-reset",
		Summary:     "Force a password reset for a user (admin only)",
		Description: "Always creates a fresh one-time reset token. With SMTP configured the link is emailed to the user; without SMTP the reset_url is returned in the response (self-hosted fallback).",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct {
		Body struct {
			Emailed  bool   `json:"emailed"`
			ResetURL string `json:"reset_url,omitempty"`
		}
	}, error) {
		if err := CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		resetURL, emailed, err := svc.AdminRequestPasswordReset(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("user not found")
			}
			return nil, huma.Error500InternalServerError("failed to request password reset", err)
		}
		return &struct {
			Body struct {
				Emailed  bool   `json:"emailed"`
				ResetURL string `json:"reset_url,omitempty"`
			}
		}{Body: struct {
			Emailed  bool   `json:"emailed"`
			ResetURL string `json:"reset_url,omitempty"`
		}{Emailed: emailed, ResetURL: resetURL}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-users",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/users",
		Summary:     "List all users (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []UserSummary `json:"items"`
		}
	}, error) {
		if err := CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		items, err := svc.ListUsers(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list users", err)
		}
		out := &struct {
			Body struct {
				Items []UserSummary `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-user",
		Method:      http.MethodPatch,
		Path:        "/api/v1/admin/users/{id}",
		Summary:     "Update user role or active status (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			Role     string `json:"role" enum:"viewer,tester,admin"`
			IsActive bool   `json:"is_active"`
		}
	}) (*struct{ Body UserSummary }, error) {
		if err := CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		caller, _ := UserFromContext(ctx)
		if caller.ID == input.ID && (!input.Body.IsActive || input.Body.Role != "admin") {
			return nil, huma.Error400BadRequest("admin cannot deactivate or demote themselves")
		}
		updated, err := svc.UpdateUser(ctx, input.ID, input.Body.Role, input.Body.IsActive)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("user not found")
			}
			return nil, huma.Error500InternalServerError("failed to update user", err)
		}
		return &struct{ Body UserSummary }{Body: updated}, nil
	})
}

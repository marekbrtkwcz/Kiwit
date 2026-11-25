package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

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
	Authorization string `header:"Authorization" doc:"Bearer JWT"`
	Body          struct {
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
		raw := strings.TrimPrefix(input.Authorization, "Bearer ")
		u, err := svc.AuthenticateBearer(ctx, raw)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthorized")
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
		Authorization string `header:"Authorization"`
		Body          struct {
			NewPassword string `json:"new_password" minLength:"8"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		raw := strings.TrimPrefix(input.Authorization, "Bearer ")
		u, err := svc.AuthenticateBearer(ctx, raw)
		if err != nil {
			return nil, huma.Error401Unauthorized("unauthorized")
		}
		if err := svc.ChangePassword(ctx, u.ID, input.Body.NewPassword); err != nil {
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
}

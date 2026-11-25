package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
)

type User struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type Service struct {
	db        *sql.DB
	jwtSecret []byte
}

func NewService(db *sql.DB, jwtSecret string) *Service {
	return &Service{db: db, jwtSecret: []byte(jwtSecret)}
}

func (s *Service) Register(ctx context.Context, email, username, password string) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (email, username, password_hash, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, 'tester', 1, ?, ?)`,
		email, username, string(hash), now, now,
	)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("last insert id: %w", err)
	}
	return User{ID: id, Email: email, Username: username, Role: "tester"}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, User, error) {
	var u User
	var passwordHash string
	var active int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, username, role, password_hash, is_active
		FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &u.Username, &u.Role, &passwordHash, &active)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", User{}, ErrInvalidCredentials
		}
		return "", User{}, fmt.Errorf("query user: %w", err)
	}
	if active != 1 {
		return "", User{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return "", User{}, ErrInvalidCredentials
	}

	claims := jwt.MapClaims{
		"sub":   u.ID,
		"email": u.Email,
		"role":  u.Role,
		"exp":   time.Now().UTC().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", User{}, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, u, nil
}

// CreateAPIToken generates a random CI/CD token (returned once) and stores its SHA-256 hash.
func (s *Service) CreateAPIToken(ctx context.Context, userID int64, name string) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	plain := "kiwit_" + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))
	tokenHash := hex.EncodeToString(sum[:])

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO api_tokens (user_id, name, token_hash, created_at)
		VALUES (?, ?, ?, ?)`,
		userID, name, tokenHash, time.Now().UTC(),
	); err != nil {
		return "", fmt.Errorf("insert api token: %w", err)
	}
	return plain, nil
}

// AuthenticateBearer validates either an API token (kiwit_*) or a JWT.
func (s *Service) AuthenticateBearer(ctx context.Context, bearerToken string) (User, error) {
	if strings.HasPrefix(bearerToken, "kiwit_") {
		sum := sha256.Sum256([]byte(bearerToken))
		tokenHash := hex.EncodeToString(sum[:])

		var u User
		err := s.db.QueryRowContext(ctx, `
			SELECT u.id, u.email, u.username, u.role
			FROM api_tokens t
			JOIN users u ON u.id = t.user_id
			WHERE t.token_hash = ? AND u.is_active = 1`, tokenHash,
		).Scan(&u.ID, &u.Email, &u.Username, &u.Role)
		if err != nil {
			return User{}, ErrUnauthorized
		}
		_, _ = s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE token_hash = ?`, time.Now().UTC(), tokenHash)
		return u, nil
	}

	parsed, err := jwt.Parse(bearerToken, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnauthorized
		}
		return s.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return User{}, ErrUnauthorized
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return User{}, ErrUnauthorized
	}
	sub, ok := claims["sub"].(float64)
	if !ok {
		return User{}, ErrUnauthorized
	}
	email, _ := claims["email"].(string)
	role, _ := claims["role"].(string)
	return User{ID: int64(sub), Email: email, Role: role}, nil
}

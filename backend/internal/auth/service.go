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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/captcha"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrRegistrationClosed = errors.New("registration is disabled")
)

type User struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type UserSummary struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type APITokenInfo struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type Service struct {
	db                *sql.DB
	pgPool            *pgxpool.Pool
	jwtSecret         []byte
	allowRegistration bool
	anonymousAccess   bool // when true, GET requests without token get AnonymousUser (read-only)
	emailSender       EmailSender
	baseURL           string
	ldapCfg           *LDAPConfig
	captcha           *captcha.Store // nil = captcha disabled
}

// SetAnonymousAccess enables public read-only access (AnonymousUser) for
// unauthenticated GET requests. Equivalent of kiwitcms AnonymousViewBackend.
func (s *Service) SetAnonymousAccess(enabled bool) {
	s.anonymousAccess = enabled
}

// AnonymousAccessEnabled reports whether anonymous read-only access is on.
func (s *Service) AnonymousAccessEnabled() bool {
	return s.anonymousAccess
}

// SetCaptcha enables the self-hosted captcha on registration.
func (s *Service) SetCaptcha(c *captcha.Store) {
	s.captcha = c
}

// CaptchaEnabled reports whether captcha verification is required on registration.
func (s *Service) CaptchaEnabled() bool {
	return s.captcha != nil
}

// VerifyCaptcha validates a captcha answer against the store.
func (s *Service) VerifyCaptcha(id, answer string) error {
	if s.captcha == nil {
		return nil
	}
	return s.captcha.Verify(id, answer)
}

// CaptchaStore returns the captcha store (for mounting the challenge endpoint).
func (s *Service) CaptchaStore() *captcha.Store {
	return s.captcha
}

const defaultBaseURL = "http://localhost:8080"

func NewService(db *sql.DB, jwtSecret string) *Service {
	return &Service{db: db, jwtSecret: []byte(jwtSecret), allowRegistration: true, baseURL: defaultBaseURL}
}

func NewPostgresService(pool *pgxpool.Pool, jwtSecret string) *Service {
	return &Service{pgPool: pool, jwtSecret: []byte(jwtSecret), allowRegistration: true, baseURL: defaultBaseURL}
}

// SetAllowRegistration controls whether self-registration is enabled after the initial admin user exists.
func (s *Service) SetAllowRegistration(allow bool) {
	s.allowRegistration = allow
}

// SetEmailSender wires the password reset e-mail channel (usually *notify.EmailClient).
func (s *Service) SetEmailSender(sender EmailSender) {
	s.emailSender = sender
}

// SetBaseURL sets the public base URL used to build password reset links.
func (s *Service) SetBaseURL(u string) {
	s.baseURL = strings.TrimSpace(u)
}

// SetLDAPConfig enables LDAP authentication when cfg is non-nil.
func (s *Service) SetLDAPConfig(cfg *LDAPConfig) {
	s.ldapCfg = cfg
}

// LDAPLogin authenticates against LDAP and issues a JWT for the matching local user.
func (s *Service) LDAPLogin(ctx context.Context, username, password string) (string, User, error) {
	if s.ldapCfg == nil {
		return "", User{}, fmt.Errorf("ldap not configured")
	}
	email, displayName, err := LDAPAuthenticate(ctx, *s.ldapCfg, username, password)
	if err != nil {
		return "", User{}, err
	}
	u, err := s.FindOrCreateSSOUser(ctx, email, displayName)
	if err != nil {
		return "", User{}, err
	}
	token, err := s.IssueJWT(u)
	if err != nil {
		return "", User{}, err
	}
	return token, u, nil
}

func (s *Service) Register(ctx context.Context, email, username, password string) (User, error) {
	if err := ValidatePassword(password, email, username); err != nil {
		return User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	if s.pgPool != nil {
		tx, err := s.pgPool.Begin(ctx)
		if err != nil {
			return User{}, fmt.Errorf("begin tx: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		var count int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
			return User{}, fmt.Errorf("count users: %w", err)
		}
		if count > 0 && !s.allowRegistration {
			return User{}, ErrRegistrationClosed
		}
		role := "tester"
		if count == 0 {
			role = "admin"
		}
		var id int64
		err = tx.QueryRow(ctx, `
			INSERT INTO users (email, username, password_hash, role, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, TRUE, $5, $6) RETURNING id`,
			email, username, string(hash), role, now, now,
		).Scan(&id)
		if err != nil {
			return User{}, fmt.Errorf("insert user: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return User{}, fmt.Errorf("commit tx: %w", err)
		}
		return User{ID: id, Email: email, Username: username, Role: role}, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return User{}, fmt.Errorf("count users: %w", err)
	}
	if count > 0 && !s.allowRegistration {
		return User{}, ErrRegistrationClosed
	}
	role := "tester"
	if count == 0 {
		role = "admin"
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO users (email, username, password_hash, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)`,
		email, username, string(hash), role, now, now,
	)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("last insert id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit tx: %w", err)
	}
	return User{ID: id, Email: email, Username: username, Role: role}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, User, error) {
	var u User
	var passwordHash string
	if s.pgPool != nil {
		var active bool
		err := s.pgPool.QueryRow(ctx, `
			SELECT id, email, username, role, password_hash, is_active
			FROM users WHERE email = $1`, email,
		).Scan(&u.ID, &u.Email, &u.Username, &u.Role, &passwordHash, &active)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", User{}, ErrInvalidCredentials
			}
			return "", User{}, fmt.Errorf("query user: %w", err)
		}
		if !active {
			return "", User{}, ErrInvalidCredentials
		}
	} else {
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

// FindOrCreateSSOUser finds or creates a user by email from an OIDC identity.
// First user ever created gets the admin role.
func (s *Service) FindOrCreateSSOUser(ctx context.Context, email, username string) (User, error) {
	if username == "" {
		username = email
	}
	now := time.Now().UTC()
	if s.pgPool != nil {
		var u User
		err := s.pgPool.QueryRow(ctx, `
			SELECT id, email, username, role FROM users WHERE email = $1`, email,
		).Scan(&u.ID, &u.Email, &u.Username, &u.Role)
		if err == nil {
			return u, nil
		}
		var count int
		if err := s.pgPool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
			return User{}, fmt.Errorf("count users: %w", err)
		}
		role := "tester"
		if count == 0 {
			role = "admin"
		}
		var id int64
		err = s.pgPool.QueryRow(ctx, `
			INSERT INTO users (email, username, password_hash, role, is_active, created_at, updated_at)
			VALUES ($1, $2, '', $3, TRUE, $4, $5) RETURNING id`,
			email, username, role, now, now,
		).Scan(&id)
		if err != nil {
			return User{}, fmt.Errorf("insert sso user: %w", err)
		}
		return User{ID: id, Email: email, Username: username, Role: role}, nil
	}

	var u User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, username, role FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &u.Username, &u.Role)
	if err == nil {
		return u, nil
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return User{}, fmt.Errorf("count users: %w", err)
	}
	role := "tester"
	if count == 0 {
		role = "admin"
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (email, username, password_hash, role, is_active, created_at, updated_at)
		VALUES (?, ?, '', ?, 1, ?, ?)`,
		email, username, role, now, now,
	)
	if err != nil {
		return User{}, fmt.Errorf("insert sso user: %w", err)
	}
	id, _ := res.LastInsertId()
	return User{ID: id, Email: email, Username: username, Role: role}, nil
}

// IssueJWT issues a signed JWT for the given user.
func (s *Service) IssueJWT(u User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   u.ID,
		"email": u.Email,
		"role":  u.Role,
		"exp":   time.Now().UTC().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
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

	if s.pgPool != nil {
		if _, err := s.pgPool.Exec(ctx, `
			INSERT INTO api_tokens (user_id, name, token_hash, created_at)
			VALUES ($1, $2, $3, $4)`,
			userID, name, tokenHash, time.Now().UTC(),
		); err != nil {
			return "", fmt.Errorf("insert api token: %w", err)
		}
		return plain, nil
	}

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
		if s.pgPool != nil {
			err := s.pgPool.QueryRow(ctx, `
				SELECT u.id, u.email, u.username, u.role
				FROM api_tokens t
				JOIN users u ON u.id = t.user_id
				WHERE t.token_hash = $1 AND u.is_active = TRUE`, tokenHash,
			).Scan(&u.ID, &u.Email, &u.Username, &u.Role)
			if err != nil {
				return User{}, ErrUnauthorized
			}
			_, _ = s.pgPool.Exec(ctx, `UPDATE api_tokens SET last_used_at = $1 WHERE token_hash = $2`, time.Now().UTC(), tokenHash)
			return u, nil
		}

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
	userID := int64(sub)
	var u User
	if s.pgPool != nil {
		var active bool
		err := s.pgPool.QueryRow(ctx, `SELECT id, email, username, role, is_active FROM users WHERE id = $1`, userID).
			Scan(&u.ID, &u.Email, &u.Username, &u.Role, &active)
		if err != nil || !active {
			return User{}, ErrUnauthorized
		}
		return u, nil
	}
	var active int
	err = s.db.QueryRowContext(ctx, `SELECT id, email, username, role, is_active FROM users WHERE id = ?`, userID).
		Scan(&u.ID, &u.Email, &u.Username, &u.Role, &active)
	if err != nil || active != 1 {
		return User{}, ErrUnauthorized
	}
	return u, nil
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword string) error {
	var passwordHash string
	if s.pgPool != nil {
		err := s.pgPool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&passwordHash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrUnauthorized
			}
			return fmt.Errorf("query user: %w", err)
		}
	} else {
		err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&passwordHash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrUnauthorized
			}
			return fmt.Errorf("query user: %w", err)
		}
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}
	if err := ValidatePassword(newPassword, "", ""); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := time.Now().UTC()
	if s.pgPool != nil {
		_, err := s.pgPool.Exec(ctx, `UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3`, string(hash), now, userID)
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, string(hash), now, userID)
	return err
}

func (s *Service) ListAPITokens(ctx context.Context, userID int64) ([]APITokenInfo, error) {
	items := make([]APITokenInfo, 0)
	if s.pgPool != nil {
		rows, err := s.pgPool.Query(ctx, `
			SELECT id, name, created_at, last_used_at
			FROM api_tokens WHERE user_id = $1 ORDER BY id DESC`, userID)
		if err != nil {
			return nil, fmt.Errorf("query api tokens: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var t APITokenInfo
			var lastUsed sql.NullTime
			if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &lastUsed); err != nil {
				return nil, fmt.Errorf("scan api token: %w", err)
			}
			if lastUsed.Valid {
				ts := lastUsed.Time
				t.LastUsedAt = &ts
			}
			items = append(items, t)
		}
		return items, rows.Err()
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, created_at, last_used_at
		FROM api_tokens WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query api tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var t APITokenInfo
		var lastUsed sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &lastUsed); err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		if lastUsed.Valid {
			ts := lastUsed.Time
			t.LastUsedAt = &ts
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (s *Service) RevokeAPIToken(ctx context.Context, userID, tokenID int64) error {
	if s.pgPool != nil {
		tag, err := s.pgPool.Exec(ctx, `DELETE FROM api_tokens WHERE id = $1 AND user_id = $2`, tokenID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return sql.ErrNoRows
		}
		return nil
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ? AND user_id = ?`, tokenID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Service) ListUsers(ctx context.Context) ([]UserSummary, error) {
	items := make([]UserSummary, 0)
	if s.pgPool != nil {
		rows, err := s.pgPool.Query(ctx, `
			SELECT id, email, username, role, is_active, created_at
			FROM users ORDER BY id ASC`)
		if err != nil {
			return nil, fmt.Errorf("query users: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var u UserSummary
			if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.Role, &u.IsActive, &u.CreatedAt); err != nil {
				return nil, fmt.Errorf("scan user: %w", err)
			}
			items = append(items, u)
		}
		return items, rows.Err()
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, username, role, is_active, created_at
		FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var u UserSummary
		var active int
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.Role, &active, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.IsActive = active == 1
		items = append(items, u)
	}
	return items, rows.Err()
}

func (s *Service) UpdateUser(ctx context.Context, userID int64, role string, isActive bool) (UserSummary, error) {
	now := time.Now().UTC()
	var u UserSummary
	if s.pgPool != nil {
		err := s.pgPool.QueryRow(ctx, `
			UPDATE users SET role = $1, is_active = $2, updated_at = $3
			WHERE id = $4
			RETURNING id, email, username, role, is_active, created_at`,
			role, isActive, now, userID,
		).Scan(&u.ID, &u.Email, &u.Username, &u.Role, &u.IsActive, &u.CreatedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return UserSummary{}, sql.ErrNoRows
			}
			return UserSummary{}, err
		}
		return u, nil
	}

	activeInt := 0
	if isActive {
		activeInt = 1
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE users SET role = ?, is_active = ?, updated_at = ? WHERE id = ?`,
		role, activeInt, now, userID,
	)
	if err != nil {
		return UserSummary{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return UserSummary{}, sql.ErrNoRows
	}
	var active int
	err = s.db.QueryRowContext(ctx, `
		SELECT id, email, username, role, is_active, created_at FROM users WHERE id = ?`, userID,
	).Scan(&u.ID, &u.Email, &u.Username, &u.Role, &active, &u.CreatedAt)
	if err != nil {
		return UserSummary{}, err
	}
	u.IsActive = active == 1
	return u, nil
}

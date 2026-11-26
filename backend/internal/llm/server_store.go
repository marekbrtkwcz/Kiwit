package llm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Server is a configured AI server (SquashTM-style), stored per instance.
type Server struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Provider    string    `json:"provider" enum:"openai,anthropic,mistral,azure,vertex,custom"`
	BaseURL     string    `json:"base_url"`
	Model       string    `json:"model"`
	APIKey      string    `json:"api_key,omitempty"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	TimeoutSec  int       `json:"timeout_sec"`
	Retries     int       `json:"retries"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ToConfig converts a stored server into a callable Config.
func (s Server) ToConfig() Config {
	return Config{
		APIKey:      s.APIKey,
		BaseURL:     strings.TrimRight(s.BaseURL, "/"),
		Model:       s.Model,
		Temperature: s.Temperature,
		MaxTokens:   s.MaxTokens,
		TimeoutSec:  s.TimeoutSec,
		Retries:     s.Retries,
	}
}

type ServerStore struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewServerStore(db *sql.DB, pg *pgxpool.Pool) *ServerStore {
	return &ServerStore{db: db, pg: pg}
}

const serverCols = `id, name, provider, base_url, model, api_key, temperature, max_tokens, timeout_sec, retries, is_default, created_at, updated_at`

func scanServer(row interface{ Scan(...any) error }) (Server, error) {
	var s Server
	var def any
	err := row.Scan(&s.ID, &s.Name, &s.Provider, &s.BaseURL, &s.Model, &s.APIKey, &s.Temperature, &s.MaxTokens, &s.TimeoutSec, &s.Retries, &def, &s.CreatedAt, &s.UpdatedAt)
	if err == nil {
		switch v := def.(type) {
		case bool:
			s.IsDefault = v
		case int64:
			s.IsDefault = v != 0
		}
	}
	return s, err
}

type ServerInput struct {
	Name        string  `json:"name"`
	Provider    string  `json:"provider,omitempty"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	APIKey      string  `json:"api_key,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	TimeoutSec  int     `json:"timeout_sec,omitempty"`
	Retries     int     `json:"retries,omitempty"`
	IsDefault   bool    `json:"is_default,omitempty"`
}

func (s *ServerStore) List(ctx context.Context) ([]Server, error) {
	q := fmt.Sprintf(`SELECT %s FROM ai_servers ORDER BY id`, serverCols)
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Server{}
		for rows.Next() {
			x, err := scanServer(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, x)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Server{}
	for rows.Next() {
		x, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *ServerStore) Get(ctx context.Context, id int64) (Server, error) {
	if s.pg != nil {
		return scanServer(s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM ai_servers WHERE id = $1", serverCols), id))
	}
	return scanServer(s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM ai_servers WHERE id = ?", serverCols), id))
}

// Default returns the default server, or env-based config when no server is stored.
func (s *ServerStore) Default(ctx context.Context) (Config, string, error) {
	q := fmt.Sprintf(`SELECT %s FROM ai_servers WHERE is_default <> 0 ORDER BY id LIMIT 1`, serverCols)
	qs := fmt.Sprintf(`SELECT %s FROM ai_servers WHERE is_default != 0 ORDER BY id LIMIT 1`, serverCols)
	var srv Server
	var err error
	if s.pg != nil {
		srv, err = scanServer(s.pg.QueryRow(ctx, q))
	} else {
		srv, err = scanServer(s.db.QueryRowContext(ctx, qs))
	}
	if err == nil {
		return srv.ToConfig(), srv.Name, nil
	}
	if err == sql.ErrNoRows {
		cfg := LoadConfig()
		if cfg.Enabled() {
			return cfg, "env", nil
		}
		return Config{}, "", sql.ErrNoRows
	}
	return Config{}, "", err
}

func (s *ServerStore) Create(ctx context.Context, in ServerInput) (Server, error) {
	if in.Provider == "" {
		in.Provider = "custom"
	}
	if in.Temperature == 0 {
		in.Temperature = 0.3
	}
	if in.MaxTokens == 0 {
		in.MaxTokens = 2000
	}
	if in.TimeoutSec == 0 {
		in.TimeoutSec = 60
	}
	if in.Retries == 0 {
		in.Retries = 1
	}
	if s.pg != nil {
		if in.IsDefault {
			_, _ = s.pg.Exec(ctx, `UPDATE ai_servers SET is_default = FALSE`)
		}
		row := s.pg.QueryRow(ctx, `INSERT INTO ai_servers (name, provider, base_url, model, api_key, temperature, max_tokens, timeout_sec, retries, is_default)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+serverCols,
			in.Name, in.Provider, in.BaseURL, in.Model, in.APIKey, in.Temperature, in.MaxTokens, in.TimeoutSec, in.Retries, in.IsDefault)
		return scanServer(row)
	}
	if in.IsDefault {
		_, _ = s.db.ExecContext(ctx, `UPDATE ai_servers SET is_default = 0`)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO ai_servers (name, provider, base_url, model, api_key, temperature, max_tokens, timeout_sec, retries, is_default)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		in.Name, in.Provider, in.BaseURL, in.Model, in.APIKey, in.Temperature, in.MaxTokens, in.TimeoutSec, in.Retries, in.IsDefault)
	if err != nil {
		return Server{}, err
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, id)
}

func (s *ServerStore) Update(ctx context.Context, id int64, in ServerInput) (Server, error) {
	if s.pg != nil {
		if in.IsDefault {
			_, _ = s.pg.Exec(ctx, `UPDATE ai_servers SET is_default = FALSE WHERE id <> $1`, id)
		}
		row := s.pg.QueryRow(ctx, `UPDATE ai_servers SET name=$1, provider=$2, base_url=$3, model=$4, api_key=$5, temperature=$6, max_tokens=$7, timeout_sec=$8, retries=$9, is_default=$10, updated_at=NOW()
			WHERE id=$11 RETURNING `+serverCols,
			in.Name, in.Provider, in.BaseURL, in.Model, in.APIKey, in.Temperature, in.MaxTokens, in.TimeoutSec, in.Retries, in.IsDefault, id)
		return scanServer(row)
	}
	if in.IsDefault {
		_, _ = s.db.ExecContext(ctx, `UPDATE ai_servers SET is_default = 0 WHERE id <> ?`, id)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE ai_servers SET name=?, provider=?, base_url=?, model=?, api_key=?, temperature=?, max_tokens=?, timeout_sec=?, retries=?, is_default=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		in.Name, in.Provider, in.BaseURL, in.Model, in.APIKey, in.Temperature, in.MaxTokens, in.TimeoutSec, in.Retries, in.IsDefault, id)
	if err != nil {
		return Server{}, err
	}
	return s.Get(ctx, id)
}

func (s *ServerStore) Delete(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM ai_servers WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM ai_servers WHERE id = ?`, id)
	return err
}

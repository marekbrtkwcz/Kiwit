package customfield

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FieldDef struct {
	ID         int64     `json:"id"`
	EntityType string    `json:"entity_type"`
	Name       string    `json:"name"`
	FieldType  string    `json:"field_type" enum:"text,number,date,list,multiselect,boolean"`
	Options    string    `json:"options"` // JSON array
	Required   bool      `json:"required"`
	CreatedAt  time.Time `json:"created_at"`
}

type FieldValue struct {
	ID         int64     `json:"id"`
	FieldID    int64     `json:"field_id"`
	EntityType string    `json:"entity_type"`
	EntityID   int64     `json:"entity_id"`
	Value      string    `json:"value"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// FieldWithValue is a definition joined with the value for one entity.
type FieldWithValue struct {
	FieldDef
	Value string `json:"value,omitempty"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

const defCols = `id, entity_type, name, field_type, options, required, created_at`

func scanDef(row interface{ Scan(...any) error }) (FieldDef, error) {
	var d FieldDef
	var req any
	err := row.Scan(&d.ID, &d.EntityType, &d.Name, &d.FieldType, &d.Options, &req, &d.CreatedAt)
	if err == nil {
		switch v := req.(type) {
		case bool:
			d.Required = v
		case int64:
			d.Required = v != 0
		}
	}
	return d, err
}

var validTypes = map[string]bool{"text": true, "number": true, "date": true, "list": true, "multiselect": true, "boolean": true}

func validOptions(fieldType, options string) bool {
	if options == "" {
		return true
	}
	var v []string
	if json.Unmarshal([]byte(options), &v) != nil {
		return false
	}
	if (fieldType == "list" || fieldType == "multiselect") && len(v) == 0 {
		return false
	}
	return true
}

// ValidateValue checks a value against the field type.
func ValidateValue(fieldType, value string) error {
	switch fieldType {
	case "number":
		var f float64
		if err := json.Unmarshal([]byte(value), &f); err != nil {
			if value == "" {
				return nil
			}
			return fmt.Errorf("value must be a number")
		}
	case "boolean":
		if value != "" && value != "true" && value != "false" {
			return fmt.Errorf("value must be true or false")
		}
	case "date":
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return fmt.Errorf("value must be a date YYYY-MM-DD")
			}
		}
	}
	return nil
}

func (s *Store) ListDefs(ctx context.Context, entityType string) ([]FieldDef, error) {
	q := fmt.Sprintf(`SELECT %s FROM custom_field_defs WHERE ($1 = '' OR entity_type = $1) ORDER BY id`, defCols)
	qs := fmt.Sprintf(`SELECT %s FROM custom_field_defs WHERE (? = '' OR entity_type = ?) ORDER BY id`, defCols)
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, q, entityType)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []FieldDef{}
		for rows.Next() {
			d, err := scanDef(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, d)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, qs, entityType, entityType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FieldDef{}
	for rows.Next() {
		d, err := scanDef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDef(ctx context.Context, id int64) (FieldDef, error) {
	if s.pg != nil {
		return scanDef(s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM custom_field_defs WHERE id = $1", defCols), id))
	}
	return scanDef(s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM custom_field_defs WHERE id = ?", defCols), id))
}

type DefInput struct {
	EntityType string `json:"entity_type"`
	Name       string `json:"name"`
	FieldType  string `json:"field_type,omitempty"`
	Options    string `json:"options,omitempty"`
	Required   bool   `json:"required,omitempty"`
}

func (s *Store) CreateDef(ctx context.Context, in DefInput) (FieldDef, error) {
	if !validTypes[in.FieldType] && in.FieldType != "" {
		return FieldDef{}, fmt.Errorf("invalid field_type")
	}
	if in.FieldType == "" {
		in.FieldType = "text"
	}
	if in.Options == "" {
		in.Options = "[]"
	}
	if !validOptions(in.FieldType, in.Options) {
		return FieldDef{}, fmt.Errorf("options must be a non-empty JSON array for list/multiselect")
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO custom_field_defs (entity_type, name, field_type, options, required) VALUES ($1,$2,$3,$4,$5) RETURNING `+defCols,
			in.EntityType, in.Name, in.FieldType, in.Options, in.Required)
		return scanDef(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO custom_field_defs (entity_type, name, field_type, options, required) VALUES (?,?,?,?,?)`,
		in.EntityType, in.Name, in.FieldType, in.Options, in.Required)
	if err != nil {
		return FieldDef{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetDef(ctx, id)
}

func (s *Store) UpdateDef(ctx context.Context, id int64, in DefInput) (FieldDef, error) {
	if !validTypes[in.FieldType] {
		return FieldDef{}, fmt.Errorf("invalid field_type")
	}
	if !validOptions(in.FieldType, in.Options) {
		return FieldDef{}, fmt.Errorf("options must be a non-empty JSON array for list/multiselect")
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `UPDATE custom_field_defs SET entity_type=$1, name=$2, field_type=$3, options=$4, required=$5 WHERE id=$6 RETURNING `+defCols,
			in.EntityType, in.Name, in.FieldType, in.Options, in.Required, id)
		return scanDef(row)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE custom_field_defs SET entity_type=?, name=?, field_type=?, options=?, required=? WHERE id=?`,
		in.EntityType, in.Name, in.FieldType, in.Options, in.Required, id)
	if err != nil {
		return FieldDef{}, err
	}
	return s.GetDef(ctx, id)
}

func (s *Store) DeleteDef(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM custom_field_defs WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM custom_field_defs WHERE id = ?`, id)
	return err
}

// SetValue upserts a value for (field, entity). Validates against field type.
func (s *Store) SetValue(ctx context.Context, fieldID int64, entityType string, entityID int64, value string) (FieldValue, error) {
	def, err := s.GetDef(ctx, fieldID)
	if err != nil {
		return FieldValue{}, err
	}
	if entityType != def.EntityType {
		return FieldValue{}, fmt.Errorf("entity_type %q does not match field entity_type %q", entityType, def.EntityType)
	}
	if err := ValidateValue(def.FieldType, value); err != nil {
		return FieldValue{}, err
	}
	if def.FieldType == "list" && value != "" {
		var opts []string
		_ = json.Unmarshal([]byte(def.Options), &opts)
		ok := false
		for _, o := range opts {
			if o == value {
				ok = true
				break
			}
		}
		if !ok {
			return FieldValue{}, fmt.Errorf("value must be one of the field options")
		}
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO custom_field_values (field_id, entity_type, entity_id, value) VALUES ($1,$2,$3,$4)
			ON CONFLICT (field_id, entity_type, entity_id) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
			RETURNING id, field_id, entity_type, entity_id, value, updated_at`, fieldID, entityType, entityID, value)
		return scanValue(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO custom_field_values (field_id, entity_type, entity_id, value) VALUES (?,?,?,?)
		ON CONFLICT (field_id, entity_type, entity_id) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, fieldID, entityType, entityID, value)
	if err != nil {
		return FieldValue{}, err
	}
	id, _ := res.LastInsertId()
	return s.getValue(ctx, id)
}

func (s *Store) getValue(ctx context.Context, id int64) (FieldValue, error) {
	if s.pg != nil {
		return scanValue(s.pg.QueryRow(ctx, `SELECT id, field_id, entity_type, entity_id, value, updated_at FROM custom_field_values WHERE id = $1`, id))
	}
	return scanValue(s.db.QueryRowContext(ctx, `SELECT id, field_id, entity_type, entity_id, value, updated_at FROM custom_field_values WHERE id = ?`, id))
}

func scanValue(row interface{ Scan(...any) error }) (FieldValue, error) {
	var v FieldValue
	err := row.Scan(&v.ID, &v.FieldID, &v.EntityType, &v.EntityID, &v.Value, &v.UpdatedAt)
	return v, err
}

// ValuesForEntity returns all field defs for entityType with the entity's values filled in.
func (s *Store) ValuesForEntity(ctx context.Context, entityType string, entityID int64) ([]FieldWithValue, error) {
	q := fmt.Sprintf(`SELECT d.%s, COALESCE(v.value, '')
		FROM custom_field_defs d
		LEFT JOIN custom_field_values v ON v.field_id = d.id AND v.entity_type = d.entity_type AND v.entity_id = ?
		WHERE d.entity_type = ? ORDER BY d.id`, strings.ReplaceAll(defCols, ", ", ", d."))
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		q = strings.ReplaceAll(q, "WHERE d.entity_type = $1", "WHERE d.entity_type = $2")
		rows, err := s.pg.Query(ctx, q, entityID, entityType)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []FieldWithValue{}
		for rows.Next() {
			var fw FieldWithValue
			var req any
			err := rows.Scan(&fw.ID, &fw.EntityType, &fw.Name, &fw.FieldType, &fw.Options, &req, &fw.CreatedAt, &fw.Value)
			if err != nil {
				return nil, err
			}
			switch v := req.(type) {
			case bool:
				fw.Required = v
			case int64:
				fw.Required = v != 0
			}
			out = append(out, fw)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, entityID, entityType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FieldWithValue{}
	for rows.Next() {
		var fw FieldWithValue
		var req any
		err := rows.Scan(&fw.ID, &fw.EntityType, &fw.Name, &fw.FieldType, &fw.Options, &req, &fw.CreatedAt, &fw.Value)
		if err != nil {
			return nil, err
		}
		switch v := req.(type) {
		case bool:
			fw.Required = v
		case int64:
			fw.Required = v != 0
		}
		out = append(out, fw)
	}
	return out, rows.Err()
}

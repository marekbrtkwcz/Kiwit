package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AccessGuard enforces product_members isolation consistently across SQLite and PostgreSQL.
type AccessGuard struct {
	db     *sql.DB
	pgPool *pgxpool.Pool
}

// NewAccessGuard creates a product access guard backed by either SQLite (*sql.DB) or PostgreSQL (*pgxpool.Pool).
func NewAccessGuard(db *sql.DB, pgPool *pgxpool.Pool) *AccessGuard {
	return &AccessGuard{db: db, pgPool: pgPool}
}

// FilterUserID returns the user ID to scope list/telemetry queries by product_members,
// or nil when the caller is an admin (or no user context is set).
func FilterUserID(ctx context.Context) *int64 {
	u, ok := UserFromContext(ctx)
	if !ok || u.Role == "admin" {
		return nil
	}
	id := u.ID
	return &id
}

// CheckProductAccess returns true if userID is allowed to read (or write when requireWrite is true) productID.
// A product with no rows in product_members is accessible to all authenticated users.
func (g *AccessGuard) CheckProductAccess(ctx context.Context, productID, userID int64, requireWrite bool) (bool, error) {
	if g == nil {
		return true, nil
	}
	// Anonymous users: read-only access to products that have no members (public products)
	if IsAnonymous(ctx) {
		var count int
		var err error
		if g.pgPool != nil {
			err = g.pgPool.QueryRow(ctx, `SELECT COUNT(*) FROM product_members WHERE product_id = $1`, productID).Scan(&count)
		} else {
			err = g.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM product_members WHERE product_id = ?`, productID).Scan(&count)
		}
		if err != nil {
			return false, err
		}
		// Public product (no members) → read allowed, write denied
		if count == 0 {
			return !requireWrite, nil
		}
		return false, nil
	}
	var count int
	if g.pgPool != nil {
		if err := g.pgPool.QueryRow(ctx, `SELECT COUNT(*) FROM product_members WHERE product_id = $1`, productID).Scan(&count); err != nil {
			return false, err
		}
		if count == 0 {
			return true, nil
		}
		var role string
		err := g.pgPool.QueryRow(ctx,
			`SELECT role FROM product_members WHERE product_id = $1 AND user_id = $2`,
			productID, userID,
		).Scan(&role)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return false, err
		}
		if err == nil && !(requireWrite && role == "viewer") {
			return true, nil
		}
		// Fall back to group-based permissions (best role across the user's groups).
		var groupRole string
		gerr := g.pgPool.QueryRow(ctx, `
			SELECT gpp.role FROM group_product_permissions gpp
			JOIN user_group_members ugm ON ugm.group_id = gpp.group_id
			WHERE gpp.product_id = $1 AND ugm.user_id = $2
			ORDER BY CASE gpp.role WHEN 'maintainer' THEN 3 WHEN 'tester' THEN 2 ELSE 1 END DESC
			LIMIT 1`, productID, userID).Scan(&groupRole)
		if gerr == nil {
			if requireWrite && groupRole == "viewer" {
				return false, nil
			}
			return true, nil
		}
		if errors.Is(gerr, pgx.ErrNoRows) {
			return false, nil
		}
		return false, gerr
	}

	if err := g.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM product_members WHERE product_id = ?`, productID).Scan(&count); err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil
	}
	var role string
	err := g.db.QueryRowContext(ctx,
		`SELECT role FROM product_members WHERE product_id = ? AND user_id = ?`,
		productID, userID,
	).Scan(&role)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if err == nil && !(requireWrite && role == "viewer") {
		return true, nil
	}
	var groupRole string
	gerr := g.db.QueryRowContext(ctx, `
		SELECT gpp.role FROM group_product_permissions gpp
		JOIN user_group_members ugm ON ugm.group_id = gpp.group_id
		WHERE gpp.product_id = ? AND ugm.user_id = ?
		ORDER BY CASE gpp.role WHEN 'maintainer' THEN 3 WHEN 'tester' THEN 2 ELSE 1 END DESC
		LIMIT 1`, productID, userID).Scan(&groupRole)
	if gerr == nil {
		if requireWrite && groupRole == "viewer" {
			return false, nil
		}
		return true, nil
	}
	if errors.Is(gerr, sql.ErrNoRows) {
		return false, nil
	}
	return false, gerr
}

// EnforceProduct verifies that the authenticated user in ctx can access productID.
func (g *AccessGuard) EnforceProduct(ctx context.Context, productID int64, requireWrite bool) error {
	if g == nil {
		return nil
	}
	u, ok := UserFromContext(ctx)
	if !ok {
		return huma.Error401Unauthorized("unauthorized")
	}
	if u.Role == "admin" {
		return nil
	}
	allowed, err := g.CheckProductAccess(ctx, productID, u.ID, requireWrite)
	if err != nil {
		return huma.Error500InternalServerError("failed to verify product access", err)
	}
	if !allowed {
		return huma.Error403Forbidden("no access to this product")
	}
	return nil
}

func (g *AccessGuard) lookupProductID(ctx context.Context, pgQuery, sqliteQuery string, id int64) (int64, error) {
	var productID int64
	if g.pgPool != nil {
		err := g.pgPool.QueryRow(ctx, pgQuery, id).Scan(&productID)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, sql.ErrNoRows
		}
		return productID, err
	}
	err := g.db.QueryRowContext(ctx, sqliteQuery, id).Scan(&productID)
	return productID, err
}

func (g *AccessGuard) enforceResolved(ctx context.Context, productID int64, err error, notFoundMsg string, requireWrite bool) error {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return huma.Error404NotFound(notFoundMsg)
		}
		return huma.Error500InternalServerError("failed to resolve resource product", err)
	}
	return g.EnforceProduct(ctx, productID, requireWrite)
}

func (g *AccessGuard) EnforceVersion(ctx context.Context, versionID int64, requireWrite bool) error {
	if g == nil {
		return nil
	}
	pid, err := g.lookupProductID(ctx,
		`SELECT product_id FROM versions WHERE id = $1`,
		`SELECT product_id FROM versions WHERE id = ?`,
		versionID,
	)
	return g.enforceResolved(ctx, pid, err, "version not found", requireWrite)
}

func (g *AccessGuard) EnforceTestCase(ctx context.Context, testCaseID int64, requireWrite bool) error {
	if g == nil {
		return nil
	}
	pid, err := g.lookupProductID(ctx,
		`SELECT product_id FROM test_cases WHERE id = $1`,
		`SELECT product_id FROM test_cases WHERE id = ?`,
		testCaseID,
	)
	return g.enforceResolved(ctx, pid, err, "test case not found", requireWrite)
}

func (g *AccessGuard) EnforceTestPlan(ctx context.Context, testPlanID int64, requireWrite bool) error {
	if g == nil {
		return nil
	}
	pid, err := g.lookupProductID(ctx,
		`SELECT product_id FROM test_plans WHERE id = $1`,
		`SELECT product_id FROM test_plans WHERE id = ?`,
		testPlanID,
	)
	return g.enforceResolved(ctx, pid, err, "test plan not found", requireWrite)
}

func (g *AccessGuard) EnforceTestRun(ctx context.Context, testRunID int64, requireWrite bool) error {
	if g == nil {
		return nil
	}
	pid, err := g.lookupProductID(ctx,
		`SELECT tp.product_id FROM test_runs tr JOIN test_plans tp ON tp.id = tr.test_plan_id WHERE tr.id = $1`,
		`SELECT tp.product_id FROM test_runs tr JOIN test_plans tp ON tp.id = tr.test_plan_id WHERE tr.id = ?`,
		testRunID,
	)
	return g.enforceResolved(ctx, pid, err, "test run not found", requireWrite)
}

func (g *AccessGuard) EnforceExecution(ctx context.Context, executionID int64, requireWrite bool) error {
	if g == nil {
		return nil
	}
	pid, err := g.lookupProductID(ctx,
		`SELECT tp.product_id FROM test_executions te JOIN test_runs tr ON tr.id = te.test_run_id JOIN test_plans tp ON tp.id = tr.test_plan_id WHERE te.id = $1`,
		`SELECT tp.product_id FROM test_executions te JOIN test_runs tr ON tr.id = te.test_run_id JOIN test_plans tp ON tp.id = tr.test_plan_id WHERE te.id = ?`,
		executionID,
	)
	return g.enforceResolved(ctx, pid, err, "execution not found", requireWrite)
}

// EnforceEntity verifies access to a polymorphic entity ("test_case", "test_plan", "test_run", "test_execution").
func (g *AccessGuard) EnforceEntity(ctx context.Context, entityType string, entityID int64, requireWrite bool) error {
	if g == nil {
		return nil
	}
	switch entityType {
	case "test_case":
		return g.EnforceTestCase(ctx, entityID, requireWrite)
	case "test_plan":
		return g.EnforceTestPlan(ctx, entityID, requireWrite)
	case "test_run":
		return g.EnforceTestRun(ctx, entityID, requireWrite)
	case "test_execution":
		return g.EnforceExecution(ctx, entityID, requireWrite)
	default:
		return huma.Error400BadRequest("unsupported entity_type")
	}
}

// WriteAccessError writes a huma status error as a standard HTTP JSON response for raw chi handlers.
// Returns true if an error was written (caller should return immediately).
func WriteAccessError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	status := http.StatusInternalServerError
	if se, ok := err.(huma.StatusError); ok {
		status = se.GetStatus()
	}
	w.Header().Set("Content-Type", "application/json")
	http.Error(w, `{"detail":"`+err.Error()+`"}`, status)
	return true
}

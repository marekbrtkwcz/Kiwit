package management_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
)

func TestSQLiteProductRepository(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := management.NewSQLiteRepository(sqlDB)

	// Initially empty
	items, err := repo.ListProducts(ctx)
	require.NoError(t, err)
	require.Empty(t, items)

	// Create product
	created, err := repo.CreateProduct(ctx, "Kiwit Core", "Main TCMS product")
	require.NoError(t, err)
	require.Equal(t, int64(1), created.ID)
	require.Equal(t, "Kiwit Core", created.Name)

	// List returns created product
	items, err = repo.ListProducts(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Main TCMS product", items[0].Description)

	// Create and list versions
	ver, err := repo.CreateVersion(ctx, created.ID, "1.0.0")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", ver.Value)

	versions, err := repo.ListVersions(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, versions, 1)

	// Create and list builds
	bld, err := repo.CreateBuild(ctx, ver.ID, "build-100")
	require.NoError(t, err)
	require.Equal(t, "build-100", bld.Name)

	builds, err := repo.ListBuilds(ctx, ver.ID)
	require.NoError(t, err)
	require.Len(t, builds, 1)

	// Create and list components
	comp, err := repo.CreateComponent(ctx, created.ID, "auth", "Authentication module")
	require.NoError(t, err)
	require.Equal(t, "auth", comp.Name)

	components, err := repo.ListComponents(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, components, 1)

	// Create and list tags
	tag, err := repo.CreateTag(ctx, "smoke")
	require.NoError(t, err)
	require.Equal(t, "smoke", tag.Name)

	tags, err := repo.ListTags(ctx)
	require.NoError(t, err)
	require.Len(t, tags, 1)
}

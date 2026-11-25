package management_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/testcase"
)

func TestM2MTagsAndComponents(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mgmtRepo := management.NewSQLiteRepository(sqlDB)
	prod, err := mgmtRepo.CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)
	comp, err := mgmtRepo.CreateComponent(ctx, prod.ID, "API", "")
	require.NoError(t, err)
	tag, err := mgmtRepo.CreateTag(ctx, "critical-path")
	require.NoError(t, err)

	tc, err := testcase.NewSQLiteRepository(sqlDB).Create(ctx, testcase.UpsertParams{
		ProductID: prod.ID, Summary: "M2M test", Format: "steps", Priority: "high", Status: "confirmed",
	})
	require.NoError(t, err)

	rel := management.NewRelationStore(sqlDB, nil)
	require.NoError(t, rel.AttachTag(ctx, "test_case_tags", "test_case_id", tc.ID, tag.ID))
	require.NoError(t, rel.AttachComponentToCase(ctx, tc.ID, comp.ID))

	tags, err := rel.ListTagsForEntity(ctx, "test_case_tags", "test_case_id", tc.ID)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	require.Equal(t, "critical-path", tags[0].Name)

	comps, err := rel.ListComponentsForCase(ctx, tc.ID)
	require.NoError(t, err)
	require.Len(t, comps, 1)
	require.Equal(t, "API", comps[0].Name)
}

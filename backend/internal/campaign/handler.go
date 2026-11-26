package campaign

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

type campaignsOutput struct {
	Body struct {
		Items []Campaign `json:"items"`
	}
}

type campaignOutput struct {
	Body Campaign
}

type treeOutput struct {
	Body CampaignTree
}

type iterationsOutput struct {
	Body struct {
		Items []Iteration `json:"items"`
	}
}

type iterationOutput struct {
	Body Iteration
}

// RegisterRoutes mounts Campaign → Iteration → Test Plan hierarchy APIs.
func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-campaigns",
		Method:      http.MethodGet,
		Path:        "/api/v1/campaigns",
		Summary:     "List campaigns, optionally per product",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ProductID int64 `query:"product_id"`
	}) (*campaignsOutput, error) {
		if input.ProductID != 0 {
			if err := guard.EnforceProduct(ctx, input.ProductID, false); err != nil {
				return nil, err
			}
		}
		items, err := store.ListCampaigns(ctx, input.ProductID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list campaigns", err)
		}
		out := &campaignsOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-campaign",
		Method:      http.MethodPost,
		Path:        "/api/v1/campaigns",
		Summary:     "Create a campaign",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		Body CampaignInput
	}) (*campaignOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceProduct(ctx, input.Body.ProductID, true); err != nil {
			return nil, err
		}
		c, err := store.CreateCampaign(ctx, input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to create campaign", err)
		}
		return &campaignOutput{Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-campaign",
		Method:      http.MethodPut,
		Path:        "/api/v1/campaigns/{id}",
		Summary:     "Update a campaign",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body CampaignInput
	}) (*campaignOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		pid, err := store.CampaignProductID(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("campaign not found")
		}
		if err := guard.EnforceProduct(ctx, pid, true); err != nil {
			return nil, err
		}
		c, err := store.UpdateCampaign(ctx, input.ID, input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to update campaign", err)
		}
		return &campaignOutput{Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-campaign",
		Method:      http.MethodDelete,
		Path:        "/api/v1/campaigns/{id}",
		Summary:     "Delete a campaign (plans keep NULL campaign)",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		pid, err := store.CampaignProductID(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("campaign not found")
		}
		if err := guard.EnforceProduct(ctx, pid, true); err != nil {
			return nil, err
		}
		if err := store.DeleteCampaign(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete campaign", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "campaign-tree",
		Method:      http.MethodGet,
		Path:        "/api/v1/campaigns/{id}/tree",
		Summary:     "Campaign with iterations and assigned test plans (suites)",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*treeOutput, error) {
		pid, err := store.CampaignProductID(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("campaign not found")
		}
		if err := guard.EnforceProduct(ctx, pid, false); err != nil {
			return nil, err
		}
		tree, err := store.Tree(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("campaign not found")
			}
			return nil, huma.Error500InternalServerError("failed to build tree", err)
		}
		return &treeOutput{Body: tree}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-iterations",
		Method:      http.MethodGet,
		Path:        "/api/v1/campaigns/{id}/iterations",
		Summary:     "List iterations of a campaign",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*iterationsOutput, error) {
		pid, err := store.CampaignProductID(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("campaign not found")
		}
		if err := guard.EnforceProduct(ctx, pid, false); err != nil {
			return nil, err
		}
		items, err := store.ListIterations(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list iterations", err)
		}
		out := &iterationsOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-iteration",
		Method:      http.MethodPost,
		Path:        "/api/v1/campaigns/{id}/iterations",
		Summary:     "Create an iteration in a campaign",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body IterationInput
	}) (*iterationOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		pid, err := store.CampaignProductID(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("campaign not found")
		}
		if err := guard.EnforceProduct(ctx, pid, true); err != nil {
			return nil, err
		}
		it, err := store.CreateIteration(ctx, input.ID, input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to create iteration", err)
		}
		return &iterationOutput{Body: it}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-iteration",
		Method:      http.MethodPut,
		Path:        "/api/v1/iterations/{id}",
		Summary:     "Update an iteration",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body IterationInput
	}) (*iterationOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		it0, err := store.GetIteration(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("iteration not found")
		}
		pid, err := store.CampaignProductID(ctx, it0.CampaignID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to resolve campaign", err)
		}
		if err := guard.EnforceProduct(ctx, pid, true); err != nil {
			return nil, err
		}
		it, err := store.UpdateIteration(ctx, input.ID, input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to update iteration", err)
		}
		return &iterationOutput{Body: it}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-iteration",
		Method:      http.MethodDelete,
		Path:        "/api/v1/iterations/{id}",
		Summary:     "Delete an iteration (plans keep NULL iteration)",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		it0, err := store.GetIteration(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("iteration not found")
		}
		pid, err := store.CampaignProductID(ctx, it0.CampaignID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to resolve campaign", err)
		}
		if err := guard.EnforceProduct(ctx, pid, true); err != nil {
			return nil, err
		}
		if err := store.DeleteIteration(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete iteration", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "assign-plan-to-campaign",
		Method:      http.MethodPost,
		Path:        "/api/v1/campaigns/{id}/assign-plan",
		Summary:     "Assign a test plan to a campaign and optionally an iteration",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			TestPlanID  int64  `json:"test_plan_id"`
			IterationID *int64 `json:"iteration_id,omitempty"`
		}
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		pid, err := store.CampaignProductID(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("campaign not found")
		}
		if err := guard.EnforceProduct(ctx, pid, true); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestPlan(ctx, input.Body.TestPlanID, true); err != nil {
			return nil, err
		}
		cid := input.ID
		if err := store.AssignPlan(ctx, input.Body.TestPlanID, &cid, input.Body.IterationID); err != nil {
			return nil, huma.Error500InternalServerError("failed to assign plan", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unassign-plan-from-campaign",
		Method:      http.MethodPost,
		Path:        "/api/v1/testplans/{id}/unassign-campaign",
		Summary:     "Remove campaign/iteration assignment from a test plan",
		Tags:        []string{"Campaigns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestPlan(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := store.AssignPlan(ctx, input.ID, nil, nil); err != nil {
			return nil, huma.Error500InternalServerError("failed to unassign plan", err)
		}
		return nil, nil
	})
}

package management

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

// Product represents a tested software product.
type Product struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateProductInput struct {
	Body struct {
		Name        string `json:"name" minLength:"1" maxLength:"120" doc:"Unique product name"`
		Description string `json:"description,omitempty" doc:"Optional product description"`
	}
}

type ProductOutput struct {
	Body Product
}

type ListProductsOutput struct {
	Body struct {
		Items []Product `json:"items"`
	}
}

type Version struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"product_id"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateVersionInput struct {
	ProductID int64 `path:"product_id"`
	Body      struct {
		Value string `json:"value" minLength:"1" maxLength:"64"`
	}
}

type VersionOutput struct {
	Body Version
}

type ListVersionsOutput struct {
	Body struct {
		Items []Version `json:"items"`
	}
}

type Build struct {
	ID        int64     `json:"id"`
	VersionID int64     `json:"version_id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateBuildInput struct {
	VersionID int64 `path:"version_id"`
	Body      struct {
		Name string `json:"name" minLength:"1" maxLength:"120"`
	}
}

type BuildOutput struct {
	Body Build
}

type ListBuildsOutput struct {
	Body struct {
		Items []Build `json:"items"`
	}
}

type Component struct {
	ID          int64     `json:"id"`
	ProductID   int64     `json:"product_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateComponentInput struct {
	ProductID int64 `path:"product_id"`
	Body      struct {
		Name        string `json:"name" minLength:"1" maxLength:"120"`
		Description string `json:"description,omitempty"`
	}
}

type ComponentOutput struct {
	Body Component
}

type ListComponentsOutput struct {
	Body struct {
		Items []Component `json:"items"`
	}
}

type Tag struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTagInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"64"`
	}
}

type TagOutput struct {
	Body Tag
}

type ListTagsOutput struct {
	Body struct {
		Items []Tag `json:"items"`
	}
}

type Category struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateCategoryInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"64"`
	}
}

type CategoryOutput struct {
	Body Category
}

type ListCategoriesOutput struct {
	Body struct {
		Items []Category `json:"items"`
	}
}

type Priority struct {
	ID        int64     `json:"id"`
	Value     string    `json:"value"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type ExecutionStatus struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Weight    int       `json:"weight"`
	CreatedAt time.Time `json:"created_at"`
}

func enforceProductAccess(ctx context.Context, repo ProductRepository, productID int64, requireWrite bool) error {
	u, ok := auth.UserFromContext(ctx)
	if !ok {
		return huma.Error401Unauthorized("unauthorized")
	}
	if u.Role == "admin" {
		return nil
	}
	allowed, err := repo.CheckProductAccess(ctx, productID, u.ID, requireWrite)
	if err != nil {
		return huma.Error500InternalServerError("failed to verify product access", err)
	}
	if !allowed {
		return huma.Error403Forbidden("no access to this product")
	}
	return nil
}

// RegisterRoutes registers management OpenAPI endpoints.
func RegisterRoutes(api huma.API, repo ProductRepository, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-products",
		Method:      http.MethodGet,
		Path:        "/api/v1/products",
		Summary:     "List products",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct{}) (*ListProductsOutput, error) {
		var items []Product
		var err error
		if u, ok := auth.UserFromContext(ctx); ok && u.Role != "admin" {
			items, err = repo.ListProductsForUser(ctx, u.ID)
		} else {
			items, err = repo.ListProducts(ctx)
		}
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list products", err)
		}
		out := &ListProductsOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-product",
		Method:      http.MethodPost,
		Path:        "/api/v1/products",
		Summary:     "Create product",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *CreateProductInput) (*ProductOutput, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		p, err := repo.CreateProduct(ctx, input.Body.Name, input.Body.Description)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create product", err)
		}
		return &ProductOutput{Body: p}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-versions",
		Method:      http.MethodGet,
		Path:        "/api/v1/products/{product_id}/versions",
		Summary:     "List product versions",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct {
		ProductID int64 `path:"product_id"`
	}) (*ListVersionsOutput, error) {
		if err := enforceProductAccess(ctx, repo, input.ProductID, false); err != nil {
			return nil, err
		}
		items, err := repo.ListVersions(ctx, input.ProductID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list versions", err)
		}
		out := &ListVersionsOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-version",
		Method:      http.MethodPost,
		Path:        "/api/v1/products/{product_id}/versions",
		Summary:     "Create product version",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *CreateVersionInput) (*VersionOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := enforceProductAccess(ctx, repo, input.ProductID, true); err != nil {
			return nil, err
		}
		v, err := repo.CreateVersion(ctx, input.ProductID, input.Body.Value)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create version", err)
		}
		return &VersionOutput{Body: v}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-builds",
		Method:      http.MethodGet,
		Path:        "/api/v1/versions/{version_id}/builds",
		Summary:     "List version builds",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct {
		VersionID int64 `path:"version_id"`
	}) (*ListBuildsOutput, error) {
		if err := guard.EnforceVersion(ctx, input.VersionID, false); err != nil {
			return nil, err
		}
		items, err := repo.ListBuilds(ctx, input.VersionID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list builds", err)
		}
		out := &ListBuildsOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-build",
		Method:      http.MethodPost,
		Path:        "/api/v1/versions/{version_id}/builds",
		Summary:     "Create version build",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *CreateBuildInput) (*BuildOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceVersion(ctx, input.VersionID, true); err != nil {
			return nil, err
		}
		b, err := repo.CreateBuild(ctx, input.VersionID, input.Body.Name)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create build", err)
		}
		return &BuildOutput{Body: b}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-components",
		Method:      http.MethodGet,
		Path:        "/api/v1/products/{product_id}/components",
		Summary:     "List product components",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct {
		ProductID int64 `path:"product_id"`
	}) (*ListComponentsOutput, error) {
		if err := enforceProductAccess(ctx, repo, input.ProductID, false); err != nil {
			return nil, err
		}
		items, err := repo.ListComponents(ctx, input.ProductID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list components", err)
		}
		out := &ListComponentsOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-component",
		Method:      http.MethodPost,
		Path:        "/api/v1/products/{product_id}/components",
		Summary:     "Create product component",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *CreateComponentInput) (*ComponentOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := enforceProductAccess(ctx, repo, input.ProductID, true); err != nil {
			return nil, err
		}
		c, err := repo.CreateComponent(ctx, input.ProductID, input.Body.Name, input.Body.Description)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create component", err)
		}
		return &ComponentOutput{Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-tags",
		Method:      http.MethodGet,
		Path:        "/api/v1/tags",
		Summary:     "List tags",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct{}) (*ListTagsOutput, error) {
		items, err := repo.ListTags(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list tags", err)
		}
		out := &ListTagsOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-tag",
		Method:      http.MethodPost,
		Path:        "/api/v1/tags",
		Summary:     "Create tag",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *CreateTagInput) (*TagOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		t, err := repo.CreateTag(ctx, input.Body.Name)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create tag", err)
		}
		return &TagOutput{Body: t}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-categories",
		Method:      http.MethodGet,
		Path:        "/api/v1/categories",
		Summary:     "List test case categories",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct{}) (*ListCategoriesOutput, error) {
		items, err := repo.ListCategories(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list categories", err)
		}
		out := &ListCategoriesOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-category",
		Method:      http.MethodPost,
		Path:        "/api/v1/categories",
		Summary:     "Create test case category",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *CreateCategoryInput) (*CategoryOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		c, err := repo.CreateCategory(ctx, input.Body.Name)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create category", err)
		}
		return &CategoryOutput{Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "add-product-member",
		Method:      http.MethodPost,
		Path:        "/api/v1/products/{product_id}/members",
		Summary:     "Assign user role to a product",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct {
		ProductID int64 `path:"product_id"`
		Body      struct {
			UserID int64  `json:"user_id" minimum:"1"`
			Role   string `json:"role" default:"tester" enum:"viewer,tester,maintainer"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := repo.AddMember(ctx, input.ProductID, input.Body.UserID, input.Body.Role); err != nil {
			return nil, huma.Error400BadRequest("failed to assign member", err)
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
		OperationID: "list-priorities",
		Method:      http.MethodGet,
		Path:        "/api/v1/priorities",
		Summary:     "List test case priorities",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []Priority `json:"items"`
		}
	}, error) {
		items, err := repo.ListPriorities(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list priorities", err)
		}
		out := &struct {
			Body struct {
				Items []Priority `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-priority",
		Method:      http.MethodPost,
		Path:        "/api/v1/priorities",
		Summary:     "Create test case priority",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Value string `json:"value" minLength:"1"`
		}
	}) (*struct{ Body Priority }, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		p, err := repo.CreatePriority(ctx, input.Body.Value)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create priority", err)
		}
		return &struct{ Body Priority }{Body: p}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-execution-statuses",
		Method:      http.MethodGet,
		Path:        "/api/v1/execution-statuses",
		Summary:     "List test execution statuses",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []ExecutionStatus `json:"items"`
		}
	}, error) {
		items, err := repo.ListExecutionStatuses(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list execution statuses", err)
		}
		out := &struct {
			Body struct {
				Items []ExecutionStatus `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-execution-status",
		Method:      http.MethodPost,
		Path:        "/api/v1/execution-statuses",
		Summary:     "Create test execution status",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Name   string `json:"name" minLength:"1"`
			Color  string `json:"color,omitempty" default:"#6b7280"`
			Weight int    `json:"weight,omitempty"`
		}
	}) (*struct{ Body ExecutionStatus }, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		s, err := repo.CreateExecutionStatus(ctx, input.Body.Name, input.Body.Color, input.Body.Weight)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create execution status", err)
		}
		return &struct{ Body ExecutionStatus }{Body: s}, nil
	})
}

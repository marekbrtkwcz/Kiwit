package management

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
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

// RegisterRoutes registers management OpenAPI endpoints.
func RegisterRoutes(api huma.API, repo ProductRepository) {
	huma.Register(api, huma.Operation{
		OperationID: "list-products",
		Method:      http.MethodGet,
		Path:        "/api/v1/products",
		Summary:     "List products",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct{}) (*ListProductsOutput, error) {
		items, err := repo.ListProducts(ctx)
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
}

package plugin

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// Plugin defines an extension module that can register its own OpenAPI routes.
type Plugin interface {
	Name() string
	Version() string
	Register(api huma.API)
}

type Info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Registry struct {
	plugins []Plugin
}

func NewRegistry() *Registry {
	return &Registry{plugins: make([]Plugin, 0)}
}

func (r *Registry) Register(p Plugin) {
	r.plugins = append(r.plugins, p)
}

func (r *Registry) List() []Info {
	out := make([]Info, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, Info{Name: p.Name(), Version: p.Version()})
	}
	return out
}

func (r *Registry) InitAll(api huma.API) {
	for _, p := range r.plugins {
		p.Register(api)
	}
}

func RegisterRoutes(api huma.API, reg *Registry) {
	huma.Register(api, huma.Operation{
		OperationID: "list-plugins",
		Method:      http.MethodGet,
		Path:        "/api/v1/plugins",
		Summary:     "List registered Kiwit plugins",
		Tags:        []string{"Plugins"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []Info `json:"items"`
		}
	}, error) {
		out := &struct {
			Body struct {
				Items []Info `json:"items"`
			}
		}{}
		out.Body.Items = reg.List()
		return out, nil
	})
}

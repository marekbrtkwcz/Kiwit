package plugin_test

import (
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/plugin"
)

type samplePlugin struct{}

func (samplePlugin) Name() string          { return "sample-exporter" }
func (samplePlugin) Version() string       { return "1.0.0" }
func (samplePlugin) Register(api huma.API) {}

func TestPluginRegistry(t *testing.T) {
	reg := plugin.NewRegistry()
	reg.Register(samplePlugin{})

	items := reg.List()
	require.Len(t, items, 1)
	require.Equal(t, "sample-exporter", items[0].Name)
	require.Equal(t, "1.0.0", items[0].Version)
}

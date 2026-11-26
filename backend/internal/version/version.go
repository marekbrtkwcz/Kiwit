package version

import (
	"encoding/json"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5"
)

// VERSION is the application release version. Overridable at build time:
// go build -ldflags "-X github.com/kiwit/kiwit/internal/version.VERSION=x.y.z"
var VERSION = "0.2.0"

// Info describes the running build. Commit and build time are read from
// Go build info (vcs stamping) when available.
type Info struct {
	Version    string `json:"version"`
	Commit     string `json:"commit,omitempty"`
	Dirty      bool   `json:"dirty,omitempty"`
	BuildTime  string `json:"build_time,omitempty"`
	GoVersion  string `json:"go_version,omitempty"`
}

// Get returns the version info of the running binary.
func Get() Info {
	info := Info{Version: VERSION}
	if bi, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = bi.GoVersion
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = s.Value
			case "vcs.time":
				info.BuildTime = s.Value
			case "vcs.modified":
				info.Dirty = s.Value == "true"
			}
		}
	}
	return info
}

// Register mounts GET /api/v1/version and the kiwitcms-compatible GET /version/.
func Register(r chi.Router) {
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Get())
	}
	r.Get("/api/v1/version", handler)
	r.Get("/version/", handler)
}

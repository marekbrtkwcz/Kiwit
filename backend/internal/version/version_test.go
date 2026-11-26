package version

import "testing"

func TestGetReturnsVersion(t *testing.T) {
	info := Get()
	if info.Version == "" {
		t.Fatal("expected non-empty version")
	}
	if info.GoVersion == "" {
		t.Fatal("expected go version from build info")
	}
}

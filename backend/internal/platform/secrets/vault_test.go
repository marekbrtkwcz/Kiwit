package secrets

import (
	"bytes"
	"strings"
	"testing"
)

func TestVaultRedactRegisteredSecrets(t *testing.T) {
	v := &Vault{}
	v.Register("sk-live-abcdef1234")
	v.Register("")   // ignored
	v.Register("ab") // too short, ignored

	out := v.Redact(`Authorization: Bearer sk-live-abcdef1234 and ab="x"`)
	if strings.Contains(out, "sk-live-abcdef1234") {
		t.Fatalf("secret leaked: %s", out)
	}
	if !strings.Contains(out, "***") {
		t.Fatalf("secret not replaced: %s", out)
	}
	if !strings.Contains(out, `ab="x"`) {
		t.Fatalf("short value must not be redacted: %s", out)
	}
}

func TestVaultWriterScrubsOutput(t *testing.T) {
	v := &Vault{}
	v.Register("super-secret-pass")
	var buf bytes.Buffer
	w := v.Writer(&buf)
	payload := `{"msg":"smtp auth","password":"super-secret-pass"}`
	n, err := w.Write([]byte(payload))
	if err != nil || n != len(payload) {
		t.Fatalf("write failed: n=%d err=%v", n, err)
	}
	if strings.Contains(buf.String(), "super-secret-pass") {
		t.Fatalf("secret leaked through writer: %s", buf.String())
	}
}

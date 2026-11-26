package secrets

import (
	"io"
	"strings"
	"sync"
)

// Vault holds secret values that must never appear in log output. Secrets are
// registered where they are read (env config, DB rows) and scrubbed from every
// log sink: the in-memory ring and the stdout/file writers.
type Vault struct {
	mu      sync.Mutex
	secrets map[string]bool
}

var defaultVault = &Vault{secrets: map[string]bool{}}

// DefaultVault is the process-wide secret registry.
func DefaultVault() *Vault { return defaultVault }

// Register adds secret values to scrub. Values shorter than 4 characters are
// ignored to avoid redacting trivial strings everywhere.
func (v *Vault) Register(values ...string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.secrets == nil {
		v.secrets = map[string]bool{}
	}
	for _, s := range values {
		if len(s) >= 4 {
			v.secrets[s] = true
		}
	}
}

// Redact replaces every occurrence of a registered secret with ***.
func (v *Vault) Redact(s string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	for sec := range v.secrets {
		s = strings.ReplaceAll(s, sec, "***")
	}
	return s
}

// Writer wraps w so every written chunk has registered secrets scrubbed.
// The write reports the length of the original data, like io.MultiWriter.
func (v *Vault) Writer(w io.Writer) io.Writer {
	return &redactingWriter{w: w, v: v}
}

type redactingWriter struct {
	w io.Writer
	v *Vault
}

func (rw *redactingWriter) Write(p []byte) (int, error) {
	_, err := rw.w.Write([]byte(rw.v.Redact(string(p))))
	return len(p), err
}

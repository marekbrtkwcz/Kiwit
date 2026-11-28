package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// LDAPConfig holds LDAP connection and search settings.
type LDAPConfig struct {
	Server       string // host:port, or a full URL: ldap://host:port / ldaps://host:port
	BindDN       string // optional service account for search
	BindPassword string
	BaseDN       string
	UserFilter   string // e.g. "(uid=%s)" — %s replaced with username (escaped)
	UsernameAttr string
	EmailAttr    string
	StartTLS     bool // upgrade a plain ldap:// connection with StartTLS
	// InsecureSkipVerify disables TLS certificate verification. Off by
	// default; only enable for internal CAs you explicitly trust.
	InsecureSkipVerify bool
}

// serverHost extracts the host part for TLS ServerName from a Server value
// that may be bare host:port or a full ldap(s):// URL.
func serverHost(server string) string {
	s := strings.TrimPrefix(server, "ldaps://")
	s = strings.TrimPrefix(s, "ldap://")
	if h, _, err := net.SplitHostPort(s); err == nil {
		return h
	}
	return s
}

// LDAPAuthenticate validates credentials against LDAP and returns email + username.
// Strategy: if BindDN set, search for the user DN first, then bind as the user.
// Otherwise bind directly with a DN template from UserFilter.
func LDAPAuthenticate(ctx context.Context, cfg LDAPConfig, username, password string) (email, displayName string, err error) {
	if cfg.Server == "" {
		return "", "", fmt.Errorf("ldap not configured")
	}
	if username == "" || password == "" {
		return "", "", ErrInvalidCredentials
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	tlsConfig := &tls.Config{ServerName: serverHost(cfg.Server), InsecureSkipVerify: cfg.InsecureSkipVerify} //nolint:gosec // opt-in only
	server := cfg.Server
	switch {
	case strings.HasPrefix(server, "ldaps://"):
		// TLS from the start, certificate verified by default.
	case strings.HasPrefix(server, "ldap://"):
		// plain, possibly upgraded via StartTLS below
	default:
		server = "ldap://" + server
	}
	conn, err := ldap.DialURL(server, ldap.DialWithDialer(dialer), ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return "", "", fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()
	conn.SetTimeout(10 * time.Second)

	// Upgrade a plain connection to TLS when configured (verify certs by default).
	if cfg.StartTLS && strings.HasPrefix(server, "ldap://") {
		if err := conn.StartTLS(tlsConfig); err != nil {
			return "", "", fmt.Errorf("ldap starttls: %w", err)
		}
	}

	// Escape username for LDAP filter (RFC 4515).
	esc := ldap.EscapeFilter(username)
	filter := strings.ReplaceAll(cfg.UserFilter, "%s", esc)

	var userDN string
	if cfg.BindDN != "" {
		// Bind with service account and search.
		if err := conn.Bind(cfg.BindDN, cfg.BindPassword); err != nil {
			return "", "", fmt.Errorf("ldap service bind: %w", err)
		}
		search := ldap.NewSearchRequest(
			cfg.BaseDN,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, 10, false,
			filter,
			[]string{cfg.UsernameAttr, cfg.EmailAttr, "cn", "displayName"},
			nil,
		)
		res, err := conn.SearchWithPaging(search, 1)
		if err != nil || len(res.Entries) == 0 {
			return "", "", ErrInvalidCredentials
		}
		userDN = res.Entries[0].DN
		email = res.Entries[0].GetAttributeValue(cfg.EmailAttr)
		displayName = res.Entries[0].GetAttributeValue(cfg.UsernameAttr)
		if displayName == "" {
			displayName = res.Entries[0].GetAttributeValue("cn")
		}
	} else {
		// Direct bind: construct DN from filter is not possible; require BindDN path.
		return "", "", fmt.Errorf("ldap bind DN required for authentication")
	}

	// Verify user password by binding as the user.
	if err := conn.Bind(userDN, password); err != nil {
		return "", "", ErrInvalidCredentials
	}
	if email == "" {
		email = username + "@ldap.local"
	}
	if displayName == "" {
		displayName = username
	}
	return email, displayName, nil
}

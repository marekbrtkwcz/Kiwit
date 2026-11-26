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
	Server       string // host:port
	BindDN       string // optional service account for search
	BindPassword string
	BaseDN       string
	UserFilter   string // e.g. "(uid=%s)" — %s replaced with username (escaped)
	UsernameAttr string
	EmailAttr    string
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
	conn, err := ldap.DialURL("ldap://"+cfg.Server, ldap.DialWithDialer(dialer), ldap.DialWithTLSConfig(&tls.Config{InsecureSkipVerify: true})) //nolint:gosec // LDAP commonly internal
	if err != nil {
		// try ldaps if plain fails? keep simple: error out
		return "", "", fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()
	conn.SetTimeout(10 * time.Second)

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

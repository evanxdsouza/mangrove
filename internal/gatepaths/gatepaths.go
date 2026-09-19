// Package gatepaths defines which request paths on a password-protected
// deployment skip the gate: how an owner-supplied list is validated, and how
// a live request path is matched against it. See
// docs/protected-deployments.md.
package gatepaths

import (
	"fmt"
	"path"
	"strings"
)

const (
	maxPatterns   = 50
	maxPatternLen = 200
)

// Normalize trims, validates and de-duplicates a user-supplied list. Blank
// entries are dropped. A pattern is an absolute path ("/pricing"), or a
// prefix if it ends in "*" ("/docs/*", "/assets*").
func Normalize(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		p := strings.TrimSpace(raw)
		if p == "" || seen[p] {
			continue
		}
		if len(p) > maxPatternLen {
			return nil, fmt.Errorf("public path %q is too long (max %d characters)", p, maxPatternLen)
		}
		if !strings.HasPrefix(p, "/") {
			return nil, fmt.Errorf("public path %q must start with /", p)
		}
		if strings.ContainsAny(p, "?# \t\r\n") {
			return nil, fmt.Errorf("public path %q must not contain spaces, ?, or #", p)
		}
		if i := strings.Index(p, "*"); i != -1 && i != len(p)-1 {
			return nil, fmt.Errorf("public path %q: * is only allowed at the end", p)
		}
		if !canonical(strings.TrimSuffix(p, "*")) {
			return nil, fmt.Errorf("public path %q must not contain //, /./, /../ or backslashes", p)
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) > maxPatterns {
		return nil, fmt.Errorf("too many public paths (max %d)", maxPatterns)
	}
	return out, nil
}

// Match reports whether a request path is public under any pattern. A path
// that isn't already in canonical form (containing "..", "//", "/./") never
// matches: the backend app might resolve it to somewhere the pattern was
// never meant to cover (e.g. "/docs/../admin" against "/docs/*").
func Match(patterns []string, reqPath string) bool {
	if len(patterns) == 0 || !canonical(reqPath) {
		return false
	}
	for _, p := range patterns {
		if prefix, ok := strings.CutSuffix(p, "*"); ok {
			if strings.HasPrefix(reqPath, prefix) {
				return true
			}
		} else if reqPath == p {
			return true
		}
	}
	return false
}

// canonical reports whether p is unchanged by path.Clean, ignoring a
// trailing slash.
func canonical(p string) bool {
	if !strings.HasPrefix(p, "/") || strings.Contains(p, `\`) {
		return false
	}
	c := path.Clean(p)
	if strings.HasSuffix(p, "/") && c != "/" {
		c += "/"
	}
	return c == p
}

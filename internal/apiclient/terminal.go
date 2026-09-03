package apiclient

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/evanxdsouza/mangrove/internal/auth"
)

// OpenTerminal dials GET /api/services/{id}/terminal -- the same websocket
// endpoint the dashboard's xterm.js Terminal tab (web/src/components/
// Terminal.tsx) and internal/api/terminal.go's serviceTerminal handler
// implement. See docs/architecture.md's "Lifecycle actions short of a full
// deploy" for the wire protocol this connection speaks: binary frames are
// raw terminal bytes in both directions, and the one text frame a caller
// ever sends is `{"type":"resize","cols":N,"rows":N}`.
//
// The returned *websocket.Conn is the caller's to drive directly (e.g.
// mangrove-tui's shell view bridges it to the local terminal in raw mode)
// -- this only handles the parts every caller needs: URL construction,
// scheme translation, and the session cookie, which a plain
// websocket.DefaultDialer.Dial wouldn't attach on its own.
func (c *Client) OpenTerminal(ctx context.Context, serviceID int64) (*websocket.Conn, error) {
	wsURL, err := c.wsURL("/api/services/" + strconv.FormatInt(serviceID, 10) + "/terminal")
	if err != nil {
		return nil, err
	}
	header := http.Header{}
	if c.session != "" {
		header.Set("Cookie", (&http.Cookie{Name: auth.SessionCookieName, Value: c.session}).String())
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrNotAuthenticated
		}
		return nil, fmt.Errorf("open terminal: %w", err)
	}
	return conn, nil
}

// wsURL rewrites c.BaseURL's scheme (http->ws, https->wss) and appends
// path.
func (c *Client) wsURL(path string) (string, error) {
	switch {
	case strings.HasPrefix(c.BaseURL, "https://"):
		return "wss://" + strings.TrimPrefix(c.BaseURL, "https://") + path, nil
	case strings.HasPrefix(c.BaseURL, "http://"):
		return "ws://" + strings.TrimPrefix(c.BaseURL, "http://") + path, nil
	default:
		return "", fmt.Errorf("base URL %q must start with http:// or https://", c.BaseURL)
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/evanxdsouza/mangrove/internal/store"
)

// terminalUpgrader upgrades a service terminal request to a websocket. No
// CheckOrigin override: gorilla's default rejects a cross-origin Origin
// header (comparing it against the request's own Host), which is the right
// behavior here -- the dashboard is always same-origin with the API, and a
// websocket upgrade carries the session cookie just like any other request,
// so this is the one thing standing between the terminal and a third-party
// page opening a shell using a logged-in user's cookies.
var terminalUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// terminalResizeMsg is the one client->server control frame the terminal
// protocol recognizes, sent as a websocket text message. Every other
// client->server message is a binary frame of raw keystrokes, written
// straight to the pty; every server->client message is a binary frame of
// raw shell output. Splitting on frame type (rather than, say, wrapping
// everything in JSON) keeps normal terminal I/O free of encoding overhead
// and JSON's UTF-8 requirement, which arbitrary shell output isn't
// guaranteed to satisfy.
type terminalResizeMsg struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// serviceTerminal opens an interactive shell inside a service's running
// container and relays it over a websocket -- the web terminal feature.
// Unlike streamServiceLogs (one-way, SSE), this is a real two-way session:
// keystrokes in, shell output back, for as long as the tab stays open.
func (s *Server) serviceTerminal(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "serviceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service id")
		return
	}
	svc, err := s.Store.GetService(r.Context(), id)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if svc.ContainerIDCurrent == "" {
		writeError(w, http.StatusConflict, "service has no running container")
		return
	}

	// Opened before upgrading so a failure (e.g. the container just died)
	// still comes back as a normal HTTP error instead of an inscrutable
	// websocket close code.
	term, err := s.Orchestrator.OpenServiceTerminal(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	conn, err := terminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		term.Close()
		return
	}
	defer conn.Close()
	defer term.Close()

	// The shell -> browser direction runs on its own goroutine; the
	// goroutine below (browser -> shell, plus resize) blocks on
	// conn.ReadMessage in the caller's goroutine. Either side exiting --
	// the pty hitting EOF because the shell exited, or the websocket
	// closing because the browser tab did -- tears down both: closing
	// term unblocks the pending Read below, and closing conn unblocks the
	// pending ReadMessage in the loop that follows.
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := term.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					term.Close()
					return
				}
			}
			if err != nil {
				conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shell exited"),
					time.Now().Add(time.Second))
				conn.Close()
				return
			}
		}
	}()

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		switch mt {
		case websocket.BinaryMessage:
			if _, err := term.Write(msg); err != nil {
				return
			}
		case websocket.TextMessage:
			var ctrl terminalResizeMsg
			if json.Unmarshal(msg, &ctrl) == nil && ctrl.Type == "resize" && ctrl.Cols > 0 && ctrl.Rows > 0 {
				term.Resize(ctrl.Cols, ctrl.Rows)
			}
		}
	}
}

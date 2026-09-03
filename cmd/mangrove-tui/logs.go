package main

import (
	"bufio"
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
	"github.com/evanxdsouza/mangrove/internal/models"
)

const maxLogLines = 2000 // matches LogViewer.tsx's own cap

type logsState struct {
	service    models.Service
	viewport   viewport.Model
	lines      []string
	ch         <-chan string
	cancel     context.CancelFunc
	connected  bool
	disconnect string // non-empty once the stream has ended, for the status line
}

func newLogsState(svc models.Service, _ bool) logsState {
	vp := viewport.New(0, 0)
	return logsState{service: svc, viewport: vp}
}

// logStreamStartedMsg carries the just-opened stream's line channel (nil on
// failure to open, in which case err explains why) and the cancel func that
// tears it down -- stored on logsState so leaving the view can stop it.
type logStreamStartedMsg struct {
	ch     <-chan string
	cancel context.CancelFunc
	err    error
}

func startLogStreamCmd(client *apiclient.Client, serviceID int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		body, err := client.StreamLogs(ctx, serviceID, 200)
		if err != nil {
			cancel()
			return logStreamStartedMsg{err: err}
		}
		ch := make(chan string, 256)
		go func() {
			defer close(ch)
			defer body.Close()
			scanner := bufio.NewScanner(body)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				if payload, ok := strings.CutPrefix(scanner.Text(), "data: "); ok {
					select {
					case ch <- payload:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
		return logStreamStartedMsg{ch: ch, cancel: cancel}
	}
}

type logLineMsg struct{ line string }
type logStreamClosedMsg struct{}

func waitForLogLineCmd(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logStreamClosedMsg{}
		}
		return logLineMsg{line: line}
	}
}

func (m model) updateLogs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case logStreamStartedMsg:
		if msg.err != nil {
			m.view = viewDetail
			return m, errStatusCmd("Failed to open log stream", msg.err)
		}
		m.logs.ch = msg.ch
		m.logs.cancel = msg.cancel
		m.logs.connected = true
		return m, waitForLogLineCmd(msg.ch)

	case logLineMsg:
		m.logs.lines = append(m.logs.lines, msg.line)
		if len(m.logs.lines) > maxLogLines {
			m.logs.lines = m.logs.lines[len(m.logs.lines)-maxLogLines:]
		}
		m.logs.viewport.SetContent(strings.Join(m.logs.lines, "\n"))
		m.logs.viewport.GotoBottom()
		return m, waitForLogLineCmd(m.logs.ch)

	case logStreamClosedMsg:
		m.logs.connected = false
		m.logs.disconnect = "Log stream ended."
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "left", "q":
			if m.logs.cancel != nil {
				m.logs.cancel()
			}
			m.view = viewDetail
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.logs.viewport, cmd = m.logs.viewport.Update(msg)
	return m, cmd
}

func (m model) viewLogs() string {
	status := "Live — " + m.logs.service.Name
	if !m.logs.connected {
		status = m.logs.disconnect
		if status == "" {
			status = "Connecting..."
		}
	}
	return styleDim.Render(status) + "\n" + m.logs.viewport.View() + "\n" +
		styleHelp.Render("↑/↓/pgup/pgdn: scroll  ·  esc/q: back")
}

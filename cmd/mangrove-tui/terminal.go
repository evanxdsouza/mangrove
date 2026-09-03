package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"golang.org/x/term"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
)

// terminalDoneMsg resumes the TUI (via RestoreTerminal, already done by the
// time this is sent) after a shell view ends.
type terminalDoneMsg struct {
	serviceName string
	err         error
}

// openShellCmd bridges the local terminal to a service's container shell,
// the same GET /api/services/{id}/terminal websocket the dashboard's
// xterm.js Terminal tab drives (see docs/architecture.md) -- just with a
// real local pty on this end instead of a browser-rendered one.
//
// It hands the terminal to the shell for the duration of the session via
// program.p.ReleaseTerminal()/RestoreTerminal() (program is populated once
// in main(), well before any key press could reach here -- see
// programHolder's doc comment in model.go), which is why this returns a
// terminalDoneMsg rather than updating state directly: it runs synchronously
// on its own, outside bubbletea's normal render loop, until the shell ends.
//
// Known rough edge: once the remote shell exits, the goroutine reading
// local stdin is still blocked on that read until the next keypress --
// there's no portable way to cancel a blocking os.Stdin.Read from another
// goroutine. The on-screen banner below says so; pressing any key
// (including Enter) after "[shell closed]" appears is what actually
// returns to the TUI.
func openShellCmd(program *programHolder, client *apiclient.Client, serviceID int64, serviceName string) tea.Cmd {
	return func() tea.Msg {
		if program.p == nil {
			return terminalDoneMsg{serviceName: serviceName, err: fmt.Errorf("internal error: program not yet initialized")}
		}
		if err := program.p.ReleaseTerminal(); err != nil {
			return terminalDoneMsg{serviceName: serviceName, err: err}
		}
		err := runShellBridge(client, serviceID, serviceName)
		if restoreErr := program.p.RestoreTerminal(); restoreErr != nil && err == nil {
			err = restoreErr
		}
		return terminalDoneMsg{serviceName: serviceName, err: err}
	}
}

func runShellBridge(client *apiclient.Client, serviceID int64, serviceName string) error {
	fmt.Printf("\r\n--- opening shell: %s (ctrl-d or `exit` to leave) ---\r\n", serviceName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := client.OpenTerminal(ctx, serviceID)
	if err != nil {
		fmt.Printf("\r\nfailed to open shell: %v\r\n", err)
		waitForKeypress()
		return nil // already reported inline; no need to also surface via the status bar
	}
	defer conn.Close()

	stdinFD := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(stdinFD)
	if err != nil {
		return fmt.Errorf("enter raw mode: %w", err)
	}
	defer term.Restore(stdinFD, oldState)

	sendSize := func() {
		cols, rows, err := term.GetSize(stdinFD)
		if err != nil {
			return
		}
		msg, _ := json.Marshal(map[string]any{"type": "resize", "cols": cols, "rows": rows})
		conn.WriteMessage(websocket.TextMessage, msg)
	}
	sendSize()

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for {
			select {
			case <-winch:
				sendSize()
			case <-ctx.Done():
				return
			}
		}
	}()

	var once sync.Once
	closed := make(chan struct{})
	markClosed := func() { once.Do(func() { close(closed) }) }

	// shell -> local stdout
	go func() {
		defer markClosed()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if mt == websocket.BinaryMessage {
				os.Stdout.Write(data)
			}
		}
	}()

	// local stdin -> shell. Runs on this goroutine (the Cmd's own), so
	// runShellBridge blocks here until either direction ends -- see the
	// doc comment above about the one-more-keypress rough edge this
	// implies once the remote side closes first.
	buf := make([]byte, 4096)
	for {
		select {
		case <-closed:
			return nil
		default:
		}
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
				markClosed()
				return nil
			}
		}
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("read local stdin: %w", err)
			}
			markClosed()
			return nil
		}
		select {
		case <-closed:
			fmt.Print("\r\n--- shell closed ---\r\n")
			return nil
		default:
		}
	}
}

// waitForKeypress is used only on the "failed to open shell at all" path,
// where there's no websocket to notice being closed -- otherwise the error
// banner above would flash and vanish the instant RestoreTerminal repaints
// the TUI underneath it.
func waitForKeypress() {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err == nil {
		defer term.Restore(fd, oldState)
	}
	one := make([]byte, 1)
	os.Stdin.Read(one)
}

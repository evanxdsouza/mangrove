package main

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
)

type loginState struct {
	email    textinput.Model
	password textinput.Model
	focus    int // 0 = email, 1 = password
	busy     bool
	setup    bool // true if this is first-run account creation, not login
	err      string
}

func newLoginState() loginState {
	email := textinput.New()
	email.Placeholder = "email"
	email.Focus()

	password := textinput.New()
	password.Placeholder = "password"
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '*'

	return loginState{email: email, password: password}
}

type authStatusMsg struct {
	setupRequired bool
	err           error
}

func loadAuthStatusCmd(client *apiclient.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		st, err := client.AuthStatus(ctx)
		return authStatusMsg{setupRequired: st.SetupRequired, err: err}
	}
}

type loginResultMsg struct {
	user apiclient.CurrentUser
	err  error
}

func doLoginCmd(client *apiclient.Client, email, password string, setup bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var user apiclient.CurrentUser
		var err error
		if setup {
			user, err = client.Setup(ctx, email, password)
		} else {
			user, err = client.Login(ctx, email, password)
		}
		return loginResultMsg{user: user, err: err}
	}
}

func (m model) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case authStatusMsg:
		if msg.err == nil {
			m.login.setup = msg.setupRequired
		}
		return m, nil

	case loginResultMsg:
		m.login.busy = false
		if msg.err != nil {
			m.login.err = msg.err.Error()
			return m, nil
		}
		m.view = viewProjects
		m.projects = newProjectsState()
		return m, tea.Batch(statusCmd(fmt.Sprintf("Logged in as %s.", msg.user.Email), false), loadProjectsCmd(m.client))

	case tea.KeyMsg:
		if m.login.busy {
			return m, nil
		}
		switch msg.String() {
		case "tab", "down":
			m.login.focus = 1 - m.login.focus
			m.focusLogin()
			return m, nil
		case "shift+tab", "up":
			m.login.focus = 1 - m.login.focus
			m.focusLogin()
			return m, nil
		case "enter":
			if m.login.email.Value() == "" || m.login.password.Value() == "" {
				m.login.err = "email and password are required"
				return m, nil
			}
			m.login.busy = true
			m.login.err = ""
			return m, doLoginCmd(m.client, m.login.email.Value(), m.login.password.Value(), m.login.setup)
		}
	}

	var cmd tea.Cmd
	if m.login.focus == 0 {
		m.login.email, cmd = m.login.email.Update(msg)
	} else {
		m.login.password, cmd = m.login.password.Update(msg)
	}
	return m, cmd
}

func (m *model) focusLogin() {
	if m.login.focus == 0 {
		m.login.email.Focus()
		m.login.password.Blur()
	} else {
		m.login.email.Blur()
		m.login.password.Focus()
	}
}

func (m model) viewLogin() string {
	action := "Log in"
	if m.login.setup {
		action = "Create the admin account"
	}
	s := "\n " + styleDim.Render(action+" to continue.") + "\n\n"
	s += "  " + m.login.email.View() + "\n"
	s += "  " + m.login.password.View() + "\n\n"
	if m.login.busy {
		s += "  " + styleDim.Render("Working...") + "\n"
	} else if m.login.err != "" {
		s += "  " + styleErr.Render(m.login.err) + "\n"
	}
	s += "\n  " + styleHelp.Render("tab: switch field  ·  enter: submit  ·  ctrl+c: quit")
	return s
}

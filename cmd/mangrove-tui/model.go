package main

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
	"github.com/evanxdsouza/mangrove/internal/models"
)

// view identifies which screen the model is currently showing. There is
// deliberately no view stack -- each view knows the one place "back" goes,
// matching the dashboard's own shallow navigation (projects -> deployments
// -> deployment detail, plus detail's logs/history/shell sub-views).
type view int

const (
	viewLogin view = iota
	viewProjects
	viewDeployments
	viewDetail
	viewHistory
	viewLogs
)

// programHolder lets a tea.Cmd reach back into the *tea.Program that's
// running it -- needed for the shell view's ReleaseTerminal/RestoreTerminal
// dance (see terminal.go), which can't be done any other way since a Cmd
// only gets a Msg, not the Program. It's a pointer, shared by every copy
// of Model Update produces, and populated once in main() right after
// tea.NewProgram returns -- by the time any key press could trigger a
// shell, the program is long since running and this is set.
type programHolder struct{ p *tea.Program }

type model struct {
	client  *apiclient.Client
	program *programHolder

	view   view
	width  int
	height int

	status    string // transient status-bar message (e.g. "Redeployed." or an error)
	statusErr bool

	login       loginState
	projects    projectsState
	deployments deploymentsState
	detail      detailState
	history     historyState
	logs        logsState
}

func initialModel(client *apiclient.Client) model {
	m := model{
		client:  client,
		program: &programHolder{},
		view:    viewLogin,
	}
	m.login = newLoginState()
	// Every list/viewport-backed state is constructed up front, not lazily
	// on first navigation -- propagateSize below calls SetSize on all of
	// them unconditionally on every tea.WindowSizeMsg (which fires
	// immediately at startup, before the user has navigated anywhere), and
	// bubbles/list's zero value has a nil delegate: calling SetSize on an
	// unconstructed list.Model panics instead of erroring. Real content
	// loads in when the user actually navigates there.
	m.projects = newProjectsState()
	m.deployments = newDeploymentsState(models.Project{})
	m.history = newHistoryState(models.Deployment{})
	m.logs = newLogsState(models.Service{}, false)
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, checkAuthCmd(m.client))
}

// ---- messages shared across views ----

type errMsg struct{ err error }

// statusMsg sets the transient status-bar line -- ok (green-ish, or just
// dim) vs isErr (red). Every mutating action (redeploy, stop, rollback,
// ...) resolves to one of these so the user always sees what happened,
// not just a screen that quietly changed or didn't.
type statusMsg struct {
	text  string
	isErr bool
}

func statusCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return statusMsg{text: text, isErr: isErr} }
}

func errStatusCmd(prefix string, err error) tea.Cmd {
	return statusCmd(prefix+": "+err.Error(), true)
}

// checkAuthMsg reports whether the loaded session (if any) is actually
// still valid server-side -- LoadSession only reads a file, it doesn't
// verify anything.
type checkAuthMsg struct {
	user apiclient.CurrentUser
	err  error
}

func checkAuthCmd(client *apiclient.Client) tea.Cmd {
	return func() tea.Msg {
		if !client.IsAuthenticated() {
			return checkAuthMsg{err: apiclient.ErrNotAuthenticated}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		user, err := client.Me(ctx)
		return checkAuthMsg{user: user, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		var cmd tea.Cmd
		m, cmd = m.propagateSize()
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case statusMsg:
		m.status, m.statusErr = msg.text, msg.isErr
		return m, nil

	case checkAuthMsg:
		if msg.err != nil {
			m.view = viewLogin
			return m, loadAuthStatusCmd(m.client)
		}
		m.view = viewProjects
		m.projects = newProjectsState()
		return m, loadProjectsCmd(m.client)
	}

	switch m.view {
	case viewLogin:
		return m.updateLogin(msg)
	case viewProjects:
		return m.updateProjects(msg)
	case viewDeployments:
		return m.updateDeployments(msg)
	case viewDetail:
		return m.updateDetail(msg)
	case viewHistory:
		return m.updateHistory(msg)
	case viewLogs:
		return m.updateLogs(msg)
	}
	return m, nil
}

func (m model) propagateSize() (model, tea.Cmd) {
	// Reserve one line for the title bar, one for the status bar.
	innerH := m.height - 2
	if innerH < 0 {
		innerH = 0
	}
	m.projects.list.SetSize(m.width, innerH)
	m.deployments.list.SetSize(m.width, innerH)
	m.history.list.SetSize(m.width, innerH)
	m.logs.viewport.Width = m.width
	m.logs.viewport.Height = innerH
	return m, nil
}

func (m model) View() string {
	var body string
	switch m.view {
	case viewLogin:
		body = m.viewLogin()
	case viewProjects:
		body = m.viewProjects()
	case viewDeployments:
		body = m.viewDeployments()
	case viewDetail:
		body = m.viewDetail()
	case viewHistory:
		body = m.viewHistory()
	case viewLogs:
		body = m.viewLogs()
	}

	title := styleTitle.Render("Mangrove")
	statusLine := ""
	if m.status != "" {
		style := styleDim
		if m.statusErr {
			style = styleErr
		}
		statusLine = styleStatusBar.Render(style.Render(m.status))
	}
	return title + "\n" + body + "\n" + statusLine
}

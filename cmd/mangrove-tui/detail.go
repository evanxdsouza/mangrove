package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
	"github.com/evanxdsouza/mangrove/internal/models"
)

type detailState struct {
	deployment models.Deployment
	services   []models.Service
	selected   int
	loading    bool
	busy       string
}

func newDetailState(d models.Deployment) detailState {
	return detailState{deployment: d, loading: true}
}

type detailLoadedMsg struct {
	deployment models.Deployment
	services   []models.Service
	err        error
}

func loadDetailCmd(client *apiclient.Client, deploymentID int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		dep, err := client.GetDeployment(ctx, deploymentID)
		if err != nil {
			return detailLoadedMsg{err: err}
		}
		services, err := client.ListServices(ctx, deploymentID)
		return detailLoadedMsg{deployment: dep, services: services, err: err}
	}
}

// actionResultMsg is the generic outcome of a mutating detail-view action
// (redeploy, restart, stop, scale) -- label goes straight into the status
// bar, prefixed with "Failed to " on error.
type actionResultMsg struct {
	label string
	err   error
}

func actionCmd(label string, fn func(ctx context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		return actionResultMsg{label: label, err: fn(ctx)}
	}
}

func outcomeErr(out apiclient.DeployOutcome, err error) error {
	if err != nil {
		return err
	}
	if out.Error != "" {
		return fmt.Errorf("%s", out.Error)
	}
	return nil
}

func (m model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case detailLoadedMsg:
		m.detail.loading = false
		if msg.err != nil {
			return m, errStatusCmd("Failed to load deployment", msg.err)
		}
		m.detail.deployment = msg.deployment
		m.detail.services = msg.services
		if m.detail.selected >= len(msg.services) {
			m.detail.selected = 0
		}
		return m, nil

	case actionResultMsg:
		m.detail.busy = ""
		if msg.err != nil {
			return m, errStatusCmd("Failed to "+msg.label, msg.err)
		}
		m.detail.loading = true
		return m, tea.Batch(statusCmd(msg.label+" succeeded.", false), loadDetailCmd(m.client, m.detail.deployment.ID))

	case tea.KeyMsg:
		if m.detail.busy != "" {
			return m, nil // one mutating action at a time
		}
		id := m.detail.deployment.ID
		switch msg.String() {
		case "esc", "left":
			m.view = viewDeployments
			return m, nil
		case "r":
			m.detail.loading = true
			return m, loadDetailCmd(m.client, id)
		case "up", "k":
			if m.detail.selected > 0 {
				m.detail.selected--
			}
			return m, nil
		case "down", "j":
			if m.detail.selected < len(m.detail.services)-1 {
				m.detail.selected++
			}
			return m, nil
		case "d":
			m.detail.busy = "Redeploy"
			return m, actionCmd("Redeploy", func(ctx context.Context) error {
				out, err := m.client.Redeploy(ctx, id)
				return outcomeErr(out, err)
			})
		case "R":
			m.detail.busy = "Restart"
			return m, actionCmd("Restart", func(ctx context.Context) error { return m.client.Restart(ctx, id) })
		case "x":
			m.detail.busy = "Stop"
			return m, actionCmd("Stop", func(ctx context.Context) error { return m.client.Stop(ctx, id) })
		case "+", "=":
			replicas := max(m.detail.deployment.Replicas, 1) + 1
			m.detail.busy = "Scale"
			return m, actionCmd("Scale", func(ctx context.Context) error {
				out, err := m.client.Scale(ctx, id, replicas)
				return outcomeErr(out, err)
			})
		case "-", "_":
			replicas := max(m.detail.deployment.Replicas, 1) - 1
			if replicas < 1 {
				return m, statusCmd("Already at the minimum of 1 replica.", true)
			}
			m.detail.busy = "Scale"
			return m, actionCmd("Scale", func(ctx context.Context) error {
				out, err := m.client.Scale(ctx, id, replicas)
				return outcomeErr(out, err)
			})
		case "h":
			m.view = viewHistory
			m.history = newHistoryState(m.detail.deployment)
			return m, loadHistoryCmd(m.client, id)
		case "l":
			svc, ok := m.selectedService()
			if !ok {
				return m, statusCmd("No services to view logs for.", true)
			}
			m.view = viewLogs
			m.logs = newLogsState(svc, false)
			return m, startLogStreamCmd(m.client, svc.ID)
		case "t":
			svc, ok := m.selectedService()
			if !ok {
				return m, statusCmd("No services to open a shell in.", true)
			}
			if svc.ContainerIDCurrent == "" {
				return m, statusCmd("Service has no running container.", true)
			}
			return m, openShellCmd(m.program, m.client, svc.ID, svc.Name)
		}
	}
	return m, nil
}

func (m model) selectedService() (models.Service, bool) {
	if m.detail.selected < 0 || m.detail.selected >= len(m.detail.services) {
		return models.Service{}, false
	}
	return m.detail.services[m.detail.selected], true
}

func (m model) viewDetail() string {
	d := m.detail.deployment
	if m.detail.loading && len(m.detail.services) == 0 {
		return "\n  " + styleDim.Render("Loading...")
	}

	statusStyle := lipgloss.NewStyle().Foreground(statusColor(d.Status)).Bold(true)
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s  %s\n", lipgloss.NewStyle().Bold(true).Render(d.Name), statusStyle.Render("● "+d.Status))
	fmt.Fprintf(&b, "  %s%s\n", styleField.Render("Slug"), d.Slug)
	fmt.Fprintf(&b, "  %s%s\n", styleField.Render("Strategy"), d.BuildStrategy)
	env := d.Environment
	if env == "" {
		env = "production"
	}
	fmt.Fprintf(&b, "  %s%s\n", styleField.Render("Environment"), env)
	fmt.Fprintf(&b, "  %s%d\n", styleField.Render("Replicas"), max(d.Replicas, 1))
	visibility := "private (proxy only)"
	if d.IsPublic {
		visibility = "public"
		if d.PasswordProtected {
			visibility += ", password-protected"
		}
	}
	fmt.Fprintf(&b, "  %s%s\n", styleField.Render("Visibility"), visibility)

	b.WriteString("\n  " + styleDim.Render("Services") + "\n")
	if len(m.detail.services) == 0 {
		b.WriteString("    " + styleDim.Render("(none)") + "\n")
	}
	for i, svc := range m.detail.services {
		cursor := "  "
		if i == m.detail.selected {
			cursor = styleTitle.Render("›") + " "
		}
		svcStatus := lipgloss.NewStyle().Foreground(statusColor(svc.Status)).Render("● " + svc.Status)
		container := svc.ContainerIDCurrent
		if len(container) > 12 {
			container = container[:12]
		}
		if container == "" {
			container = "(no container)"
		}
		fmt.Fprintf(&b, "  %s%-20s %-14s %s\n", cursor, svc.Name, svcStatus, styleDim.Render(container))
	}

	b.WriteString("\n")
	if m.detail.busy != "" {
		b.WriteString("  " + styleDim.Render(m.detail.busy+"...") + "\n")
	}

	b.WriteString("\n  " + styleHelp.Render(
		"j/k: select service  ·  d: redeploy  ·  R: restart  ·  x: stop  ·  +/-: scale  ·  l: logs  ·  t: shell  ·  h: history  ·  esc: back"))
	return b.String()
}

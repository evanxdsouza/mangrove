package main

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
	"github.com/evanxdsouza/mangrove/internal/models"
)

type deploymentItem struct{ d models.Deployment }

func (i deploymentItem) FilterValue() string { return i.d.Name }
func (i deploymentItem) Title() string {
	statusStyle := lipgloss.NewStyle().Foreground(statusColor(i.d.Status))
	env := ""
	if i.d.Environment != "" && i.d.Environment != "production" {
		env = " [" + i.d.Environment + "]"
	}
	return i.d.Name + env + "  " + statusStyle.Render("● "+i.d.Status)
}
func (i deploymentItem) Description() string {
	return fmt.Sprintf("%s  ·  %s  ·  %dx", i.d.Slug, i.d.BuildStrategy, max(i.d.Replicas, 1))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type deploymentsState struct {
	project models.Project
	list    list.Model
	loading bool
}

func newDeploymentsState(project models.Project) deploymentsState {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Deployments — " + project.Name
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	return deploymentsState{project: project, list: l, loading: true}
}

type deploymentsLoadedMsg struct {
	deployments []models.Deployment
	err         error
}

func loadDeploymentsCmd(client *apiclient.Client, projectID int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		deployments, err := client.ListDeployments(ctx, projectID)
		return deploymentsLoadedMsg{deployments: deployments, err: err}
	}
}

func (m model) updateDeployments(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case deploymentsLoadedMsg:
		m.deployments.loading = false
		if msg.err != nil {
			return m, errStatusCmd("Failed to load deployments", msg.err)
		}
		items := make([]list.Item, len(msg.deployments))
		for i, d := range msg.deployments {
			items[i] = deploymentItem{d: d}
		}
		m.deployments.list.SetItems(items)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "left":
			m.view = viewProjects
			return m, nil
		case "r":
			m.deployments.loading = true
			return m, loadDeploymentsCmd(m.client, m.deployments.project.ID)
		case "enter":
			if it, ok := m.deployments.list.SelectedItem().(deploymentItem); ok {
				m.view = viewDetail
				m.detail = newDetailState(it.d)
				return m, loadDetailCmd(m.client, it.d.ID)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.deployments.list, cmd = m.deployments.list.Update(msg)
	return m, cmd
}

func (m model) viewDeployments() string {
	if m.deployments.loading {
		return "\n  " + styleDim.Render("Loading deployments...")
	}
	return m.deployments.list.View() + "\n" + styleHelp.Render("enter: open  ·  r: refresh  ·  esc: back  ·  ctrl+c: quit")
}

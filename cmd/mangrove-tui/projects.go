package main

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/evanxdsouza/mangrove/internal/apiclient"
	"github.com/evanxdsouza/mangrove/internal/store"
)

type projectItem struct{ p store.ProjectWithWorkspace }

func (i projectItem) FilterValue() string { return i.p.Name }
func (i projectItem) Title() string       { return i.p.Name }
func (i projectItem) Description() string {
	return fmt.Sprintf("%s  ·  workspace: %s", i.p.Slug, i.p.WorkspaceName)
}

type projectsState struct {
	list    list.Model
	loading bool
}

func newProjectsState() projectsState {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Projects"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowTitle(true)
	return projectsState{list: l, loading: true}
}

type projectsLoadedMsg struct {
	projects []store.ProjectWithWorkspace
	err      error
}

func loadProjectsCmd(client *apiclient.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		projects, err := client.ListProjects(ctx, nil)
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

func (m model) updateProjects(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		m.projects.loading = false
		if msg.err != nil {
			return m, errStatusCmd("Failed to load projects", msg.err)
		}
		items := make([]list.Item, len(msg.projects))
		for i, p := range msg.projects {
			items[i] = projectItem{p: p}
		}
		m.projects.list.SetItems(items)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.projects.loading = true
			return m, loadProjectsCmd(m.client)
		case "enter":
			if it, ok := m.projects.list.SelectedItem().(projectItem); ok {
				m.view = viewDeployments
				m.deployments = newDeploymentsState(it.p.Project)
				return m, loadDeploymentsCmd(m.client, it.p.ID)
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.projects.list, cmd = m.projects.list.Update(msg)
	return m, cmd
}

func (m model) viewProjects() string {
	if m.projects.loading {
		return "\n  " + styleDim.Render("Loading projects...")
	}
	return m.projects.list.View() + "\n" + styleHelp.Render("enter: open  ·  r: refresh  ·  ctrl+c: quit")
}

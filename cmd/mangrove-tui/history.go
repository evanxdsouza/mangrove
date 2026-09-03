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

type historyItem struct{ h models.DeployHistory }

func (i historyItem) FilterValue() string { return i.h.CommitMessage }
func (i historyItem) Title() string {
	statusStyle := lipgloss.NewStyle().Foreground(statusColor(i.h.Status))
	current := ""
	if i.h.IsCurrent {
		current = "  (current)"
	}
	sha := i.h.CommitSHA
	if len(sha) > 8 {
		sha = sha[:8]
	}
	return fmt.Sprintf("#%d  %s  %s%s", i.h.ID, statusStyle.Render("● "+i.h.Status), sha, current)
}
func (i historyItem) Description() string {
	msg := i.h.CommitMessage
	if msg == "" {
		msg = "(" + i.h.TriggeredBy + ")"
	}
	return fmt.Sprintf("%s  ·  %s", i.h.StartedAt.Local().Format("2006-01-02 15:04"), msg)
}

type historyState struct {
	deployment models.Deployment
	list       list.Model
	loading    bool
}

func newHistoryState(d models.Deployment) historyState {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "History — " + d.Name
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	return historyState{deployment: d, list: l, loading: true}
}

type historyLoadedMsg struct {
	history []models.DeployHistory
	err     error
}

func loadHistoryCmd(client *apiclient.Client, deploymentID int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		history, err := client.ListDeployHistory(ctx, deploymentID)
		return historyLoadedMsg{history: history, err: err}
	}
}

func (m model) updateHistory(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		m.history.loading = false
		if msg.err != nil {
			return m, errStatusCmd("Failed to load history", msg.err)
		}
		items := make([]list.Item, len(msg.history))
		for i, h := range msg.history {
			items[i] = historyItem{h: h}
		}
		m.history.list.SetItems(items)
		return m, nil

	case actionResultMsg:
		if msg.err != nil {
			return m, errStatusCmd("Failed to "+msg.label, msg.err)
		}
		m.view = viewDetail
		m.detail.loading = true
		return m, tea.Batch(statusCmd(msg.label+" succeeded.", false), loadDetailCmd(m.client, m.history.deployment.ID))

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "left":
			m.view = viewDetail
			return m, nil
		case "r":
			m.history.loading = true
			return m, loadHistoryCmd(m.client, m.history.deployment.ID)
		case "enter":
			it, ok := m.history.list.SelectedItem().(historyItem)
			if !ok {
				return m, nil
			}
			if it.h.IsCurrent {
				return m, statusCmd("That's already the current deploy.", true)
			}
			return m, actionCmd("Rollback", func(ctx context.Context) error {
				out, err := m.client.Rollback(ctx, it.h.ID)
				return outcomeErr(out, err)
			})
		}
	}

	var cmd tea.Cmd
	m.history.list, cmd = m.history.list.Update(msg)
	return m, cmd
}

func (m model) viewHistory() string {
	if m.history.loading {
		return "\n  " + styleDim.Render("Loading history...")
	}
	return m.history.list.View() + "\n" + styleHelp.Render("enter: roll back to this deploy  ·  r: refresh  ·  esc: back")
}

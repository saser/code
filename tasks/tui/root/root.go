package root

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	pb "go.saser.se/tasks/tasks_go_proto"
)

type Model struct {
	ctx    context.Context
	cancel context.CancelFunc

	client pb.TasksClient
}

func New(ctx context.Context, client pb.TasksClient) *Model {
	m := &Model{
		client: client,
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, m.quit
		}
	}
	return m, nil
}

func (m *Model) View() string {
	var b strings.Builder
	fmt.Fprintln(&b, "Press Ctrl-C to exit.")
	return b.String()
}

func (m *Model) quit() tea.Msg {
	m.cancel()
	return tea.Quit()
}

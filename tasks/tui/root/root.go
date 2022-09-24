package root

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	pb "go.saser.se/tasks/tasks_go_proto"
	"go.saser.se/tasks/tui/tasklist"
)

type Model struct {
	ctx    context.Context
	cancel context.CancelFunc

	client pb.TasksClient

	list *tasklist.Model
}

func New(ctx context.Context, client pb.TasksClient) *Model {
	m := &Model{
		client: client,
		list:   tasklist.New(client),
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.list.InitContext(m.ctx),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.UpdateContext(m.ctx, msg)
}

func (m *Model) UpdateContext(ctx context.Context, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, m.quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.UpdateContext(ctx, msg)
	return m, cmd
}

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString(m.list.View())
	return b.String()
}

func (m *Model) quit() tea.Msg {
	m.cancel()
	return tea.Quit()
}

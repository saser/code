package tasklist

import (
	"context"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	pb "go.saser.se/tasks/tasks_go_proto"
	"go.saser.se/tasks/tui/tasksclient"
)

type Model struct {
	client pb.TasksClient
	list   list.Model
}

type item struct {
	task *pb.Task
}

func (i *item) Title() string       { return i.task.GetTitle() }
func (i *item) Description() string { return i.task.GetDescription() }
func (i *item) FilterValue() string { return i.Title() }

var _ list.DefaultItem = (*item)(nil)

func New(client pb.TasksClient) *Model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	return &Model{
		client: client,
		list:   l,
	}
}

func (m *Model) InitContext(ctx context.Context) tea.Cmd {
	return tasksclient.ListTasksCmd(ctx, m.client, &pb.ListTasksRequest{})
}

func (m *Model) UpdateContext(ctx context.Context, msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case *tasksclient.ListTasksDone:
		if msg.Err != nil {
			// TODO: error handling
		}
		var items []list.Item
		for _, t := range msg.Response.GetTasks() {
			items = append(items, &item{t})
		}
		return m, m.list.SetItems(items)
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlR:
			return m, tasksclient.ListTasksCmd(ctx, m.client, &pb.ListTasksRequest{})
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *Model) View() string {
	return m.list.View()
}

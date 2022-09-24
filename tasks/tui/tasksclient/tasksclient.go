package tasksclient

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	pb "go.saser.se/tasks/tasks_go_proto"
)

type ListTasksDone struct {
	Response *pb.ListTasksResponse
	Err      error
}

func ListTasksCmd(ctx context.Context, c pb.TasksClient, req *pb.ListTasksRequest) tea.Cmd {
	return func() tea.Msg {
		res, err := c.ListTasks(ctx, req)
		return &ListTasksDone{
			Response: res,
			Err:      err,
		}
	}
}

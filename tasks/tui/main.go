package main

import (
	"context"
	"flag"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"
	"go.saser.se/tasks/tui/root"
	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(flag.CommandLine)
}

func errmain() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := tea.NewProgram(root.New(ctx), tea.WithAltScreen()).Start(); err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()
	if err := errmain(); err != nil {
		klog.Exit(err)
	}
}

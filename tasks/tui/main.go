package main

import (
	"flag"

	tea "github.com/charmbracelet/bubbletea"
	"go.saser.se/tasks/tui/root"
	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(flag.CommandLine)
}

func errmain() error {
	if err := tea.NewProgram(root.New(), tea.WithAltScreen()).Start(); err != nil {
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

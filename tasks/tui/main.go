package main

import (
	"context"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"
	"go.saser.se/auth/n/basic"
	pb "go.saser.se/tasks/tasks_go_proto"
	"go.saser.se/tasks/tui/root"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(flag.CommandLine)
}

var (
	addr     = flag.String("addr", "", "Address of the Tasks service.")
	username = flag.String("username", "", "Username to authenticate with.")
	password = flag.String("password", "", "Password to authenticate with.")
	certFile = flag.String("cert_file", "", "Path to TLS certificate. If unset the system's certificate pool will be used.")
)

func errmain() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Validate all flags.
	if *addr == "" {
		return errors.New("-addr is required")
	}
	if *username == "" || *password == "" {
		return fmt.Errorf("-username=%q and -password=%q; both are required", *username, *password)
	}

	// Set up a gRPC connection based on the flags.
	var transportCreds credentials.TransportCredentials
	if *certFile != "" {
		creds, err := credentials.NewClientTLSFromFile(*certFile, "")
		if err != nil {
			return err
		}
		transportCreds = creds
	} else {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return err
		}
		transportCreds = credentials.NewClientTLSFromCert(pool, "")
	}
	perRPCCreds := basic.Credentials{
		Username: *username,
		Password: *password,
	}
	cc, err := grpc.DialContext(
		ctx,
		*addr,
		grpc.WithTransportCredentials(transportCreds),
		grpc.WithPerRPCCredentials(perRPCCreds),
	)
	if err != nil {
		return err
	}
	client := pb.NewTasksClient(cc)

	// Run the bubbletea application, injecting the client.
	if err := tea.NewProgram(root.New(ctx, client), tea.WithAltScreen()).Start(); err != nil {
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

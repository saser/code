package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"

	"go.saser.se/tasks/fake"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"k8s.io/klog/v2"

	// Imported for side-effects.
	_ "go.saser.se/grpclog/klogger"
)

func init() {
	klog.InitFlags(flag.CommandLine)
}

var port = flag.Int("port", 8080, "The port to serve the gRPC service on.")

func errMain() error {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	addr := fmt.Sprintf(":%d", *port)
	klog.Infof("Will open listener on address %q.", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	klog.Infof("Listening on %q.", addr)
	defer func() {
		if err := lis.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			klog.Errorf("Closing listener failed: %v", err)
		}
	}()

	srv := grpc.NewServer()
	f := fake.New()
	pb.RegisterTasksServer(srv, f)
	reflection.Register(srv)

	errc := make(chan error, 1)
	go func() {
		klog.Infof("Serving gRPC server on %v.", lis.Addr())
		errc <- srv.Serve(lis)
	}()

	klog.Info("Waiting for context to be canceled.")
	<-ctx.Done()
	klog.Info("Context cancelled, stopping gRPC server...")
	srv.GracefulStop()
	klog.Info("gRPC server stopped.")
	if err := <-errc; err != nil {
		return err
	}

	return nil
}

func main() {
	fmt.Println("hello, world")
	if err := errMain(); err != nil {
		klog.Exit(err)
	}
}

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/golang/glog"
	"go.saser.se/tasks/fake"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	// Imported for side-effects.
	_ "google.golang.org/grpc/grpclog/glogger"
)

var port = flag.Int("port", 8080, "The port to serve the gRPC service on.")

func errMain() error {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	addr := fmt.Sprintf(":%d", *port)
	glog.Infof("Will open listener on address %q.", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	glog.Infof("Listening on %q.", addr)
	defer func() {
		if err := lis.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			glog.Errorf("Closing listener failed: %v", err)
		}
	}()

	srv := grpc.NewServer()
	f := fake.New()
	pb.RegisterTasksServer(srv, f)
	reflection.Register(srv)

	errc := make(chan error, 1)
	go func() {
		glog.Infof("Serving gRPC server on %v.", lis.Addr())
		errc <- srv.Serve(lis)
	}()

	glog.Info("Waiting for context to be canceled.")
	<-ctx.Done()
	glog.Info("Context cancelled, stopping gRPC server...")
	srv.GracefulStop()
	glog.Info("gRPC server stopped.")
	if err := <-errc; err != nil {
		return err
	}

	return nil
}

func main() {
	fmt.Println("hello, world")
	if err := errMain(); err != nil {
		glog.Exit(err)
	}
}

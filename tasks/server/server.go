// Command server serves gRPC requests for the `tasks` service. It is configured
// to use static HTTP Basic authentication. This command is suitable to put into
// a container image intended for Google Cloud Run.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"

	"github.com/golang/glog"
	"go.saser.se/auth/n/basic"
	"go.saser.se/postgres"
	"go.saser.se/tasks/service"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc"

	// Imported for side-effects.
	_ "google.golang.org/grpc/grpclog/glogger"
)

var (
	portFlag           = flag.Int("port", -1, "Port to serve gRPC requests on. If negative, use the PORT environment variable instead. If zero, use whatever the operating system gives us.")
	postgresConnString = flag.String("postgres_conn_string", "", "Connection string to backing Postgres database.")
	username           = flag.String("username", "", "Username to be used for basic authentication.")
	password           = flag.String("password", "", "Password to be used for basic authentication.")
)

func errmain() error {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	port := *portFlag
	if port < 0 {
		envPort := os.Getenv("PORT")
		glog.Infof("Flag -port=%d is negative; using the environment variable PORT=%q instead", port, envPort)
		var err error
		port, err = strconv.Atoi(envPort)
		if err != nil {
			return fmt.Errorf("using $PORT failed: %w", err)
		}
	}
	glog.Infof("Will listen on port %d.", port)
	if *postgresConnString == "" {
		return errors.New("-postgres_conn_string is empty")
	}
	glog.Infof("Will connect to Postgres with connection string: %q", *postgresConnString)
	if *username == "" || *password == "" {
		return fmt.Errorf("-username=%q and -password=%q; both must be non-empty", *username, *password)
	}

	listenAddr := ":" + strconv.Itoa(port)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to create listener on address %q: %w", listenAddr, err)
	}
	addr := lis.Addr().String()
	defer func() {
		// If we successfully serve and subsequently stop the gRPC server on
		// this listener, the listener will already have been closed. So we only
		// log the error if it is something else.
		if err := lis.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			glog.Errorf("Failed to close listener on address %q: %v", addr, err)
		}
	}()
	glog.Infof("Created listener on address %q.", addr)

	pool, err := postgres.Open(ctx, *postgresConnString)
	if err != nil {
		return fmt.Errorf("failed to connect to Postgres: %w", err)
	}
	defer pool.Close()
	glog.Infof("Created Postgres connection pool with connection string: %q", *postgresConnString)

	interceptor, err := basic.Interceptor(*username, *password)
	if err != nil {
		return fmt.Errorf("failed to create basic authentication interceptor: %w", err)
	}
	srv := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	srv.RegisterService(&pb.Tasks_ServiceDesc, service.New(pool))

	errc := make(chan error, 1)
	go func() {
		glog.Infof("Serving gRPC server on %q...", addr)
		errc <- srv.Serve(lis)
	}()

	glog.Info("Blocking on context cancellation...")
	<-ctx.Done()
	glog.Info("Context cancelled; gracefully stopping gRPC server...")
	srv.GracefulStop()
	glog.Info("Stopped gRPC server.")

	if err := <-errc; err != nil {
		return fmt.Errorf("failed to serve gRPC server: %w", err)
	}

	return nil
}

func main() {
	if err := errmain(); err != nil {
		glog.Exit(err)
	}
}

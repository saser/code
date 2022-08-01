// Package echo contains an implementation of the Echo service.
package echo

import (
	"context"

	pb "go.saser.se/testing/echo_go_proto"
)

// Server implements the Echo service. It simply returns the input message.
type Server struct {
	pb.UnimplementedEchoServer
}

var _ pb.EchoServer = Server{}

func (Server) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	return &pb.EchoResponse{Message: req.GetMessage()}, nil
}

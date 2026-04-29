package example

import (
	"context"
	"fmt"

	pb "rpc-server/pb/example"
	"rpc-server/internal/rpc"
)

type UserService struct{}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	return &pb.GetUserResponse{
		Id:   req.Id,
		Name: fmt.Sprintf("user-%d", req.Id),
		Age:  25,
	}, nil
}

var _ rpc.UserServiceHandler = (*UserService)(nil)

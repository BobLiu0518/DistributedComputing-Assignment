package example

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	pb "rpc-server/pb/example"
)

func GetUserHandler(ctx context.Context, payload []byte) ([]byte, error) {
	req := &pb.GetUserRequest{}
	if err := proto.Unmarshal(payload, req); err != nil {
		return nil, fmt.Errorf("unmarshal GetUserRequest: %w", err)
	}

	resp := &pb.GetUserResponse{
		Id:   req.Id,
		Name: fmt.Sprintf("user-%d", req.Id),
		Age:  25,
	}

	return proto.Marshal(resp)
}

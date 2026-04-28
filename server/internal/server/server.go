package server

import (
	"context"
	"log"
	"net"

	"google.golang.org/protobuf/proto"

	pb "rpc-server/pb"
	"rpc-server/internal/codec"
	"rpc-server/internal/registry"
	"rpc-server/internal/router"
)

type Server struct {
	addr     string
	router   *router.Router
	registry *registry.Client
}

func New(addr string, r *router.Router, reg *registry.Client) *Server {
	return &Server{
		addr:     addr,
		router:   r,
		registry: reg,
	}
}

func (s *Server) Start(ctx context.Context) error {
	if err := s.registry.Register(ctx); err != nil {
		return err
	}

	go s.registry.HeartbeatLoop(ctx)

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", s.addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	log.Printf("[server] listening on %s", s.addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				log.Printf("[server] accept error: %v", err)
				continue
			}
		}
		go s.handleConnection(ctx, conn)
	}
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	for {
		data, err := codec.ReadFrame(conn)
		if err != nil {
			return
		}

		req := &pb.RpcRequest{}
		if err := proto.Unmarshal(data, req); err != nil {
			s.writeError(conn, "", 400, "invalid request: "+err.Error())
			continue
		}

		handler, ok := s.router.Route(req.Service, req.Method)
		if !ok {
			s.writeError(conn, req.RequestId, 404,
				"method not found: "+req.Service+"."+req.Method)
			continue
		}

		respBytes, handlerErr := handler(ctx, req.Payload)

		resp := &pb.RpcResponse{RequestId: req.RequestId}
		if handlerErr != nil {
			resp.Error = &pb.Error{
				Code:    500,
				Message: handlerErr.Error(),
			}
		} else {
			resp.Payload = respBytes
		}

		respData, err := proto.Marshal(resp)
		if err != nil {
			log.Printf("[server] marshal response: %v", err)
			continue
		}
		if err := codec.WriteFrame(conn, respData); err != nil {
			return
		}
	}
}

func (s *Server) writeError(conn net.Conn, requestID string, code int32, message string) {
	resp := &pb.RpcResponse{
		RequestId: requestID,
		Error:     &pb.Error{Code: code, Message: message},
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		return
	}
	codec.WriteFrame(conn, data)
}

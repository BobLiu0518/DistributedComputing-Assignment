package server

import (
	"context"
	"log"
	"net"
	"sync"

	"google.golang.org/protobuf/proto"

	"rpc-server/internal/codec"
	"rpc-server/internal/registry"
	"rpc-server/internal/router"
	pb "rpc-server/pb"
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

	go s.registry.Run(ctx)

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

func (s *Server) Stop() {
	s.registry.Stop()
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	var writeMu sync.Mutex

	for {
		data, err := codec.ReadFrame(conn)
		if err != nil {
			return
		}

		req := &pb.RpcRequest{}
		if err := proto.Unmarshal(data, req); err != nil {
			writeMu.Lock()
			s.writeError(conn, "", 400, "invalid request: "+err.Error())
			writeMu.Unlock()
			continue
		}

		handler, ok := s.router.Route(req.Service, req.Method)
		if !ok {
			writeMu.Lock()
			s.writeError(conn, req.RequestId, 404,
				"method not found: "+req.Service+"."+req.Method)
			writeMu.Unlock()
			continue
		}

		go func(req *pb.RpcRequest, h router.HandlerFunc) {
			respBytes, handlerErr := h(ctx, req.Payload)

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
				return
			}

			writeMu.Lock()
			defer writeMu.Unlock()
			if err := codec.WriteFrame(conn, respData); err != nil {
				log.Printf("[server] write response: %v", err)
			}
		}(req, handler)
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

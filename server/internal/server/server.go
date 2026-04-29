package server

import (
	"context"
	"log/slog"
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
	listener net.Listener
	logger   *slog.Logger
}

func New(addr string, r *router.Router, reg *registry.Client) *Server {
	return &Server{
		addr:     addr,
		router:   r,
		registry: reg,
		logger:   slog.Default().With("component", "server"),
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
	s.listener = listener
	s.logger.Info("listening", "addr", s.addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				s.logger.Error("accept error", "error", err)
				continue
			}
		}
		go s.handleConnection(ctx, conn)
	}
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	s.registry.Stop()
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	peer := conn.RemoteAddr().String()
	logger := s.logger.With("peer", peer)
	var writeMu sync.Mutex

	for {
		data, err := codec.ReadFrame(conn)
		if err != nil {
			logger.Debug("connection closed", "error", err)
			return
		}

		req := &pb.RpcRequest{}
		if err := proto.Unmarshal(data, req); err != nil {
			logger.Warn("invalid request", "error", err)
			writeMu.Lock()
			s.writeError(conn, "", 400, "invalid request: "+err.Error())
			writeMu.Unlock()
			continue
		}

		reqLogger := logger.With("request_id", req.RequestId, "service", req.Service, "method", req.Method)

		handler, ok := s.router.Route(req.Service, req.Method)
		if !ok {
			reqLogger.Warn("method not found")
			writeMu.Lock()
			s.writeError(conn, req.RequestId, 404,
				"method not found: "+req.Service+"."+req.Method)
			writeMu.Unlock()
			continue
		}

		reqLogger.Debug("handling request")
		go func(req *pb.RpcRequest, h router.HandlerFunc) {
			defer func() {
				if r := recover(); r != nil {
					reqLogger.Error("handler panic", "panic", r)
				}
			}()

			respBytes, handlerErr := h(ctx, req.Payload)

			resp := &pb.RpcResponse{RequestId: req.RequestId}
			if handlerErr != nil {
				reqLogger.Error("handler error", "error", handlerErr)
				resp.Error = &pb.Error{
					Code:    500,
					Message: handlerErr.Error(),
				}
			} else {
				resp.Payload = respBytes
			}

			respData, err := proto.Marshal(resp)
			if err != nil {
				reqLogger.Error("marshal response", "error", err)
				return
			}

			writeMu.Lock()
			defer writeMu.Unlock()
			if err := codec.WriteFrame(conn, respData); err != nil {
				reqLogger.Error("write response", "error", err)
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
	if err := codec.WriteFrame(conn, data); err != nil {
		s.logger.Warn("write error response failed", "error", err, "request_id", requestID)
	}
}

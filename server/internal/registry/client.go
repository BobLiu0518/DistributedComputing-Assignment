package registry

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"rpc-server/internal/codec"
	pb "rpc-server/pb"
)

type Client struct {
	registryAddr        string
	selfPort            int
	serviceName         string
	heartbeatInterval   time.Duration
	heartbeatMaxFail    int
	reconnectBackoff    time.Duration
	reconnectMaxBackoff time.Duration

	conn     net.Conn
	connMu   sync.RWMutex
	stopCh   chan struct{}
	stopOnce sync.Once
	logger   *slog.Logger
}

func NewClient(
	registryAddr string,
	selfPort int,
	serviceName string,
	heartbeatInterval time.Duration,
	heartbeatMaxFail int,
	reconnectBackoff time.Duration,
	reconnectMaxBackoff time.Duration,
) *Client {
	return &Client{
		registryAddr:        registryAddr,
		selfPort:            selfPort,
		serviceName:         serviceName,
		heartbeatInterval:   heartbeatInterval,
		heartbeatMaxFail:    heartbeatMaxFail,
		reconnectBackoff:    reconnectBackoff,
		reconnectMaxBackoff: reconnectMaxBackoff,
		stopCh:              make(chan struct{}),
		logger:              slog.Default().With("component", "registry"),
	}
}

func (c *Client) Register(ctx context.Context) error {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", c.registryAddr)
	if err != nil {
		return fmt.Errorf("connect to registry: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	msg := &pb.RegistryMessage{
		Payload: &pb.RegistryMessage_Register{
			Register: &pb.RegisterRequest{
				Port:    int32(c.selfPort),
				Service: c.serviceName,
			},
		},
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		c.closeConn()
		return fmt.Errorf("marshal register: %w", err)
	}

	if err := c.writeFrame(data); err != nil {
		c.closeConn()
		return fmt.Errorf("write register: %w", err)
	}

	respData, err := c.readFrame()
	if err != nil {
		c.closeConn()
		return fmt.Errorf("read register response: %w", err)
	}

	resp := &pb.RegistryMessage{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		c.closeConn()
		return fmt.Errorf("unmarshal register response: %w", err)
	}

	r, ok := resp.Payload.(*pb.RegistryMessage_Response)
	if !ok || r == nil || !r.Response.Success {
		c.closeConn()
		msg := "unknown error"
		if r != nil {
			msg = r.Response.Message
		}
		return fmt.Errorf("registration failed: %s", msg)
	}

	c.logger.Info("registered", "message", r.Response.Message)
	return nil
}

func (c *Client) Deregister() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	c.closeConn()
}

func (c *Client) Run(ctx context.Context) {
	backoff := c.reconnectBackoff
	maxBackoff := c.reconnectMaxBackoff

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		if err := c.Register(ctx); err != nil {
			c.logger.Warn("register failed, retrying", "error", err, "backoff", backoff)
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			default:
			}
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}

		backoff = c.reconnectBackoff

		ctxReader, cancelReader := context.WithCancel(ctx)
		doneCh := make(chan struct{})

		go func() {
			defer close(doneCh)
			c.readLoop(ctxReader)
		}()

		c.heartbeatLoop(ctx)

		cancelReader()
		<-doneCh

		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
			c.logger.Warn("connection lost, reconnecting", "backoff", backoff)
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
			}
		}
	}
}

func (c *Client) Stop() {
	c.Deregister()
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	failCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			msg := &pb.RegistryMessage{
				Payload: &pb.RegistryMessage_Heartbeat{
					Heartbeat: &pb.HeartbeatRequest{
						Port: int32(c.selfPort),
					},
				},
			}
			data, err := proto.Marshal(msg)
			if err != nil {
				c.logger.Warn("marshal heartbeat", "error", err)
				continue
			}
			if err := c.writeFrame(data); err != nil {
				failCount++
				c.logger.Warn("heartbeat failed",
					"error", err,
					"fail_count", failCount,
					"max_fail", c.heartbeatMaxFail)

				if failCount >= c.heartbeatMaxFail {
					return
				}
			} else {
				failCount = 0
			}
		}
	}
}

func (c *Client) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		data, err := c.readFrame()
		if err != nil {
			c.logger.Debug("read loop: connection closed", "error", err)
			return
		}

		resp := &pb.RegistryMessage{}
		if err := proto.Unmarshal(data, resp); err != nil {
			c.logger.Warn("read loop: unmarshal error", "error", err)
			continue
		}

		r, ok := resp.Payload.(*pb.RegistryMessage_Response)
		if ok && r != nil {
			c.logger.Debug("registry response",
				"success", r.Response.Success,
				"message", r.Response.Message)
			if !r.Response.Success {
				c.logger.Warn("registry rejected, closing connection")
				c.closeConn()
				return
			}
		}
	}
}

func (c *Client) writeFrame(data []byte) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}
	return codec.WriteFrame(conn, data)
}

func (c *Client) readFrame() ([]byte, error) {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	return codec.ReadFrame(conn)
}

func (c *Client) closeConn() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

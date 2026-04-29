package registry

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"rpc-server/internal/codec"
	pb "rpc-server/pb"
)

type Client struct {
	registryAddr      string
	selfIP            string
	selfPort          int
	serviceName       string
	heartbeatInterval time.Duration

	conn     net.Conn
	connMu   sync.RWMutex
	stopCh   chan struct{}
	stopOnce sync.Once
}

func NewClient(registryAddr, selfIP string, selfPort int, serviceName string) *Client {
	return &Client{
		registryAddr:      registryAddr,
		selfIP:            selfIP,
		selfPort:          selfPort,
		serviceName:       serviceName,
		heartbeatInterval: 10 * time.Second,
		stopCh:            make(chan struct{}),
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
				Ip:      c.selfIP,
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

	log.Printf("[registry] registered successfully: %s", r.Response.Message)
	return nil
}

func (c *Client) Deregister() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	c.closeConn()
}

func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

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
			log.Printf("[registry] connection lost, reconnect in %v...", backoff)
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
	const maxFailBeforeReconnect = 2

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
						Ip:   c.selfIP,
						Port: int32(c.selfPort),
					},
				},
			}
			data, err := proto.Marshal(msg)
			if err != nil {
				log.Printf("[registry] marshal heartbeat: %v", err)
				continue
			}
			if err := c.writeFrame(data); err != nil {
				failCount++
				log.Printf("[registry] heartbeat failed (%d/%d): %v",
					failCount, maxFailBeforeReconnect, err)

				if failCount >= maxFailBeforeReconnect {
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
			log.Printf("[registry] read loop: connection error: %v", err)
			return
		}

		resp := &pb.RegistryMessage{}
		if err := proto.Unmarshal(data, resp); err != nil {
			log.Printf("[registry] read loop: unmarshal error: %v", err)
			continue
		}

		r, ok := resp.Payload.(*pb.RegistryMessage_Response)
		if ok && r != nil {
			log.Printf("[registry] received: success=%v, message=%s",
				r.Response.Success, r.Response.Message)
			if !r.Response.Success {
				log.Printf("[registry] registry rejected us, closing connection")
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

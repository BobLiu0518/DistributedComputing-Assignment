package registry

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/protobuf/proto"

	pb "rpc-server/pb"
	"rpc-server/internal/codec"
)

type Client struct {
	registryAddr      string
	selfIP            string
	selfPort          int
	serviceName       string
	heartbeatInterval time.Duration
	conn              net.Conn
}

func NewClient(registryAddr, selfIP string, selfPort int, serviceName string) *Client {
	return &Client{
		registryAddr:      registryAddr,
		selfIP:            selfIP,
		selfPort:          selfPort,
		serviceName:       serviceName,
		heartbeatInterval: 10 * time.Second,
	}
}

func (c *Client) Register(ctx context.Context) error {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", c.registryAddr)
	if err != nil {
		return fmt.Errorf("connect to registry: %w", err)
	}
	c.conn = conn

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
		return fmt.Errorf("marshal register: %w", err)
	}
	if err := codec.WriteFrame(conn, data); err != nil {
		return fmt.Errorf("write register: %w", err)
	}

	respData, err := codec.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("read register response: %w", err)
	}

	resp := &pb.RegistryMessage{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return fmt.Errorf("unmarshal register response: %w", err)
	}

	r, ok := resp.Payload.(*pb.RegistryMessage_Response)
	if !ok || r == nil || !r.Response.Success {
		msg := "unknown error"
		if r != nil {
			msg = r.Response.Message
		}
		return fmt.Errorf("registration failed: %s", msg)
	}

	log.Printf("[registry] registered successfully: %s", r.Response.Message)
	return nil
}

func (c *Client) HeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	failCount := 0
	const maxFailBeforeReconnect = 3

	for {
		select {
		case <-ctx.Done():
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
			if err := codec.WriteFrame(c.conn, data); err != nil {
				failCount++
				log.Printf("[registry] heartbeat failed (%d/%d): %v",
					failCount, maxFailBeforeReconnect, err)

				if failCount >= maxFailBeforeReconnect {
					log.Printf("[registry] attempting reconnect...")
					c.conn.Close()
					if regErr := c.Register(ctx); regErr != nil {
						log.Printf("[registry] reconnect failed: %v", regErr)
						return
					}
					log.Printf("[registry] reconnected successfully")
					failCount = 0
				}
			} else {
				failCount = 0
			}
		}
	}
}

func (c *Client) ReadLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		data, err := codec.ReadFrame(c.conn)
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
				c.conn.Close()
				return
			}
		}
	}
}

package registry

import (
	"context"
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
		return err
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
		return err
	}
	if err := codec.WriteFrame(conn, data); err != nil {
		return err
	}

	respData, err := codec.ReadFrame(conn)
	if err != nil {
		return err
	}

	resp := &pb.RegistryMessage{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return err
	}

	if r, ok := resp.Payload.(*pb.RegistryMessage_Response); ok && r != nil {
		log.Printf("[registry] response: success=%v message=%s",
			r.Response.Success, r.Response.Message)
	}

	return nil
}

func (c *Client) HeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

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
				log.Printf("[registry] heartbeat failed: %v", err)
				return
			}
		}
	}
}

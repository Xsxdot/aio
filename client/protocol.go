package client

import (
	"context"
	"fmt"
	"time"

	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/network"
	"github.com/xsxdot/aio/pkg/protocol"
	"go.uber.org/zap"
)

type ProtocolService struct {
	options   *ProtocolOptions
	manager   *protocol.ProtocolManager
	conn      map[string]*network.Connection
	addr2conn map[string]string
	log       *zap.Logger
	client    *Client
}

func NewProtocolService(client *Client, options *ProtocolOptions) *ProtocolService {
	p := &ProtocolService{
		options:   options,
		client:    client,
		log:       common.GetLogger().GetZapLogger("aio-protocol-client"),
		conn:      make(map[string]*network.Connection),
		addr2conn: make(map[string]string),
	}

	enableAuth := false
	if options.ClientID != "" || options.ClientSecret != "" {
		enableAuth = true
	}

	clientOptions := &protocol.ClientOptions{
		EnableAuth:        enableAuth,
		ClientID:          options.ClientID,
		ClientSecret:      options.ClientSecret,
		ReadTimeout:       options.ConnectionTimeout,
		WriteTimeout:      options.ConnectionTimeout,
		IdleTimeout:       options.ConnectionTimeout * 2,
		MaxConnections:    100,
		BufferSize:        4096,
		EnableKeepAlive:   true,
		HeartbeatInterval: 30 * time.Second,
	}

	p.manager = protocol.NewClientWithOptions(clientOptions)

	return p
}

func (s *ProtocolService) Start(ctx context.Context) error {
	for _, server := range s.options.servers {
		for i := range s.options.RetryCount {
			err := s.connectNode(server)
			if err != nil {
				s.log.Error("连接节点失败", zap.String("addr", server), zap.Error(err), zap.Int("retryCount", i+1))
				time.Sleep(s.options.RetryInterval)
			} else {
				break
			}
		}
	}

	if len(s.addr2conn) == 0 {
		return fmt.Errorf("没有可用的节点")
	}

	return nil
}

func (s *ProtocolService) connectNode(addr string) error {
	if _, ok := s.conn[addr]; ok {
		return nil
	}

	options := network.Options{
		//ReadTimeout:       s.options.ConnectionTimeout,
		WriteTimeout:      s.options.ConnectionTimeout,
		IdleTimeout:       s.options.ConnectionTimeout * 2,
		MaxConnections:    100,
		BufferSize:        4096,
		EnableKeepAlive:   true,
		HeartbeatInterval: 30 * time.Second,
		OnlyClient:        true,
	}
	connection, err := s.manager.Connect(addr, &options)
	if err != nil {
		return err
	}

	s.conn[connection.ID()] = connection
	s.addr2conn[addr] = connection.ID()

	return nil
}

func (s *ProtocolService) Stop(ctx context.Context) error {
	for _, conn := range s.conn {
		err := conn.Close()
		if err != nil {
			s.log.Error("关闭连接失败", zap.String("addr", conn.RemoteAddr().String()), zap.Error(err))
		}
	}

	return nil
}

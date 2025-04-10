package sdk_v2

import (
	"context"
	"fmt"
	"github.com/xsxdot/aio/pkg/auth"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed"
	"github.com/xsxdot/aio/pkg/network"
	"github.com/xsxdot/aio/pkg/protocol"
	"go.uber.org/zap"
	"time"
)

type ProtocolService struct {
	options   *ProtocolOptions
	manager   *protocol.ProtocolManager
	token     *auth.Token
	conn      map[string]*network.Connection
	addr2conn map[string]string
	leader    *NodeInfo
	log       *zap.Logger
}

func NewProtocolService(options *ProtocolOptions) *ProtocolService {
	p := &ProtocolService{
		options: options,
		log:     common.GetLogger().GetZapLogger("aio-protocol-client"),
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
		ReadTimeout:       s.options.ConnectionTimeout,
		WriteTimeout:      s.options.ConnectionTimeout,
		IdleTimeout:       s.options.ConnectionTimeout * 2,
		MaxConnections:    100,
		BufferSize:        4096,
		EnableKeepAlive:   true,
		HeartbeatInterval: 30 * time.Second,
	}
	var connection *network.Connection
	var err error
	var token *auth.Token
	if s.options.ClientID != "" && s.options.ClientSecret != "" {
		connection, token, err = s.manager.ConnectWithAuth(addr, &options)
		if err != nil {
			return err
		}
	} else {
		connection, err = s.manager.Connect(addr, &options)
		if err != nil {
			return err
		}
	}

	s.token = token
	s.conn[connection.ID()] = connection
	s.addr2conn[addr] = connection.ID()

	return nil
}

func (s *ProtocolService) updateLeaderInfo(d *distributed.LeaderInfo) {

}

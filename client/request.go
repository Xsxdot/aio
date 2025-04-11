package client

import (
	"github.com/xsxdot/aio/pkg/network"
	"github.com/xsxdot/aio/pkg/protocol"
)

type RequestService struct {
	client         *Client
	p              *ProtocolService
	requestManager *protocol.RequestManager
}

func NewRequestService(client *Client, manager *ProtocolService) *RequestService {
	return &RequestService{
		client:         client,
		p:              manager,
		requestManager: manager.manager.RequestManager,
	}
}

func (r *RequestService) Request(msg *protocol.CustomMessage, result interface{}) error {
	leaderConnId, err := r.client.GetLeaderConnId()
	if err == nil {
		msg.Header().ConnID = leaderConnId
		err := r.requestManager.Request(msg, result)
		if err != nil {
			if !network.IsUnavailable(err) {
				return err
			}
		} else {
			return nil
		}
	}

	for _, conn := range r.p.conn {
		msg.Header().ConnID = conn.ID()
		err := r.requestManager.Request(msg, result)
		if err != nil {
			if network.IsUnavailable(err) {
				continue
			}
			return err
		}
		return nil
	}
	return nil
}

func (r *RequestService) RequestRaw(msg *protocol.CustomMessage) ([]byte, error) {
	leaderConnId, err := r.client.GetLeaderConnId()
	if err == nil {
		msg.Header().ConnID = leaderConnId
		bytes, err := r.requestManager.RequestRaw(msg)
		if err != nil {
			if !network.IsUnavailable(err) {
				return nil, err
			}
		} else {
			return bytes, nil
		}
	}

	for _, conn := range r.p.conn {
		msg.Header().ConnID = conn.ID()
		bytes, err := r.requestManager.RequestRaw(msg)
		if err != nil {
			if network.IsUnavailable(err) {
				continue
			}
			return nil, err
		}
		return bytes, nil
	}
	return []byte{}, nil
}

func (r *RequestService) RequestIgnore(msg *protocol.CustomMessage) error {
	leaderConnId, err := r.client.GetLeaderConnId()
	if err == nil {
		msg.Header().ConnID = leaderConnId
		err := r.requestManager.RequestIgnore(msg)
		if err != nil {
			if !network.IsUnavailable(err) {
				return err
			}
		} else {
			return nil
		}
	}

	for _, conn := range r.p.conn {
		msg.Header().ConnID = conn.ID()
		err := r.requestManager.RequestIgnore(msg)
		if err != nil {
			if network.IsUnavailable(err) {
				continue
			}
			return err
		}
		return nil
	}
	return nil
}

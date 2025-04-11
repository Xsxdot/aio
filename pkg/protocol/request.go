package protocol

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/xsxdot/aio/pkg/common"
	"go.uber.org/zap"
)

type RequestManager struct {
	logger         *zap.Logger               // 日志记录器
	requestTimeout time.Duration             // 请求超时时间
	mu             sync.RWMutex              // 互斥锁
	responseChans  map[string]chan *Response // 响应通道映射
	manager        *ProtocolManager
}

func NewRequestManager(requestTimeout time.Duration, manager *ProtocolManager) *RequestManager {
	return &RequestManager{
		logger:         common.GetLogger().GetZapLogger("aio-protocol-request"),
		requestTimeout: requestTimeout,
		responseChans:  make(map[string]chan *Response),
		manager:        manager,
	}
}

// HandleResponse 处理接收到的响应
func (m *RequestManager) HandleResponse(connID string, msg *CustomMessage) (interface{}, error) {
	r := new(Response)
	r.header = msg.Header()
	err := json.Unmarshal(msg.Payload(), r)
	if err != nil {
		m.logger.Error("unmarshal response failed", zap.Error(err))
	}

	if ch, ok := m.responseChans[r.OriginMsgId]; ok {
		ch <- r
	}

	return nil, nil
}

func (m *RequestManager) Request(msg *CustomMessage, result interface{}) error {
	bytes, err := m.RequestRaw(msg)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, result)
}

func (m *RequestManager) RequestRaw(msg *CustomMessage) ([]byte, error) {
	// 创建响应通道
	respChan := make(chan *Response, 1)

	// 注册响应通道
	msgID := msg.Header().MessageID
	m.mu.Lock()
	m.responseChans[msgID] = respChan
	m.mu.Unlock()

	// 发送消息
	if err := m.manager.Send(msg); err != nil {
		m.mu.Lock()
		delete(m.responseChans, msgID)
		m.mu.Unlock()
		return nil, err
	}

	// 等待响应或超时
	select {
	case resp := <-respChan:
		// 处理响应结果
		if resp.Header().MessageType == MsgTypeResponseSuccess {
			// 成功响应，返回原始数据
			return resp.Data, nil
		} else if resp.Header().MessageType == MsgTypeResponseFail {
			// 失败响应，返回错误信息
			return nil, errors.New(string(resp.Data))
		} else {
			return nil, ErrInvalidMessageType
		}
	case <-time.After(m.requestTimeout):
		// 清理通道
		m.mu.Lock()
		delete(m.responseChans, msgID)
		m.mu.Unlock()
		return nil, errors.New("请求超时")
	}
}

func (m *RequestManager) RequestIgnore(msg *CustomMessage) error {
	// 直接发送消息，不等待响应
	return m.manager.Send(msg)
}

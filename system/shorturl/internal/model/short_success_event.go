package model

import (
	"time"
	"github.com/xsxdot/aio/pkg/core/model/common"
)

// ShortSuccessEvent 短链接成功上报事件
type ShortSuccessEvent struct {
	common.Model
	LinkID    int64       `gorm:"type:bigint;not null;index;comment:短链接ID" json:"linkId" comment:"短链接ID"`
	EventID   string      `gorm:"type:varchar(100);uniqueIndex;comment:事件ID（用于幂等）" json:"eventId" comment:"事件ID"`
	Attrs     common.JSON `gorm:"serializer:json;comment:自定义参数JSON" json:"attrs" comment:"自定义参数JSON"`
	CreatedAt time.Time   `gorm:"type:datetime;not null;index;comment:上报时间" json:"createdAt" comment:"上报时间"`
}

// TableName 设置表名
func (ShortSuccessEvent) TableName() string {
	return "shorturl_success_events"
}



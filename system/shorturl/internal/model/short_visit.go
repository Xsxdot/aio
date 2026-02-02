package model

import (
	"time"
	"xiaozhizhang/pkg/core/model/common"
)

// ShortVisit 短链接访问记录
type ShortVisit struct {
	common.Model
	LinkID    int64     `gorm:"type:bigint;not null;index;comment:短链接ID" json:"linkId" comment:"短链接ID"`
	IP        string    `gorm:"type:varchar(100);comment:访问者IP" json:"ip" comment:"访问者IP"`
	UserAgent string    `gorm:"type:varchar(500);comment:User-Agent" json:"userAgent" comment:"User-Agent"`
	Referer   string    `gorm:"type:varchar(500);comment:Referer" json:"referer" comment:"Referer"`
	VisitedAt time.Time `gorm:"type:datetime;not null;index;comment:访问时间" json:"visitedAt" comment:"访问时间"`
}

// TableName 设置表名
func (ShortVisit) TableName() string {
	return "shorturl_visits"
}


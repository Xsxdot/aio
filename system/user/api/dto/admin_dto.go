package dto

import "time"

// AdminDTO 管理员对外 DTO
type AdminDTO struct {
	ID        int64     `json:"id" comment:"管理员ID"`
	Account   string    `json:"account" comment:"管理员账号"`
	Status    int8      `json:"status" comment:"状态：1=启用，0=禁用"`
	Remark    string    `json:"remark" comment:"备注信息"`
	CreatedAt time.Time `json:"createdAt" comment:"创建时间"`
	UpdatedAt time.Time `json:"updatedAt" comment:"更新时间"`
}

// AdminDetailDTO 管理员详情 DTO
type AdminDetailDTO struct {
	AdminDTO
}

// AdminListDTO 管理员列表 DTO
type AdminListDTO struct {
	Total   int64       `json:"total" comment:"总数"`
	Content []*AdminDTO `json:"content" comment:"管理员列表"`
}




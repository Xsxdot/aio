package dto

// CreateAdminReq 创建管理员请求
type CreateAdminReq struct {
	Account  string `json:"account" validate:"required,min=3,max=50" comment:"管理员账号"`
	Password string `json:"password" validate:"required,min=6" comment:"管理员密码"`
	Remark   string `json:"remark" validate:"max=500" comment:"备注信息"`
}

// UpdateAdminPasswordReq 更新管理员密码请求
type UpdateAdminPasswordReq struct {
	AdminID     int64  `json:"adminId" validate:"required" comment:"管理员ID"`
	OldPassword string `json:"oldPassword" validate:"required" comment:"旧密码"`
	NewPassword string `json:"newPassword" validate:"required,min=6" comment:"新密码"`
}

// ResetAdminPasswordReq 重置管理员密码请求（管理员操作，无需旧密码）
type ResetAdminPasswordReq struct {
	AdminID     int64  `json:"adminId" validate:"required" comment:"管理员ID"`
	NewPassword string `json:"newPassword" validate:"required,min=6" comment:"新密码"`
}

// UpdateAdminStatusReq 更新管理员状态请求
type UpdateAdminStatusReq struct {
	AdminID int64 `json:"adminId" validate:"required" comment:"管理员ID"`
	Status  int8  `json:"status" validate:"required,oneof=0 1" comment:"状态：1=启用，0=禁用"`
}

// AdminLoginReq 管理员登录请求
type AdminLoginReq struct {
	Account  string `json:"account" validate:"required" comment:"管理员账号"`
	Password string `json:"password" validate:"required" comment:"管理员密码"`
}




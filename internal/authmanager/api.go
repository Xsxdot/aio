package authmanager

import (
	"fmt"
	"time"

	"github.com/xsxdot/aio/pkg/utils"

	"github.com/xsxdot/aio/pkg/auth"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// API 提供了authmanager的HTTP API接口
type API struct {
	manager *AuthManager
}

// NewAPI 创建一个新的authmanager API处理器
func NewAPI(manager *AuthManager) *API {
	return &API{
		manager: manager,
	}
}

// RegisterRoutes 注册API路由
func (a *API) RegisterRoutes(app fiber.Router, baseAuth func(c *fiber.Ctx) error, adminAuth func(c *fiber.Ctx) error) {
	// 创建一个auth组路由
	auth := app.Group("/auth")

	// 公共API端点
	auth.Post("/login", a.Login)
	auth.Post("/logout", a.LoginOut)
	auth.Post("/client", a.ClientAuth)
	auth.Get("/verify", a.VerifyToken)
	app.Post("/user/info", baseAuth, a.GetUserBySelf)

	// 用户管理API端点（需要认证）
	users := auth.Group("/users", baseAuth, adminAuth)
	users.Get("/", a.ListUsers)
	users.Post("/", a.CreateUser)
	users.Get("/:id", a.GetUser)
	users.Put("/:id", a.UpdateUser)
	users.Delete("/:id", a.DeleteUser)
	users.Put("/:id/password", a.UpdateUserPassword)

	// 角色管理API端点（需要认证）
	roles := auth.Group("/roles", baseAuth, adminAuth)
	roles.Get("/", a.ListRoles)
	roles.Post("/", a.CreateRole)
	roles.Get("/:id", a.GetRole)
	roles.Put("/:id", a.UpdateRole)
	roles.Delete("/:id", a.DeleteRole)

	// 客户端凭证管理API端点（需要认证）
	clients := auth.Group("/clients", baseAuth, adminAuth)
	clients.Get("/", a.ListClients)
	clients.Post("/", a.CreateClient)
	clients.Post("/service", a.CreateServiceClient)
	clients.Get("/:id", a.GetClient)
	clients.Delete("/:id", a.DeleteClient)
}

// 中间件 - 认证检查
func (a *API) AuthMiddleware(c *fiber.Ctx) error {
	// 从请求头获取令牌
	token := c.Get("Authorization")
	if token == "" {
		return utils.FailResponse(c, utils.StatusUnauthorized, "未提供认证令牌")
	}

	// 移除Bearer前缀（如果有）
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// 验证令牌
	authInfo, err := a.manager.jwtService.ValidateToken(token)
	if err != nil {
		return utils.FailResponse(c, utils.StatusUnauthorized, "无效的认证令牌")
	}

	// 将令牌信息存储在上下文中，以便后续使用
	c.Locals("authInfo", authInfo)
	return c.Next()
}

// 中间件 - 管理员角色检查
func (a *API) AdminRoleMiddleware(c *fiber.Ctx) error {
	// 从上下文获取认证信息
	authInfo, ok := c.Locals("authInfo").(*auth.AuthInfo)
	if !ok {
		return utils.FailResponse(c, utils.StatusUnauthorized, "缺少认证信息")
	}

	// 检查是否具有管理员角色
	hasAdminRole := false
	for _, role := range authInfo.Roles {
		if role == "admin" {
			hasAdminRole = true
			break
		}
	}

	if !hasAdminRole {
		return utils.FailResponse(c, utils.StatusForbidden, "需要管理员权限")
	}

	return c.Next()
}

// Login 处理用户登录请求
func (a *API) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求格式")
	}

	// 获取IP和用户代理
	ip := c.IP()
	userAgent := c.Get("User-Agent")

	// 尝试登录
	resp, err := a.manager.Login(req, ip, userAgent)
	if err != nil {
		var status int
		switch err {
		case ErrUserNotFound, ErrInvalidCredentials:
			status = utils.StatusUnauthorized
		case ErrUserDisabled, ErrUserLocked:
			status = utils.StatusForbidden
		default:
			status = utils.StatusInternalError
		}
		return utils.FailResponse(c, status, err.Error())
	}

	// 设置Session ID到Cookie
	cookie := new(fiber.Cookie)
	cookie.Name = "session_id"
	cookie.Value = resp.Token.AccessToken
	cookie.Expires = time.Now().Add(time.Hour * 24)
	cookie.HTTPOnly = true
	cookie.Secure = true
	cookie.SameSite = "Strict"
	c.Cookie(cookie)

	return utils.SuccessResponse(c, resp)
}

// ClientAuth 处理客户端认证请求
func (a *API) ClientAuth(c *fiber.Ctx) error {
	var req ClientAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求格式")
	}

	// 尝试客户端认证
	token, err := a.manager.AuthenticateClient(req)
	if err != nil {
		var status int
		switch err {
		case ErrInvalidCredentials:
			status = utils.StatusUnauthorized
		case ErrUserDisabled:
			status = utils.StatusForbidden
		default:
			status = utils.StatusInternalError
		}
		return utils.FailResponse(c, status, err.Error())
	}

	return utils.SuccessResponse(c, token)
}

// VerifyToken 验证令牌有效性和权限
func (a *API) VerifyToken(c *fiber.Ctx) error {
	token := c.Query("token")
	resource := c.Query("resource")
	action := c.Query("action")

	if token == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少令牌参数")
	}

	if resource == "" || action == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少资源或操作参数")
	}

	// 验证权限
	verifyResp, err := a.manager.VerifyPermission(token, resource, action)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, err.Error())
	}

	return utils.SuccessResponse(c, verifyResp)
}

// ListUsers 获取所有用户
func (a *API) ListUsers(c *fiber.Ctx) error {
	users, err := a.manager.ListUsers()
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取用户列表失败")
	}

	return utils.SuccessResponse(c, users)
}

// CreateUser 创建新用户
func (a *API) CreateUser(c *fiber.Ctx) error {
	type CreateUserRequest struct {
		User     User     `json:"user"`
		Password string   `json:"password"`
		Roles    []string `json:"roles"`
	}

	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求格式")
	}

	// 验证必要字段
	if req.User.Username == "" || req.Password == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "用户名和密码为必填项")
	}

	// 创建用户
	err := a.manager.CreateUser(&req.User, req.Password, req.Roles)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "创建用户失败: "+err.Error())
	}

	return utils.SuccessResponse(c, req.User)
}

// GetUser 获取特定用户
func (a *API) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少用户ID")
	}

	user, err := a.manager.GetUser(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, "用户不存在")
	}

	return utils.SuccessResponse(c, user)
}

// UpdateUser 更新用户信息
func (a *API) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少用户ID")
	}

	// 先获取现有用户
	existingUser, err := a.manager.GetUser(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, "用户不存在")
	}

	// 解析请求体中的用户数据
	var updateData User
	if err := c.BodyParser(&updateData); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求格式")
	}

	// 确保ID匹配
	updateData.ID = id

	// 只更新非零值字段
	if updateData.Username != "" {
		existingUser.Username = updateData.Username
	}
	if updateData.DisplayName != "" {
		existingUser.DisplayName = updateData.DisplayName
	}
	if updateData.Status != "" {
		existingUser.Status = updateData.Status
	}
	// 保留其他重要字段，如创建时间和最后登录时间
	// 最后登录时间不应从客户端更新，由系统在用户登录时更新

	// 更新用户
	err = a.manager.UpdateUser(existingUser)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "更新用户失败")
	}

	return utils.SuccessResponse(c, existingUser)
}

// DeleteUser 删除用户
func (a *API) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少用户ID")
	}

	// 此处需要添加删除用户的方法到AuthManager
	// 目前示例代码中未实现此方法
	err := a.manager.storage.DeleteUser(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "删除用户失败")
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "用户已删除",
	})
}

// UpdateUserPassword 更新用户密码
func (a *API) UpdateUserPassword(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少用户ID")
	}

	type PasswordUpdateRequest struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	var req PasswordUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求格式")
	}

	// 更新密码
	err := a.manager.UpdateUserPassword(id, req.OldPassword, req.NewPassword)
	if err != nil {
		var status int
		switch err {
		case ErrUserNotFound:
			status = utils.StatusNotFound
		case ErrInvalidCredentials:
			status = utils.StatusUnauthorized
		default:
			status = utils.StatusInternalError
		}
		return utils.FailResponse(c, status, err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "密码已更新",
	})
}

// ListRoles 获取所有角色
func (a *API) ListRoles(c *fiber.Ctx) error {
	roles, err := a.manager.ListRoles()
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取角色列表失败")
	}

	return utils.SuccessResponse(c, roles)
}

// CreateRole 创建新角色
func (a *API) CreateRole(c *fiber.Ctx) error {
	var role auth.Role
	if err := c.BodyParser(&role); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求格式")
	}

	// 验证必要字段
	if role.ID == "" || role.Name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "角色ID和名称为必填项")
	}

	// 如果角色ID为空，生成一个UUID
	if role.ID == "" {
		role.ID = uuid.New().String()
	}

	// 创建角色
	err := a.manager.SaveRole(&role)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "创建角色失败")
	}

	return utils.SuccessResponse(c, role)
}

// GetRole 获取特定角色
func (a *API) GetRole(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少角色ID")
	}

	role, err := a.manager.GetRole(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, "角色不存在")
	}

	return utils.SuccessResponse(c, role)
}

// UpdateRole 更新角色
func (a *API) UpdateRole(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少角色ID")
	}

	var role auth.Role
	if err := c.BodyParser(&role); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求格式")
	}

	// 确保ID匹配
	if role.ID != id {
		role.ID = id
	}

	err := a.manager.SaveRole(&role)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "更新角色失败")
	}

	return utils.SuccessResponse(c, role)
}

// DeleteRole 删除角色
func (a *API) DeleteRole(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少角色ID")
	}

	err := a.manager.DeleteRole(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "删除角色失败")
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "角色已删除",
	})
}

// ListClients 获取所有客户端
func (a *API) ListClients(c *fiber.Ctx) error {
	// 根据类型筛选客户端
	clientType := c.Query("type")

	var subjectType auth.SubjectType
	if clientType != "" {
		subjectType = auth.SubjectType(clientType)
	} else {
		// 默认获取所有服务类型的客户端
		subjectType = auth.SubjectTypeService
	}

	// 获取所有指定类型的主体
	subjects, err := a.manager.storage.ListSubjects(subjectType)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "获取客户端列表失败")
	}

	return utils.SuccessResponse(c, subjects)
}

// CreateClient 创建新客户端
func (a *API) CreateClient(c *fiber.Ctx) error {
	type CreateClientRequest struct {
		ClientID     string           `json:"client_id"`
		ClientSecret string           `json:"client_secret"`
		ClientType   auth.SubjectType `json:"client_type"`
		ClientName   string           `json:"client_name"`
		Roles        []string         `json:"roles"`
	}

	var req CreateClientRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求格式")
	}

	// 验证必要字段
	if req.ClientID == "" || req.ClientName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "客户端ID和名称为必填项")
	}

	// 验证客户端类型
	if req.ClientType == "" {
		req.ClientType = auth.SubjectTypeService // 默认为服务类型
	}

	// 创建客户端（如果密钥为空，会自动生成随机密钥）
	clientSecret, err := a.manager.CreateClient(req.ClientID, req.ClientSecret, req.ClientType, req.Roles, req.ClientName)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "创建客户端失败: "+err.Error())
	}

	// 返回客户端信息和密钥
	return utils.SuccessResponse(c, fiber.Map{
		"client_id":     req.ClientID,
		"client_secret": clientSecret,
		"client_type":   req.ClientType,
		"client_name":   req.ClientName,
		"roles":         req.Roles,
		"message":       "客户端创建成功",
	})
}

// CreateServiceClient 创建服务类型客户端（简化版）
func (a *API) CreateServiceClient(c *fiber.Ctx) error {
	type CreateServiceClientRequest struct {
		ServiceName string   `json:"service_name"`
		ServiceDesc string   `json:"service_desc"`
		Roles       []string `json:"roles"`
	}

	var req CreateServiceClientRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求格式")
	}

	// 验证必要字段
	if req.ServiceName == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务名称为必填项")
	}

	// 创建服务客户端
	clientID, clientSecret, err := a.manager.CreateServiceClient(req.ServiceName, req.Roles, req.ServiceDesc)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "创建服务客户端失败: "+err.Error())
	}

	// 返回客户端信息和密钥
	return utils.SuccessResponse(c, fiber.Map{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"client_type":   auth.SubjectTypeService,
		"client_name":   formatServiceName(req.ServiceName, req.ServiceDesc),
		"roles":         req.Roles,
		"message":       "服务客户端创建成功",
	})
}

// formatServiceName 格式化服务名称
func formatServiceName(serviceName, serviceDesc string) string {
	if serviceDesc != "" {
		return fmt.Sprintf("%s (%s)", serviceName, serviceDesc)
	}
	return serviceName
}

// GetClient 获取特定客户端
func (a *API) GetClient(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少客户端ID")
	}

	// 获取客户端主体
	subject, err := a.manager.storage.GetSubject(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusNotFound, "客户端不存在")
	}

	// 注意：出于安全考虑，不返回客户端密钥

	return utils.SuccessResponse(c, subject)
}

// DeleteClient 删除客户端
func (a *API) DeleteClient(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少客户端ID")
	}

	// 删除客户端
	err := a.manager.DeleteClient(id)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "删除客户端失败: "+err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "客户端已删除",
	})
}

func (a *API) LoginOut(ctx *fiber.Ctx) error {
	return utils.SuccessResponse(ctx, fiber.Map{
		"message": "退出成功",
	})
}

func (a *API) GetUserBySelf(ctx *fiber.Ctx) error {
	authInfo, ok := ctx.Locals("authInfo").(*auth.AuthInfo)
	if !ok {
		return utils.FailResponse(ctx, utils.StatusInternalError, "获取用户信息失败")
	}
	user, err := a.manager.GetUser(authInfo.SubjectID)
	if err != nil {
		return utils.FailResponse(ctx, utils.StatusNotFound, "用户不存在")
	}
	return utils.SuccessResponse(ctx, user)
}

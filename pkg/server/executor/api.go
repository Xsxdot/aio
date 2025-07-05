package executor

import (
	"github.com/gofiber/fiber/v2"
	"github.com/xsxdot/aio/pkg/server"
	"github.com/xsxdot/aio/pkg/utils"
	"go.uber.org/zap"
)

// API Git克隆API处理器
type API struct {
	executor server.Executor
	logger   *zap.Logger
}

// NewAPI 创建Git克隆API
func NewAPI(executor server.Executor, logger *zap.Logger) *API {
	return &API{
		executor: executor,
		logger:   logger,
	}
}

// RegisterRoutes 注册路由
func (api *API) RegisterRoutes(app *fiber.App) {
	git := app.Group("/api/v1/git")

	// Git仓库克隆
	git.Post("/clone", api.CloneRepository)

	// 获取克隆状态
	git.Get("/clone/:requestId", api.GetCloneStatus)
}

// CloneRepository 克隆Git仓库
func (api *API) CloneRepository(c *fiber.Ctx) error {
	var req server.GitCloneRequest
	if err := c.BodyParser(&req); err != nil {
		api.logger.Error("解析Git克隆请求失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的请求参数")
	}

	// 验证必填字段
	if req.ServerID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "服务器ID不能为空")
	}
	if req.RepoURL == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "仓库URL不能为空")
	}
	if req.TargetDir == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "目标目录不能为空")
	}

	api.logger.Info("开始克隆Git仓库",
		zap.String("serverID", req.ServerID),
		zap.String("repoURL", req.RepoURL),
		zap.String("targetDir", req.TargetDir))

	// 执行Git克隆
	result, err := api.executor.CloneGitRepository(c.Context(), &req)
	if err != nil {
		api.logger.Error("Git仓库克隆失败",
			zap.String("serverID", req.ServerID),
			zap.String("repoURL", req.RepoURL),
			zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, "Git仓库克隆失败: "+err.Error())
	}

	api.logger.Info("Git仓库克隆完成",
		zap.String("requestID", result.RequestID),
		zap.String("status", string(result.Status)),
		zap.Duration("duration", result.Duration))

	return utils.SuccessResponse(c, result)
}

// GetCloneStatus 获取克隆状态
func (api *API) GetCloneStatus(c *fiber.Ctx) error {
	requestID := c.Params("requestId")
	if requestID == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求ID不能为空")
	}

	// 由于这是同步操作，直接返回不支持
	return utils.FailResponse(c, utils.StatusBadRequest, "Git克隆是同步操作，请直接查看克隆结果")
}

// CloneRepositoryResponse Git克隆响应
type CloneRepositoryResponse struct {
	RequestID string                 `json:"request_id"`
	Status    server.CommandStatus   `json:"status"`
	Result    *server.GitCloneResult `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

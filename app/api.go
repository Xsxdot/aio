package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	configInternal "github.com/xsxdot/aio/pkg/config"
	"github.com/xsxdot/aio/pkg/utils"

	"github.com/gofiber/fiber/v2"
	consts "github.com/xsxdot/aio/app/const"
)

// APIHandler 表示API处理器
type APIHandler struct {
	app *App
}

func NewAPIHandler(app *App) *APIHandler {
	return &APIHandler{
		app: app,
	}
}

// SetupRoutes 设置所有API路由
func (h *APIHandler) SetupRoutes(router fiber.Router, authHandler func(*fiber.Ctx) error, adminRoleHandler func(*fiber.Ctx) error) {
	// 创建API组
	apiGroup := router.Group("/system", authHandler, adminRoleHandler)

	// 组件管理相关路由
	apiGroup.Get("/components", h.getAllComponentsStatus)
	apiGroup.Post("/components/:name/start", h.startComponent)
	apiGroup.Post("/components/:name/stop", h.stopComponent)
	apiGroup.Post("/components/:name/restart", h.restartComponent)
	apiGroup.Get("/components/:name/default-config", h.getComponentDefaultConfig)

	// 应用状态相关路由
	apiGroup.Post("/restart", h.restartApp)

	// 配置管理相关路由
	apiGroup.Post("/config/update", h.updateConfig)
	apiGroup.Get("/config", h.getConfig)
}

// componentStatusToString 将组件状态转换为字符串
func componentStatusToString(status consts.ComponentStatus) string {
	switch status {
	case consts.StatusNotInitialized:
		return "未初始化"
	case consts.StatusInitialized:
		return "已初始化"
	case consts.StatusRunning:
		return "运行中"
	case consts.StatusStopped:
		return "已停止"
	case consts.StatusError:
		return "错误"
	default:
		return "未知状态: " + strconv.Itoa(int(status))
	}
}

type ComponentInfo struct {
	Name          string        `json:"name"`
	Status        string        `json:"status"`
	ComponentType ComponentType `json:"component_type"`
	Enable        bool          `json:"enable"`
}

type ConfigInfo struct {
	Name   string        `json:"name"`
	Data   interface{}   `json:"data"`
	Enable bool          `json:"enable"`
	Type   ComponentType `json:"type"`
}

// getAllComponentsStatus 获取所有组件的状态
func (h *APIHandler) getAllComponentsStatus(c *fiber.Ctx) error {
	return utils.SuccessResponse(c, h.app.Manager.ListComponents())
}

// startComponent 启动指定组件
func (h *APIHandler) startComponent(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "组件名称不能为空")
	}

	component := h.app.Manager.Get(name)
	if component == nil {
		return utils.FailResponse(c, utils.StatusNotFound, "找不到指定的组件")
	}

	status := component.Status()
	if status == consts.StatusRunning {
		return utils.FailResponse(c, utils.StatusBadRequest, "组件已经在运行中")
	}

	ctx := context.Background()
	err := h.app.Manager.Start(ctx, name)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "启动组件失败: "+err.Error())
	}

	// 获取更新后的状态
	updatedStatus := component.Status()

	return utils.SuccessResponse(c, map[string]interface{}{
		"name":   name,
		"status": componentStatusToString(updatedStatus),
	})
}

// stopComponent 停止指定组件
func (h *APIHandler) stopComponent(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "组件名称不能为空")
	}

	component := h.app.Manager.Get(name)
	if component == nil {
		return utils.FailResponse(c, utils.StatusNotFound, "找不到指定的组件")
	}

	status := component.Status()
	if status != consts.StatusRunning {
		return utils.FailResponse(c, utils.StatusBadRequest, "组件当前未在运行")
	}

	ctx := context.Background()
	err := h.app.Manager.Stop(ctx, name)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "停止组件失败: "+err.Error())
	}

	// 获取更新后的状态
	updatedStatus := component.Status()

	return utils.SuccessResponse(c, map[string]interface{}{
		"name":   name,
		"status": componentStatusToString(updatedStatus),
	})
}

// restartComponent 重启指定组件
func (h *APIHandler) restartComponent(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "组件名称不能为空")
	}

	component := h.app.Manager.Get(name)
	if component == nil {
		return utils.FailResponse(c, utils.StatusNotFound, "找不到指定的组件")
	}

	ctx := context.Background()

	err := h.app.Manager.Restart(ctx, name)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "重启组件失败: "+err.Error())
	}

	// 获取更新后的状态
	updatedStatus := component.Status()

	return utils.SuccessResponse(c, map[string]interface{}{
		"name":   name,
		"status": componentStatusToString(updatedStatus),
	})
}

// restartApp 重启应用
func (h *APIHandler) restartApp(c *fiber.Ctx) error {
	// 先返回成功响应
	err := utils.SuccessResponse(c, map[string]string{
		"message": "应用重启指令已发送，应用将在2秒后重启",
	})

	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "发送重启响应失败: "+err.Error())
	}

	// 使用goroutine异步执行重启操作
	go func() {
		// 延迟2秒执行重启，确保响应能够完成发送
		time.Sleep(2 * time.Second)
		if restartErr := h.app.Restart(); restartErr != nil {
			fmt.Printf("重启应用失败: %s\n", restartErr.Error())
		}
	}()

	return nil
}

// updateConfig 更新配置
func (h *APIHandler) updateConfig(c *fiber.Ctx) error {
	// 解析请求体
	var request ConfigInfo

	if err := c.BodyParser(&request); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "请求格式错误: "+err.Error())
	}

	component := h.app.Manager.Get(request.Name)
	if component == nil {
		return utils.FailResponse(c, utils.StatusNotFound, "找不到指定的组件")
	}
	if component.Type != TypeNormal {
		return utils.FailResponse(c, utils.StatusBadRequest, "该组件无法热更新配置")
	}

	jsonData, err := json.Marshal(request.Data)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, "配置数据格式错误: "+err.Error())
	}

	cfg := new(configInternal.ConfigItem)
	err = json.Unmarshal(jsonData, cfg)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "解析初始化保存的配置失败: "+err.Error())
	}
	err = h.app.ConfigService.Set(context.Background(), component.Name(), cfg.Value, cfg.Metadata)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "保存配置到配置中心失败: "+err.Error())
	}
	if component.Type == TypeNormal {
		oldEnable := component.Enable
		enable, ok := cfg.Metadata["enable"]
		if ok {
			component.Enable = enable == "true"
		}
		if oldEnable != component.Enable {
			if component.Enable {
				err := h.app.Manager.Start(context.Background(), component.Name())
				if err != nil {
					return utils.FailResponse(c, utils.StatusInternalError, "启动组件失败: "+err.Error())
				}
			} else {
				err := h.app.Manager.Stop(context.Background(), component.Name())
				if err != nil {
					return utils.FailResponse(c, utils.StatusInternalError, "停止组件失败: "+err.Error())
				}
			}
		}
	}

	return utils.SuccessResponse(c, map[string]string{
		"message": "配置更新成功",
	})
}

// getConfig 获取配置文件内容
// 此API仅在系统初始化完成之前可用
func (h *APIHandler) getConfig(c *fiber.Ctx) error {
	// 从查询参数获取要查询的配置类型
	fileType := c.Query("type", "aio") // 默认为aio配置

	// 验证文件类型
	if fileType != "aio" && fileType != "etcd" {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的配置文件类型，必须是 'aio' 或 'etcd'")
	}

	// 构建文件路径
	fileName := fileType + ".yaml"
	configPath := filepath.Join(h.app.configDirPath, fileName)

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("%s 配置文件不存在", fileType))
	}

	// 读取配置文件内容
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("读取配置文件失败: %s", err.Error()))
	}

	// 返回原始内容
	return utils.SuccessResponse(c, map[string]interface{}{
		"file_type": fileType,
		"config":    string(content), // 直接返回原始YAML内容
	})
}

// getComponentDefaultConfig 获取指定组件的默认配置
func (h *APIHandler) getComponentDefaultConfig(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "组件名称不能为空")
	}

	entity := h.app.Manager.Get(name)
	if entity == nil {
		return utils.FailResponse(c, utils.StatusNotFound, "找不到指定的组件")
	}

	data := h.app.Manager.GetConfigData(entity)

	m := make(map[string]interface{})
	err := json.Unmarshal(data, &m)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, "解析组件配置数据失败: "+err.Error())
	}

	return utils.SuccessResponse(c, &ConfigInfo{
		Name:   entity.Name(),
		Data:   m,
		Enable: entity.Enable,
		Type:   entity.Type,
	})
}

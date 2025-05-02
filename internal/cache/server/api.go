// Package server 提供缓存服务器HTTP API实现
package server

import (
	"fmt"
	"strconv"
	"strings"

	cache2 "github.com/xsxdot/aio/internal/cache"
	"github.com/xsxdot/aio/internal/cache/protocol"
	"github.com/xsxdot/aio/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// API 表示缓存服务器的HTTP API
type API struct {
	server *Server
	logger *zap.Logger
}

// NewAPI 创建一个新的缓存服务器API
func NewAPI(server *Server, logger *zap.Logger) *API {
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			panic(fmt.Sprintf("创建日志记录器失败: %v", err))
		}
	}

	return &API{
		server: server,
		logger: logger,
	}
}

// RegisterRoutes 注册API路由
func (a *API) RegisterRoutes(router fiber.Router, authHandler func(*fiber.Ctx) error, adminRoleHandler func(*fiber.Ctx) error) {
	// 创建缓存服务API组
	api := router.Group("/cache")

	// 服务器状态和管理
	api.Get("/status", authHandler, a.GetStatus)
	api.Get("/cluster/info", authHandler, a.GetClusterInfo)
	api.Post("/server/start", authHandler, adminRoleHandler, a.StartServer)
	api.Post("/server/stop", authHandler, adminRoleHandler, a.StopServer)
	api.Post("/server/restart", authHandler, adminRoleHandler, a.RestartServer)

	// 键值操作
	api.Get("/keys", authHandler, a.GetKeys)
	api.Get("/key/:key", authHandler, a.GetValue)
	api.Put("/key/:key", authHandler, adminRoleHandler, a.SetValue)
	api.Delete("/key/:key", authHandler, adminRoleHandler, a.DeleteValue)

	// 高级命令
	api.Post("/command", authHandler, adminRoleHandler, a.ExecCommand)
	api.Post("/flushall", authHandler, adminRoleHandler, a.FlushAll)
	api.Post("/flushdb", authHandler, adminRoleHandler, a.FlushDB)

	// 数据库操作
	api.Post("/select/:db", authHandler, adminRoleHandler, a.SelectDB)

	a.logger.Info("已注册缓存服务器API路由")
}

// GetStatus 获取服务器状态
func (a *API) GetStatus(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	stats := a.server.GetStats()
	return utils.SuccessResponse(c, stats)
}

// GetClusterInfo 获取集群信息
func (a *API) GetClusterInfo(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 通过INFO命令获取集群信息
	reply := a.server.execInfo([]string{})
	info := reply.String()

	// 解析INFO命令返回的结果
	sections := make(map[string]map[string]string)
	currentSection := ""

	for _, line := range strings.Split(info, "\r\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "#") {
			// 新的部分开始
			currentSection = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			sections[currentSection] = make(map[string]string)
		} else if strings.Contains(line, ":") {
			// 键值对
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && currentSection != "" {
				sections[currentSection][parts[0]] = parts[1]
			}
		}
	}

	// 构建响应
	resp := map[string]interface{}{
		"server": sections["Server"],
	}

	// 如果有复制信息，添加到响应
	if _, ok := sections["Replication"]; ok {
		resp["replication"] = sections["Replication"]
	}

	// 添加内存信息
	if _, ok := sections["Memory"]; ok {
		resp["memory"] = sections["Memory"]
	}

	// 添加键空间信息
	if _, ok := sections["Keyspace"]; ok {
		resp["keyspace"] = sections["Keyspace"]
	}

	return utils.SuccessResponse(c, resp)
}

// StartServer 启动服务器
func (a *API) StartServer(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 解析配置
	var req struct {
		Config *cache2.Config `json:"config"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的请求体: %v", err))
	}

	var err error
	// 如果提供了配置，使用提供的配置启动
	if req.Config != nil {
		err = a.server.StartWithConfig()
	} else {
		// 否则使用默认配置启动
		err = a.server.Start(nil)
	}

	if err != nil {
		a.logger.Error("启动服务器失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("启动服务器失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "服务器已成功启动",
		"port":    a.server.GetPort(),
	})
}

// StopServer 停止服务器
func (a *API) StopServer(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	if err := a.server.Stop(nil); err != nil {
		a.logger.Error("停止服务器失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("停止服务器失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "服务器已成功停止",
	})
}

// RestartServer 重启服务器
func (a *API) RestartServer(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 解析配置
	var req struct {
		Config *cache2.Config `json:"config"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的请求体: %v", err))
	}

	// 先停止服务器
	if err := a.server.Stop(nil); err != nil {
		a.logger.Error("停止服务器失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("重启时停止服务器失败: %v", err))
	}

	var err error
	// 如果提供了配置，使用提供的配置启动
	if req.Config != nil {
		err = a.server.StartWithConfig()
	} else {
		// 否则使用默认配置启动
		err = a.server.Start(nil)
	}

	if err != nil {
		a.logger.Error("启动服务器失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("重启时启动服务器失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "服务器已成功重启",
		"port":    a.server.GetPort(),
	})
}

// GetKeys 获取键列表
func (a *API) GetKeys(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 解析查询参数
	pattern := c.Query("pattern", "*")
	limitStr := c.Query("limit", "100")
	dbStr := c.Query("db", "0")

	// 转换参数
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的limit参数，应为正整数")
	}

	// 限制最大查询数量
	if limit > 1000 {
		limit = 1000
	}

	db, err := strconv.Atoi(dbStr)
	if err != nil || db < 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的db参数，应为非负整数")
	}

	// 创建KEYS命令
	keysCmd := protocol.NewRESPCommand("KEYS", []string{pattern}, "http-api")
	keysCmd.SetDbIndex(db)

	// 执行KEYS命令
	reply, err := a.server.Handle(c.Context(), keysCmd)
	if err != nil {
		a.logger.Error("执行KEYS命令失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取键列表失败: %v", err))
	}

	if reply.Type() == cache2.ReplyError {
		a.logger.Error("执行KEYS命令失败", zap.String("error", reply.String()))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取键列表失败: %s", reply.String()))
	}

	// 处理结果
	keys := make([]string, 0)
	if reply.Type() == cache2.ReplyMultiBulk {
		// 解析多行内容为字符串数组
		content := reply.String()
		if content != "" {
			// 解析格式: ${length}:${content}|${length}:${content}|...
			keysMap := make(map[string]struct{})
			parts := strings.Split(content, "|")

			for _, part := range parts {
				if len(part) == 0 {
					continue
				}

				colonPos := strings.Index(part, ":")
				if colonPos == -1 {
					continue
				}

				lengthStr := part[:colonPos]
				length, err := strconv.Atoi(lengthStr)
				if err != nil || length < 0 {
					continue
				}

				// 提取键内容
				if colonPos+1+length <= len(part) {
					key := part[colonPos+1 : colonPos+1+length]
					keysMap[key] = struct{}{}
				}
			}

			// 从map重新构建去重后的键列表
			keys = make([]string, 0, len(keysMap))
			for key := range keysMap {
				keys = append(keys, key)
			}

			// 如果超过限制，只返回限制数量的键
			if len(keys) > limit {
				keys = keys[:limit]
			}
		}
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"keys":  keys,
		"count": len(keys),
		"db":    db,
	})
}

// GetValue 获取键的值
func (a *API) GetValue(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 获取键名
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "键名不能为空")
	}

	// 获取数据库参数
	dbStr := c.Query("db", "0")
	db, err := strconv.Atoi(dbStr)
	if err != nil || db < 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "无效的db参数，应为非负整数")
	}

	// 先获取键的类型
	typeCmd := protocol.NewRESPCommand("TYPE", []string{key}, "http-api")
	typeCmd.SetDbIndex(db)
	typeReply, err := a.server.Handle(c.Context(), typeCmd)
	if err != nil {
		a.logger.Error("获取键类型失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取键类型失败: %v", err))
	}

	if typeReply.Type() == cache2.ReplyError {
		a.logger.Error("获取键类型失败", zap.String("error", typeReply.String()))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取键类型失败: %s", typeReply.String()))
	}

	keyType := typeReply.String()
	if keyType == "none" {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("键 '%s' 不存在", key))
	}

	// 根据键类型执行不同的命令
	var value interface{}
	var valueCmd *protocol.RESPCommand

	switch keyType {
	case "string":
		valueCmd = protocol.NewRESPCommand("GET", []string{key}, "http-api")
		valueCmd.SetDbIndex(db)
		reply, err := a.server.Handle(c.Context(), valueCmd)
		if err != nil {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取字符串值失败: %v", err))
		}
		if reply.Type() == cache2.ReplyError {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取字符串值失败: %s", reply.String()))
		}
		value = reply.String()

	case "list":
		valueCmd = protocol.NewRESPCommand("LRANGE", []string{key, "0", "99"}, "http-api") // 限制返回100个元素
		valueCmd.SetDbIndex(db)
		reply, err := a.server.Handle(c.Context(), valueCmd)
		if err != nil {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取列表值失败: %v", err))
		}
		if reply.Type() == cache2.ReplyError {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取列表值失败: %s", reply.String()))
		}
		if reply.Type() == cache2.ReplyMultiBulk {
			// 将换行分隔的内容转换为字符串列表
			content := reply.String()
			if content != "" {
				value = strings.Split(content, "\n")
			} else {
				value = []string{}
			}
		}

	case "set":
		valueCmd = protocol.NewRESPCommand("SMEMBERS", []string{key}, "http-api")
		valueCmd.SetDbIndex(db)
		reply, err := a.server.Handle(c.Context(), valueCmd)
		if err != nil {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取集合值失败: %v", err))
		}
		if reply.Type() == cache2.ReplyError {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取集合值失败: %s", reply.String()))
		}
		if reply.Type() == cache2.ReplyMultiBulk {
			// 将换行分隔的内容转换为字符串列表
			content := reply.String()
			if content != "" {
				value = strings.Split(content, "\n")
			} else {
				value = []string{}
			}
		}

	case "hash":
		valueCmd = protocol.NewRESPCommand("HGETALL", []string{key}, "http-api")
		valueCmd.SetDbIndex(db)
		reply, err := a.server.Handle(c.Context(), valueCmd)
		if err != nil {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取哈希值失败: %v", err))
		}
		if reply.Type() == cache2.ReplyError {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取哈希值失败: %s", reply.String()))
		}
		if reply.Type() == cache2.ReplyMultiBulk {
			// 将换行分隔的内容转换为哈希表
			content := reply.String()
			items := strings.Split(content, "\n")
			hash := make(map[string]string)
			for i := 0; i < len(items)-1; i += 2 {
				if i+1 < len(items) {
					hash[items[i]] = items[i+1]
				}
			}
			value = hash
		}

	case "zset":
		valueCmd = protocol.NewRESPCommand("ZRANGE", []string{key, "0", "99", "WITHSCORES"}, "http-api") // 限制返回100个元素
		valueCmd.SetDbIndex(db)
		reply, err := a.server.Handle(c.Context(), valueCmd)
		if err != nil {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取有序集合值失败: %v", err))
		}
		if reply.Type() == cache2.ReplyError {
			return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取有序集合值失败: %s", reply.String()))
		}
		if reply.Type() == cache2.ReplyMultiBulk {
			// 将换行分隔的内容转换为有序集合
			content := reply.String()
			items := strings.Split(content, "\n")
			zset := make(map[string]string)
			for i := 0; i < len(items)-1; i += 2 {
				if i+1 < len(items) {
					zset[items[i]] = items[i+1]
				}
			}
			value = zset
		}

	default:
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("不支持的键类型: %s", keyType))
	}

	// 获取键的过期时间
	ttlCmd := protocol.NewRESPCommand("TTL", []string{key}, "http-api")
	ttlCmd.SetDbIndex(db)
	ttlReply, err := a.server.Handle(c.Context(), ttlCmd)
	if err != nil {
		a.logger.Warn("获取键过期时间失败", zap.Error(err))
	}

	var ttl int64 = -1
	if ttlReply != nil && ttlReply.Type() == cache2.ReplyInteger {
		// 尝试将ttlReply.String()转换为int64
		ttl, _ = strconv.ParseInt(ttlReply.String(), 10, 64)
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"key":   key,
		"type":  keyType,
		"value": value,
		"ttl":   ttl,
		"db":    db,
	})
}

// SetValue 设置键值
func (a *API) SetValue(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 获取键名
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少键名")
	}

	// 解析请求体
	var req struct {
		Value string `json:"value"`
		EX    int    `json:"ex"`
		PX    int    `json:"px"`
		NX    bool   `json:"nx"`
		XX    bool   `json:"xx"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的请求体: %v", err))
	}

	if req.Value == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "值不能为空")
	}

	// 获取要使用的数据库索引
	dbIndex := 0
	if dbStr := c.Query("db"); dbStr != "" {
		var err error
		dbIndex, err = strconv.Atoi(dbStr)
		if err != nil {
			return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的数据库索引: %v", err))
		}
	}

	// 构建SET命令的参数
	args := []string{key, req.Value}

	// 添加过期时间参数
	if req.EX > 0 {
		args = append(args, "EX", strconv.Itoa(req.EX))
	} else if req.PX > 0 {
		args = append(args, "PX", strconv.Itoa(req.PX))
	}

	// 添加条件参数
	if req.NX {
		args = append(args, "NX")
	} else if req.XX {
		args = append(args, "XX")
	}

	// 创建SET命令
	cmd := protocol.NewRESPCommand("SET", args, "http-api")
	cmd.SetDbIndex(dbIndex)

	// 处理命令
	reply, err := a.server.Handle(c.Context(), cmd)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("处理SET命令失败: %v", err))
	}

	// 解析回复
	replyStr := reply.String()

	// 根据回复类型返回不同的响应
	if replyType := reply.Type(); replyType == cache2.ReplyStatus {
		if replyStr == "OK" {
			return utils.SuccessResponse(c, map[string]interface{}{
				"key":    key,
				"value":  req.Value,
				"db":     dbIndex,
				"result": "OK",
			})
		}
	} else if replyType == cache2.ReplyNil {
		// 条件不满足（NX/XX）
		return utils.FailResponse(c, fiber.StatusConflict, "条件不满足，键值未设置")
	} else if replyType == cache2.ReplyError {
		// 错误回复
		return utils.FailResponse(c, utils.StatusInternalError, replyStr)
	}

	// 其他类型的回复
	return utils.SuccessResponse(c, map[string]interface{}{
		"key":    key,
		"value":  req.Value,
		"db":     dbIndex,
		"result": replyStr,
	})
}

// DeleteValue 删除键
func (a *API) DeleteValue(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 获取键名
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "缺少键名")
	}

	// 获取要使用的数据库索引
	dbIndex := 0
	if dbStr := c.Query("db"); dbStr != "" {
		var err error
		dbIndex, err = strconv.Atoi(dbStr)
		if err != nil {
			return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的数据库索引: %v", err))
		}
	}

	// 创建DEL命令
	cmd := protocol.NewRESPCommand("DEL", []string{key}, "http-api")
	cmd.SetDbIndex(dbIndex)

	// 处理命令
	reply, err := a.server.Handle(c.Context(), cmd)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("处理DEL命令失败: %v", err))
	}

	// 解析回复
	replyStr := reply.String()
	count, _ := strconv.Atoi(replyStr)

	// 根据回复类型返回不同的响应
	if count > 0 {
		return utils.SuccessResponse(c, map[string]interface{}{
			"key":     key,
			"deleted": count,
			"db":      dbIndex,
		})
	} else {
		return utils.FailResponse(c, fiber.StatusNotFound, "键不存在")
	}
}

// ExecCommand 执行任意缓存命令
func (a *API) ExecCommand(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 解析请求体
	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
		DB      int      `json:"db"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的请求体: %v", err))
	}

	if req.Command == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "命令不能为空")
	}

	// 创建命令
	cmd := protocol.NewRESPCommand(req.Command, req.Args, "http-api")
	cmd.SetDbIndex(req.DB)

	// 处理命令
	reply, err := a.server.Handle(c.Context(), cmd)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("处理命令失败: %v", err))
	}

	// 解析回复
	replyStr := reply.String()
	replyType := reply.Type()

	// 将回复类型转换为字符串
	typeStr := "unknown"
	switch replyType {
	case cache2.ReplyStatus:
		typeStr = "status"
	case cache2.ReplyError:
		typeStr = "error"
	case cache2.ReplyInteger:
		typeStr = "integer"
	case cache2.ReplyBulk:
		typeStr = "bulk"
	case cache2.ReplyMultiBulk:
		typeStr = "multibulk"
	case cache2.ReplyNil:
		typeStr = "nil"
	}

	// 构建响应
	resp := map[string]interface{}{
		"command": req.Command,
		"args":    req.Args,
		"db":      req.DB,
		"result":  replyStr,
		"type":    typeStr,
	}

	// 如果是错误回复，返回错误响应
	if replyType == cache2.ReplyError {
		return utils.FailResponse(c, fiber.StatusBadRequest, replyStr)
	}

	return utils.SuccessResponse(c, resp)
}

// FlushAll 清空所有数据库
func (a *API) FlushAll(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 创建FLUSHALL命令
	cmd := protocol.NewRESPCommand("FLUSHALL", []string{}, "http-api")

	// 处理命令
	reply, err := a.server.Handle(c.Context(), cmd)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("处理FLUSHALL命令失败: %v", err))
	}

	// 解析回复
	replyStr := reply.String()

	// 根据回复类型返回不同的响应
	if replyType := reply.Type(); replyType == cache2.ReplyStatus {
		if replyStr == "OK" {
			return utils.SuccessResponse(c, map[string]interface{}{
				"message": "所有数据库已成功清空",
				"result":  "OK",
			})
		}
	} else if replyType == cache2.ReplyError {
		// 错误回复
		return utils.FailResponse(c, utils.StatusInternalError, replyStr)
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"result": replyStr,
	})
}

// FlushDB 清空当前数据库
func (a *API) FlushDB(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 获取要使用的数据库索引
	dbIndex := 0
	if dbStr := c.Query("db"); dbStr != "" {
		var err error
		dbIndex, err = strconv.Atoi(dbStr)
		if err != nil {
			return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的数据库索引: %v", err))
		}
	}

	// 创建FLUSHDB命令
	cmd := protocol.NewRESPCommand("FLUSHDB", []string{}, "http-api")
	cmd.SetDbIndex(dbIndex)

	// 处理命令
	reply, err := a.server.Handle(c.Context(), cmd)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("处理FLUSHDB命令失败: %v", err))
	}

	// 解析回复
	replyStr := reply.String()

	// 根据回复类型返回不同的响应
	if replyType := reply.Type(); replyType == cache2.ReplyStatus {
		if replyStr == "OK" {
			return utils.SuccessResponse(c, map[string]interface{}{
				"message": fmt.Sprintf("数据库 %d 已成功清空", dbIndex),
				"db":      dbIndex,
				"result":  "OK",
			})
		}
	} else if replyType == cache2.ReplyError {
		// 错误回复
		return utils.FailResponse(c, utils.StatusInternalError, replyStr)
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"db":     dbIndex,
		"result": replyStr,
	})
}

// SelectDB 选择数据库
func (a *API) SelectDB(c *fiber.Ctx) error {
	if a.server == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "缓存服务器未初始化")
	}

	// 获取数据库索引
	dbStr := c.Params("db")
	dbIndex, err := strconv.Atoi(dbStr)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的数据库索引: %v", err))
	}

	// 创建SELECT命令
	cmd := protocol.NewRESPCommand("SELECT", []string{dbStr}, "http-api")

	// 处理命令
	reply, err := a.server.Handle(c.Context(), cmd)
	if err != nil {
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("处理SELECT命令失败: %v", err))
	}

	// 解析回复
	replyStr := reply.String()

	// 根据回复类型返回不同的响应
	if replyType := reply.Type(); replyType == cache2.ReplyStatus {
		if replyStr == "OK" {
			return utils.SuccessResponse(c, map[string]interface{}{
				"message": fmt.Sprintf("已选择数据库 %d", dbIndex),
				"db":      dbIndex,
				"result":  "OK",
			})
		}
	} else if replyType == cache2.ReplyError {
		// 错误回复
		return utils.FailResponse(c, utils.StatusInternalError, replyStr)
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"db":     dbIndex,
		"result": replyStr,
	})
}

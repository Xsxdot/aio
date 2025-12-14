// system/bom/controller/bom_controller.go

func (c *BomController) RegisterRoutes(api fiber.Router) {
	// 创建缓存配置
	cacheConfig := middleware.SmartCacheConfig{
		TTL: 10 * time.Minute, // 缓存10分钟
		// 定义哪些路径需要缓存
		CachePatterns: []string{
			"/boms/*",     // 所有 bom 相关的 GET 请求
			"/products/*", // 所有 product 相关的 GET 请求
		},
		// 定义失效规则：当左边的路径被修改时，清理右边的缓存
		InvalidationRules: map[string][]string{
			"/boms/:id": {
				"/boms/:id",  // 精确清理：清理这个具体的 bom 缓存
				"/boms*",     // 模糊清理：清理所有 bom 列表缓存
			},
			"/boms": {
				"/boms*",     // 新增 bom 时，清理所有 bom 相关缓存
			},
		},
	}

	bomGroup := api.Group("/boms")
	
	// 只需要添加这一行中间件！
	bomGroup.Use(middleware.SmartCache(c.cacheManager, cacheConfig))

	// 你的原有路由定义完全不变
	bomGroup.Get("/:id", c.GetByID)
	bomGroup.Get("/", c.GetAll)
	bomGroup.Put("/:id", c.Update)
	bomGroup.Post("/", c.Create)
	bomGroup.Delete("/:id", c.Delete)
}


// app/fiber.go 或 main.go

func setupCache(app *fiber.App, cacheManager cache.ICacheManager) {
	// 全局缓存配置
	globalCacheConfig := middleware.SmartCacheConfig{
		TTL: 10 * time.Minute,
		CachePatterns: []string{
			"/api/boms/*",
			"/api/products/*",
			"/api/missions/*",
			// 添加更多需要缓存的路径
		},
		InvalidationRules: map[string][]string{
			// BOM 相关
			"/api/boms/:id": {"/api/boms/:id", "/api/boms*"},
			"/api/boms":     {"/api/boms*"},
			
			// Product 相关
			"/api/products/:id": {"/api/products/:id", "/api/products*"},
			"/api/products":     {"/api/products*"},
			
			// Mission 相关
			"/api/missions/:id": {"/api/missions/:id", "/api/missions*"},
			"/api/missions":     {"/api/missions*"},
		},
	}

	// 对整个 /api 路径应用缓存
	api := app.Group("/api")
	api.Use(middleware.SmartCache(cacheManager, globalCacheConfig))
}
package executor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/xsxdot/aio/pkg/server"
	"go.uber.org/zap"
)

// SQLiteStorage SQLite存储实现
type SQLiteStorage struct {
	db     *sql.DB
	logger *zap.Logger
}

// SQLiteConfig SQLite配置
type SQLiteConfig struct {
	DatabasePath string      // 数据库文件路径
	Logger       *zap.Logger // 日志器
}

// NewSQLiteStorage 创建SQLite存储
func NewSQLiteStorage(config SQLiteConfig) (server.ExecutorStorage, error) {
	if config.Logger == nil {
		config.Logger, _ = zap.NewProduction()
	}

	// 如果没有指定路径，使用默认路径
	if config.DatabasePath == "" {
		config.DatabasePath = "./data/executor.db"
	}

	// 确保目录存在
	dir := filepath.Dir(config.DatabasePath)
	if err := ensureDir(dir); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite3", config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("打开SQLite数据库失败: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	storage := &SQLiteStorage{
		db:     db,
		logger: config.Logger,
	}

	// 初始化数据库表
	if err := storage.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库表失败: %w", err)
	}

	return storage, nil
}

// initTables 初始化数据库表
func (s *SQLiteStorage) initTables() error {
	// 创建执行结果表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS execute_results (
		request_id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		server_id TEXT NOT NULL,
		async INTEGER NOT NULL DEFAULT 0,
		start_time INTEGER NOT NULL,
		end_time INTEGER,
		command_result TEXT,
		batch_result TEXT,
		created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
		updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
	);

	-- 创建索引
	CREATE INDEX IF NOT EXISTS idx_execute_results_server_id ON execute_results(server_id);
	CREATE INDEX IF NOT EXISTS idx_execute_results_start_time ON execute_results(start_time);
	CREATE INDEX IF NOT EXISTS idx_execute_results_created_at ON execute_results(created_at);
	CREATE INDEX IF NOT EXISTS idx_execute_results_type ON execute_results(type);

	-- 创建触发器用于更新 updated_at
	CREATE TRIGGER IF NOT EXISTS update_execute_results_updated_at 
		AFTER UPDATE ON execute_results
	BEGIN
		UPDATE execute_results SET updated_at = strftime('%s', 'now') WHERE request_id = NEW.request_id;
	END;
	`

	_, err := s.db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("创建表失败: %w", err)
	}

	s.logger.Info("SQLite数据库表初始化完成")
	return nil
}

// SaveExecuteResult 保存执行记录
func (s *SQLiteStorage) SaveExecuteResult(ctx context.Context, result *server.ExecuteResult) error {
	if result == nil {
		return fmt.Errorf("执行结果不能为空")
	}

	// 序列化命令结果和批量结果
	var commandResultJSON, batchResultJSON []byte
	var err error

	if result.CommandResult != nil {
		commandResultJSON, err = json.Marshal(result.CommandResult)
		if err != nil {
			return fmt.Errorf("序列化命令结果失败: %w", err)
		}
	}

	if result.BatchResult != nil {
		batchResultJSON, err = json.Marshal(result.BatchResult)
		if err != nil {
			return fmt.Errorf("序列化批量结果失败: %w", err)
		}
	}

	// 执行插入或更新
	query := `
	INSERT OR REPLACE INTO execute_results (
		request_id, type, server_id, async, start_time, end_time, 
		command_result, batch_result
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		result.RequestID,
		string(result.Type),
		result.ServerID,
		boolToInt(result.Async),
		result.StartTime.Unix(),
		timeToNullableInt(result.EndTime),
		nullableBytes(commandResultJSON),
		nullableBytes(batchResultJSON),
	)

	if err != nil {
		return fmt.Errorf("保存执行记录失败: %w", err)
	}

	s.logger.Debug("执行记录保存成功",
		zap.String("requestID", result.RequestID),
		zap.String("serverID", result.ServerID))

	return nil
}

// GetExecuteResult 获取执行记录
func (s *SQLiteStorage) GetExecuteResult(ctx context.Context, requestID string) (*server.ExecuteResult, error) {
	if requestID == "" {
		return nil, fmt.Errorf("请求ID不能为空")
	}

	query := `
	SELECT request_id, type, server_id, async, start_time, end_time, 
		   command_result, batch_result
	FROM execute_results 
	WHERE request_id = ?
	`

	row := s.db.QueryRowContext(ctx, query, requestID)

	var result server.ExecuteResult
	var commandResultJSON, batchResultJSON sql.NullString
	var async int
	var startTime, endTime sql.NullInt64

	err := row.Scan(
		&result.RequestID,
		&result.Type,
		&result.ServerID,
		&async,
		&startTime,
		&endTime,
		&commandResultJSON,
		&batchResultJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("未找到请求ID为 %s 的执行记录", requestID)
		}
		return nil, fmt.Errorf("查询执行记录失败: %w", err)
	}

	// 转换数据类型
	result.Async = intToBool(async)
	result.StartTime = time.Unix(startTime.Int64, 0)
	if endTime.Valid {
		result.EndTime = time.Unix(endTime.Int64, 0)
	}

	// 反序列化结果
	if commandResultJSON.Valid && commandResultJSON.String != "" {
		var commandResult server.CommandResult
		if err := json.Unmarshal([]byte(commandResultJSON.String), &commandResult); err != nil {
			return nil, fmt.Errorf("反序列化命令结果失败: %w", err)
		}
		result.CommandResult = &commandResult
	}

	if batchResultJSON.Valid && batchResultJSON.String != "" {
		var batchResult server.BatchResult
		if err := json.Unmarshal([]byte(batchResultJSON.String), &batchResult); err != nil {
			return nil, fmt.Errorf("反序列化批量结果失败: %w", err)
		}
		result.BatchResult = &batchResult
	}

	return &result, nil
}

// GetServerExecuteHistory 获取服务器执行历史
func (s *SQLiteStorage) GetServerExecuteHistory(ctx context.Context, serverID string, limit int, offset int) ([]*server.ExecuteResult, int, error) {
	if serverID == "" {
		return nil, 0, fmt.Errorf("服务器ID不能为空")
	}

	// 设置默认值
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// 查询总数
	countQuery := "SELECT COUNT(*) FROM execute_results WHERE server_id = ?"
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, serverID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("查询执行记录总数失败: %w", err)
	}

	// 查询记录
	query := `
	SELECT request_id, type, server_id, async, start_time, end_time,
		   command_result, batch_result
	FROM execute_results 
	WHERE server_id = ?
	ORDER BY start_time DESC
	LIMIT ? OFFSET ?
	`

	rows, err := s.db.QueryContext(ctx, query, serverID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询执行历史失败: %w", err)
	}
	defer rows.Close()

	var results []*server.ExecuteResult
	for rows.Next() {
		var result server.ExecuteResult
		var commandResultJSON, batchResultJSON sql.NullString
		var async int
		var startTime, endTime sql.NullInt64

		err := rows.Scan(
			&result.RequestID,
			&result.Type,
			&result.ServerID,
			&async,
			&startTime,
			&endTime,
			&commandResultJSON,
			&batchResultJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("扫描执行记录失败: %w", err)
		}

		// 转换数据类型
		result.Async = intToBool(async)
		result.StartTime = time.Unix(startTime.Int64, 0)
		if endTime.Valid {
			result.EndTime = time.Unix(endTime.Int64, 0)
		}

		// 反序列化结果
		if commandResultJSON.Valid && commandResultJSON.String != "" {
			var commandResult server.CommandResult
			if err := json.Unmarshal([]byte(commandResultJSON.String), &commandResult); err != nil {
				s.logger.Warn("反序列化命令结果失败",
					zap.String("requestID", result.RequestID),
					zap.Error(err))
				continue
			}
			result.CommandResult = &commandResult
		}

		if batchResultJSON.Valid && batchResultJSON.String != "" {
			var batchResult server.BatchResult
			if err := json.Unmarshal([]byte(batchResultJSON.String), &batchResult); err != nil {
				s.logger.Warn("反序列化批量结果失败",
					zap.String("requestID", result.RequestID),
					zap.Error(err))
				continue
			}
			result.BatchResult = &batchResult
		}

		results = append(results, &result)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历执行记录失败: %w", err)
	}

	s.logger.Debug("查询服务器执行历史成功",
		zap.String("serverID", serverID),
		zap.Int("total", total),
		zap.Int("returned", len(results)))

	return results, total, nil
}

// DeleteExecuteResult 删除执行记录
func (s *SQLiteStorage) DeleteExecuteResult(ctx context.Context, requestID string) error {
	if requestID == "" {
		return fmt.Errorf("请求ID不能为空")
	}

	query := "DELETE FROM execute_results WHERE request_id = ?"
	result, err := s.db.ExecContext(ctx, query, requestID)
	if err != nil {
		return fmt.Errorf("删除执行记录失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除行数失败: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("未找到请求ID为 %s 的执行记录", requestID)
	}

	s.logger.Debug("执行记录删除成功", zap.String("requestID", requestID))
	return nil
}

// CleanupExpiredResults 清理过期记录
func (s *SQLiteStorage) CleanupExpiredResults(ctx context.Context, expiration time.Duration) error {
	if expiration <= 0 {
		return fmt.Errorf("过期时间必须大于0")
	}

	expireTime := time.Now().Add(-expiration).Unix()
	query := "DELETE FROM execute_results WHERE created_at < ?"

	result, err := s.db.ExecContext(ctx, query, expireTime)
	if err != nil {
		return fmt.Errorf("清理过期记录失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取清理行数失败: %w", err)
	}

	s.logger.Info("清理过期执行记录完成",
		zap.Int64("cleanedCount", rowsAffected),
		zap.Duration("expiration", expiration))

	return nil
}

// Close 关闭存储连接
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// 辅助函数

// ensureDir 确保目录存在
func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}

	return os.MkdirAll(dir, 0755)
}

// boolToInt 布尔值转整数
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool 整数转布尔值
func intToBool(i int) bool {
	return i != 0
}

// timeToNullableInt 时间转可空整数
func timeToNullableInt(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

// nullableBytes 字节数组转可空字符串
func nullableBytes(data []byte) sql.NullString {
	if len(data) == 0 {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: string(data), Valid: true}
}

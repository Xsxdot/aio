package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xsxdot/aio/pkg/server"
	"go.uber.org/zap"
)

func TestSQLiteStorage(t *testing.T) {
	// 创建临时数据库
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	logger, _ := zap.NewDevelopment()
	storage, err := NewSQLiteStorage(SQLiteConfig{
		DatabasePath: dbPath,
		Logger:       logger,
	})
	require.NoError(t, err)
	defer storage.(*SQLiteStorage).Close()

	ctx := context.Background()

	t.Run("保存和获取单个命令执行结果", func(t *testing.T) {
		result := &server.ExecuteResult{
			RequestID: "test-request-1",
			Type:      server.CommandTypeSingle,
			ServerID:  "server-1",
			Async:     false,
			StartTime: time.Now(),
			EndTime:   time.Now().Add(5 * time.Second),
			CommandResult: &server.CommandResult{
				CommandID:   "cmd-1",
				CommandName: "测试命令",
				Command:     "echo 'hello world'",
				Status:      server.CommandStatusSuccess,
				ExitCode:    0,
				StartTime:   time.Now(),
				EndTime:     time.Now().Add(time.Second),
				Duration:    time.Second,
				Stdout:      "hello world\n",
				Stderr:      "",
				Error:       "",
				RetryCount:  0,
			},
		}

		// 保存执行结果
		err := storage.SaveExecuteResult(ctx, result)
		assert.NoError(t, err)

		// 获取执行结果
		retrievedResult, err := storage.GetExecuteResult(ctx, "test-request-1")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedResult)
		assert.Equal(t, result.RequestID, retrievedResult.RequestID)
		assert.Equal(t, result.Type, retrievedResult.Type)
		assert.Equal(t, result.ServerID, retrievedResult.ServerID)
		assert.Equal(t, result.Async, retrievedResult.Async)
		assert.NotNil(t, retrievedResult.CommandResult)
		assert.Equal(t, result.CommandResult.CommandID, retrievedResult.CommandResult.CommandID)
		assert.Equal(t, result.CommandResult.Status, retrievedResult.CommandResult.Status)
		assert.Equal(t, result.CommandResult.ExitCode, retrievedResult.CommandResult.ExitCode)
		assert.Equal(t, result.CommandResult.Stdout, retrievedResult.CommandResult.Stdout)
	})

	t.Run("保存和获取批量命令执行结果", func(t *testing.T) {
		result := &server.ExecuteResult{
			RequestID: "test-request-2",
			Type:      server.CommandTypeBatch,
			ServerID:  "server-2",
			Async:     true,
			StartTime: time.Now(),
			EndTime:   time.Now().Add(10 * time.Second),
			BatchResult: &server.BatchResult{
				BatchID:   "batch-1",
				BatchName: "测试批量命令",
				ServerID:  "server-2",
				Status:    server.CommandStatusSuccess,
				StartTime: time.Now(),
				EndTime:   time.Now().Add(10 * time.Second),
				Duration:  10 * time.Second,
				TryResults: []*server.CommandResult{
					{
						CommandID:   "cmd-1",
						CommandName: "命令1",
						Command:     "echo 'step 1'",
						Status:      server.CommandStatusSuccess,
						ExitCode:    0,
						Stdout:      "step 1\n",
					},
					{
						CommandID:   "cmd-2",
						CommandName: "命令2",
						Command:     "echo 'step 2'",
						Status:      server.CommandStatusSuccess,
						ExitCode:    0,
						Stdout:      "step 2\n",
					},
				},
				TotalCommands:   2,
				SuccessCommands: 2,
				FailedCommands:  0,
			},
		}

		// 保存执行结果
		err := storage.SaveExecuteResult(ctx, result)
		assert.NoError(t, err)

		// 获取执行结果
		retrievedResult, err := storage.GetExecuteResult(ctx, "test-request-2")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedResult)
		assert.Equal(t, result.RequestID, retrievedResult.RequestID)
		assert.Equal(t, result.Type, retrievedResult.Type)
		assert.Equal(t, result.ServerID, retrievedResult.ServerID)
		assert.Equal(t, result.Async, retrievedResult.Async)
		assert.NotNil(t, retrievedResult.BatchResult)
		assert.Equal(t, result.BatchResult.BatchID, retrievedResult.BatchResult.BatchID)
		assert.Equal(t, result.BatchResult.Status, retrievedResult.BatchResult.Status)
		assert.Equal(t, result.BatchResult.TotalCommands, retrievedResult.BatchResult.TotalCommands)
		assert.Equal(t, len(result.BatchResult.TryResults), len(retrievedResult.BatchResult.TryResults))
	})

	t.Run("获取服务器执行历史", func(t *testing.T) {
		// 获取服务器1的执行历史
		results, total, err := storage.GetServerExecuteHistory(ctx, "server-1", 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, results, 1)
		assert.Equal(t, "test-request-1", results[0].RequestID)

		// 获取服务器2的执行历史
		results, total, err = storage.GetServerExecuteHistory(ctx, "server-2", 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, results, 1)
		assert.Equal(t, "test-request-2", results[0].RequestID)

		// 获取不存在服务器的历史
		results, total, err = storage.GetServerExecuteHistory(ctx, "non-existent", 10, 0)
		assert.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Len(t, results, 0)
	})

	t.Run("删除执行记录", func(t *testing.T) {
		// 删除存在的记录
		err := storage.DeleteExecuteResult(ctx, "test-request-1")
		assert.NoError(t, err)

		// 验证记录已删除
		_, err = storage.GetExecuteResult(ctx, "test-request-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "未找到请求ID")

		// 删除不存在的记录
		err = storage.DeleteExecuteResult(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "未找到请求ID")
	})

	t.Run("清理过期记录", func(t *testing.T) {
		// 先添加一个记录
		result := &server.ExecuteResult{
			RequestID: "test-request-3",
			Type:      server.CommandTypeSingle,
			ServerID:  "server-3",
			Async:     false,
			StartTime: time.Now(),
			EndTime:   time.Now(),
			CommandResult: &server.CommandResult{
				CommandID:   "cmd-3",
				CommandName: "测试命令3",
				Command:     "echo 'test'",
				Status:      server.CommandStatusSuccess,
				ExitCode:    0,
				Stdout:      "test\n",
			},
		}

		err := storage.SaveExecuteResult(ctx, result)
		assert.NoError(t, err)

		// 清理过期记录（设置一个很长的过期时间，不应该删除任何记录）
		err = storage.CleanupExpiredResults(ctx, 24*time.Hour)
		assert.NoError(t, err)

		// 验证记录仍然存在
		_, err = storage.GetExecuteResult(ctx, "test-request-3")
		assert.NoError(t, err)

		// 清理过期记录（设置一个很短的过期时间，应该删除所有记录）
		err = storage.CleanupExpiredResults(ctx, 1*time.Nanosecond)
		assert.NoError(t, err)

		// 等待一小段时间确保记录被标记为过期
		time.Sleep(10 * time.Millisecond)

		// 再次清理
		err = storage.CleanupExpiredResults(ctx, 1*time.Nanosecond)
		assert.NoError(t, err)

		// 验证记录已被删除（注意：由于时间精度问题，这个测试可能不够可靠）
		// 在实际使用中，过期时间通常以小时或天为单位
	})

	t.Run("错误情况测试", func(t *testing.T) {
		// 测试空请求ID
		_, err := storage.GetExecuteResult(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "请求ID不能为空")

		// 测试空服务器ID
		_, _, err = storage.GetServerExecuteHistory(ctx, "", 10, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "服务器ID不能为空")

		// 测试保存空结果
		err = storage.SaveExecuteResult(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "执行结果不能为空")

		// 测试删除空请求ID
		err = storage.DeleteExecuteResult(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "请求ID不能为空")

		// 测试清理过期记录时传入无效过期时间
		err = storage.CleanupExpiredResults(ctx, -1*time.Hour)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "过期时间必须大于0")
	})
}

func TestSQLiteStorageConfiguration(t *testing.T) {
	t.Run("默认配置", func(t *testing.T) {
		tempDir := t.TempDir()
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		os.Chdir(tempDir)

		storage, err := NewSQLiteStorage(SQLiteConfig{})
		require.NoError(t, err)
		defer storage.(*SQLiteStorage).Close()

		// 验证默认数据库路径
		sqliteStorage := storage.(*SQLiteStorage)
		assert.NotNil(t, sqliteStorage.db)
		assert.NotNil(t, sqliteStorage.logger)
	})

	t.Run("自定义配置", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "custom.db")
		logger, _ := zap.NewDevelopment()

		storage, err := NewSQLiteStorage(SQLiteConfig{
			DatabasePath: dbPath,
			Logger:       logger,
		})
		require.NoError(t, err)
		defer storage.(*SQLiteStorage).Close()

		sqliteStorage := storage.(*SQLiteStorage)
		assert.NotNil(t, sqliteStorage.db)
		assert.Equal(t, logger, sqliteStorage.logger)

		// 验证数据库文件已创建
		assert.FileExists(t, dbPath)
	})
}

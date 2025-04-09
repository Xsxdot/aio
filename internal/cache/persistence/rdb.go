package persistence

import (
	"encoding/binary"
	"fmt"
	cache2 "github.com/xsxdot/aio/internal/cache"
	ds2 "github.com/xsxdot/aio/internal/cache/ds"
	"github.com/xsxdot/aio/pkg/common"
	"os"
	"sync"
	"time"
)

// RDBManager 管理RDB持久化
type RDBManager struct {
	config       *cache2.Config
	lastSaveTime time.Time
	dbAccessor   DBAccessor
	mutex        sync.Mutex
	wg           *sync.WaitGroup
	shutdownCh   chan struct{}
	logger       *common.Logger // 日志记录器
}

// DBAccessor 数据库访问接口
type DBAccessor interface {
	// GetAllData 获取数据库中的所有数据
	GetAllData() (map[string]cache2.Value, map[string]time.Time)
	// LoadData 从数据中加载数据
	LoadData(data map[string]cache2.Value, expires map[string]time.Time) error
}

// NewRDBManager 创建一个新的RDB管理器
func NewRDBManager(config *cache2.Config, accessor DBAccessor, wg *sync.WaitGroup) *RDBManager {
	return &RDBManager{
		config:       config,
		lastSaveTime: time.Now(),
		dbAccessor:   accessor,
		mutex:        sync.Mutex{},
		wg:           wg,
		shutdownCh:   make(chan struct{}),
		logger:       common.GetLogger(), // 初始化logger
	}
}

// StartPeriodicSave 启动定期保存
func (rm *RDBManager) StartPeriodicSave() {
	rm.wg.Add(1)
	go rm.periodicSave()
}

// Shutdown 关闭RDB管理器
func (rm *RDBManager) Shutdown() {
	close(rm.shutdownCh)
}

// periodicSave 定期保存RDB文件
func (rm *RDBManager) periodicSave() {
	defer rm.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rm.shutdownCh:
			// 系统关闭，保存一次数据
			if err := rm.SaveToRDB(); err != nil {
				rm.logger.Errorf("Error saving RDB on shutdown: %v", err)
			}
			return
		case <-ticker.C:
			// 检查是否需要保存
			if rm.shouldSave() {
				if err := rm.SaveToRDB(); err != nil {
					rm.logger.Errorf("Error saving RDB: %v", err)
				} else {
					rm.lastSaveTime = time.Now()
				}
			}
		}
	}
}

// shouldSave 判断是否应该保存RDB
func (rm *RDBManager) shouldSave() bool {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// 如果距离上次保存时间超过配置的间隔，触发保存
	return time.Since(rm.lastSaveTime) > time.Duration(rm.config.RDBSaveInterval)*time.Second
}

// SaveToRDB 保存数据到RDB文件
func (rm *RDBManager) SaveToRDB() error {
	// 如果禁用了RDB，则跳过保存
	if !rm.config.EnableRDB {
		rm.logger.Infof("RDB persistence is disabled, skipping save")
		return nil
	}

	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// 确保目录存在
	if err := EnsureDir(rm.config.RDBFilePath); err != nil {
		return fmt.Errorf("error ensuring RDB directory: %v", err)
	}

	// 创建临时文件
	tempFilePath := rm.config.RDBFilePath + ".temp"
	file, err := os.Create(tempFilePath)
	if err != nil {
		return fmt.Errorf("error creating temporary RDB file: %v", err)
	}
	defer file.Close()

	// 获取数据库数据
	data, expires := rm.dbAccessor.GetAllData()
	rm.logger.Infof("Saving %d keys to RDB file", len(data))

	// 写入RDB文件头部（确保长度为8字节）
	header := "REDIS001" // 正好8个字节
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("error writing RDB header: %v", err)
	}
	rm.logger.Infof("Wrote header: %q (length: %d)", header, len(header))

	// 写入数据库索引（假设只有0号数据库）
	if err := binary.Write(file, binary.BigEndian, uint32(0)); err != nil {
		return fmt.Errorf("error writing database index: %v", err)
	}

	// 写入键数量
	validKeys := 0
	now := time.Now()
	for _, expireTime := range expires {
		if !expireTime.Before(now) {
			validKeys++
		}
	}
	for k := range data {
		if _, hasExpire := expires[k]; !hasExpire {
			validKeys++
		}
	}

	if err := binary.Write(file, binary.BigEndian, uint32(validKeys)); err != nil {
		return fmt.Errorf("error writing keys count: %v", err)
	}

	// 写入数据
	for key, value := range data {
		// 检查是否过期
		if expireTime, hasExpire := expires[key]; hasExpire && expireTime.Before(now) {
			continue
		}

		// 写入键长度和内容
		if err := binary.Write(file, binary.BigEndian, uint32(len(key))); err != nil {
			return fmt.Errorf("error writing key length: %v", err)
		}
		if _, err := file.WriteString(key); err != nil {
			return fmt.Errorf("error writing key: %v", err)
		}

		// 写入值类型
		if err := binary.Write(file, binary.BigEndian, uint8(value.Type())); err != nil {
			return fmt.Errorf("error writing value type: %v", err)
		}

		// 编码值
		encodedValue, err := value.Encode()
		if err != nil {
			return fmt.Errorf("error encoding value for key %s: %v", key, err)
		}

		// 写入值长度和内容
		if err := binary.Write(file, binary.BigEndian, uint32(len(encodedValue))); err != nil {
			return fmt.Errorf("error writing value length: %v", err)
		}
		if _, err := file.Write(encodedValue); err != nil {
			return fmt.Errorf("error writing value: %v", err)
		}

		// 写入过期时间（如果有）
		if expireTime, hasExpire := expires[key]; hasExpire {
			// 写入标记，表示有过期时间
			if err := binary.Write(file, binary.BigEndian, uint8(1)); err != nil {
				return fmt.Errorf("error writing expiry flag: %v", err)
			}
			// 写入过期时间
			expireUnix := expireTime.UnixNano()
			if err := binary.Write(file, binary.BigEndian, expireUnix); err != nil {
				return fmt.Errorf("error writing expiry time: %v", err)
			}
		} else {
			// 写入标记，表示没有过期时间
			if err := binary.Write(file, binary.BigEndian, uint8(0)); err != nil {
				return fmt.Errorf("error writing expiry flag: %v", err)
			}
		}
	}

	// 写入文件尾部
	if _, err := file.WriteString("EOF"); err != nil {
		return fmt.Errorf("error writing RDB footer: %v", err)
	}

	// 确保数据写入磁盘
	if err := file.Sync(); err != nil {
		return fmt.Errorf("error syncing RDB file: %v", err)
	}

	// 关闭临时文件
	if err := file.Close(); err != nil {
		return fmt.Errorf("error closing temporary RDB file: %v", err)
	}

	// 用临时文件替换正式文件
	if err := os.Rename(tempFilePath, rm.config.RDBFilePath); err != nil {
		return fmt.Errorf("error replacing RDB file: %v", err)
	}

	rm.logger.Infof("RDB saved successfully to %s", rm.config.RDBFilePath)
	return nil
}

// LoadFromRDB 从RDB文件加载数据
func (rm *RDBManager) LoadFromRDB() error {
	// 如果禁用了RDB，则跳过加载
	if !rm.config.EnableRDB {
		rm.logger.Infof("RDB persistence is disabled, skipping load")
		return nil
	}

	// 检查文件是否存在
	fileInfo, err := os.Stat(rm.config.RDBFilePath)
	if os.IsNotExist(err) {
		rm.logger.Infof("No RDB file found at %s", rm.config.RDBFilePath)
		return nil // 文件不存在，不需要加载
	} else if err != nil {
		return fmt.Errorf("error checking RDB file: %v", err)
	}

	rm.logger.Infof("Loading RDB file %s (size: %d bytes)", rm.config.RDBFilePath, fileInfo.Size())

	// 读取文件内容
	fileData, err := os.ReadFile(rm.config.RDBFilePath)
	if err != nil {
		return fmt.Errorf("error reading RDB file: %v", err)
	}

	rm.logger.Infof("Read %d bytes from RDB file", len(fileData))

	// 验证文件长度
	if len(fileData) < 8 {
		return fmt.Errorf("invalid RDB file: too short (%d bytes)", len(fileData))
	}

	// 验证文件头部
	header := string(fileData[:8])
	rm.logger.Infof("RDB header: %q (length: %d)", header, len(header))

	if header != "REDIS001" && header != "REDIS000" {
		return fmt.Errorf("invalid RDB format: unexpected header %q", header)
	}

	// 从内存中读取数据
	pos := 8

	// 读取数据库索引
	if pos+4 > len(fileData) {
		return fmt.Errorf("invalid RDB format: file too short to read database index")
	}
	// 读取但不使用数据库索引，我们的实现只支持单个数据库
	_ = binary.BigEndian.Uint32(fileData[pos : pos+4])
	pos += 4

	// 读取键数量
	if pos+4 > len(fileData) {
		return fmt.Errorf("invalid RDB format: file too short to read key count")
	}
	keysCount := binary.BigEndian.Uint32(fileData[pos : pos+4])
	pos += 4

	// 读取数据
	data := make(map[string]cache2.Value)
	expires := make(map[string]time.Time)

	for i := uint32(0); i < keysCount; i++ {
		// 读取键长度
		if pos+4 > len(fileData) {
			return fmt.Errorf("invalid RDB format: file too short to read key length for key %d", i)
		}
		keyLength := binary.BigEndian.Uint32(fileData[pos : pos+4])
		pos += 4

		// 读取键
		if pos+int(keyLength) > len(fileData) {
			return fmt.Errorf("invalid RDB format: file too short to read key %d", i)
		}
		key := string(fileData[pos : pos+int(keyLength)])
		pos += int(keyLength)

		// 读取值类型
		if pos+1 > len(fileData) {
			return fmt.Errorf("invalid RDB format: file too short to read value type for key %s", key)
		}
		valueType := fileData[pos]
		pos++

		// 读取值长度
		if pos+4 > len(fileData) {
			return fmt.Errorf("invalid RDB format: file too short to read value length for key %s", key)
		}
		valueLength := binary.BigEndian.Uint32(fileData[pos : pos+4])
		pos += 4

		// 读取值
		if pos+int(valueLength) > len(fileData) {
			return fmt.Errorf("invalid RDB format: file too short to read value for key %s", key)
		}
		valueBytes := fileData[pos : pos+int(valueLength)]
		pos += int(valueLength)

		// 根据类型创建值对象
		var value cache2.Value
		var decodeErr error

		switch cache2.DataType(valueType) {
		case cache2.TypeString:
			value, decodeErr = ds2.DecodeString(valueBytes)
		case cache2.TypeList:
			value, decodeErr = ds2.DecodeList(valueBytes)
		case cache2.TypeHash:
			value, decodeErr = ds2.DecodeHash(valueBytes)
		case cache2.TypeSet:
			value, decodeErr = ds2.DecodeSet(valueBytes)
		case cache2.TypeZSet:
			value, decodeErr = ds2.DecodeZSet(valueBytes)
		default:
			return fmt.Errorf("unknown data type: %d for key %s", valueType, key)
		}

		if decodeErr != nil {
			return fmt.Errorf("error decoding value for key %s: %v", key, decodeErr)
		}

		data[key] = value

		// 读取过期时间标记
		if pos+1 > len(fileData) {
			return fmt.Errorf("invalid RDB format: file too short to read expiry flag for key %s", key)
		}
		expiryFlag := fileData[pos]
		pos++

		// 如果有过期时间，读取过期时间
		if expiryFlag == 1 {
			if pos+8 > len(fileData) {
				return fmt.Errorf("invalid RDB format: file too short to read expiry time for key %s", key)
			}
			// 使用Uint64然后转换为int64
			expireUnixBits := binary.BigEndian.Uint64(fileData[pos : pos+8])
			expireUnix := int64(expireUnixBits)
			pos += 8
			expires[key] = time.Unix(0, expireUnix)
		}
	}

	// 检查是否到达文件尾部标记
	if pos+3 > len(fileData) {
		return fmt.Errorf("invalid RDB format: file too short to read footer")
	}
	footerStr := string(fileData[pos : pos+3])
	if footerStr != "EOF" {
		return fmt.Errorf("invalid RDB footer: %q", footerStr)
	}

	// 将数据加载到数据库
	if err := rm.dbAccessor.LoadData(data, expires); err != nil {
		return fmt.Errorf("error loading data into database: %v", err)
	}

	rm.logger.Infof("Successfully loaded %d keys from RDB file %s", len(data), rm.config.RDBFilePath)
	return nil
}

package dialect

import (
	"strings"

	"gorm.io/gorm"
)

// DialectName 返回当前数据库方言名称："mysql" 或 "postgres"
func DialectName(db *gorm.DB) string {
	if db == nil {
		return "mysql" // 默认
	}
	return strings.ToLower(db.Dialector.Name())
}

// IsMySQL 判断当前数据库是否为 MySQL
func IsMySQL(db *gorm.DB) bool {
	return DialectName(db) == "mysql"
}

// IsPostgres 判断当前数据库是否为 PostgreSQL
func IsPostgres(db *gorm.DB) bool {
	return DialectName(db) == "postgres"
}

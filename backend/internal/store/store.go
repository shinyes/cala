// Package store 是 SQLite 持久化的唯一 owner。
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // 纯 Go 驱动，支持 CGO_ENABLED=0 静态构建
)

// pragmas 会被拼进 DSN。
//
// 关键：foreign_keys 必须在 DSN 中开启，不能靠 Open 之后执行
// db.Exec("PRAGMA foreign_keys=ON")。SQLite 的该 pragma 是「每连接」生效，
// 而 database/sql 是连接池 —— 那样写只有部分连接启用外键，
// 会让 (project, user) 级联删除静默失效，直接破坏功能8 与 D4。
// 该结论由 evidence/sqlite-cascade-spike 实测得出。
const pragmas = "_pragma=foreign_keys(1)" +
	"&_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=synchronous(NORMAL)"

// Store 持有数据库连接池。
type Store struct {
	db *sql.DB
}

// Open 打开（必要时创建）数据库并应用迁移。
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?%s", path, pragmas)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 单连接：彻底消除 SQLITE_BUSY 与写-写竞争。
	// 本项目是单进程自部署场景，写入量极小，串行化的性能代价可忽略；
	// 换来的是「无需处理并发写冲突」这一整类复杂度的消失。
	// 退役触发条件：出现明确的读并发瓶颈时，再改为读写分离连接池。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭连接池。
func (s *Store) Close() error { return s.db.Close() }

// DB 暴露底层连接，仅供同包与测试使用。
func (s *Store) DB() *sql.DB { return s.db }

// Now 返回统一的时间表示（RFC3339 UTC），所有写入列都用它。
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

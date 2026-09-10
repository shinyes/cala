package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound 表示目标记录不存在。调用方据此返回 404。
var ErrNotFound = errors.New("记录不存在")

// ErrUsernameTaken 表示用户名已被占用。
var ErrUsernameTaken = errors.New("用户名已被占用")

// User 是对外可见的用户表示，永不包含口令哈希。
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	IsAdmin   bool   `json:"isAdmin"`
	Disabled  bool   `json:"disabled"`
	CreatedAt string `json:"createdAt"`
}

// CountUsers 返回用户总数。用于判定「是否为首个用户」。
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM "user"`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计用户数失败: %w", err)
	}
	return n, nil
}

// CreateUser 插入用户并返回其表示。
func (s *Store) CreateUser(username, passwordHash string, isAdmin bool) (User, error) {
	u := User{Username: username, IsAdmin: isAdmin, CreatedAt: Now()}
	res, err := s.db.Exec(
		`INSERT INTO "user"(username, password_hash, is_admin, disabled, created_at)
		 VALUES (?, ?, ?, 0, ?)`,
		username, passwordHash, boolToInt(isAdmin), u.CreatedAt)
	if err != nil {
		// UNIQUE 约束冲突 -> 用户名被占用
		if isUniqueViolation(err) {
			return User{}, ErrUsernameTaken
		}
		return User{}, fmt.Errorf("创建用户失败: %w", err)
	}
	u.ID, err = res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("读取新用户 ID 失败: %w", err)
	}
	return u, nil
}

// AuthenticateLookup 按用户名取出口令哈希与用户信息，供口令校验使用。
func (s *Store) AuthenticateLookup(username string) (User, string, error) {
	var (
		u    User
		hash string
		adm  int
		dis  int
	)
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, is_admin, disabled, created_at
		 FROM "user" WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &hash, &adm, &dis, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("查询用户失败: %w", err)
	}
	u.IsAdmin = adm != 0
	u.Disabled = dis != 0
	return u, hash, nil
}

// GetUser 按 ID 取用户。
func (s *Store) GetUser(id int64) (User, error) {
	var (
		u   User
		adm int
		dis int
	)
	err := s.db.QueryRow(
		`SELECT id, username, is_admin, disabled, created_at FROM "user" WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &adm, &dis, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("查询用户失败: %w", err)
	}
	u.IsAdmin = adm != 0
	u.Disabled = dis != 0
	return u, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isUniqueViolation 判断错误是否为 SQLite 唯一约束冲突。
// 纯 Go 驱动不导出可判别的错误类型，故匹配错误文本。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: UNIQUE")
}

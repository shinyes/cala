package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
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

// PasswordHash 取出指定用户的口令哈希，供改密时校验当前口令使用。
//
// 单独成方法而不是复用 AuthenticateLookup：后者按**用户名**查询，
// 而改密的调用方手里只有会话里的用户 ID。绕经用户名多一次查询，
// 也把「按 ID 取哈希」这一需求藏在了语义不符的方法名后面。
func (s *Store) PasswordHash(userID int64) (string, error) {
	var hash string
	err := s.db.QueryRow(
		`SELECT password_hash FROM "user" WHERE id = ?`, userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("查询口令哈希失败: %w", err)
	}
	return hash, nil
}

// SetPasswordAndRotateSessions 在一个事务内完成三件事：
// 更新口令哈希、吊销该用户的**全部**会话、写入一条新会话。
//
// 为什么必须原子：这是同一条安全不变式「改密即吊销其他会话」的三个步骤。
// 若分开执行，任何一步失败都会留下自相矛盾的状态 ——
// 例如口令已改但旧会话仍有效，于是攻击者用被窃取的令牌继续访问，
// 而用户以为改密已经解决问题（这正是改密最常见的动机）。
//
// 为什么新会话也要在事务内：否则「口令已改、会话已清、但新令牌签发失败」
// 会让客户端收到错误、以为改密失败，实际上口令已经变了 ——
// 用户随后用旧口令重试，得到的是「当前口令不正确」，无从理解。
// 放进来即保证要么全部生效，要么全部不生效。
//
// tokenHash 由调用方（auth 层）生成：令牌的生成属于 auth 包，
// store 不应知道令牌如何产生，只负责存它的摘要。
func (s *Store) SetPasswordAndRotateSessions(
	userID int64, passwordHash, newTokenHash string, expiresAt time.Time,
) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback() // 已提交时是 no-op

	res, err := tx.Exec(
		`UPDATE "user" SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("更新口令失败: %w", err)
	}
	// 影响行数为 0 说明用户不存在。明确报错而不是静默成功 ——
	// 静默成功会让客户端显示「口令已更改」而实际什么都没发生。
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("读取更新行数失败: %w", err)
	} else if n == 0 {
		return ErrNotFound
	}

	if _, err := tx.Exec(`DELETE FROM session WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("吊销用户会话失败: %w", err)
	}

	if _, err := tx.Exec(
		`INSERT INTO session(token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		newTokenHash, userID, expiresAt.UTC().Format(time.RFC3339), Now()); err != nil {
		return fmt.Errorf("创建新会话失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交改密失败: %w", err)
	}
	return nil
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

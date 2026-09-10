package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateSession 记录会话摘要与过期时间。
func (s *Store) CreateSession(tokenHash string, userID int64, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO session(token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, expiresAt.UTC().Format(time.RFC3339), Now())
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	return nil
}

// SessionUser 按令牌摘要取出未过期会话所属用户。
// 同时校验用户未被禁用——禁用账号应立即失去访问权。
func (s *Store) SessionUser(tokenHash string) (User, error) {
	var (
		u         User
		adm, dis  int
		expiresAt string
	)
	err := s.db.QueryRow(
		`SELECT u.id, u.username, u.is_admin, u.disabled, u.created_at, s.expires_at
		 FROM session s JOIN "user" u ON u.id = s.user_id
		 WHERE s.token_hash = ?`, tokenHash).
		Scan(&u.ID, &u.Username, &adm, &dis, &u.CreatedAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("查询会话失败: %w", err)
	}

	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return User{}, fmt.Errorf("会话过期时间格式非法: %w", err)
	}
	if time.Now().UTC().After(exp) {
		return User{}, ErrNotFound
	}

	u.IsAdmin = adm != 0
	u.Disabled = dis != 0
	if u.Disabled {
		return User{}, ErrNotFound
	}
	return u, nil
}

// DeleteSession 吊销单个会话（退出登录）。
func (s *Store) DeleteSession(tokenHash string) error {
	if _, err := s.db.Exec(`DELETE FROM session WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("吊销会话失败: %w", err)
	}
	return nil
}

// DeleteUserSessions 吊销某用户的全部会话（改密/禁用时使用）。
func (s *Store) DeleteUserSessions(userID int64) error {
	if _, err := s.db.Exec(`DELETE FROM session WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("吊销用户会话失败: %w", err)
	}
	return nil
}

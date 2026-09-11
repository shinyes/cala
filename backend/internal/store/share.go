package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SetShareToken 写入或替换项目的分享 token。
//
// 替换会使旧链接立即失效——这就是「重置分享链接」的实现方式，
// 因此不需要额外的 revoked 标记：token 不匹配即无法订阅。
//
// 关于存储方式：分享 token 与会话令牌不同，**以明文入库**。
//
//	会话令牌  —— 凭据本身即授权，泄露即可冒用身份，故只存 SHA-256 摘要（见 internal/auth）
//	分享 token —— 是「可被作者反复查看并转发」的分享凭据。
//	              作者需要能重新看到链接，摘要无法反查原文。
//	              它的能力上限也只是「订阅一个项目」，不涉及身份。
//
// 这个区别是有意为之，不是疏漏。
func (s *Store) SetShareToken(projectID int64, token string) error {
	res, err := s.db.Exec(
		`UPDATE project SET share_token = ?, share_token_updated_at = ? WHERE id = ?`,
		token, Now(), projectID)
	if err != nil {
		return fmt.Errorf("写入分享令牌失败: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("读取影响行数失败: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearShareToken 撤销分享（把 token 置空）。
func (s *Store) ClearShareToken(projectID int64) error {
	res, err := s.db.Exec(
		`UPDATE project SET share_token = NULL, share_token_updated_at = ? WHERE id = ?`,
		Now(), projectID)
	if err != nil {
		return fmt.Errorf("撤销分享失败: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("读取影响行数失败: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ProjectByShareToken 按分享 token 查项目。订阅时用它解析链接。
//
// 空 token 直接返回 ErrNotFound：否则撤销分享后（token 为空串）
// 可能匹配到同样为空的项目，造成意外的「订阅成功」。
func (s *Store) ProjectByShareToken(token string) (Project, error) {
	if token == "" {
		return Project{}, ErrNotFound
	}
	row := s.db.QueryRow(
		`SELECT `+projectColumns+` FROM project WHERE share_token = ?`, token)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("按分享令牌查询项目失败: %w", err)
	}
	return p, nil
}

// ShareTokenSetAt 返回 token 的最近写入时间（用于展示「链接何时重置」）。
// 未分享时返回零值。
func (s *Store) ShareTokenSetAt(projectID int64) (time.Time, error) {
	var raw sql.NullString
	err := s.db.QueryRow(
		`SELECT share_token_updated_at FROM project WHERE id = ?`, projectID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, ErrNotFound
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("查询分享时间失败: %w", err)
	}
	if !raw.Valid || raw.String == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, raw.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("分享时间格式非法: %w", err)
	}
	return t, nil
}

package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Subscribe 建立订阅关系（规格 §6.1(1) 访问权的第二个来源）。
//
// 重复订阅视为成功（幂等）：导出多链接时同一条链接出现两次不应报错。
//
// 注意：本文件**刻意不提供**「单独删除订阅行」的函数。
// 退订在产品语义上必须同时清除该用户在本项目的做题记录（D4），
// 把两件事拆成两个可单独调用的原语会留下「删了订阅却忘了清历史」的
// 错误用法。P6 将以一个事务性的整体操作提供退订，使半途而废不可能发生。
func (s *Store) Subscribe(userID, projectID int64) error {
	_, err := s.db.Exec(
		`INSERT INTO subscription(user_id, project_id, created_at) VALUES (?, ?, ?)
		 ON CONFLICT(user_id, project_id) DO NOTHING`,
		userID, projectID, Now())
	if err != nil {
		return fmt.Errorf("创建订阅失败: %w", err)
	}
	return nil
}

// IsSubscribed 判断用户是否已订阅该项目。
func (s *Store) IsSubscribed(userID, projectID int64) (bool, error) {
	var one int
	err := s.db.QueryRow(
		`SELECT 1 FROM subscription WHERE user_id = ? AND project_id = ?`,
		userID, projectID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("查询订阅失败: %w", err)
	}
	return true, nil
}

// CountSubscriptions 返回某项目的订阅者数量（不含作者）。
func (s *Store) CountSubscriptions(projectID int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM subscription WHERE project_id = ?`, projectID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计订阅数失败: %w", err)
	}
	return n, nil
}

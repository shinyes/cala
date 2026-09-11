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
// 因此退订由本文件末尾的 Unsubscribe 以一个事务性的整体操作提供，
// 使「删了订阅却忘了清历史」在结构上不可能发生。
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

// Unsubscribe 终止订阅关系，并清除该用户在该项目下的全部做题记录。
//
// 这是一个**单一的原子操作**，而不是「删订阅行」+「删记录」两个可分别调用的函数。
// 拆成两个原语会留下「删了订阅却忘了清历史」的错误用法——那正是本项目
// 一直在消除的「同一规则两处实现」。因此本文件刻意不提供单独的删除订阅行函数。
//
// 规则（规格 §6.1(2)）：(project, user) 关系终止 ⇒ 该用户在该项目下的
// practice_round / attempt 消亡。
//
// attempt 由 practice_round 的外键级联删除——**不得**在应用层再写一遍 attempt
// 的删除，否则级联规则就有了第二个执行者，两者迟早不一致。
//
// 返回被删除的轮次数，使界面能在操作前/后明示代价。
func (s *Store) Unsubscribe(userID, projectID int64) (deletedRounds int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback() // 已提交时是 no-op

	// 1) 删除本人的轮次。attempt 随 practice_round 的外键级联删除——
	//    应用层不再写一遍 attempt 的删除，否则级联规则会有第二个执行者。
	res, err := tx.Exec(
		`DELETE FROM practice_round WHERE user_id = ? AND project_id = ?`,
		userID, projectID)
	if err != nil {
		return 0, fmt.Errorf("清除做题记录失败: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("读取删除行数失败: %w", err)
	}

	// 2) 删除订阅关系
	res, err = tx.Exec(
		`DELETE FROM subscription WHERE user_id = ? AND project_id = ?`,
		userID, projectID)
	if err != nil {
		return 0, fmt.Errorf("删除订阅失败: %w", err)
	}
	subs, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("读取删除行数失败: %w", err)
	}
	if subs == 0 {
		// 关系本就不存在：明确报错而不是静默成功。
		// 静默成功会让界面显示「已退订」而实际什么都没发生。
		return 0, ErrNotFound
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交退订失败: %w", err)
	}
	return int(affected), nil
}

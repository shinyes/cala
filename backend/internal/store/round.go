package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// NewAttempt 是一道题的落库内容。
type NewAttempt struct {
	Index           int
	QSnapshot       string
	ASnapshot       string
	EnvelopeJSON    string
	UserInput       string
	ClientIsCorrect bool
	// ServerIsCorrect 是权威判分值（§6.1(5)）。
	ServerIsCorrect bool
	ElapsedMs       int64
}

// NewRound 是一轮练习的落库内容，含其全部 attempts。
type NewRound struct {
	ProjectID     int64
	UserID        int64
	Seed          int64
	StartedAt     string
	FinishedAt    string
	TotalMs       int64
	QuestionCount int
	CorrectCount  int
	Attempts      []NewAttempt
}

// CreateRound 在一个事务中写入轮次与全部 attempts，返回轮次 ID。
//
// 必须是事务：半写入的轮次（有 round 行但缺 attempt 行）会让统计与错题重练
// 读到不完整的数据。跨两次独立写入无法保证这一点。
func (s *Store) CreateRound(in NewRound) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback() // 已提交时是 no-op

	res, err := tx.Exec(
		`INSERT INTO practice_round(project_id, user_id, seed, started_at, finished_at,
			total_ms, question_count, correct_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		in.ProjectID, in.UserID, in.Seed, in.StartedAt, in.FinishedAt,
		in.TotalMs, in.QuestionCount, in.CorrectCount)
	if err != nil {
		return 0, fmt.Errorf("写入轮次失败: %w", err)
	}
	roundID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("读取轮次 ID 失败: %w", err)
	}

	stmt, err := tx.Prepare(
		`INSERT INTO attempt(round_id, idx, q_snapshot, a_snapshot, a_envelope_json,
			user_input, client_is_correct, server_is_correct, elapsed_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("准备 attempt 写入失败: %w", err)
	}
	defer stmt.Close()

	for _, a := range in.Attempts {
		if _, err := stmt.Exec(roundID, a.Index, a.QSnapshot, a.ASnapshot, a.EnvelopeJSON,
			a.UserInput, boolToInt(a.ClientIsCorrect), boolToInt(a.ServerIsCorrect),
			a.ElapsedMs); err != nil {
			return 0, fmt.Errorf("写入第 %d 题失败: %w", a.Index, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交轮次失败: %w", err)
	}
	return roundID, nil
}

// Attempt 是读回的一道题记录，供统计与错题重练使用。
type Attempt struct {
	Index           int
	QSnapshot       string
	ASnapshot       string
	EnvelopeJSON    string
	UserInput       string
	ClientIsCorrect bool
	ServerIsCorrect bool
	ElapsedMs       int64
}

// ListAttemptsByRound 读取某轮的全部题目记录，按题号升序。
func (s *Store) ListAttemptsByRound(roundID int64) ([]Attempt, error) {
	rows, err := s.db.Query(
		`SELECT idx, q_snapshot, a_snapshot, a_envelope_json, user_input,
			client_is_correct, server_is_correct, elapsed_ms
		 FROM attempt WHERE round_id = ? ORDER BY idx ASC`, roundID)
	if err != nil {
		return nil, fmt.Errorf("查询题目记录失败: %w", err)
	}
	defer rows.Close()

	out := []Attempt{}
	for rows.Next() {
		var (
			a         Attempt
			clientInt int
			serverInt int
		)
		if err := rows.Scan(&a.Index, &a.QSnapshot, &a.ASnapshot, &a.EnvelopeJSON,
			&a.UserInput, &clientInt, &serverInt, &a.ElapsedMs); err != nil {
			return nil, fmt.Errorf("读取题目记录失败: %w", err)
		}
		a.ClientIsCorrect = clientInt != 0
		a.ServerIsCorrect = serverInt != 0
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历题目记录失败: %w", err)
	}
	return out, nil
}

// Round 是一轮练习的读回表示。
type Round struct {
	ID            int64
	ProjectID     int64
	UserID        int64
	Seed          int64
	StartedAt     string
	FinishedAt    string
	TotalMs       int64
	QuestionCount int
	CorrectCount  int
}

// GetRound 按 ID 读取轮次。
func (s *Store) GetRound(id int64) (Round, error) {
	var r Round
	err := s.db.QueryRow(
		`SELECT id, project_id, user_id, seed, started_at, finished_at,
			total_ms, question_count, correct_count
		 FROM practice_round WHERE id = ?`, id).
		Scan(&r.ID, &r.ProjectID, &r.UserID, &r.Seed, &r.StartedAt, &r.FinishedAt,
			&r.TotalMs, &r.QuestionCount, &r.CorrectCount)
	if errors.Is(err, sql.ErrNoRows) {
		return Round{}, ErrNotFound
	}
	if err != nil {
		return Round{}, fmt.Errorf("查询轮次失败: %w", err)
	}
	return r, nil
}

// CountRoundsForUserProject 统计某用户在某项目下的轮次数。
// 用于退订前提示「将删除 N 轮记录」（D4 的破坏性操作需要明示代价）。
func (s *Store) CountRoundsForUserProject(userID, projectID int64) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM practice_round WHERE user_id = ? AND project_id = ?`,
		userID, projectID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计轮次数失败: %w", err)
	}
	return n, nil
}

// ListRoundsForUserProject 读取某用户在某项目下的全部轮次，按完成时间升序。
// 统计（规格 §8.1）需要取回区间内的全部轮次在 Go 内分桶，故提供本函数。
func (s *Store) ListRoundsForUserProject(userID, projectID int64) ([]Round, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, user_id, seed, started_at, finished_at,
			total_ms, question_count, correct_count
		 FROM practice_round WHERE user_id = ? AND project_id = ?
		 ORDER BY finished_at ASC, id ASC`, userID, projectID)
	if err != nil {
		return nil, fmt.Errorf("查询轮次列表失败: %w", err)
	}
	defer rows.Close()

	out := []Round{}
	for rows.Next() {
		var r Round
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.UserID, &r.Seed, &r.StartedAt,
			&r.FinishedAt, &r.TotalMs, &r.QuestionCount, &r.CorrectCount); err != nil {
			return nil, fmt.Errorf("读取轮次失败: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历轮次失败: %w", err)
	}
	return out, nil
}

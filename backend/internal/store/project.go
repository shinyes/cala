package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// 访问权取值（规格 §6.1(1)：拥有 或 已订阅）。
const (
	AccessOwner      = "owner"
	AccessSubscriber = "subscriber"
)

// ErrNoAccess 表示项目存在但调用者既非作者也未订阅。
var ErrNoAccess = errors.New("无权访问该项目")

// Project 是项目表示。
type Project struct {
	ID            int64  `json:"id"`
	OwnerID       int64  `json:"ownerId"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	QuestionCount int    `json:"questionCount"`
	CfgJSON       string `json:"cfgJson"`
	RuleSource    string `json:"ruleSource"`
	// ToleranceNum/Den 同时为空表示精确比较（规格 §6）。
	ToleranceNum *int64  `json:"toleranceNum"`
	ToleranceDen *int64  `json:"toleranceDen"`
	ShareToken   *string `json:"shareToken,omitempty"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
	// Access 由查询填充，说明调用者与该项目的关系。
	Access string `json:"access,omitempty"`
}

// NewProject 是创建项目的输入。
type NewProject struct {
	OwnerID       int64
	Title         string
	Description   string
	QuestionCount int
	CfgJSON       string
	RuleSource    string
	ToleranceNum  *int64
	ToleranceDen  *int64
}

const projectColumns = `id, owner_id, title, description, question_count, cfg_json,
	rule_source, tolerance_num, tolerance_den, share_token, created_at, updated_at`

func scanProject(sc interface {
	Scan(dest ...any) error
}) (Project, error) {
	var (
		p       Project
		tolNum  sql.NullInt64
		tolDen  sql.NullInt64
		shareID sql.NullString
	)
	err := sc.Scan(&p.ID, &p.OwnerID, &p.Title, &p.Description, &p.QuestionCount,
		&p.CfgJSON, &p.RuleSource, &tolNum, &tolDen, &shareID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Project{}, err
	}
	if tolNum.Valid {
		v := tolNum.Int64
		p.ToleranceNum = &v
	}
	if tolDen.Valid {
		v := tolDen.Int64
		p.ToleranceDen = &v
	}
	if shareID.Valid {
		v := shareID.String
		p.ShareToken = &v
	}
	return p, nil
}

// CreateProject 创建项目并返回其表示。
func (s *Store) CreateProject(in NewProject) (Project, error) {
	now := Now()
	res, err := s.db.Exec(
		`INSERT INTO project(owner_id, title, description, question_count, cfg_json,
			rule_source, tolerance_num, tolerance_den, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.OwnerID, in.Title, in.Description, in.QuestionCount, in.CfgJSON,
		in.RuleSource, in.ToleranceNum, in.ToleranceDen, now, now)
	if err != nil {
		return Project{}, fmt.Errorf("创建项目失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Project{}, fmt.Errorf("读取新项目 ID 失败: %w", err)
	}
	return s.GetProject(id)
}

// GetProject 按 ID 取项目（不判定访问权）。
func (s *Store) GetProject(id int64) (Project, error) {
	row := s.db.QueryRow(`SELECT `+projectColumns+` FROM project WHERE id = ?`, id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("查询项目失败: %w", err)
	}
	return p, nil
}

// ProjectAccess 判定用户对项目的访问权，是访问权判定的**唯一入口**。
//
// 规格 §6.1(1)：访问权 = owner_id == userID 或存在 subscription 行。
// API 与 service 都必须经由此函数，不得各自实现判定逻辑。
//
// 返回 ErrNotFound（项目不存在）或 ErrNoAccess（存在但无权）。
// 刻意不向调用方区分这两种情况之外的细节，避免泄露项目存在性给无权用户。
func (s *Store) ProjectAccess(userID, projectID int64) (string, error) {
	var ownerID int64
	err := s.db.QueryRow(`SELECT owner_id FROM project WHERE id = ?`, projectID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("查询项目归属失败: %w", err)
	}
	if ownerID == userID {
		return AccessOwner, nil
	}

	var exists int
	err = s.db.QueryRow(
		`SELECT 1 FROM subscription WHERE user_id = ? AND project_id = ?`,
		userID, projectID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoAccess
	}
	if err != nil {
		return "", fmt.Errorf("查询订阅失败: %w", err)
	}
	return AccessSubscriber, nil
}

// ListProjectsForUser 返回用户拥有的项目与已订阅的项目。
//
// 两者分开返回而非混在一起：前端需要分组显示（规格 §4.3），
// 且在 service 层合并会丢掉「这个项目是我的还是订阅的」这一信息。
func (s *Store) ListProjectsForUser(userID int64) (owned, subscribed []Project, err error) {
	owned, err = s.queryProjects(
		`SELECT `+projectColumns+` FROM project WHERE owner_id = ? ORDER BY updated_at DESC, id DESC`,
		userID)
	if err != nil {
		return nil, nil, err
	}
	owned = setAccess(owned, AccessOwner)

	subscribed, err = s.queryProjects(
		`SELECT p.id, p.owner_id, p.title, p.description, p.question_count, p.cfg_json,
			p.rule_source, p.tolerance_num, p.tolerance_den, p.share_token, p.created_at, p.updated_at
		 FROM project p JOIN subscription s ON s.project_id = p.id
		 WHERE s.user_id = ? ORDER BY p.updated_at DESC, p.id DESC`,
		userID)
	if err != nil {
		return nil, nil, err
	}
	subscribed = setAccess(subscribed, AccessSubscriber)

	return owned, subscribed, nil
}

func (s *Store) queryProjects(query string, args ...any) ([]Project, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询项目列表失败: %w", err)
	}
	defer rows.Close()

	out := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("读取项目行失败: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历项目列表失败: %w", err)
	}
	return out, nil
}

func setAccess(ps []Project, access string) []Project {
	for i := range ps {
		ps[i].Access = access
	}
	return ps
}

// ProjectUpdate 是修改项目的输入（PUT 语义：整体替换可变更字段）。
type ProjectUpdate struct {
	Title         string
	Description   string
	QuestionCount int
	CfgJSON       string
	RuleSource    string
	ToleranceNum  *int64
	ToleranceDen  *int64
}

// UpdateProject 更新项目并返回更新后的表示。
// 只有传入的 id 对应的行会被改动；所有权判定由 service 层负责。
func (s *Store) UpdateProject(id int64, u ProjectUpdate) (Project, error) {
	res, err := s.db.Exec(
		`UPDATE project SET title = ?, description = ?, question_count = ?, cfg_json = ?,
			rule_source = ?, tolerance_num = ?, tolerance_den = ?, updated_at = ?
		 WHERE id = ?`,
		u.Title, u.Description, u.QuestionCount, u.CfgJSON,
		u.RuleSource, u.ToleranceNum, u.ToleranceDen, Now(), id)
	if err != nil {
		return Project{}, fmt.Errorf("更新项目失败: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Project{}, fmt.Errorf("读取更新影响行数失败: %w", err)
	}
	if n == 0 {
		return Project{}, ErrNotFound
	}
	return s.GetProject(id)
}

// DeleteProject 删除项目。practice_round / attempt / subscription 由外键级联删除
// （规格 §6.1(2)，功能8）。应用层**不写补偿删除逻辑**。
func (s *Store) DeleteProject(id int64) error {
	res, err := s.db.Exec(`DELETE FROM project WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除项目失败: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("读取删除影响行数失败: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

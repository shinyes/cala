package store

import (
	"errors"
	"testing"
)

// seedUser 建一个用户并返回其 ID。
func seedUser(t *testing.T, s *Store, name string, admin bool) int64 {
	t.Helper()
	u, err := s.CreateUser(name, "hash", admin)
	if err != nil {
		t.Fatalf("CreateUser(%s) 失败: %v", name, err)
	}
	return u.ID
}

func newProject(t *testing.T, s *Store, ownerID int64, title string) Project {
	t.Helper()
	p, err := s.CreateProject(NewProject{
		OwnerID:       ownerID,
		Title:         title,
		Description:   "描述",
		QuestionCount: 20,
		CfgJSON:       `{"min":1,"max":9}`,
		RuleSource:    "function generate(cfg){ return {q:'1+1',a:'2'} }",
	})
	if err != nil {
		t.Fatalf("CreateProject 失败: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// 迁移 0002
// ---------------------------------------------------------------------------

func TestMigration0002AddsToleranceColumns(t *testing.T) {
	s := newTestStore(t)

	rows, err := s.DB().Query(`SELECT name FROM pragma_table_info('project')`)
	if err != nil {
		t.Fatalf("查询 project 列失败: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("读取列名失败: %v", err)
		}
		cols[n] = true
	}
	for _, want := range []string{"tolerance_num", "tolerance_den"} {
		if !cols[want] {
			t.Errorf("迁移 0002 后应存在列 %s", want)
		}
	}

	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM schema_migration`).Scan(&n); err != nil {
		t.Fatalf("查询迁移数失败: %v", err)
	}
	if n != 2 {
		t.Errorf("已应用迁移数 = %d, 期望 2", n)
	}
}

// TestMigration0002IsIdempotent 保证既有库重复打开不会重复执行迁移。
func TestMigration0002IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.db"

	first, err := Open(path)
	if err != nil {
		t.Fatalf("首次 Open 失败: %v", err)
	}
	first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("二次 Open 失败（迁移不可重复执行？）: %v", err)
	}
	defer second.Close()

	var n int
	if err := second.DB().QueryRow(`SELECT COUNT(*) FROM schema_migration`).Scan(&n); err != nil {
		t.Fatalf("查询迁移数失败: %v", err)
	}
	if n != 2 {
		t.Errorf("二次 Open 后迁移数 = %d, 期望仍为 2", n)
	}
}

func TestToleranceColumnsDefaultNull(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	p := newProject(t, s, owner, "口算")

	if p.ToleranceNum != nil || p.ToleranceDen != nil {
		t.Errorf("未指定容差时应为 nil, 得到 %v/%v", p.ToleranceNum, p.ToleranceDen)
	}
}

// ---------------------------------------------------------------------------
// 项目 CRUD
// ---------------------------------------------------------------------------

func TestCreateAndGetProject(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	p := newProject(t, s, owner, "口算")

	if p.ID == 0 {
		t.Fatal("应返回自增 ID")
	}
	if p.Title != "口算" || p.QuestionCount != 20 {
		t.Errorf("字段不符: %+v", p)
	}
	if p.CreatedAt == "" || p.UpdatedAt == "" {
		t.Error("时间字段不应为空")
	}

	got, err := s.GetProject(p.ID)
	if err != nil {
		t.Fatalf("GetProject 失败: %v", err)
	}
	if got.Title != p.Title || got.OwnerID != owner {
		t.Errorf("读回不符: %+v", got)
	}

	if _, err := s.GetProject(999999); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在项目应返回 ErrNotFound, 得到 %v", err)
	}
}

func TestCreateProjectWithTolerance(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)

	num, den := int64(1), int64(100)
	p, err := s.CreateProject(NewProject{
		OwnerID: owner, Title: "带容差", QuestionCount: 10,
		CfgJSON: "{}", RuleSource: "function generate(c){}",
		ToleranceNum: &num, ToleranceDen: &den,
	})
	if err != nil {
		t.Fatalf("CreateProject 失败: %v", err)
	}
	if p.ToleranceNum == nil || *p.ToleranceNum != 1 {
		t.Errorf("ToleranceNum 读回错误: %v", p.ToleranceNum)
	}
	if p.ToleranceDen == nil || *p.ToleranceDen != 100 {
		t.Errorf("ToleranceDen 读回错误: %v", p.ToleranceDen)
	}
}

// ---------------------------------------------------------------------------
// 访问权判定（规格 §6.1(1)）
// ---------------------------------------------------------------------------

func TestProjectAccess(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	subscriber := seedUser(t, s, "sub", false)
	stranger := seedUser(t, s, "stranger", false)
	p := newProject(t, s, owner, "口算")

	if err := s.Subscribe(subscriber, p.ID); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}

	cases := []struct {
		name   string
		user   int64
		want   string
		wantOK bool
	}{
		{"作者", owner, AccessOwner, true},
		{"订阅者", subscriber, AccessSubscriber, true},
		{"无关用户", stranger, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.ProjectAccess(tc.user, p.ID)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("应有访问权, 得到错误 %v", err)
				}
				if got != tc.want {
					t.Errorf("访问权 = %q, 期望 %q", got, tc.want)
				}
				return
			}
			if !errors.Is(err, ErrNoAccess) {
				t.Errorf("无访问权应返回 ErrNoAccess, 得到 %v", err)
			}
		})
	}

	t.Run("项目不存在", func(t *testing.T) {
		if _, err := s.ProjectAccess(owner, 999999); !errors.Is(err, ErrNotFound) {
			t.Errorf("应返回 ErrNotFound, 得到 %v", err)
		}
	})
}

// TestListProjectsForUserSeparatesOwnedAndSubscribed 保证分组信息不丢失。
func TestListProjectsForUserSeparatesOwnedAndSubscribed(t *testing.T) {
	s := newTestStore(t)
	me := seedUser(t, s, "me", false)
	other := seedUser(t, s, "other", false)

	mine := newProject(t, s, me, "我的项目")
	theirs := newProject(t, s, other, "他人项目")
	if err := s.Subscribe(me, theirs.ID); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}
	// 一个我没订阅的他人项目，不应出现在任何列表里
	newProject(t, s, other, "与我无关")

	owned, subscribed, err := s.ListProjectsForUser(me)
	if err != nil {
		t.Fatalf("ListProjectsForUser 失败: %v", err)
	}
	if len(owned) != 1 || owned[0].ID != mine.ID {
		t.Errorf("owned 应为 1 项且是我的项目, 得到 %+v", owned)
	}
	if len(subscribed) != 1 || subscribed[0].ID != theirs.ID {
		t.Errorf("subscribed 应为 1 项且是他人项目, 得到 %+v", subscribed)
	}
	if owned[0].Access != AccessOwner {
		t.Errorf("owned 的 Access 应为 owner, 得到 %q", owned[0].Access)
	}
	if subscribed[0].Access != AccessSubscriber {
		t.Errorf("subscribed 的 Access 应为 subscriber, 得到 %q", subscribed[0].Access)
	}
}

// TestListProjectsForUserEmpty 返回空切片而非 nil，便于 JSON 序列化为 [] 而非 null。
func TestListProjectsForUserEmpty(t *testing.T) {
	s := newTestStore(t)
	me := seedUser(t, s, "me", false)

	owned, subscribed, err := s.ListProjectsForUser(me)
	if err != nil {
		t.Fatalf("ListProjectsForUser 失败: %v", err)
	}
	if owned == nil || subscribed == nil {
		t.Error("应返回空切片而非 nil（否则 JSON 会序列化为 null）")
	}
	if len(owned) != 0 || len(subscribed) != 0 {
		t.Errorf("期望两者皆空, 得到 %d/%d", len(owned), len(subscribed))
	}
}

// ---------------------------------------------------------------------------
// 更新与删除
// ---------------------------------------------------------------------------

func TestUpdateProject(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	p := newProject(t, s, owner, "旧标题")

	num, den := int64(1), int64(1000)
	updated, err := s.UpdateProject(p.ID, ProjectUpdate{
		Title: "新标题", Description: "新描述", QuestionCount: 30,
		CfgJSON: `{"min":2}`, RuleSource: "function generate(c){ return {q:'2',a:'2'} }",
		ToleranceNum: &num, ToleranceDen: &den,
	})
	if err != nil {
		t.Fatalf("UpdateProject 失败: %v", err)
	}
	if updated.Title != "新标题" || updated.QuestionCount != 30 {
		t.Errorf("更新未生效: %+v", updated)
	}
	if updated.ToleranceNum == nil || *updated.ToleranceNum != 1 {
		t.Errorf("容差未更新: %v", updated.ToleranceNum)
	}
	if updated.UpdatedAt == "" {
		t.Error("UpdatedAt 不应为空")
	}

	// 可以把容差改回精确比较（两列置空）
	cleared, err := s.UpdateProject(p.ID, ProjectUpdate{
		Title: "新标题", Description: "新描述", QuestionCount: 30,
		CfgJSON: `{"min":2}`, RuleSource: "function generate(c){ return {q:'2',a:'2'} }",
	})
	if err != nil {
		t.Fatalf("UpdateProject 失败: %v", err)
	}
	if cleared.ToleranceNum != nil || cleared.ToleranceDen != nil {
		t.Errorf("容差应被清空, 得到 %v/%v", cleared.ToleranceNum, cleared.ToleranceDen)
	}

	if _, err := s.UpdateProject(999999, ProjectUpdate{Title: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("更新不存在的项目应返回 ErrNotFound, 得到 %v", err)
	}
}

// TestUpdateProjectRaisesUpdatedAt 是 P3.4「staleProject」判定的基础：
// 作者改规则后 updated_at 必须变大，否则交卷时无法识别出题后规则被改。
func TestUpdateProjectRaisesUpdatedAt(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	p := newProject(t, s, owner, "项目")

	// 强制把 updated_at 设为过去，避免同秒内比较失去意义
	if _, err := s.DB().Exec(
		`UPDATE project SET updated_at = '2020-01-01T00:00:00Z' WHERE id = ?`, p.ID); err != nil {
		t.Fatalf("预置时间失败: %v", err)
	}

	updated, err := s.UpdateProject(p.ID, ProjectUpdate{
		Title: "项目", Description: "", QuestionCount: 20,
		CfgJSON: "{}", RuleSource: "function generate(c){}",
	})
	if err != nil {
		t.Fatalf("UpdateProject 失败: %v", err)
	}
	if updated.UpdatedAt <= "2020-01-01T00:00:00Z" {
		t.Errorf("updated_at 应变大, 得到 %q", updated.UpdatedAt)
	}
}

func TestDeleteProject(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	p := newProject(t, s, owner, "项目")

	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatalf("DeleteProject 失败: %v", err)
	}
	if _, err := s.GetProject(p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("删除后查询应返回 ErrNotFound, 得到 %v", err)
	}
	if err := s.DeleteProject(p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("重复删除应返回 ErrNotFound, 得到 %v", err)
	}
}

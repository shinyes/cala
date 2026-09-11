package store

import (
	"errors"
	"testing"
)

// seedSubscribers 建三个用户、一个项目、三条订阅，各若干轮记录。
// 返回 (作者ID, 订阅者B, 订阅者C, 项目ID, B的轮次数, C的轮次数)。
func seedSubscribers(t *testing.T, s *Store, bRounds, cRounds int) (int64, int64, int64, int64) {
	t.Helper()

	owner := seedUser(t, s, "owner", true)
	bob := seedUser(t, s, "bob", false)
	carol := seedUser(t, s, "carol", false)
	p := newProject(t, s, owner, "共享项目")

	for _, sub := range []int64{bob, carol} {
		if err := s.Subscribe(sub, p.ID); err != nil {
			t.Fatalf("Subscribe 失败: %v", err)
		}
	}

	addRounds := func(userID int64, n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			if _, err := s.CreateRound(NewRound{
				ProjectID: p.ID, UserID: userID, Seed: int64(i + 1),
				StartedAt: Now(), FinishedAt: Now(),
				TotalMs: 1000, QuestionCount: 1, CorrectCount: 1,
				Attempts: []NewAttempt{{
					Index: 0, QSnapshot: "1+1", ASnapshot: "2",
					EnvelopeJSON: `{"kind":"rational","num":"2","den":"1"}`,
					UserInput:    "2", ClientIsCorrect: true, ServerIsCorrect: true, ElapsedMs: 1000,
				}},
			}); err != nil {
				t.Fatalf("CreateRound 失败: %v", err)
			}
		}
	}
	addRounds(owner, 2)
	addRounds(bob, bRounds)
	addRounds(carol, cRounds)

	return owner, bob, carol, p.ID
}

func countRoundsFor(t *testing.T, s *Store, userID, projectID int64) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM practice_round WHERE user_id = ? AND project_id = ?`,
		userID, projectID).Scan(&n); err != nil {
		t.Fatalf("统计轮次失败: %v", err)
	}
	return n
}

func countAttemptsFor(t *testing.T, s *Store, userID, projectID int64) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM attempt a JOIN practice_round r ON r.id = a.round_id
		 WHERE r.user_id = ? AND r.project_id = ?`,
		userID, projectID).Scan(&n); err != nil {
		t.Fatalf("统计题目记录失败: %v", err)
	}
	return n
}

// TestUnsubscribeIsolation 是 A6 的数据层断言：
// 退订只清本人记录，他人记录与项目本身完好。
//
// 放在 store 包而非 api 包：这里能直接验证数据层语义，
// 不必经过 HTTP 层，失败时定位更直接。
func TestUnsubscribeIsolation(t *testing.T) {
	s := newTestStore(t)
	owner, bob, carol, projectID := seedSubscribers(t, s, 3, 4)

	if got := countRoundsFor(t, s, bob, projectID); got != 3 {
		t.Fatalf("种子异常: bob 应有 3 轮, 得到 %d", got)
	}

	deleted, err := s.Unsubscribe(bob, projectID)
	if err != nil {
		t.Fatalf("Unsubscribe 失败: %v", err)
	}
	if deleted != 3 {
		t.Errorf("deletedRounds = %d, 期望 3（界面据此提示代价）", deleted)
	}

	// 1) 本人记录消失
	if got := countRoundsFor(t, s, bob, projectID); got != 0 {
		t.Errorf("bob 的轮次残留 = %d, 期望 0", got)
	}
	// attempt 应随外键级联删除 —— 应用层不应再写一遍删除逻辑
	if got := countAttemptsFor(t, s, bob, projectID); got != 0 {
		t.Errorf("bob 的题目记录残留 = %d, 期望 0（外键级联）", got)
	}

	// 2) 他人记录完好
	if got := countRoundsFor(t, s, owner, projectID); got != 2 {
		t.Errorf("作者的轮次被误删: %d, 期望 2", got)
	}
	if got := countRoundsFor(t, s, carol, projectID); got != 4 {
		t.Errorf("其他订阅者的轮次被误删: %d, 期望 4", got)
	}

	// 3) 订阅关系只删自己那条
	if sub, _ := s.IsSubscribed(bob, projectID); sub {
		t.Error("bob 的订阅关系应已删除")
	}
	if sub, _ := s.IsSubscribed(carol, projectID); !sub {
		t.Error("carol 的订阅关系不应受影响")
	}

	// 4) 项目本身仍在
	if _, err := s.GetProject(projectID); err != nil {
		t.Errorf("项目不应被删除: %v", err)
	}
}

// TestUnsubscribeOnlyTargetProject 确认退订不影响同一用户的其他项目。
func TestUnsubscribeOnlyTargetProject(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	bob := seedUser(t, s, "bob", false)

	p1 := newProject(t, s, owner, "项目一")
	p2 := newProject(t, s, owner, "项目二")
	for _, pid := range []int64{p1.ID, p2.ID} {
		if err := s.Subscribe(bob, pid); err != nil {
			t.Fatalf("Subscribe 失败: %v", err)
		}
	}

	add := func(projectID int64, n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			if _, err := s.CreateRound(NewRound{
				ProjectID: projectID, UserID: bob, Seed: int64(i), StartedAt: Now(), FinishedAt: Now(),
				TotalMs: 100, QuestionCount: 1, CorrectCount: 1,
			}); err != nil {
				t.Fatalf("CreateRound 失败: %v", err)
			}
		}
	}
	add(p1.ID, 2)
	add(p2.ID, 5)

	if _, err := s.Unsubscribe(bob, p1.ID); err != nil {
		t.Fatalf("Unsubscribe 失败: %v", err)
	}

	if got := countRoundsFor(t, s, bob, p1.ID); got != 0 {
		t.Errorf("项目一的记录应被清除, 残留 %d", got)
	}
	if got := countRoundsFor(t, s, bob, p2.ID); got != 5 {
		t.Errorf("项目二的记录不应受影响, 得到 %d, 期望 5", got)
	}
	if sub, _ := s.IsSubscribed(bob, p2.ID); !sub {
		t.Error("项目二的订阅关系不应受影响")
	}
}

// TestUnsubscribeWithoutSubscriptionFails 确认未订阅时明确报错。
//
// 静默成功会让界面显示「已退订」而实际什么都没发生。
func TestUnsubscribeWithoutSubscriptionFails(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	stranger := seedUser(t, s, "stranger", false)
	p := newProject(t, s, owner, "项目")

	if _, err := s.Unsubscribe(stranger, p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("未订阅时退订应返回 ErrNotFound, 得到 %v", err)
	}
}

// TestUnsubscribeTwiceFails 确认重复退订明确报错。
func TestUnsubscribeTwiceFails(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	bob := seedUser(t, s, "bob", false)
	p := newProject(t, s, owner, "项目")
	if err := s.Subscribe(bob, p.ID); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}

	if _, err := s.Unsubscribe(bob, p.ID); err != nil {
		t.Fatalf("首次退订应成功: %v", err)
	}
	if _, err := s.Unsubscribe(bob, p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("重复退订应返回 ErrNotFound, 得到 %v", err)
	}
}

// TestUnsubscribeDoesNotTouchProjectOwner 确认作者不是订阅者时不受影响。
func TestUnsubscribeDoesNotTouchProjectOwner(t *testing.T) {
	s := newTestStore(t)
	owner, bob, _ /*carol*/, projectID := seedSubscribers(t, s, 2, 2)

	if _, err := s.Unsubscribe(bob, projectID); err != nil {
		t.Fatalf("Unsubscribe 失败: %v", err)
	}

	// 作者对该项目仍有访问权（他是 owner，不需要订阅行）
	access, err := s.ProjectAccess(owner, projectID)
	if err != nil {
		t.Fatalf("作者应仍有访问权: %v", err)
	}
	if access != AccessOwner {
		t.Errorf("访问权 = %q, 期望 %q", access, AccessOwner)
	}
}

// ---------------------------------------------------------------------------
// 分享 token
// ---------------------------------------------------------------------------

func TestShareTokenLifecycle(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	p := newProject(t, s, owner, "项目")

	// 未分享时按 token 查不到
	if _, err := s.ProjectByShareToken("anything"); !errors.Is(err, ErrNotFound) {
		t.Errorf("未分享时查询应返回 ErrNotFound, 得到 %v", err)
	}

	if err := s.SetShareToken(p.ID, "tok1"); err != nil {
		t.Fatalf("SetShareToken 失败: %v", err)
	}
	got, err := s.ProjectByShareToken("tok1")
	if err != nil {
		t.Fatalf("按 token 查询失败: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("查到项目 %d, 期望 %d", got.ID, p.ID)
	}

	// 重置
	if err := s.SetShareToken(p.ID, "tok2"); err != nil {
		t.Fatalf("重置失败: %v", err)
	}
	if _, err := s.ProjectByShareToken("tok1"); !errors.Is(err, ErrNotFound) {
		t.Error("重置后旧 token 应失效")
	}
	if _, err := s.ProjectByShareToken("tok2"); err != nil {
		t.Errorf("新 token 应可用: %v", err)
	}

	// 撤销
	if err := s.ClearShareToken(p.ID); err != nil {
		t.Fatalf("撤销失败: %v", err)
	}
	if _, err := s.ProjectByShareToken("tok2"); !errors.Is(err, ErrNotFound) {
		t.Error("撤销后 token 应失效")
	}
}

// TestEmptyShareTokenNeverMatches 是一个边界断言。
//
// 撤销分享会把 share_token 置为 NULL。若查询不排除空 token，
// 一个「token 为空串」的请求可能匹配到未分享的项目，造成意外的订阅成功。
func TestEmptyShareTokenNeverMatches(t *testing.T) {
	s := newTestStore(t)
	owner := seedUser(t, s, "owner", false)
	_ = newProject(t, s, owner, "未分享的项目")

	if _, err := s.ProjectByShareToken(""); !errors.Is(err, ErrNotFound) {
		t.Errorf("空 token 应始终返回 ErrNotFound, 得到 %v", err)
	}
}

func TestSetShareTokenOnMissingProject(t *testing.T) {
	s := newTestStore(t)
	if err := s.SetShareToken(999999, "tok"); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在的项目应返回 ErrNotFound, 得到 %v", err)
	}
	if err := s.ClearShareToken(999999); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在的项目应返回 ErrNotFound, 得到 %v", err)
	}
}

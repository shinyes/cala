package store

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// newTestStore 为每个测试建立独立的临时数据库。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenAppliesMigrations(t *testing.T) {
	s := newTestStore(t)

	want := []string{"user", "session", "setting", "project", "subscription", "practice_round", "attempt"}
	for _, table := range want {
		var name string
		err := s.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("表 %s 不存在: %v", table, err)
		}
	}

	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM schema_migration`).Scan(&n); err != nil {
		t.Fatalf("查询 schema_migration 失败: %v", err)
	}
	if n != 1 {
		t.Errorf("已应用迁移数 = %d, 期望 1", n)
	}
}

// TestMigrateIsIdempotent 保证重复 Open 不会重复执行迁移。
func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("首次 Open 失败: %v", err)
	}
	first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("二次 Open 失败: %v", err)
	}
	defer second.Close()

	var n int
	if err := second.DB().QueryRow(`SELECT COUNT(*) FROM schema_migration`).Scan(&n); err != nil {
		t.Fatalf("查询 schema_migration 失败: %v", err)
	}
	if n != 1 {
		t.Errorf("二次 Open 后已应用迁移数 = %d, 期望仍为 1", n)
	}
}

// TestPragmasEnableForeignKeysAcrossConnections 是兼容边界第 2 条的 falsifier。
//
// 它直接针对 `pragmas` 常量建一个**多连接**连接池，逐个检查每个连接的外键开关。
// 之所以不复用 Store 自己的池：Store 刻意设了 MaxOpenConns(1) 以消除写-写竞争，
// 而单连接池会让「pragma 写在 DSN 里」与「pragma 写在 Open 之后」两种做法
// 表现一致，从而掩盖这个差异。本测试用独立的多连接池把该差异暴露出来。
//
// 若有人把 foreign_keys 从 `pragmas` 中移除，只有部分连接会启用外键，
// 本测试即失败。
func TestPragmasEnableForeignKeysAcrossConnections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pragma.db")

	dsn := fmt.Sprintf("file:%s?%s", filepath.ToSlash(path), pragmas)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	const n = 4
	db.SetMaxOpenConns(n)
	db.SetMaxIdleConns(n)

	// 同时持有 n 个连接，确保它们是不同的物理连接
	conns := make([]*sql.Conn, 0, n)
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	for i := 0; i < n; i++ {
		conn, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("取第 %d 个连接失败: %v", i, err)
		}
		conns = append(conns, conn)

		var fk int
		if err := conn.QueryRowContext(t.Context(), `PRAGMA foreign_keys`).Scan(&fk); err != nil {
			t.Fatalf("查询 PRAGMA foreign_keys 失败: %v", err)
		}
		if fk != 1 {
			t.Fatalf("第 %d 个连接 foreign_keys = %d, 期望 1 —— pragmas 常量未包含 foreign_keys(1)", i, fk)
		}
	}
}

// TestStorePoolIsSingleConnection 记录 Store 的连接池约定。
// 该约定是「用串行化换掉一整类写-写竞争复杂度」这一设计决策的可执行表达；
// 若有人放宽它，应当是有意为之并同步更新本测试与 store.go 的退役触发条件注释。
func TestStorePoolIsSingleConnection(t *testing.T) {
	s := newTestStore(t)
	if got := s.DB().Stats().MaxOpenConnections; got != 1 {
		t.Errorf("MaxOpenConnections = %d, 期望 1", got)
	}

	var fk int
	if err := s.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatalf("查询 PRAGMA foreign_keys 失败: %v", err)
	}
	if fk != 1 {
		t.Errorf("Store 连接 foreign_keys = %d, 期望 1", fk)
	}
}

// seedProjectWithRounds 建 作者/订阅者/旁观者、两个项目与若干轮答题。
//
// 种子规模（后续测试的期望值以此为准）：
//   - 项目 10：作者 / bob / carol 各 1 轮，每轮 1 题 → 3 轮、3 题
//   - 项目 20：作者 1 轮，0 题 → 1 轮、0 题
//   - 合计：4 轮、3 题；订阅 2 行
func seedProjectWithRounds(t *testing.T, s *Store) {
	t.Helper()
	now := Now()
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.DB().Exec(q, args...); err != nil {
			t.Fatalf("执行失败: %v\nSQL: %s", err, q)
		}
	}
	mustExec(`INSERT INTO "user"(id,username,password_hash,is_admin,created_at) VALUES
		(1,'author','x',1,?),(2,'bob','x',0,?),(3,'carol','x',0,?)`, now, now, now)
	mustExec(`INSERT INTO project(id,owner_id,title,question_count,rule_source,created_at,updated_at)
		VALUES (10,1,'口算',10,'function generate(cfg){return {q:"1+1",a:"2"}}',?,?)`, now, now)
	mustExec(`INSERT INTO project(id,owner_id,title,question_count,rule_source,created_at,updated_at)
		VALUES (20,1,'竖式',5,'function generate(cfg){return {q:"2+2",a:"4"}}',?,?)`, now, now)
	mustExec(`INSERT INTO subscription(user_id,project_id,created_at) VALUES (2,10,?),(3,10,?)`, now, now)

	// 三人在项目 10 各做一轮
	for _, u := range []int{1, 2, 3} {
		res, err := s.DB().Exec(`INSERT INTO practice_round(project_id,user_id,seed,started_at,finished_at,total_ms,question_count,correct_count)
			VALUES (10,?,1,?,?,1000,1,1)`, u, now, now)
		if err != nil {
			t.Fatalf("插入 round 失败: %v", err)
		}
		rid, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("取 LastInsertId 失败: %v", err)
		}
		mustExec(`INSERT INTO attempt(round_id,idx,q_snapshot,a_snapshot,a_envelope_json,user_input,client_is_correct,server_is_correct,elapsed_ms)
			VALUES (?,0,'1+1','2','{"kind":"rational","num":"2","den":"1"}','2',1,1,500)`, rid)
	}
	// 项目 20 一轮（无 attempt）
	mustExec(`INSERT INTO practice_round(project_id,user_id,seed,started_at,finished_at,total_ms,question_count,correct_count)
		VALUES (20,1,1,?,?,500,1,1)`, now, now)
}

func count(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("统计 %s 失败: %v", table, err)
	}
	return n
}

// TestCascadeOnProjectDelete 覆盖功能8：作者删项目 -> 所有人记录一起删。
func TestCascadeOnProjectDelete(t *testing.T) {
	s := newTestStore(t)
	seedProjectWithRounds(t, s)

	if got := count(t, s, "practice_round"); got != 4 {
		t.Fatalf("种子异常: practice_round = %d, 期望 4", got)
	}

	if _, err := s.DB().Exec(`DELETE FROM project WHERE id=10`); err != nil {
		t.Fatalf("删除项目失败: %v", err)
	}

	// 项目 10 的 3 轮 3 题全部消失；项目 20 的 1 轮保留
	if got := count(t, s, "practice_round"); got != 1 {
		t.Errorf("practice_round 剩余 = %d, 期望 1（仅项目 20）", got)
	}
	if got := count(t, s, "attempt"); got != 0 {
		t.Errorf("attempt 剩余 = %d, 期望 0", got)
	}
	if got := count(t, s, "subscription"); got != 0 {
		t.Errorf("subscription 剩余 = %d, 期望 0", got)
	}
}

// TestCascadeOnUnsubscribe 覆盖 D4：退订仅清除本人历史，他人不受影响。
func TestCascadeOnUnsubscribe(t *testing.T) {
	s := newTestStore(t)
	seedProjectWithRounds(t, s)

	// 退订 = 删 subscription + 显式删本人在本项目的 rounds（attempt 由外键级联）
	if _, err := s.DB().Exec(`DELETE FROM subscription WHERE user_id=2 AND project_id=10`); err != nil {
		t.Fatalf("删除订阅失败: %v", err)
	}
	if _, err := s.DB().Exec(`DELETE FROM practice_round WHERE project_id=10 AND user_id=2`); err != nil {
		t.Fatalf("删除本人轮次失败: %v", err)
	}

	var bobLeft int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM practice_round WHERE project_id=10 AND user_id=2`).Scan(&bobLeft); err != nil {
		t.Fatalf("查询 bob 残留失败: %v", err)
	}
	if bobLeft != 0 {
		t.Errorf("bob 残留轮次 = %d, 期望 0", bobLeft)
	}

	// 种子共 4 轮 3 题。bob 的 1 轮被显式删除，其 1 题由外键级联删除。
	// 剩余：作者(项目10) + carol(项目10) + 作者(项目20) = 3 轮；3-1 = 2 题。
	if got := count(t, s, "practice_round"); got != 3 {
		t.Errorf("practice_round 剩余 = %d, 期望 3", got)
	}
	if got := count(t, s, "attempt"); got != 2 {
		t.Errorf("attempt 剩余 = %d, 期望 2（bob 那题应被级联删除）", got)
	}

	// 必须确认作者与 carol 的数据完好，否则「退订误删他人数据」不会被发现
	var others int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM practice_round WHERE project_id=10 AND user_id IN (1,3)`).Scan(&others); err != nil {
		t.Fatalf("查询他人数据失败: %v", err)
	}
	if others != 2 {
		t.Errorf("作者与 carol 在项目 10 的轮次 = %d, 期望 2", others)
	}
}

// TestAttemptUniquePerRound 保证同一轮内题号不重复。
func TestAttemptUniquePerRound(t *testing.T) {
	s := newTestStore(t)
	seedProjectWithRounds(t, s)

	var rid int64
	if err := s.DB().QueryRow(`SELECT id FROM practice_round LIMIT 1`).Scan(&rid); err != nil {
		t.Fatalf("取 round 失败: %v", err)
	}
	_, err := s.DB().Exec(`INSERT INTO attempt(round_id,idx,q_snapshot,a_snapshot,a_envelope_json,user_input,client_is_correct,server_is_correct,elapsed_ms)
		VALUES (?,0,'dup','1','{}','1',1,1,1)`, rid)
	if err == nil {
		t.Fatal("同轮重复 idx 应当违反 UNIQUE(round_id, idx)")
	}
}

// ---------- P1.3 用户 / 会话 / 设置 ----------

func TestCountUsersStartsAtZero(t *testing.T) {
	s := newTestStore(t)
	n, err := s.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers 失败: %v", err)
	}
	if n != 0 {
		t.Errorf("新库用户数 = %d, 期望 0（引导管理员判定依赖此值）", n)
	}
}

func TestCreateUserAndLookup(t *testing.T) {
	s := newTestStore(t)

	u, err := s.CreateUser("admin", "hash", true)
	if err != nil {
		t.Fatalf("CreateUser 失败: %v", err)
	}
	if u.ID == 0 || !u.IsAdmin {
		t.Errorf("返回值异常: %#v", u)
	}
	if u.CreatedAt == "" {
		t.Error("CreatedAt 不应为空")
	}

	if _, err := s.CreateUser("admin", "hash2", false); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("重复用户名应返回 ErrUsernameTaken, 得到 %v", err)
	}

	got, hash, err := s.AuthenticateLookup("admin")
	if err != nil {
		t.Fatalf("AuthenticateLookup 失败: %v", err)
	}
	if got.ID != u.ID || hash != "hash" {
		t.Errorf("查询结果不符: %#v hash=%q", got, hash)
	}
	if !got.IsAdmin {
		t.Error("管理员标记应被读出")
	}

	if _, _, err := s.AuthenticateLookup("nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在用户应返回 ErrNotFound, 得到 %v", err)
	}
}

func TestGetUser(t *testing.T) {
	s := newTestStore(t)
	created, err := s.CreateUser("bob", "hash", false)
	if err != nil {
		t.Fatalf("CreateUser 失败: %v", err)
	}

	got, err := s.GetUser(created.ID)
	if err != nil {
		t.Fatalf("GetUser 失败: %v", err)
	}
	if got.Username != "bob" || got.IsAdmin || got.Disabled {
		t.Errorf("用户表示不符: %#v", got)
	}

	if _, err := s.GetUser(99999); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在 ID 应返回 ErrNotFound, 得到 %v", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := newTestStore(t)
	u, err := s.CreateUser("bob", "hash", false)
	if err != nil {
		t.Fatalf("CreateUser 失败: %v", err)
	}

	if err := s.CreateSession("tok", u.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}
	got, err := s.SessionUser("tok")
	if err != nil {
		t.Fatalf("SessionUser 失败: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("会话用户 = %d, 期望 %d", got.ID, u.ID)
	}

	// 过期会话不可用
	if err := s.CreateSession("expired", u.ID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}
	if _, err := s.SessionUser("expired"); !errors.Is(err, ErrNotFound) {
		t.Errorf("过期会话应返回 ErrNotFound, 得到 %v", err)
	}

	// 未知令牌不可用
	if _, err := s.SessionUser("never-issued"); !errors.Is(err, ErrNotFound) {
		t.Errorf("未知令牌应返回 ErrNotFound, 得到 %v", err)
	}

	if err := s.DeleteSession("tok"); err != nil {
		t.Fatalf("DeleteSession 失败: %v", err)
	}
	if _, err := s.SessionUser("tok"); !errors.Is(err, ErrNotFound) {
		t.Errorf("已吊销会话应返回 ErrNotFound, 得到 %v", err)
	}
}

// TestSessionUserRejectsDisabledUser 覆盖「禁用账号应立即失去访问权」。
// D7 决定暂不提供禁用入口，但数据模型与查询语义需就位，否则日后启用该功能时
// 会出现「已禁用用户仍可凭旧令牌访问」的漏洞。
func TestSessionUserRejectsDisabledUser(t *testing.T) {
	s := newTestStore(t)
	u, err := s.CreateUser("bob", "hash", false)
	if err != nil {
		t.Fatalf("CreateUser 失败: %v", err)
	}
	if err := s.CreateSession("tok", u.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}

	if _, err := s.DB().Exec(`UPDATE "user" SET disabled = 1 WHERE id = ?`, u.ID); err != nil {
		t.Fatalf("禁用用户失败: %v", err)
	}

	if _, err := s.SessionUser("tok"); !errors.Is(err, ErrNotFound) {
		t.Errorf("已禁用用户的会话应返回 ErrNotFound, 得到 %v", err)
	}
}

// TestDeleteUserSessionsRevokesAll 校验「一次吊销该用户全部会话」。
func TestDeleteUserSessionsRevokesAll(t *testing.T) {
	s := newTestStore(t)
	u, err := s.CreateUser("bob", "hash", false)
	if err != nil {
		t.Fatalf("CreateUser 失败: %v", err)
	}
	other, err := s.CreateUser("carol", "hash", false)
	if err != nil {
		t.Fatalf("CreateUser 失败: %v", err)
	}
	for _, tok := range []string{"t1", "t2", "t3"} {
		if err := s.CreateSession(tok, u.ID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("CreateSession 失败: %v", err)
		}
	}
	if err := s.CreateSession("keep", other.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}

	if err := s.DeleteUserSessions(u.ID); err != nil {
		t.Fatalf("DeleteUserSessions 失败: %v", err)
	}

	for _, tok := range []string{"t1", "t2", "t3"} {
		if _, err := s.SessionUser(tok); !errors.Is(err, ErrNotFound) {
			t.Errorf("令牌 %s 应已失效, 得到 %v", tok, err)
		}
	}
	// 他人会话不受影响
	if _, err := s.SessionUser("keep"); err != nil {
		t.Errorf("他人会话不应被吊销: %v", err)
	}
}

func TestRegistrationOpenDefaultsAndPersists(t *testing.T) {
	s := newTestStore(t)

	got, err := s.RegistrationOpen(true)
	if err != nil {
		t.Fatalf("RegistrationOpen 失败: %v", err)
	}
	if !got {
		t.Error("首次读取应返回 fallback=true")
	}

	// fallback 应已落库，故换成 false 作为 fallback 也仍读到 true
	got, err = s.RegistrationOpen(false)
	if err != nil {
		t.Fatalf("RegistrationOpen 失败: %v", err)
	}
	if !got {
		t.Error("首次写入后应读到已持久化的 true, 而非新 fallback")
	}

	if err := s.SetRegistrationOpen(false); err != nil {
		t.Fatalf("SetRegistrationOpen 失败: %v", err)
	}
	got, err = s.RegistrationOpen(true)
	if err != nil {
		t.Fatalf("RegistrationOpen 失败: %v", err)
	}
	if got {
		t.Error("写入 false 后应返回 false, 而不是 fallback")
	}
}

// TestRegistrationOpenSurvivesCorruptValue 保证存量值损坏时不致服务不可用。
func TestRegistrationOpenSurvivesCorruptValue(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.DB().Exec(
		`INSERT INTO setting(key,value) VALUES (?,?)`, SettingRegistrationOpen, "not-a-bool"); err != nil {
		t.Fatalf("写入损坏值失败: %v", err)
	}

	got, err := s.RegistrationOpen(false)
	if err != nil {
		t.Fatalf("损坏值不应导致报错: %v", err)
	}
	if got {
		t.Error("损坏值应退回 fallback=false")
	}
}

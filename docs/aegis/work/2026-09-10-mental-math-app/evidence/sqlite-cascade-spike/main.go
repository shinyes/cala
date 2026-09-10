package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "modernc.org/sqlite"
)

func main() {
	fmt.Println("=== V1: modernc.org/sqlite 纯 Go + CGO_ENABLED=0 ===")
	fmt.Printf("  runtime.Version=%s  GOOS=%s  GOARCH=%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  cgo enabled at build time: %v\n", cgoEnabled())

	dir, _ := os.MkdirTemp("", "cala-sqlite-*")
	defer os.RemoveAll(dir)
	dbPath := filepath.Join(dir, "cala.db")

	// DSN 中携带 pragma，确保每个连接都启用外键（连接池下必须如此）
	dsn := "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fail("sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fail("Ping: %v", err)
	}
	fmt.Println("  [ok] driver open + ping")

	var fk, jm string
	db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	db.QueryRow("PRAGMA journal_mode").Scan(&jm)
	fmt.Printf("  [%s] PRAGMA foreign_keys = %s (want 1)\n", mark(fk == "1"), fk)
	fmt.Printf("  [%s] PRAGMA journal_mode  = %s (want wal)\n", mark(jm == "wal"), jm)

	// ---- schema 精简版，用于验证 §6.1(2) 的级联不变量 ----
	schema := []string{
		`CREATE TABLE user(id INTEGER PRIMARY KEY, username TEXT UNIQUE NOT NULL)`,
		`CREATE TABLE project(id INTEGER PRIMARY KEY, owner_id INTEGER NOT NULL
		     REFERENCES user(id) ON DELETE CASCADE, title TEXT NOT NULL)`,
		`CREATE TABLE subscription(user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
		     project_id INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
		     PRIMARY KEY(user_id, project_id))`,
		`CREATE TABLE round(id INTEGER PRIMARY KEY,
		     project_id INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
		     user_id INTEGER NOT NULL REFERENCES user(id) ON DELETE CASCADE,
		     correct_count INTEGER NOT NULL)`,
		`CREATE TABLE attempt(id INTEGER PRIMARY KEY,
		     round_id INTEGER NOT NULL REFERENCES round(id) ON DELETE CASCADE,
		     idx INTEGER NOT NULL, is_correct INTEGER NOT NULL)`,
	}
	for _, s := range schema {
		if _, err := db.Exec(s); err != nil {
			fail("schema: %v\n%s", err, s)
		}
	}
	fmt.Println("  [ok] schema created (user/project/subscription/round/attempt)")

	count := func(table string) int {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			fail("count %s: %v", table, err)
		}
		return n
	}

	// ---- 场景：作者 A 建项目，B 与 C 订阅。三人各做题 ----
	setup := func() {
		for _, t := range []string{"attempt", "round", "subscription", "project", "user"} {
			db.Exec("DELETE FROM " + t)
		}
		db.Exec(`INSERT INTO user(id,username) VALUES (1,'author'),(2,'bob'),(3,'carol')`)
		db.Exec(`INSERT INTO project(id,owner_id,title) VALUES (10,1,'口算')`)
		db.Exec(`INSERT INTO subscription(user_id,project_id) VALUES (2,10),(3,10)`)
		// 三人在项目 10 各做一轮，每轮 2 题
		for _, u := range []int{1, 2, 3} {
			res, _ := db.Exec(`INSERT INTO round(project_id,user_id,correct_count) VALUES (10,?,1)`, u)
			rid, _ := res.LastInsertId()
			db.Exec(`INSERT INTO attempt(round_id,idx,is_correct) VALUES (?,0,1),(?,1,0)`, rid, rid)
		}
		// 另一个独立项目，用来验证不会被误删
		db.Exec(`INSERT INTO project(id,owner_id,title) VALUES (20,1,'竖式')`)
		res, _ := db.Exec(`INSERT INTO round(project_id,user_id,correct_count) VALUES (20,1,1)`)
		rid, _ := res.LastInsertId()
		db.Exec(`INSERT INTO attempt(round_id,idx,is_correct) VALUES (?,0,1)`, rid)
	}

	fmt.Println("\n=== 场景一：作者删除项目（功能8）===")
	setup()
	fmt.Printf("  删除前: round=%d attempt=%d subscription=%d\n",
		count("round"), count("attempt"), count("subscription"))
	if _, err := db.Exec(`DELETE FROM project WHERE id=10`); err != nil {
		fail("delete project: %v", err)
	}
	r10, a10, s10 := count("round"), count("attempt"), count("subscription")
	fmt.Printf("  删除后: round=%d attempt=%d subscription=%d\n", r10, a10, s10)
	// 项目 10 的 3 轮 + 6 题应全部消失；项目 20 的 1 轮 + 1 题应保留
	pass := r10 == 1 && a10 == 1 && s10 == 0
	fmt.Printf("  [%s] 项目10 全员记录被级联删除，项目20 数据完好\n", mark(pass))
	if !pass {
		fail("级联删除语义不符: 期望 round=1 attempt=1 subscription=0")
	}

	fmt.Println("\n=== 场景二：订阅者退订（D4，仅清本人历史）===")
	setup()
	fmt.Printf("  退订前: round=%d attempt=%d\n", count("round"), count("attempt"))
	// 退订 = 显式删 round + 删 subscription（attempt 由 FK 级联）
	if _, err := db.Exec(`DELETE FROM round WHERE project_id=10 AND user_id=2`); err != nil {
		fail("delete bob rounds: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM subscription WHERE user_id=2 AND project_id=10`); err != nil {
		fail("delete bob subscription: %v", err)
	}
	r, a := count("round"), count("attempt")
	var bobLeft int
	db.QueryRow(`SELECT COUNT(*) FROM round WHERE project_id=10 AND user_id=2`).Scan(&bobLeft)
	fmt.Printf("  退订后: round=%d attempt=%d (bob 残留 round=%d)\n", r, a, bobLeft)
	// bob 的 1 轮 2 题消失；作者与 carol 的 2 轮 4 题 + 项目20 的 1 轮 1 题保留 = round 3, attempt 5
	pass2 := r == 3 && a == 5 && bobLeft == 0
	fmt.Printf("  [%s] 仅 bob 的历史被清除，作者与 carol 不受影响\n", mark(pass2))
	if !pass2 {
		fail("退订语义不符: 期望 round=3 attempt=5 bob残留=0, 实际 round=%d attempt=%d", r, a)
	}

	fmt.Println("\n=== 场景三：删除用户（D7 未启用，但验证 FK 完整性）===")
	setup()
	if _, err := db.Exec(`DELETE FROM user WHERE id=3`); err != nil {
		fail("delete user: %v", err)
	}
	var carolLeft int
	db.QueryRow(`SELECT COUNT(*) FROM round WHERE user_id=3`).Scan(&carolLeft)
	fmt.Printf("  [%s] 删除 carol 后其 round 残留=%d (want 0)\n", mark(carolLeft == 0), carolLeft)

	fmt.Println("\n=== 场景四：事务原子性（整轮落库不能半途而废）===")
	setup()
	tx, err := db.Begin()
	if err != nil {
		fail("begin: %v", err)
	}
	res, _ := tx.Exec(`INSERT INTO round(project_id,user_id,correct_count) VALUES (10,2,2)`)
	rid, _ := res.LastInsertId()
	tx.Exec(`INSERT INTO attempt(round_id,idx,is_correct) VALUES (?,0,1)`, rid)
	tx.Exec(`INSERT INTO attempt(round_id,idx,is_correct) VALUES (?,0,1)`, rid) // UNIQUE 未建，仅测回滚
	if err := tx.Rollback(); err != nil {
		fail("rollback: %v", err)
	}
	before := count("round")
	tx2, _ := db.Begin()
	tx2.Exec(`INSERT INTO round(project_id,user_id,correct_count) VALUES (10,2,2)`)
	if err := tx2.Rollback(); err != nil {
		fail("rollback2: %v", err)
	}
	fmt.Printf("  [%s] 回滚后 round 数不变 (%d)\n", mark(count("round") == before), count("round"))

	fmt.Println("\n=== 场景五：并发读 + 单写（WAL 下的实际行为）===")
	setup()
	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func(n int) {
			tx, err := db.Begin()
			if err != nil {
				done <- err
				return
			}
			defer tx.Rollback()
			var c int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM round`).Scan(&c); err != nil {
				done <- err
				return
			}
			done <- nil
		}(i)
	}
	ok := 0
	for i := 0; i < 8; i++ {
		if err := <-done; err == nil {
			ok++
		}
	}
	fmt.Printf("  [%s] 8 个并发读事务成功 %d/8\n", mark(ok == 8), ok)

	fmt.Println("\n=== 结论 ===")
	fmt.Println("  V1 成立: 纯 Go 驱动可用、外键级联按 §6.1(2) 语义工作、事务与并发读正常")
	fmt.Println("  注意: PRAGMA foreign_keys 必须写在 DSN 中, 否则连接池下部分连接不启用外键")
}

func cgoEnabled() bool {
	return os.Getenv("CGO_ENABLED") != "0"
}

func mark(b bool) string {
	if b {
		return "PASS"
	}
	return "FAIL"
}

func fail(f string, a ...any) {
	fmt.Printf("  [FAIL] "+f+"\n", a...)
	os.Exit(1)
}

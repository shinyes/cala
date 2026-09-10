# 实施计划 —— P0 骨架与 P1 认证

- Date: `2026-09-10`
- Design Spec: [`docs/aegis/specs/2026-09-10-mental-math-app-design.md`](../specs/2026-09-10-mental-math-app-design.md)
- Baseline: [`docs/aegis/baseline/2026-09-10-initial-baseline.md`](../baseline/2026-09-10-initial-baseline.md)
- 覆盖阶段：规格 §16 的 **P0 骨架与风险先行** 与 **P1 认证与引导**
- 后续阶段（P2/P3/P3.5/P4/P5/P6/P7）在本计划验收通过后各自另立计划

---

## Goal

建立一个可启动、可测试的 Go 服务端骨架，并完成账号与引导流程：首个注册用户自动成为
管理员、管理员可开关注册、会话基于可吊销的不透明 token。同时完成 Flutter 工程初始化与
多阶段 Dockerfile，使 P0 出口条件（`go build` 通过、容器可启动）可验证。

**本计划不包含**：项目 CRUD、规则引擎、练习运行时、统计、订阅、CI 工作流。

## Architecture

```
backend/
  cmd/server/main.go            装配与启动
  internal/config/              环境变量配置
  internal/store/               SQLite: 连接、迁移、查询
  internal/api/                 Fiber: 路由、中间件、handler
  internal/service/             业务逻辑（P1 起）
  internal/auth/                口令哈希与会话 token（P1 起）
  migrations/                   内嵌 SQL，按序应用
app/                            Flutter 工程（P0.5 初始化）
```

依赖方向：`api → service → store`；`service → auth`。禁止反向依赖。

## Tech Stack

| 项 | 版本/选择 | 依据 |
|---|---|---|
| Go | 1.25.5（本机） | 已确认 |
| Fiber | `github.com/gofiber/fiber/v2` | 需求指定 |
| SQLite 驱动 | `modernc.org/sqlite v1.58.0` | **V1 已实测通过**（纯 Go，`CGO_ENABLED=0` 静态构建产出 6.08 MB 二进制） |
| 口令哈希 | `golang.org/x/crypto/bcrypt` | 成熟 |
| Flutter | 3.44.9 / Dart 3.12.2（本机） | 已确认 |

## Baseline / Authority Refs

- 设计规格 §6（数据模型与不变量）、§7（API 契约）、§6.2（引导管理员）、§10.3（应用标识）
- 初始基线 §4.2（不可协商项 1/2）、§5.2（架构不可协商项）、§9（兼容边界）
- 证据：[`evidence/sqlite-cascade-spike/`](../work/2026-09-10-mental-math-app/evidence/sqlite-cascade-spike/OUTPUT.txt)

## Compatibility Boundary

本计划必须维持：

1. **`(project, user)` 单一级联不变量**——P0 建表时必须带上正确的 `ON DELETE CASCADE`
   （已实测语义正确，见证据）。
2. **`PRAGMA foreign_keys` 必须写在 DSN 中**——实测发现：若在 `Open` 之后用
   `db.Exec("PRAGMA foreign_keys=ON")` 设置，连接池下只有部分连接生效，
   会导致级联删除**静默失效**，直接破坏功能8 与 D4。这是本项目最容易踩的坑。
3. **首个用户必须在注册开关关闭时仍可注册**，否则全新部署自我锁死（规格 §6.2）。
4. `server_is_correct` 的权威地位（本计划只建表，不写入）。

## TDD Route

```text
TDD Route:
- Mode: off
- Decision: skipped
- Strict authority: not applicable
- Test posture: post-change regression
- Reason: 项目配置 tdd_mode = "off"，用户未要求严格 TDD；Aegis 禁止仅凭风险推断 strict
- Verification: 每个 Task 完成后运行 `go test ./...`（含新增回归测试）
```

> 虽然 Decision 是 `skipped`，本计划仍为**契约承载点**（迁移、级联、引导管理员、
> token 哈希）编写回归测试——这些是「错了就静默损坏数据」的地方，
> 属 proportional regression，而非 RED/GREEN 循环。

## Verification（全局命令）

```powershell
cd backend
go build ./...                 # 编译通过
go vet ./...                   # 静态检查
go test ./...                  # 全部测试
go run ./cmd/server            # 启动，期望监听 :8080
```

```powershell
cd app
flutter analyze
flutter test
```

---

## Scope Check

**Aegis Visibility**：本计划要落地的是迁移与引导逻辑这两个「错了会静默损坏数据或
锁死部署」的地方，因此即使 TDD 路由是 `skipped`，也为它们写回归测试并把
DSN 外键配置固化为代码常量与注释，避免后续实现者用直觉（`db.Exec("PRAGMA …")`）踩坑。

```text
Plan Basis:
- 已批准设计规格（用户明确确认进入实施计划）
- 实测证据：goja 沙箱 8 组实验、sqlite 级联 5 场景
```

```text
BaselineUsageDraft:
- Required baseline refs: 规格 §6/§7/§6.2/§10.3；基线 §4.2/§5.2/§9
- Delivered context refs: (none — host did not project a payload)
- Acknowledged before plan refs: 规格全文、基线全文、两份证据 OUTPUT
- Cited in plan refs: 规格 §5.2/§6/§6.1/§6.2/§7/§10.3/§16；基线 §5.2/§9
- Missing refs: 无
- Decision: continue
```

```text
Requirement Ready Check:
- Requirement source refs: 用户十项功能描述 + D1-D20 决策记录
- Goals and scope refs: 规格 §16 P0/P1 行
- User / scenario refs: 功能1（多用户与管理员）
- Requirement item refs: 功能1
- Acceptance / verification criteria refs: A1（§13）
- Open blocker questions: 无
- Decision: ready
```

```text
Change Necessity:
- User-visible need: 需要能注册登录、且首个用户自动成为管理员的服务端
- No-change / non-code option: 不存在——当前仓库零行产品代码
- Why code change is necessary: 无任何既有实现可复用
- Minimum change boundary: backend/ 的 config/store/api 三个包 + 迁移 SQL；不触及
  项目、规则、统计、订阅等后续阶段
- Decision: code-change
```

```text
Existence Check:
- Proposed new surface: ①`internal/store` 迁移机制（schema_migration 表 + 内嵌 SQL）
  ②`internal/auth` 包 ③Flutter 工程
- Existing owner / reuse candidate: 无既有代码可复用（greenfield）
- Why existing surface is insufficient: 无可复用 owner
- Creation proof: ①迁移机制是 §6 全部表落地的唯一路径，且规格 §16 明确 P0 一次性建全
  schema；②鉴权是功能1 的硬需求；③功能3/6/7 需要 Flutter 工程
- Entropy / retirement impact: 无旧路径需退役。**明确拒绝**引入第三方迁移框架
  （如 golang-migrate）——内嵌 SQL 按序应用约 60 行即可，不足以支撑一个依赖
- Decision: add-with-proof
```

```text
Architecture Integrity Lens:
- Invariant: 答题记录生命周期绑定 (project,user) 关系（基线 §5.2(3)）
- Canonical owner / contract: SQLite 外键级联是唯一执行者；应用层不得另写补偿删除逻辑
- Responsibility overlap: 无——本计划不实现删除逻辑，只保证约束就位
- Higher-level simplification: 用 DB 约束表达不变量，而非应用层代码，是本项目的最高层简化
- Retirement / falsifier: 若日后有人用 `db.Exec("PRAGMA foreign_keys=ON")` 替代 DSN，
  级联将静默失效 → 由 Task P0.3 的回归测试设为 falsifier
- Verdict: 无阻塞问题，可进入任务分解
```

```text
Plan Pressure Test:
- Owner / contract / retirement: 契约承载点已识别（迁移、DSN 外键、引导管理员、token 哈希）
- Architecture integrity / higher-level path: 已确认用 DB 约束而非应用层代码
- Verification scope: 每个 Task 有可执行命令；A1 有端到端验证
- Task executability: 每步含完整代码与确切命令
- Pressure result: proceed
```

```text
Plan-Time Complexity Check:
- Target files: backend/internal/store/store.go（连接+迁移+查询）
- Existing size / shape signals: 新文件，无既有压力
- Owner fit: store 包是数据访问的唯一 owner
- Add-in-place risk: 若把迁移、用户、会话、设置查询全塞进 store.go 会超预算
- Better file boundary: store.go（连接/迁移）+ user.go / session.go / setting.go（各表查询）
- Recommendation: add owner file（按表拆分文件）
```

---

## Task P0.1 —— 仓库骨架与 Go module

**Files**
- Create: `backend/go.mod`
- Create: `backend/.dockerignore`

**Why**：为后续所有后端工作提供可编译的模块根。

**Change Necessity**：`code-change`。无既有 module。

**Impact / Compatibility**：无。纯新增。

**Steps**

1. 创建 `backend/.dockerignore`：

```
.git
bin/
tmp/
*.db
*.db-wal
*.db-shm
*_test.go
docs/
app/
```

2. 初始化模块并拉取依赖：

```powershell
cd D:\Desktop\Cala\backend
go mod init github.com/shinyes/cala/backend
go get github.com/gofiber/fiber/v2@latest
go get modernc.org/sqlite@latest
go get golang.org/x/crypto/bcrypt
go mod tidy
```

3. 验证：

```powershell
cd D:\Desktop\Cala\backend
go build ./...
```

期望输出：空（无包时 `go build ./...` 可能提示 `no Go files`，属正常；Task P0.2 起消除）。

**Verify**：`backend/go.mod` 存在，且 `go.sum` 含 `modernc.org/sqlite`。

4. **Commit**：`chore(backend): 初始化 Go module 与依赖`

---

## Task P0.2 —— 配置加载

**Files**
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/config/config_test.go`

**Why**：把监听地址、数据库路径、会话有效期、开发期 CORS 来源、注册开关初值集中为
一处，避免散落的 `os.Getenv`。

**Change Necessity**：`code-change`。配置读取是服务启动的必要条件。

**Impact / Compatibility**：无。纯新增。

**Steps**

1. 写 `backend/internal/config/config.go`：

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 是服务端全部可调参数。所有值来自环境变量，便于容器化部署。
type Config struct {
	// Addr 是 HTTP 监听地址。
	Addr string
	// DBPath 是 SQLite 数据库文件路径。
	DBPath string
	// SessionTTL 是会话有效期。
	SessionTTL time.Duration
	// DevCORSOrigins 是开发期允许的浏览器来源（D9：后端不内嵌前端，
	// 因此 flutter run -d chrome 必须显式放行）。
	DevCORSOrigins []string
	// RegistrationOpenDefault 仅在 setting 表中尚无 registration_open 时使用。
	RegistrationOpenDefault bool
}

// Load 从环境变量读取配置，未设置时使用适合本地开发的默认值。
func Load() (Config, error) {
	c := Config{
		Addr:                    env("CALA_ADDR", ":8080"),
		DBPath:                  env("CALA_DB", "cala.db"),
		RegistrationOpenDefault: envBool("CALA_REGISTRATION_OPEN", true),
	}

	ttl, err := time.ParseDuration(env("CALA_SESSION_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("CALA_SESSION_TTL 不是合法时长: %w", err)
	}
	c.SessionTTL = ttl

	// 空字符串 -> 空列表（即完全不启用 CORS 放行）
	for _, o := range strings.Split(env("CALA_DEV_CORS_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.DevCORSOrigins = append(c.DevCORSOrigins, o)
		}
	}
	return c, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
```

2. 写 `backend/internal/config/config_test.go`：

```go
package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}
	if c.Addr != ":8080" {
		t.Errorf("默认 Addr = %q, 期望 :8080", c.Addr)
	}
	if c.SessionTTL != 720*time.Hour {
		t.Errorf("默认 SessionTTL = %v, 期望 720h", c.SessionTTL)
	}
	if !c.RegistrationOpenDefault {
		t.Error("默认 RegistrationOpenDefault 应为 true")
	}
	if len(c.DevCORSOrigins) == 0 {
		t.Error("默认应放行至少一个开发期来源")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("CALA_ADDR", "127.0.0.1:9000")
	t.Setenv("CALA_SESSION_TTL", "1h")
	t.Setenv("CALA_REGISTRATION_OPEN", "false")
	t.Setenv("CALA_DEV_CORS_ORIGINS", "http://a.test , http://b.test,")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}
	if c.Addr != "127.0.0.1:9000" {
		t.Errorf("Addr = %q", c.Addr)
	}
	if c.SessionTTL != time.Hour {
		t.Errorf("SessionTTL = %v", c.SessionTTL)
	}
	if c.RegistrationOpenDefault {
		t.Error("RegistrationOpenDefault 应为 false")
	}
	if len(c.DevCORSOrigins) != 2 || c.DevCORSOrigins[0] != "http://a.test" || c.DevCORSOrigins[1] != "http://b.test" {
		t.Errorf("DevCORSOrigins = %#v, 期望去除空白与空项后的两个来源", c.DevCORSOrigins)
	}
}

func TestLoadRejectsBadTTL(t *testing.T) {
	t.Setenv("CALA_SESSION_TTL", "不是时长")
	if _, err := Load(); err == nil {
		t.Fatal("非法 CALA_SESSION_TTL 应当报错")
	}
}
```

3. 验证：

```powershell
cd D:\Desktop\Cala\backend
go test ./internal/config/ -v
```

期望：`--- PASS: TestLoadDefaults`、`TestLoadFromEnv`、`TestLoadRejectsBadTTL`。

4. **Commit**：`feat(backend): 环境变量配置加载`

---

## Task P0.3 —— 存储层、迁移与级联不变量

**Files**
- Create: `backend/internal/store/store.go`
- Create: `backend/internal/store/migrate.go`
- Create: `backend/internal/store/migrations/0001_init.sql`
- Create: `backend/internal/store/store_test.go`

**Why**：这是规格 §6 全部数据模型落地的唯一路径，也是**兼容边界第 1、2 条**的承载点。
一次建全 schema（规格 §16 明确决定），避免后续阶段产生迁移噪声。

**Change Necessity**：`code-change`。无持久化层则一切功能无从落地。

**Impact / Compatibility**：
- 必须维持 `(project, user)` 级联不变量（基线 §5.2(3)）。
- **`PRAGMA foreign_keys` 必须写在 DSN 中**——实测发现若在 Open 之后执行
  `db.Exec("PRAGMA foreign_keys=ON")`，连接池下仅部分连接生效，级联删除**静默失效**。

**Repair Track**（针对上述实测坑）
- Root cause：SQLite 的 `foreign_keys` 是**每连接**设置，`database/sql` 是连接池。
- Canonical owner：`store.Open` 的 DSN 构造是唯一设置点。
- Minimal sufficient stable repair：把 pragma 全部写进 DSN，并用常量集中声明。
- Compat boundary：所有经由 `store.Open` 的连接都启用外键。
- Verification：`TestCascadeOnProjectDelete`、`TestCascadeOnUnsubscribe`。

**Retirement Track**
- Old owner/fallback：无（新建）。
- Active status：N/A。
- Keep reason / deletion trigger：无兼容包袱。

**Steps**

1. 写 `backend/internal/store/migrations/0001_init.sql`：

```sql
-- Cala 初始 schema。依据设计规格 §6。
-- 表名用 practice_round 而非 round：避免与 SQL 内建函数 ROUND() 混淆。
-- 所有时间列均为 RFC3339 UTC 文本，便于字典序排序与区间查询。

CREATE TABLE "user" (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    disabled      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT    NOT NULL
);

CREATE TABLE session (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash TEXT    NOT NULL UNIQUE,
    user_id    INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    expires_at TEXT    NOT NULL,
    created_at TEXT    NOT NULL
);
CREATE INDEX idx_session_user ON session(user_id);

CREATE TABLE setting (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE project (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id              INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    title                 TEXT    NOT NULL,
    description           TEXT    NOT NULL DEFAULT '',
    question_count        INTEGER NOT NULL,
    cfg_json              TEXT    NOT NULL DEFAULT '{}',
    rule_source           TEXT    NOT NULL,
    share_token           TEXT    UNIQUE,
    share_token_updated_at TEXT,
    created_at            TEXT    NOT NULL,
    updated_at            TEXT    NOT NULL
);
CREATE INDEX idx_project_owner ON project(owner_id);

-- 订阅：纯只读跟随（D4）。退订即删除本行，并由应用层同时清除该用户在本项目的做题记录。
CREATE TABLE subscription (
    user_id    INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    project_id INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    created_at TEXT    NOT NULL,
    PRIMARY KEY (user_id, project_id)
);

-- 做题事实源之一。统计一律由此派生，不物化（D5）。
CREATE TABLE practice_round (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id     INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    user_id        INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    seed           INTEGER NOT NULL,
    started_at     TEXT    NOT NULL,
    finished_at    TEXT    NOT NULL,
    total_ms       INTEGER NOT NULL,
    question_count INTEGER NOT NULL,
    correct_count  INTEGER NOT NULL
);
CREATE INDEX idx_round_project_user  ON practice_round(project_id, user_id);
CREATE INDEX idx_round_user_finished ON practice_round(user_id, finished_at);

-- 判分双记录（D16）：server_is_correct 权威，client_is_correct 仅用于即时反馈与分歧告警。
-- 题面与答案以快照落库，使作者事后改规则不影响历史（规格 §6.1(3)）。
CREATE TABLE attempt (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    round_id          INTEGER NOT NULL REFERENCES practice_round(id) ON DELETE CASCADE,
    idx               INTEGER NOT NULL,
    q_snapshot        TEXT    NOT NULL,
    a_snapshot        TEXT    NOT NULL,
    a_envelope_json   TEXT    NOT NULL,
    user_input        TEXT    NOT NULL,
    client_is_correct INTEGER NOT NULL,
    server_is_correct INTEGER NOT NULL,
    elapsed_ms        INTEGER NOT NULL,
    UNIQUE (round_id, idx)
);
```

2. 写 `backend/internal/store/store.go`：

```go
// Package store 是 SQLite 持久化的唯一 owner。
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // 纯 Go 驱动，支持 CGO_ENABLED=0 静态构建
)

// pragmas 会被拼进 DSN。
//
// 关键：foreign_keys 必须在 DSN 中开启，不能靠 Open 之后执行
// db.Exec("PRAGMA foreign_keys=ON")。SQLite 的该 pragma 是「每连接」生效，
// 而 database/sql 是连接池 —— 那样写只有部分连接启用外键，
// 会让 (project, user) 级联删除静默失效，直接破坏功能8 与 D4。
// 该结论由 evidence/sqlite-cascade-spike 实测得出。
const pragmas = "_pragma=foreign_keys(1)" +
	"&_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=synchronous(NORMAL)"

// Store 持有数据库连接池。
type Store struct {
	db *sql.DB
}

// Open 打开（必要时创建）数据库并应用迁移。
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?%s", path, pragmas)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 单连接：彻底消除 SQLITE_BUSY 与写-写竞争。
	// 本项目是单进程自部署场景，写入量极小，串行化的性能代价可忽略；
	// 换来的是「无需处理并发写冲突」这一整类复杂度的消失。
	// 退役触发条件：出现明确的读并发瓶颈时，再改为读写分离连接池。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭连接池。
func (s *Store) Close() error { return s.db.Close() }

// DB 暴露底层连接，仅供同包与测试使用。
func (s *Store) DB() *sql.DB { return s.db }

// Now 返回统一的时间表示（RFC3339 UTC），所有写入列都用它。
func Now() string { return time.Now().UTC().Format(time.RFC3339) }
```

3. 写 `backend/internal/store/migrate.go`：

```go
package store

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrate 按文件名序号顺序应用尚未执行的迁移，每个迁移在独立事务中完成。
func (s *Store) migrate() error {
	if _, err := s.db.Exec(
		`CREATE TABLE IF NOT EXISTS schema_migration (
			version    INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("创建 schema_migration 失败: %w", err)
	}

	applied, err := s.appliedVersions()
	if err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("读取内嵌迁移失败: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		version, err := parseVersion(name)
		if err != nil {
			return err
		}
		if applied[version] {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("读取迁移 %s 失败: %w", name, err)
		}
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("迁移 %s 开启事务失败: %w", name, err)
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("迁移 %s 执行失败: %w", name, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_migration(version, name, applied_at) VALUES (?, ?, ?)`,
			version, name, Now()); err != nil {
			tx.Rollback()
			return fmt.Errorf("迁移 %s 记录版本失败: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("迁移 %s 提交失败: %w", name, err)
		}
	}
	return nil
}

func (s *Store) appliedVersions() (map[int]bool, error) {
	rows, err := s.db.Query(`SELECT version FROM schema_migration`)
	if err != nil {
		return nil, fmt.Errorf("查询已应用迁移失败: %w", err)
	}
	defer rows.Close()

	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// parseVersion 从 "0001_init.sql" 提取 1。
func parseVersion(name string) (int, error) {
	i := strings.IndexByte(name, '_')
	if i <= 0 {
		return 0, fmt.Errorf("迁移文件名 %q 不符合 NNNN_name.sql 约定", name)
	}
	v, err := strconv.Atoi(name[:i])
	if err != nil {
		return 0, fmt.Errorf("迁移文件名 %q 的版本号非法: %w", name, err)
	}
	return v, nil
}
```

4. 写 `backend/internal/store/store_test.go`：

```go
package store

import (
	"path/filepath"
	"testing"
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

// TestForeignKeysEnabledOnPooledConnections 是兼容边界第 2 条的 falsifier。
// 若有人把 DSN 中的 _pragma=foreign_keys(1) 换成 Open 后的 db.Exec(...)，
// 本测试在并发取连接时会失败。
func TestForeignKeysEnabledOnPooledConnections(t *testing.T) {
	s := newTestStore(t)

	// 并发持有多个连接，逐个确认外键开启。
	const n = 8
	conns := make([]interface{ Close() error }, 0, n)
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	for i := 0; i < n; i++ {
		conn, err := s.DB().Conn(t.Context())
		if err != nil {
			t.Fatalf("取连接失败: %v", err)
		}
		conns = append(conns, conn)
		var fk int
		if err := conn.QueryRowContext(t.Context(), `PRAGMA foreign_keys`).Scan(&fk); err != nil {
			t.Fatalf("查询 PRAGMA foreign_keys 失败: %v", err)
		}
		if fk != 1 {
			t.Fatalf("第 %d 个连接 foreign_keys = %d, 期望 1 —— DSN pragma 未生效", i, fk)
		}
	}
}

// seedProjectWithRounds 建 作者/订阅者/旁观者、一个项目与若干轮答题。
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

	// 三人各在项目 10 做一轮，另有一轮在项目 20
	for _, u := range []int{1, 2, 3} {
		res, err := s.DB().Exec(`INSERT INTO practice_round(project_id,user_id,seed,started_at,finished_at,total_ms,question_count,correct_count)
			VALUES (10,?,1,?,?,1000,1,1)`, u, now, now)
		if err != nil {
			t.Fatalf("插入 round 失败: %v", err)
		}
		rid, _ := res.LastInsertId()
		mustExec(`INSERT INTO attempt(round_id,idx,q_snapshot,a_snapshot,a_envelope_json,user_input,client_is_correct,server_is_correct,elapsed_ms)
			VALUES (?,0,'1+1','2','{"kind":"rational","num":"2","den":"1"}','2',1,1,500)`, rid)
	}
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

	if _, err := s.DB().Exec(`DELETE FROM project WHERE id=10`); err != nil {
		t.Fatalf("删除项目失败: %v", err)
	}

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

// TestCascadeOnUnsubscribe 覆盖 D4：退订仅清除本人历史。
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
	// 故剩余：作者(项目10) + carol(项目10) + 作者(项目20) = 3 轮；3-1 = 2 题。
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
```

> **种子数据规模**（P1.3 各测试的期望值以此为准）：
> - 项目 10：作者 / bob / carol 各 1 轮，每轮 1 题 → 3 轮、3 题
> - 项目 20：作者 1 轮，0 题 → 1 轮、0 题
> - **合计：4 轮、3 题**；订阅 2 行

5. 验证：

```powershell
cd D:\Desktop\Cala\backend
CGO_ENABLED=0 go test ./internal/store/ -v
CGO_ENABLED=0 go vet ./...
```

期望：全部 PASS；特别确认 `TestForeignKeysEnabledOnPooledConnections` 与
`TestCascadeOnProjectDelete` 通过。

> 注意：`LastInsertId()` 返回 `int64`，直接以 `?` 传参即可；不要写 `?::int`
> 之类 SQLite 不支持的强转语法。

6. **Commit**：`feat(backend): SQLite 存储层、迁移机制与级联不变量`

---

## Task P0.4 —— HTTP 服务、中间件与健康检查

**Files**
- Create: `backend/internal/api/errors.go`
- Create: `backend/internal/api/router.go`
- Create: `backend/internal/api/health.go`
- Create: `backend/cmd/server/main.go`

**Why**：让服务真正可启动、可探测，并统一错误响应格式，避免后续每个 handler 各自决定
错误形状。

**Change Necessity**：`code-change`。P0 出口条件要求容器可启动。

**Impact / Compatibility**：确立 `{"error":{"code","message"}}` 为全局错误契约，
后续阶段必须复用。

**Steps**

1. 写 `backend/internal/api/errors.go`：

```go
package api

import "github.com/gofiber/fiber/v2"

// 错误码：客户端据此分支，不解析 message。
const (
	CodeBadRequest       = "bad_request"
	CodeUnauthorized     = "unauthorized"
	CodeForbidden        = "forbidden"
	CodeNotFound         = "not_found"
	CodeConflict         = "conflict"
	CodeRegistrationClosed = "registration_closed"
	CodeRuleInvalid      = "rule_invalid"
	CodeInternal         = "internal"
)

// APIError 是统一的错误响应体。
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// fail 输出统一错误响应。
func fail(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": APIError{Code: code, Message: message}})
}
```

2. 写 `backend/internal/api/health.go`：

```go
package api

import "github.com/gofiber/fiber/v2"

// registerHealth 注册健康检查。用于容器 HEALTHCHECK 与本地探活。
func registerHealth(r fiber.Router) {
	r.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
}
```

3. 写 `backend/internal/api/router.go`：

```go
// Package api 是 HTTP 传输层的唯一 owner：路由、中间件、请求/响应形状。
package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/shinyes/cala/backend/internal/config"
	"github.com/shinyes/cala/backend/internal/store"
)

// Deps 是路由所需的依赖集合。
type Deps struct {
	Config config.Config
	Store  *store.Store
}

// NewRouter 装配 Fiber 应用。
func NewRouter(d Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "cala",
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if fe, ok := err.(*fiber.Error); ok {
				return fail(c, fe.Code, CodeBadRequest, fe.Message)
			}
			return fail(c, fiber.StatusInternalServerError, CodeInternal, "服务内部错误")
		},
	})

	// panic 不应杀死进程
	app.Use(recover.New())

	// D9：后端不内嵌前端，开发期 flutter run -d chrome 是独立来源，必须显式放行。
	// 不使用通配符 + 凭据的组合。
	if len(d.Config.DevCORSOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins:     joinOrigins(d.Config.DevCORSOrigins),
			AllowHeaders:     "Content-Type, Authorization",
			AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
			AllowCredentials: true,
		}))
	}

	api := app.Group("/api")
	registerHealth(api)

	return app
}

func joinOrigins(origins []string) string {
	out := ""
	for i, o := range origins {
		if i > 0 {
			out += ","
		}
		out += o
	}
	return out
}
```

4. 写 `backend/cmd/server/main.go`：

```go
// Command server 是 Cala 后端入口。
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shinyes/cala/backend/internal/api"
	"github.com/shinyes/cala/backend/internal/config"
	"github.com/shinyes/cala/backend/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer st.Close()

	app := api.NewRouter(api.Deps{Config: cfg, Store: st})

	// 优雅关闭：容器收到 SIGTERM 时先停止接收新请求
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("正在关闭服务…")
		if err := app.Shutdown(); err != nil {
			log.Printf("关闭出错: %v", err)
		}
	}()

	log.Printf("Cala 后端监听 %s，数据库 %s", cfg.Addr, cfg.DBPath)
	if err := app.Listen(cfg.Addr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}
```

5. 验证：

```powershell
cd D:\Desktop\Cala\backend
go build ./...
go vet ./...
go run ./cmd/server
```

在另一个终端：

```powershell
curl.exe -s http://127.0.0.1:8080/api/healthz
```

期望：`{"status":"ok"}`。确认 `cala.db` 文件生成且含全部表。

6. **Commit**：`feat(backend): Fiber 路由、统一错误契约与健康检查`

---

## Task P0.5 —— Flutter 工程初始化

**Files**
- Create: `app/`（由 `flutter create` 生成，随后裁剪）

**Why**：功能3/6/7 需要 Flutter 工程存在；P0 出口条件之一。

**Change Necessity**：`code-change`。无工程则前端无从开始。

**Impact / Compatibility**：`applicationId` 必须为 `cc.lcyk.cala`（D20）。
applicationId 一旦发布不可更改，故**必须现在定对**。

**Steps**

1. 生成工程：

```powershell
cd D:\Desktop\Cala
flutter create --org cc.lcyk --project-name cala --platforms android,web app
```

2. 确认 `app/android/app/build.gradle.kts`（或 `.gradle`）中的 `applicationId`
   为 `cc.lcyk.cala`。若 `flutter create` 生成为 `cc.lcyk.cala` 则无需改动；
   若因 `--project-name` 变成其他值，手工改为 `cc.lcyk.cala` 与
   `namespace = "cc.lcyk.cala"`。

3. 删除 `app/test/widget_test.dart` 中的计数器模板测试，避免无意义失败。

4. 添加依赖：

```powershell
cd D:\Desktop\Cala\app
flutter pub add dio shared_preferences flutter_riverpod
flutter pub add --dev flutter_lints
```

5. 验证：

```powershell
cd D:\Desktop\Cala\app
flutter analyze
flutter test
flutter run -d chrome
```

期望：`analyze` 无 issue；`test` 无失败（可能为 "No tests ran"，可接受）；
`chrome` 中打开默认页面。

6. **Commit**：`chore(app): 初始化 Flutter 工程（applicationId cc.lcyk.cala）`

---

## Task P0.6 —— 多阶段 Dockerfile 与本地编排

**Files**
- Create: `backend/Dockerfile`
- Create: `docker-compose.yml`

**Why**：P0 出口条件「容器可启动」；功能10 的镜像基础。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：本机**未安装 Docker**（V2 已验证），故 Dockerfile 的正确性
只能在 P7 的 CI 中验证。本地仅做静态检查（`go build` + 交叉编译）。

**Steps**

1. 写 `backend/Dockerfile`：

```dockerfile
# ---- 构建阶段 ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# 先只拷贝依赖描述，利用层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0：modernc.org/sqlite 是纯 Go 实现，无需 cgo，可静态链接（V1 已实测）
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cala ./cmd/server

# ---- 运行阶段 ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/cala /cala
# 数据库文件默认落在 /data，部署时挂载卷
ENV CALA_DB=/data/cala.db
ENV CALA_ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/cala"]
```

2. 写 `docker-compose.yml`：

```yaml
services:
  cala:
    build:
      context: ./backend
    image: ghcr.io/shinyes/cala:local
    ports:
      - "8080:8080"
    environment:
      CALA_DB: /data/cala.db
      CALA_ADDR: ":8080"
      # 生产环境请显式设置为 false，并先注册管理员账号
      CALA_REGISTRATION_OPEN: "true"
    volumes:
      - cala-data:/data
    restart: unless-stopped

volumes:
  cala-data:
```

3. 本地可做的静态验证（无 Docker）：

```powershell
cd D:\Desktop\Cala\backend
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="amd64"
go build -trimpath -ldflags="-s -w" -o $env:TEMP\cala-linux ./cmd/server
Write-Output "exit=$LASTEXITCODE; size=$((Get-Item $env:TEMP\cala-linux).Length)"
```

期望：`exit=0`，二进制约 6–8 MB。

4. **Commit**：`feat(backend): 多阶段 Dockerfile 与 docker-compose`

---

## Task P1.1 —— 口令哈希

**Files**
- Create: `backend/internal/auth/password.go`
- Create: `backend/internal/auth/password_test.go`

**Why**：功能1 要求多用户账号；口令绝不可明文或弱哈希存储。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：哈希格式（bcrypt）一旦有用户注册即成为持久契约。

**Steps**

1. 写 `backend/internal/auth/password.go`：

```go
// Package auth 拥有口令哈希与会话令牌这两项安全敏感的唯一实现。
package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// ErrPasswordTooShort 用于给用户可读的提示。
var ErrPasswordTooShort = errors.New("口令至少需要 8 个字符")

// MinPasswordLen 是口令最小长度。
const MinPasswordLen = 8

// HashPassword 返回 bcrypt 哈希。cost 使用库默认值（当前为 10）。
func HashPassword(plain string) (string, error) {
	if len([]rune(plain)) < MinPasswordLen {
		return "", ErrPasswordTooShort
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("生成口令哈希失败: %w", err)
	}
	return string(h), nil
}

// VerifyPassword 校验明文口令与哈希是否匹配。
// 使用 bcrypt 的常数时间比较，避免计时侧信道。
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// dummyHash 是一个**真实格式**的 bcrypt 哈希，用于在用户名不存在时执行等价的计算量。
//
// 必须是真实哈希：若手写一个格式非法的字符串，CompareHashAndPassword 会立即
// 返回格式错误而不做密钥派生，反而让「用户不存在」比「口令错误」快得多，
// 恰好制造出它本应消除的计时差异。
var dummyHash = mustHash("cala-login-timing-equalizer")

// BurnPasswordComparison 在用户不存在时调用，使两条路径耗时相近。
func BurnPasswordComparison(plain string) {
	bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(plain))
}

func mustHash(s string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	if err != nil {
		// 仅可能因 cost 非法而失败，属编程错误
		panic("auth: 无法生成 dummy 哈希: " + err.Error())
	}
	return string(h)
}
```

2. 写 `backend/internal/auth/password_test.go`：

```go
package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordRejectsShort(t *testing.T) {
	if _, err := HashPassword("short"); err != ErrPasswordTooShort {
		t.Fatalf("过短口令应返回 ErrPasswordTooShort, 得到 %v", err)
	}
}

func TestHashPasswordVerifyRoundTrip(t *testing.T) {
	const pw = "correct horse battery"
	h, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if strings.Contains(h, pw) {
		t.Fatal("哈希中不应包含明文")
	}
	if !VerifyPassword(h, pw) {
		t.Error("正确口令应当校验通过")
	}
	if VerifyPassword(h, pw+"x") {
		t.Error("错误口令不应通过")
	}
}

func TestHashIsSalted(t *testing.T) {
	const pw = "same password twice"
	h1, _ := HashPassword(pw)
	h2, _ := HashPassword(pw)
	if h1 == h2 {
		t.Error("相同口令两次哈希应因随机 salt 而不同")
	}
	if !VerifyPassword(h1, pw) || !VerifyPassword(h2, pw) {
		t.Error("两个哈希都应能校验原口令")
	}
}
```

3. 验证：

```powershell
cd D:\Desktop\Cala\backend
go test ./internal/auth/ -v
```

4. **Commit**：`feat(backend): bcrypt 口令哈希`

---

## Task P1.2 —— 会话令牌

**Files**
- Create: `backend/internal/auth/token.go`
- Create: `backend/internal/auth/token_test.go`

**Why**：规格 §7 选择**不透明 token 且哈希入库**，以便服务端可吊销、且数据库泄露时
不能直接冒用会话。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：token 形态（32 字节随机，base64url 无填充）为客户端契约。

**Steps**

1. 写 `backend/internal/auth/token.go`：

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// tokenBytes 是会话令牌的熵，32 字节 = 256 位，远超暴力破解可行范围。
const tokenBytes = 32

// NewToken 生成一个新的不透明会话令牌，返回：
//   - raw：交给客户端的明文令牌（仅此一次可见）
//   - hash：入库的 SHA-256 十六进制摘要
//
// 使用 SHA-256 而非 bcrypt：令牌本身是高熵随机值，不存在字典攻击面，
// 无需慢哈希；此处的目标是「数据库泄露不等于会话可被冒用」。
func NewToken() (raw, hash string, err error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("生成随机令牌失败: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

// HashToken 返回令牌的入库摘要。查表时对客户端提交的令牌用同一函数处理。
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
```

2. 写 `backend/internal/auth/token_test.go`：

```go
package auth

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewTokenFormatAndUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		raw, hash, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken 失败: %v", err)
		}
		if seen[raw] {
			t.Fatal("生成了重复令牌")
		}
		seen[raw] = true

		if strings.ContainsAny(raw, "+/=") {
			t.Errorf("令牌应为 base64url 无填充, 得到 %q", raw)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			t.Fatalf("令牌不是合法 base64url: %v", err)
		}
		if len(decoded) != tokenBytes {
			t.Errorf("令牌熵 = %d 字节, 期望 %d", len(decoded), tokenBytes)
		}
		if len(hash) != 64 {
			t.Errorf("SHA-256 十六进制摘要长度 = %d, 期望 64", len(hash))
		}
		if hash != HashToken(raw) {
			t.Error("NewToken 返回的 hash 应与 HashToken(raw) 一致")
		}
	}
}

func TestHashTokenIsDeterministic(t *testing.T) {
	const raw = "some-token"
	if HashToken(raw) != HashToken(raw) {
		t.Error("HashToken 应是确定性的")
	}
	if HashToken(raw) == HashToken(raw+"x") {
		t.Error("不同令牌不应产生相同摘要")
	}
}
```

3. 验证：

```powershell
cd D:\Desktop\Cala\backend
go test ./internal/auth/ -v
```

4. **Commit**：`feat(backend): 不透明会话令牌与摘要`

---

## Task P1.3 —— 用户、会话与设置的数据访问

**Files**
- Create: `backend/internal/store/user.go`
- Create: `backend/internal/store/session.go`
- Create: `backend/internal/store/setting.go`
- Create: `backend/internal/store/store_test.go`（追加）

**Why**：为 P1.4 的认证服务提供数据访问；规格 §7 的 `/api/me`、注册开关都依赖它。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：`CountUsers` 的语义是「引导管理员」判定的基础（基线 §4.2(2)）。

**Steps**

1. 写 `backend/internal/store/user.go`：

```go
package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound 表示目标记录不存在。调用方据此返回 404。
var ErrNotFound = errors.New("记录不存在")

// ErrUsernameTaken 表示用户名已被占用。
var ErrUsernameTaken = errors.New("用户名已被占用")

// User 是对外可见的用户表示，永不包含口令哈希。
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	IsAdmin   bool   `json:"isAdmin"`
	Disabled  bool   `json:"disabled"`
	CreatedAt string `json:"createdAt"`
}

// CountUsers 返回用户总数。用于判定「是否为首个用户」。
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM "user"`).Scan(&n)
	return n, err
}

// CreateUser 插入用户并返回其表示。
func (s *Store) CreateUser(username, passwordHash string, isAdmin bool) (User, error) {
	u := User{Username: username, IsAdmin: isAdmin, CreatedAt: Now()}
	res, err := s.db.Exec(
		`INSERT INTO "user"(username, password_hash, is_admin, disabled, created_at)
		 VALUES (?, ?, ?, 0, ?)`,
		username, passwordHash, boolToInt(isAdmin), u.CreatedAt)
	if err != nil {
		// UNIQUE 约束冲突 -> 用户名被占用
		if isUniqueViolation(err) {
			return User{}, ErrUsernameTaken
		}
		return User{}, fmt.Errorf("创建用户失败: %w", err)
	}
	u.ID, err = res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("读取新用户 ID 失败: %w", err)
	}
	return u, nil
}

// AuthenticateLookup 按用户名取出口令哈希与用户信息，供口令校验使用。
func (s *Store) AuthenticateLookup(username string) (User, string, error) {
	var (
		u    User
		hash string
		adm  int
		dis  int
	)
	err := s.db.QueryRow(
		`SELECT id, username, password_hash, is_admin, disabled, created_at
		 FROM "user" WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &hash, &adm, &dis, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("查询用户失败: %w", err)
	}
	u.IsAdmin = adm != 0
	u.Disabled = dis != 0
	return u, hash, nil
}

// GetUser 按 ID 取用户。
func (s *Store) GetUser(id int64) (User, error) {
	var (
		u   User
		adm int
		dis int
	)
	err := s.db.QueryRow(
		`SELECT id, username, is_admin, disabled, created_at FROM "user" WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &adm, &dis, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("查询用户失败: %w", err)
	}
	u.IsAdmin = adm != 0
	u.Disabled = dis != 0
	return u, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isUniqueViolation 判断错误是否为 SQLite 唯一约束冲突。
// 纯 Go 驱动不导出可判别的错误类型，故匹配错误文本。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: UNIQUE")
}
```

需在 `user.go` 的 import 中加入 `"strings"`。

2. 写 `backend/internal/store/session.go`：

```go
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
	_, err := s.db.Exec(`DELETE FROM session WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteUserSessions 吊销某用户的全部会话（改密/禁用时使用）。
func (s *Store) DeleteUserSessions(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM session WHERE user_id = ?`, userID)
	return err
}
```

3. 写 `backend/internal/store/setting.go`：

```go
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// SettingRegistrationOpen 是注册开关的键名。
const SettingRegistrationOpen = "registration_open"

// RegistrationOpen 读取注册开关。键不存在时写入并返回 fallback，
// 使首次启动即有一个明确、可被管理员修改的值。
func (s *Store) RegistrationOpen(fallback bool) (bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM setting WHERE key = ?`, SettingRegistrationOpen).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		if err := s.SetRegistrationOpen(fallback); err != nil {
			return false, err
		}
		return fallback, nil
	}
	if err != nil {
		return false, fmt.Errorf("读取注册开关失败: %w", err)
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback, nil
	}
	return b, nil
}

// SetRegistrationOpen 写入注册开关。
func (s *Store) SetRegistrationOpen(open bool) error {
	_, err := s.db.Exec(
		`INSERT INTO setting(key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		SettingRegistrationOpen, strconv.FormatBool(open))
	if err != nil {
		return fmt.Errorf("写入注册开关失败: %w", err)
	}
	return nil
}
```

4. 在 `store_test.go` 追加：

```go
func TestCreateUserAndLookup(t *testing.T) {
	s := newTestStore(t)

	u, err := s.CreateUser("admin", "hash", true)
	if err != nil {
		t.Fatalf("CreateUser 失败: %v", err)
	}
	if u.ID == 0 || !u.IsAdmin {
		t.Errorf("返回值异常: %#v", u)
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

	if _, _, err := s.AuthenticateLookup("nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("不存在用户应返回 ErrNotFound, 得到 %v", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := newTestStore(t)
	u, _ := s.CreateUser("bob", "hash", false)

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

	if err := s.DeleteSession("tok"); err != nil {
		t.Fatalf("DeleteSession 失败: %v", err)
	}
	if _, err := s.SessionUser("tok"); !errors.Is(err, ErrNotFound) {
		t.Errorf("已吊销会话应返回 ErrNotFound, 得到 %v", err)
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

	if err := s.SetRegistrationOpen(false); err != nil {
		t.Fatalf("SetRegistrationOpen 失败: %v", err)
	}
	got, err = s.RegistrationOpen(true)
	if err != nil {
		t.Fatalf("RegistrationOpen 失败: %v", err)
	}
	if got {
		t.Error("写入 false 后应返回 false，而不是 fallback")
	}
}
```

需在 `store_test.go` 顶部 import 块补 `"errors"` 与 `"time"`。

5. 验证：

```powershell
cd D:\Desktop\Cala\backend
go test ./internal/store/ -v
```

6. **Commit**：`feat(backend): 用户、会话与设置的数据访问`

---

## Task P1.4 —— 认证服务（含引导管理员）

**Files**
- Create: `backend/internal/service/auth.go`
- Create: `backend/internal/service/auth_test.go`

**Why**：**基线 §4.2(2) 的载体**——首个用户必须在注册开关关闭时仍可注册，
否则全新部署自我锁死。这是本项目最容易在「加个开关」时写错的地方。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：`Register` 的返回语义（是否成为管理员）是功能1 的核心契约。

**Steps**

1. 写 `backend/internal/service/auth.go`：

```go
// Package service 承载业务规则，是应用层逻辑的唯一 owner。
package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shinyes/cala/backend/internal/auth"
	"github.com/shinyes/cala/backend/internal/store"
)

var (
	// ErrRegistrationClosed 表示管理员已关闭注册。
	ErrRegistrationClosed = errors.New("注册已关闭")
	// ErrInvalidCredentials 表示用户名或口令错误。
	// 刻意不区分「用户不存在」与「口令错误」，避免用户名枚举。
	ErrInvalidCredentials = errors.New("用户名或口令错误")
	// ErrInvalidUsername 表示用户名不合法。
	ErrInvalidUsername = errors.New("用户名需为 3-32 个字符，仅限字母、数字、下划线、连字符")
)

// AuthService 实现注册、登录、登出。
type AuthService struct {
	store          *store.Store
	sessionTTL     time.Duration
	regOpenDefault bool
}

// NewAuthService 构造认证服务。
func NewAuthService(st *store.Store, sessionTTL time.Duration, regOpenDefault bool) *AuthService {
	return &AuthService{store: st, sessionTTL: sessionTTL, regOpenDefault: regOpenDefault}
}

// RegisterResult 是注册结果。
type RegisterResult struct {
	User  store.User
	Token string
	// BecameAdmin 为 true 表示这是引导管理员（首个用户）。
	BecameAdmin bool
}

// Register 创建用户。
//
// 关键规则（规格 §6.2）：当系统尚无任何用户时，无论注册开关如何，
// 都允许注册并将其设为管理员。否则全新部署会因「开关默认关闭」而永久无法使用。
func (s *AuthService) Register(username, password string) (RegisterResult, error) {
	username = strings.TrimSpace(username)
	if !validUsername(username) {
		return RegisterResult{}, ErrInvalidUsername
	}

	// 先校验口令，避免在口令不合法时白做一次事务
	if len([]rune(password)) < auth.MinPasswordLen {
		return RegisterResult{}, auth.ErrPasswordTooShort
	}

	count, err := s.store.CountUsers()
	if err != nil {
		return RegisterResult{}, fmt.Errorf("统计用户数失败: %w", err)
	}

	bootstrap := count == 0
	if !bootstrap {
		open, err := s.store.RegistrationOpen(s.regOpenDefault)
		if err != nil {
			return RegisterResult{}, err
		}
		if !open {
			return RegisterResult{}, ErrRegistrationClosed
		}
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return RegisterResult{}, err
	}

	u, err := s.store.CreateUser(username, hash, bootstrap)
	if err != nil {
		return RegisterResult{}, err
	}

	token, err := s.issueSession(u.ID)
	if err != nil {
		// 用户已建成但会话签发失败：不掩盖错误，调用方可见
		return RegisterResult{}, err
	}
	return RegisterResult{User: u, Token: token, BecameAdmin: bootstrap}, nil
}

// Login 校验凭证并签发会话。
func (s *AuthService) Login(username, password string) (store.User, string, error) {
	username = strings.TrimSpace(username)

	u, hash, err := s.store.AuthenticateLookup(username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// 执行等价计算量，抹平「用户不存在」与「口令错误」的耗时差异
			auth.BurnPasswordComparison(password)
			return store.User{}, "", ErrInvalidCredentials
		}
		return store.User{}, "", err
	}
	// 先校验口令，再判定禁用状态：两条分支的响应体相同（均为 ErrInvalidCredentials），
	// 但先校验口令可保证耗时相近，不泄露账号存在性。
	if !auth.VerifyPassword(hash, password) {
		return store.User{}, "", ErrInvalidCredentials
	}
	if u.Disabled {
		return store.User{}, "", ErrInvalidCredentials
	}

	token, err := s.issueSession(u.ID)
	if err != nil {
		return store.User{}, "", err
	}
	return u, token, nil
}

// Logout 吊销会话。
func (s *AuthService) Logout(rawToken string) error {
	return s.store.DeleteSession(auth.HashToken(rawToken))
}

// Me 解析会话令牌对应的用户。
func (s *AuthService) Me(rawToken string) (store.User, error) {
	return s.store.SessionUser(auth.HashToken(rawToken))
}

// RegistrationOpen 读取当前注册开关，供公开配置端点使用。
func (s *AuthService) RegistrationOpen() (bool, error) {
	return s.store.RegistrationOpen(s.regOpenDefault)
}

// SetRegistrationOpen 修改注册开关（仅管理员，权限判定在 API 层）。
func (s *AuthService) SetRegistrationOpen(open bool) error {
	return s.store.SetRegistrationOpen(open)
}

func (s *AuthService) issueSession(userID int64) (string, error) {
	raw, hash, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	if err := s.store.CreateSession(hash, userID, time.Now().UTC().Add(s.sessionTTL)); err != nil {
		return "", err
	}
	return raw, nil
}

func validUsername(u string) bool {
	n := len([]rune(u))
	if n < 3 || n > 32 {
		return false
	}
	for _, r := range u {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}
```

2. 写 `backend/internal/service/auth_test.go`：

```go
package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/shinyes/cala/backend/internal/auth"
	"github.com/shinyes/cala/backend/internal/store"
)

func newAuthService(t *testing.T, regOpenDefault bool) *AuthService {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewAuthService(st, time.Hour, regOpenDefault)
}

// TestFirstUserBecomesAdminEvenWhenRegistrationClosed 是本项目最关键的引导规则。
// 若实现忽略 bootstrap 分支，全新部署将永久无法创建管理员。
func TestFirstUserBecomesAdminEvenWhenRegistrationClosed(t *testing.T) {
	// 注册开关初值设为 false —— 模拟管理员关闭注册后的全新部署
	svc := newAuthService(t, false)

	res, err := svc.Register("firstuser", "password123")
	if err != nil {
		t.Fatalf("首个用户应能注册, 得到错误: %v", err)
	}
	if !res.BecameAdmin || !res.User.IsAdmin {
		t.Error("首个用户必须成为管理员")
	}
	if res.Token == "" {
		t.Error("注册应同时签发会话")
	}

	// 第二个用户在开关关闭时必须被拒
	if _, err := svc.Register("seconduser", "password123"); !errors.Is(err, ErrRegistrationClosed) {
		t.Errorf("开关关闭时第二个用户应被拒, 得到 %v", err)
	}
}

func TestSecondUserIsNotAdminWhenOpen(t *testing.T) {
	svc := newAuthService(t, true)

	first, err := svc.Register("firstuser", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if !first.BecameAdmin {
		t.Error("首个用户应为管理员")
	}

	second, err := svc.Register("seconduser", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if second.BecameAdmin || second.User.IsAdmin {
		t.Error("第二个用户不应是管理员")
	}
}

func TestRegisterValidation(t *testing.T) {
	svc := newAuthService(t, true)

	if _, err := svc.Register("ab", "password123"); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("过短用户名应被拒, 得到 %v", err)
	}
	if _, err := svc.Register("bad name", "password123"); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("含空格用户名应被拒, 得到 %v", err)
	}
	if _, err := svc.Register("gooduser", "short"); !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Errorf("过短口令应被拒, 得到 %v", err)
	}
	if _, err := svc.Register("gooduser", "password123"); err != nil {
		t.Errorf("合法注册不应失败: %v", err)
	}
	if _, err := svc.Register("gooduser", "password123"); !errors.Is(err, store.ErrUsernameTaken) {
		t.Errorf("重复用户名应被拒, 得到 %v", err)
	}
}

func TestLoginLogoutAndMe(t *testing.T) {
	svc := newAuthService(t, true)
	if _, err := svc.Register("alice", "password123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	u, token, err := svc.Login("alice", "password123")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("用户名 = %q", u.Username)
	}

	if _, err := svc.Login("alice", "wrongpassword"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("错误口令应返回 ErrInvalidCredentials, 得到 %v", err)
	}
	if _, err := svc.Login("nobody", "password123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("不存在用户应返回 ErrInvalidCredentials（不泄露存在性）, 得到 %v", err)
	}

	me, err := svc.Me(token)
	if err != nil {
		t.Fatalf("Me 失败: %v", err)
	}
	if me.ID != u.ID {
		t.Errorf("Me 返回用户 = %d, 期望 %d", me.ID, u.ID)
	}

	if err := svc.Logout(token); err != nil {
		t.Fatalf("Logout 失败: %v", err)
	}
	if _, err := svc.Me(token); err == nil {
		t.Error("登出后会话应失效")
	}
}

func TestSetRegistrationOpenRoundTrip(t *testing.T) {
	svc := newAuthService(t, true)

	if err := svc.SetRegistrationOpen(false); err != nil {
		t.Fatalf("SetRegistrationOpen 失败: %v", err)
	}
	open, err := svc.RegistrationOpen()
	if err != nil {
		t.Fatalf("RegistrationOpen 失败: %v", err)
	}
	if open {
		t.Error("写入 false 后应为 false")
	}
}
```

3. 验证：

```powershell
cd D:\Desktop\Cala\backend
go test ./... -v
```

期望：全部 PASS，特别是
`TestFirstUserBecomesAdminEvenWhenRegistrationClosed`。

4. **Commit**：`feat(backend): 认证服务与引导管理员规则`

---

## Task P1.5 —— 认证 API 与鉴权中间件

**Files**
- Create: `backend/internal/api/auth.go`
- Create: `backend/internal/api/middleware_auth.go`
- Modify: `backend/internal/api/router.go`（接线）
- Create: `backend/internal/api/auth_test.go`

**Why**：把 P1.4 的业务能力暴露为规格 §7 定义的 HTTP 契约。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：`/api/auth/*` 与 `/api/me` 的请求/响应形状成为客户端契约。

**Steps**

1. 写 `backend/internal/api/middleware_auth.go`：

```go
package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/store"
)

// 存放在 fiber.Ctx 中的本地键
const (
	localUser  = "user"
	localToken = "token"
)

// requireAuth 校验 Authorization: Bearer <token>。
func (h *Handlers) requireAuth(c *fiber.Ctx) error {
	raw := bearerToken(c)
	if raw == "" {
		return fail(c, fiber.StatusUnauthorized, CodeUnauthorized, "缺少访问令牌")
	}
	u, err := h.Auth.Me(raw)
	if err != nil {
		return fail(c, fiber.StatusUnauthorized, CodeUnauthorized, "访问令牌无效或已过期")
	}
	c.Locals(localUser, u)
	c.Locals(localToken, raw)
	return c.Next()
}

// requireAdmin 必须在 requireAuth 之后使用。
func (h *Handlers) requireAdmin(c *fiber.Ctx) error {
	u, ok := c.Locals(localUser).(store.User)
	if !ok {
		return fail(c, fiber.StatusUnauthorized, CodeUnauthorized, "缺少访问令牌")
	}
	if !u.IsAdmin {
		return fail(c, fiber.StatusForbidden, CodeForbidden, "需要管理员权限")
	}
	return c.Next()
}

// currentUser 取出 requireAuth 注入的用户。仅在受保护路由中调用。
func currentUser(c *fiber.Ctx) store.User {
	u, _ := c.Locals(localUser).(store.User)
	return u
}

func bearerToken(c *fiber.Ctx) string {
	h := c.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
```

2. 写 `backend/internal/api/auth.go`：

```go
package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/auth"
	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
)

// Handlers 持有各 API handler 的依赖。
type Handlers struct {
	Auth *service.AuthService
}

// 请求体
type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type setRegistrationReq struct {
	Open bool `json:"open"`
}

// registerAuth 注册认证相关路由。
func (h *Handlers) registerAuth(r fiber.Router) {
	r.Post("/auth/register", h.handleRegister)
	r.Post("/auth/login", h.handleLogin)
	r.Post("/auth/logout", h.requireAuth, h.handleLogout)

	r.Get("/me", h.requireAuth, h.handleMe)

	// 公开配置：客户端据此决定是否显示注册入口
	r.Get("/settings/public", h.handlePublicSettings)

	// 管理员
	r.Put("/admin/settings", h.requireAuth, h.requireAdmin, h.handleSetSettings)
}

func (h *Handlers) handleRegister(c *fiber.Ctx) error {
	var req registerReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}

	res, err := h.Auth.Register(req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRegistrationClosed):
			return fail(c, fiber.StatusForbidden, CodeRegistrationClosed, "管理员已关闭注册")
		case errors.Is(err, service.ErrInvalidUsername):
			return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
		case errors.Is(err, auth.ErrPasswordTooShort):
			return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
		case errors.Is(err, store.ErrUsernameTaken):
			return fail(c, fiber.StatusConflict, CodeConflict, "用户名已被占用")
		default:
			return fail(c, fiber.StatusInternalServerError, CodeInternal, "注册失败")
		}
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user":        res.User,
		"token":       res.Token,
		"becameAdmin": res.BecameAdmin,
	})
}

func (h *Handlers) handleLogin(c *fiber.Ctx) error {
	var req loginReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}
	u, token, err := h.Auth.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return fail(c, fiber.StatusUnauthorized, CodeUnauthorized, "用户名或口令错误")
		}
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "登录失败")
	}
	return c.JSON(fiber.Map{"user": u, "token": token})
}

func (h *Handlers) handleLogout(c *fiber.Ctx) error {
	token, _ := c.Locals(localToken).(string)
	if err := h.Auth.Logout(token); err != nil {
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "登出失败")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handlers) handleMe(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"user": currentUser(c)})
}

func (h *Handlers) handlePublicSettings(c *fiber.Ctx) error {
	open, err := h.Auth.RegistrationOpen()
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "读取配置失败")
	}
	return c.JSON(fiber.Map{"registrationOpen": open})
}

func (h *Handlers) handleSetSettings(c *fiber.Ctx) error {
	var req setRegistrationReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}
	if err := h.Auth.SetRegistrationOpen(req.Open); err != nil {
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "写入配置失败")
	}
	return c.JSON(fiber.Map{"registrationOpen": req.Open})
}
```

3. 修改 `backend/internal/api/router.go`：在 `Deps` 增加 `Handlers`，并接线。
把 `NewRouter` 改为：

```go
// NewRouter 装配 Fiber 应用。
func NewRouter(d Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "cala",
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if fe, ok := err.(*fiber.Error); ok {
				return fail(c, fe.Code, CodeBadRequest, fe.Message)
			}
			return fail(c, fiber.StatusInternalServerError, CodeInternal, "服务内部错误")
		},
	})

	app.Use(recover.New())

	if len(d.Config.DevCORSOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins:     joinOrigins(d.Config.DevCORSOrigins),
			AllowHeaders:     "Content-Type, Authorization",
			AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
			AllowCredentials: true,
		}))
	}

	api := app.Group("/api")
	registerHealth(api)
	d.Handlers.registerAuth(api)

	return app
}
```

并把 `Deps` 改为：

```go
type Deps struct {
	Config   config.Config
	Store    *store.Store
	Handlers *Handlers
}
```

同时修改 `backend/cmd/server/main.go` 的装配（并在 import 中加入
`"github.com/shinyes/cala/backend/internal/service"`）：

```go
	authSvc := service.NewAuthService(st, cfg.SessionTTL, cfg.RegistrationOpenDefault)
	app := api.NewRouter(api.Deps{
		Config:   cfg,
		Store:    st,
		Handlers: &api.Handlers{Auth: authSvc},
	})
```

4. 写 `backend/internal/api/auth_test.go`（用 `fiber.App.Test` 做无网络端到端）：

```go
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/config"
	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
)

// newTestApp 构建一个使用临时数据库的完整路由。
func newTestApp(t *testing.T, regOpen bool) *fiber.App {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := config.Config{SessionTTL: time.Hour, RegistrationOpenDefault: regOpen}
	svc := service.NewAuthService(st, cfg.SessionTTL, regOpen)
	return NewRouter(Deps{Config: cfg, Store: st, Handlers: &Handlers{Auth: svc}})
}

// do 发起一次请求，返回状态码与解析后的 JSON 体（体为空时返回 nil）。
func do(t *testing.T, app *fiber.App, method, path, body, token string) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("请求 %s %s 失败: %v", method, path, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}
	if len(raw) == 0 {
		return res.StatusCode, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("响应不是合法 JSON: %v\n原文: %s", err, raw)
	}
	return res.StatusCode, out
}

// errCode 从统一错误体中取出 error.code。
func errCode(t *testing.T, body map[string]any) string {
	t.Helper()
	e, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 error 对象: %#v", body)
	}
	c, _ := e["code"].(string)
	return c
}

// register 注册一个用户并返回其 token。
func register(t *testing.T, app *fiber.App, username, password string) string {
	t.Helper()
	status, body := do(t, app, http.MethodPost, "/api/auth/register",
		`{"username":"`+username+`","password":"`+password+`"}`, "")
	if status != http.StatusCreated {
		t.Fatalf("注册 %s 应返回 201, 得到 %d: %#v", username, status, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatalf("注册 %s 未返回 token", username)
	}
	return token
}

// TestRegisterFirstUserIsAdminEvenWhenClosed 覆盖验收标准 A1 与基线 §4.2(2)。
func TestRegisterFirstUserIsAdminEvenWhenClosed(t *testing.T) {
	app := newTestApp(t, false) // 注册开关为 false

	status, body := do(t, app, http.MethodPost, "/api/auth/register",
		`{"username":"firstuser","password":"password123"}`, "")
	if status != http.StatusCreated {
		t.Fatalf("开关关闭时首个用户仍应可注册, 得到 %d: %#v", status, body)
	}
	if became, _ := body["becameAdmin"].(bool); !became {
		t.Error("首个用户必须标记 becameAdmin=true")
	}
	u, _ := body["user"].(map[string]any)
	if isAdmin, _ := u["isAdmin"].(bool); !isAdmin {
		t.Error("首个用户的 isAdmin 必须为 true")
	}

	// 第二个人必须被拒
	status, body = do(t, app, http.MethodPost, "/api/auth/register",
		`{"username":"seconduser","password":"password123"}`, "")
	if status != http.StatusForbidden {
		t.Fatalf("开关关闭时第二个用户应返回 403, 得到 %d", status)
	}
	if code := errCode(t, body); code != CodeRegistrationClosed {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeRegistrationClosed)
	}
}

func TestRegisterDuplicateUsernameConflicts(t *testing.T) {
	app := newTestApp(t, true)
	register(t, app, "alice", "password123")

	status, body := do(t, app, http.MethodPost, "/api/auth/register",
		`{"username":"alice","password":"password123"}`, "")
	if status != http.StatusConflict {
		t.Fatalf("重复用户名应返回 409, 得到 %d", status)
	}
	if code := errCode(t, body); code != CodeConflict {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeConflict)
	}
}

func TestLoginAndMeAndLogout(t *testing.T) {
	app := newTestApp(t, true)
	register(t, app, "alice", "password123")

	// 正确凭证
	status, body := do(t, app, http.MethodPost, "/api/auth/login",
		`{"username":"alice","password":"password123"}`, "")
	if status != http.StatusOK {
		t.Fatalf("登录应返回 200, 得到 %d: %#v", status, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("登录未返回 token")
	}

	// 错误口令
	status, body = do(t, app, http.MethodPost, "/api/auth/login",
		`{"username":"alice","password":"wrongpassword"}`, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("错误口令应返回 401, 得到 %d", status)
	}

	// /api/me：带 token 与不带 token
	if status, body = do(t, app, http.MethodGet, "/api/me", "", token); status != http.StatusOK {
		t.Errorf("带 token 访问 /api/me 应返回 200, 得到 %d: %#v", status, body)
	}
	if status, body = do(t, app, http.MethodGet, "/api/me", "", ""); status != http.StatusUnauthorized {
		t.Errorf("不带 token 访问 /api/me 应返回 401, 得到 %d", status)
	}

	// 登出后原 token 失效
	if status, _ = do(t, app, http.MethodPost, "/api/auth/logout", "", token); status != http.StatusNoContent {
		t.Fatalf("登出应返回 204, 得到 %d", status)
	}
	if status, _ = do(t, app, http.MethodGet, "/api/me", "", token); status != http.StatusUnauthorized {
		t.Errorf("登出后 /api/me 应返回 401, 得到 %d", status)
	}
}

func TestAdminSettingsRequiresAdmin(t *testing.T) {
	app := newTestApp(t, true)
	adminToken := register(t, app, "admin", "password123") // 首个用户 = 管理员
	userToken := register(t, app, "bob", "password123")    // 第二个 = 普通用户

	// 普通用户被拒
	status, body := do(t, app, http.MethodPut, "/api/admin/settings", `{"open":false}`, userToken)
	if status != http.StatusForbidden {
		t.Fatalf("普通用户修改设置应返回 403, 得到 %d: %#v", status, body)
	}
	if code := errCode(t, body); code != CodeForbidden {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeForbidden)
	}

	// 管理员可改
	if status, body = do(t, app, http.MethodPut, "/api/admin/settings", `{"open":false}`, adminToken); status != http.StatusOK {
		t.Fatalf("管理员修改设置应返回 200, 得到 %d: %#v", status, body)
	}

	// 公开配置反映新值
	status, body = do(t, app, http.MethodGet, "/api/settings/public", "", "")
	if status != http.StatusOK {
		t.Fatalf("公开配置应返回 200, 得到 %d", status)
	}
	if open, _ := body["registrationOpen"].(bool); open {
		t.Error("管理员写入 false 后 registrationOpen 应为 false")
	}
}
```

> 实现说明：`fiber.App.Test` 的签名是
> `Test(req *http.Request, msTimeout ...int) (*http.Response, error)`，
> 入参是标准库 `*http.Request`（由 `httptest.NewRequest` 构造），不是 `httptest` 自定义类型。

5. 验证：

```powershell
cd D:\Desktop\Cala\backend
go build ./... ; go vet ./... ; go test ./...
CGO_ENABLED=0 go test ./...
```

另做一次手工端到端：

```powershell
cd D:\Desktop\Cala\backend
Remove-Item cala.db* -ErrorAction SilentlyContinue
go run ./cmd/server
```

另一个终端：

```powershell
curl.exe -s -X POST http://127.0.0.1:8080/api/auth/register -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"password123\"}"
curl.exe -s http://127.0.0.1:8080/api/settings/public
```

期望：第一条返回含 `"becameAdmin":true` 与 `token`；第二条返回
`{"registrationOpen":true}`。

6. **Commit**：`feat(backend): 认证 API 与鉴权中间件`

---

## Task P1.6 —— P0/P1 验收

**Files**：无新增（仅验证与文档）

**Steps**

1. 全量验证：

```powershell
cd D:\Desktop\Cala\backend
go build ./... ; go vet ./... ; CGO_ENABLED=0 go test ./... -count=1
cd D:\Desktop\Cala\app
flutter analyze ; flutter test
```

2. 逐条核对验收标准（规格 §13）。本步骤的产出是**证据记录**，不是填空：
   把每条命令的实际输出写入
   `docs/aegis/work/2026-09-10-mental-math-app/evidence/p0-p1/`。

| 标准 | 证明它的确切命令 |
|---|---|
| A1 首个注册用户为管理员 | `go test ./internal/service/ -run TestFirstUserBecomesAdminEvenWhenRegistrationClosed -v` 与 `go test ./internal/api/ -run TestRegisterFirstUserIsAdminEvenWhenClosed -v` |
| A1 开关关闭后新注册被拒 | 同上（第二段断言 403 + `registration_closed`） |
| V1 sqlite 纯 Go 静态构建 | Task P0.6 步骤 3 的交叉编译命令与二进制大小 |
| 兼容边界 1：`(project,user)` 级联 | `go test ./internal/store/ -run 'TestCascade' -v` |
| 兼容边界 2：DSN 外键 | `go test ./internal/store/ -run TestForeignKeysEnabledOnPooledConnections -v` |
| 全局 | `go build ./... && go vet ./... && CGO_ENABLED=0 go test ./... -count=1` |

3. 更新工作记录 checkpoint 与证据，提交。

4. **Commit**：`test(backend): P0/P1 验收记录`

---

## Risks

| # | 风险 | 处置 |
|---|---|---|
| P-R1 | 实现者用 `db.Exec("PRAGMA foreign_keys=ON")` 代替 DSN pragma，级联静默失效 | 代码注释 + `TestForeignKeysEnabledOnPooledConnections` 设为 falsifier |
| P-R2 | `go mod` 拉取依赖受网络影响（本机走 `goproxy.cn`） | 已确认可用；若失败改用 `GOPROXY=https://proxy.golang.org,direct` |
| P-R3 | `LastInsertId()` 的 `int64` 与 `?` 占位符绑定 | 计划已直接以 `int64` 传参；不使用 `::` 强转 |
| P-R4 | 本机无 Docker，Dockerfile 只能静态检查 | 已知并接受；P7 在 CI 首次真实验证 |
| P-R5 | `flutter create` 生成的 applicationId 与 D20 不符 | Task P0.5 步骤 2 明确要求核对并改正 |

## Retirement

- **Old owner / fallback**：无。本计划全部为新增，无旧路径、无兼容分支、无 fallback。
- **Active status**：N/A。
- **Deletion trigger**：无。

> 依规格 §16，明确**拒绝**引入第三方迁移框架与 Flutter 状态管理之外的额外抽象。

## ADR Signals（保留待 P1 完成后回填）

本计划不产生新的 ADR。规格 §12 的 ADR-1..4 在对应阶段完成后回填；
本计划涉及的沙箱（ADR-1）与级联（ADR-2）仅是**落地**，不是新决策。

---

## Execution Readiness View

```text
Execution Readiness View:
- Intent Lock: 交付可启动的 Go 服务端骨架 + 账号/引导流程（功能1）；不做项目、规则、
  练习、统计、订阅、CI
- Scope Fence: 仅 backend/ 的 config/store/api/service/auth 与 app/ 初始化；
  不触碰规格 §16 的 P2 及以后范围
- Baseline Lock: 规格 §6/§7/§6.2/§10.3 + 基线 §4.2/§5.2/§9；两份实测证据
- Approved Behavior: 首个用户无论开关如何都成为管理员；会话为哈希入库的不透明 token；
  统一错误体 {"error":{"code","message"}}
- Owner / Contract Constraints: store 是持久化唯一 owner；api 是传输层唯一 owner；
  service 是业务规则唯一 owner；auth 是口令与令牌唯一 owner；外键级联由 DB 执行，
  应用层不得另写补偿删除
- Compatibility Boundary: 规格 §6.1 五条不变量 + 基线 §9 六条；本计划显式承载第 1、2 条
- Retirement Boundary: 无退役对象；明确拒绝迁移框架与额外抽象
- Task Batches: P0.1-P0.6（骨架，可独立验证）→ P1.1-P1.5（认证）→ P1.6（验收）
- Test Obligations: 配置 3 项、store 6 项、auth 5 项、service 5 项、api 7 项断言
- Review Gates: 每个 Task 完成后跑 go build/vet/test 并提交；P1.6 逐条核对 A1
- Drift / Rewind Rules: 若实现中需要新增 owner、契约或表结构，停止并回到规格 §16，
  不得在本计划内扩张范围
- Evidence Required Before Completion: go build/vet/test 全绿的输出；
  A1 的端到端 curl 结果；级联与外键两项测试通过
- Advisory Boundary: method-pack execution guidance only; not GateDecision,
  PolicySnapshot, or completion authority
```

## Execution Route

```text
Execution Route:
- Decision: inline
- Evidence: P0.1-P0.6 严格顺序依赖（config → store → api → main）；P1.1-P1.5 依次依赖
  前一步的包；任务间共享 backend/ 同一包结构，并行会产生写冲突而非收益
- Fallback: 若某任务需并行（如 P0.5 Flutter 初始化与 P0.2/P0.3 无交集），可单独委派，
  但默认不启用
- User confirmation required: no
```

**REQUIRED SUB-SKILL**：aegis:executing-plans

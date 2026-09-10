# P0/P1 验收证据

- Date: `2026-09-10`
- Plan: [`docs/aegis/plans/2026-09-10-p0-p1-foundation-auth.md`](../../../plans/2026-09-10-p0-p1-foundation-auth.md)
- 范围：规格 §16 的 **P0 骨架与风险先行** 与 **P1 认证与引导**

---

## 1. 全量命令结果

| 命令 | 结果 |
|---|---|
| `cd backend && CGO_ENABLED=0 go build ./...` | exit 0 |
| `cd backend && go vet ./...` | exit 0（无告警） |
| `cd backend && CGO_ENABLED=0 go test ./... -count=1` | 5 个包全部 `ok` |
| `cd app && flutter analyze` | `No issues found!` |
| `cd app && flutter test` | `All tests passed!` |
| `cd app && flutter build apk --debug` | `✓ Built app-debug.apk` |
| `cd app && flutter build web` | `✓ Built build\web` |

后端测试分布：

```
?   github.com/shinyes/cala/backend/cmd/server        [no test files]
ok  github.com/shinyes/cala/backend/internal/api      1.753s
ok  github.com/shinyes/cala/backend/internal/auth     1.542s
ok  github.com/shinyes/cala/backend/internal/config   0.590s
ok  github.com/shinyes/cala/backend/internal/service  3.412s
ok  github.com/shinyes/cala/backend/internal/store    2.295s
```

---

## 2. 验收标准逐条

### A1 首个注册用户为管理员；开关关闭后新注册被拒且首个用户仍可注册

**单元/集成测试**（两项，分别覆盖 service 与 HTTP 层）：

- `internal/service`: `TestFirstUserBecomesAdminEvenWhenRegistrationClosed` PASS
  —— 以 `regOpenDefault=false` 构造服务，首个用户仍成功且 `BecameAdmin=true`，第二个用户被拒。
- `internal/api`: `TestRegisterFirstUserIsAdminEvenWhenClosed` PASS
  —— 断言 201 + `becameAdmin=true` + `user.isAdmin=true`，随后第二个用户 403 + `registration_closed`。

**手工端到端**（真实进程、真实 HTTP、真实 SQLite 文件）：

```
--- 1. healthz ---
200 {"status":"ok"}
--- 2. public settings (未登录) ---
200 {"registrationOpen":true}
--- 3. 首个用户注册 ---
201 {"becameAdmin":true,"token":"VNelzqO4...","user":{"id":1,"username":"admin","isAdmin":true,"disabled":false,...}}
--- 4. me (带 token) ---
200 {"user":{"id":1,"username":"admin","isAdmin":true,...}}
--- 5. me (不带 token) ---
401
--- 6. 管理员关闭注册 ---
200 {"registrationOpen":false}
--- 7. 关闭后再注册 ---
403
--- 8. 登录 ---
200 {"token":"T24g8R3Z...","user":{"id":1,"username":"admi...
```

### V1 `modernc.org/sqlite` 纯 Go 静态构建

- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` 成功，10.34 MB；
  二进制中无 `libc.so` / `libpthread` / `libdl` 引用，确认为静态链接。
- 本机 `CGO_ENABLED` 默认即为 `0`（无 C 编译器），故本地测试与目标部署形态同构。

### 兼容边界 1：`(project, user)` 单一级联不变量（功能8 + D4）

`internal/store` 全部 PASS：

| 测试 | 断言 |
|---|---|
| `TestCascadeOnProjectDelete` | 删项目 10 后 `practice_round` 4→1、`attempt` 3→0、`subscription` 2→0；项目 20 数据完好 |
| `TestCascadeOnUnsubscribe` | 退订后仅 bob 记录消失（残留 0），`practice_round`→3、`attempt`→2，作者与 carol 的 2 轮完好 |
| `TestAttemptUniquePerRound` | 同轮重复 `idx` 违反 `UNIQUE(round_id, idx)` |

**变异测试取证**（证明测试确有捕获能力，而非碰巧通过）：
把 `pragmas` 中的 `foreign_keys(1)` 改为 `(0)` 后，三项测试全部按预期失败：

```
TestPragmasEnableForeignKeysAcrossConnections: 第 0 个连接 foreign_keys = 0, 期望 1
TestCascadeOnProjectDelete: practice_round 剩余 = 4, 期望 1; attempt 剩余 = 3, 期望 0
TestCascadeOnUnsubscribe:   attempt 剩余 = 3, 期望 2
```

还原后全绿。

### 兼容边界 2：`PRAGMA foreign_keys` 必须在 DSN 中

- `TestPragmasEnableForeignKeysAcrossConnections` 针对 `pragmas` 常量另建**多连接**池，
  逐连接确认 `foreign_keys = 1`。
  之所以不复用 `Store` 自己的池：`Store` 刻意设 `MaxOpenConns(1)`，
  单连接池会让「pragma 写在 DSN」与「pragma 写在 Open 之后」表现一致而**掩盖差异**。
- `TestStorePoolIsSingleConnection` 记录该池约定，并在池上复核 `foreign_keys = 1`。

### 数据安全（非计划要求，额外取证）

在真实 e2e 数据库（`cala.db` + `cala.db-wal`，180 KB）的全部字节中搜索：

| 断言 | 结果 |
|---|---|
| 含明文会话令牌 | **False** ✅ |
| 含明文口令 `password123` | **False** ✅ |
| 含 bcrypt 前缀 `$2a$` | True ✅（口令已哈希） |
| 含用户名 `admin`（对照组） | True ✅（证明搜索确实生效） |
| 64 位十六进制摘要 | 6 处 ✅（会话令牌摘要） |

> 对照组很重要：若搜索本身失效（例如编码用错），前两行也会假报 False。
> 本次取证即曾因 PowerShell 5.1 不存在 `Encoding::Latin1` 而使检查静默失败，
> 改用 `GetEncoding(28591)` 后重做。

---

## 3. 执行期发现并修复的问题

### P-R8 Windows 跨盘符导致 `compileDebugKotlin` 失败（根因修复）

**症状**：`:shared_preferences_android:compileDebugKotlin` 失败，
报 `Could not close incremental caches ... class-fq-name-to-source.tab, ...`。

**根因**（藏在 `--stacktrace` 的 `Suppressed` 链中）：

```
IllegalArgumentException: this and base files have different roots:
C:\Users\<user>\AppData\Local\Pub\Cache\hosted\...\LegacySharedPreferencesPlugin.kt
and
D:\Desktop\Cala\app\android
```

pub cache 在 C:、项目在 D:，Kotlin 增量编译器 flush 缓存时做 `Path.relativize()`，
Windows 下跨盘符无法相对化 → 抛异常 → 被外层包装成「增量缓存」错误。

**修复**：用户级 `PUB_CACHE=D:\Programs\Pub\Cache`，使两者同盘。

**已排除的假设**（单变量实验）：

| # | 假设 | 结果 |
|---|---|---|
| 1 | Gradle/Kotlin 守护进程持有文件句柄 | 杀全部守护进程 + `flutter clean` 后仍复现 → 排除 |
| 2 | 编码问题（`-Dfile.encoding=UTF-8`） | 加上后仍复现 → 排除 |
| 3 | 源文件含 BOM 或非法 UTF-8 | 全仓扫描 app/ 与 android/ 无 BOM → 排除 |

**隔离验证**：仅设 `PUB_CACHE` 到 D:、去掉 `-Dfile.encoding=UTF-8` 后仍
`✓ Built app-debug.apk`（exit 0）→ **承重改动只有「同盘」一项**；
被证伪的编码参数已从 `gradle.properties` 移除，不保留无依据的配置。

### P-R9 PowerShell 5.1 写出非法 UTF-8（自伤，已修复）

执行期用 `Add-Content`（未指定 `-Encoding`）向 `gradle.properties` 追加中文注释，
本机 ANSI 代码页为 GBK，产生非法 UTF-8，导致 Flutter 工具读取该文件时直接崩溃：

```
FileSystemException: Failed to decode data using encoding 'utf-8',
path = 'D:\Desktop\Cala\app\android\gradle.properties'
```

字节取证：偏移 308 处为 `0xBC`，无法译为 Unicode。

**修复**：
1. 以 UTF-8 重写 `gradle.properties`，并保持**纯 ASCII**；
2. 文件内留下注释说明该约束，防止后来者重犯；
3. 全仓校验：**无 BOM、无非法 UTF-8**；
4. 证据目录中两个 `OUTPUT.txt` 原有 UTF-8 BOM（由 `Out-File -Encoding utf8` 产生）已去除。

### 计划缺陷修正（执行期发现）

`Store.Open` 设 `MaxOpenConns(1)`，而计划原测试试图在该池上并发持有 8 个连接 ——
自相矛盾，实测**永久阻塞**（挂死 7 分钟）。改为针对 `pragmas` 常量另建多连接池验证，
这反而更强：单连接池会掩盖 DSN pragma 与 `db.Exec` pragma 的差异。

### 环境修正：Go 版本

`golang.org/x/crypto v0.57.0` 要求 `go >= 1.26.0`；本机安装 1.25.5，
`GOTOOLCHAIN=local` 直接报错。故 `go.mod` 声明 `go 1.26.0`，
由 `GOTOOLCHAIN=auto` 解析（1.26.0 已缓存），Dockerfile 相应改用 `golang:1.26-alpine`。

---

## 4. 未验证项（诚实记录）

| # | 项 | 原因 | 何时可验 |
|---|---|---|---|
| 1 | Docker 镜像真实构建 | 本机无 Docker；Docker Hub 在本环境不可达 | P7 的 CI |
| 2 | `golang:1.26-alpine` 与 `distroless/static-debian12` 的 tag 存在性 | 同上 | P7 的 CI（若有误会立即且显眼地失败，修复代价仅改字符串） |
| 3 | 正式签名 APK | 需用户提供 keystore 与 4 个 GitHub Secret | P7，且依赖用户提供凭据 |
| 4 | `flutter run -d chrome` 交互式启动 | 需要交互会话；已用等价的 `flutter build web` 成功替代 | 随时可人工运行 |
| 5 | 真机安装运行 | 无连接设备（`flutter doctor` 显示有一台 Android 设备，本次未使用） | P4 前端里程碑 |

---

## 5. 结论

规格 §16 的 **P0 与 P1 出口条件均已满足**：
`go build` 通过、容器可启动（配置就位，镜像构建留待 CI）、A1 通过。

**产品代码规模**：后端 5 个包（config / store / api / service / auth）+ `cmd/server`，
前端 Flutter 工程（Cupertino 骨架）。

**下一步**：P2 规则引擎（goja 沙箱、契约、保存期校验、表驱动单测），
出口条件为 A2 / A3 / A4 通过。

# P3 项目与练习闭环（后端）验收证据

- Date: `2026-09-10`
- Plan: [`docs/aegis/plans/2026-09-10-p3-project-rounds.md`](../../../plans/2026-09-10-p3-project-rounds.md)
- 范围：规格 §16 的 **P3 项目与练习闭环（后端）**

---

## 1. 全量测试结果

```
cd backend && CGO_ENABLED=0 go build ./...        exit 0
cd backend && go vet ./...                        exit 0
cd backend && gofmt -l internal cmd               无输出（全部合规）
cd backend && go test ./... -count=1              7 个包全部 ok
```

各包测试数（`-count=1`，无缓存）：

| 包 | PASS | FAIL |
|---|---|---|
| `internal/scoring` | **114** | 0 |
| `internal/rules` | 69 | 0 |
| `internal/service` | 68 | 0 |
| `internal/api` | 44 | 0 |
| `internal/store` | 30 | 0 |
| `internal/auth` | 8 | 0 |
| `internal/config` | 3 | 0 |
| **合计** | **336** | **0** |

---

## 2. 验收标准逐条

### A11 作者返回非有理数且非规范文本的 `a` 时，拒绝保存并给出可读错误

**单元层** — `internal/scoring`：

| 测试 | 断言 |
|---|---|
| `TestClassifyRejectsMalformed` | `1/0`、`1.2.3`、`--5`、`1/`、`/2`、`-`、`+`、`.`、`/`、`1.`、`1/2/3` 全部报错 |
| `TestClassifyRejectsEmptyControlAndOverlong` | 空串、纯空白、含 NUL、超长均报错 |
| `TestClassifyRationalForms` | 16 种合法数值形态正确分类 |
| `TestClassifyTextForms` | 含中文的文本答案正确分类（本项由规格修正后新增） |

**服务层** — `TestCreateProjectRejectsMalformedAnswer`：
`1/0`、`1.2.3`、空答案、含控制字符四类规则在**保存期**被拒，
错误信息包含「无法分类」并指出题号。

**HTTP 层** — `TestCreateProjectRejectsMalformedAnswer`（api 包）实际返回的错误信息：

```
项目配置不合法: 第 1 题的答案 "1/0" 无法分类：
答案 "1/0" 看起来是数值但无法解析（分母不能为零）；
若要作为文本答案，请包含数值以外的字符
```

→ 错误可读、指出题号、并给出修复方向。

### A14 人为注入前后端分歧时，服务端记录告警且统计仍以 `server_is_correct` 为准

**服务层** — `TestDiscrepancyIsRecorded`：
客户端对全部 6 题声称「正确」但提交错误作答，结果：

- 服务端重算 `correctCount = 0`
- `Discrepancies = 6`，全局 `DiscrepancyCount() = 6`
- 落库时 `client_is_correct = true`、`server_is_correct = false`，**两值各自独立**

**手工端到端**（真实进程 / HTTP / SQLite）：

```
1. 注册成为管理员: becameAdmin=True
2. 建项目: id=1 题数=6
3. 出题: 题数=6 seed=1582383401763631224 清洗表项=32 版本=1
   示例: 29 + 98 = ? 答案=127 信封=rational:127/1
4. 交卷: 201
   服务端正确数=3/6 (期望 3/6)
   分歧数=3 (客户端全说对，实际一半错)
   总耗时=6000ms (服务端从 elapsedMs 派生)
```

**直接查库取证**（A14 的关键证据——两个判定确实独立落库）：

```
practice_round=1  attempt=6

idx  题面           答案   信封                                        作答      客户端  服务端  elapsed
0    29 + 98 = ?   127   {"kind":"rational","num":"127","den":"1"}   127       true   true    1000
1    76 + 29 = ?   105   {"kind":"rational","num":"105","den":"1"}   999999    true   false   1000
2    89 + 52 = ?   141   {"kind":"rational","num":"141","den":"1"}   141       true   true    1000
3    14 + 83 = ?    97   {"kind":"rational","num":"97","den":"1"}    999999    true   false   1000
4    50 + 96 = ?   146   {"kind":"rational","num":"146","den":"1"}   146       true   true    1000
5    55 + 27 = ?    82   {"kind":"rational","num":"82","den":"1"}    999999    true   false   1000
```

`client_is_correct` 6 行全为 `true`（**原样保存客户端上报，未被覆盖**），
`server_is_correct` 仅 3 行为 `true`（权威重算）。

> 注：A14 中「统计一律基于 `server_is_correct`」的**统计部分**要到 P5 才能完整验证，
> 本阶段只验证到「落库值与分歧告警」。这一点已在验收表中如实标注。

### 完成度核对（计划 P3.5 表）

| 标准 | 命令 | 结果 |
|---|---|---|
| A11 畸形答案被拒 | `go test ./internal/scoring/ -run TestClassifyRejectsMalformed -v` | PASS |
| A11 保存期拦截 | `go test ./internal/api/ -run TestCreateProjectRejectsMalformedAnswer -v` | PASS |
| A14 分歧被告警 | `go test ./internal/service/ -run TestDiscrepancyIsRecorded -v` | PASS |
| 快照不受规则变更影响 | `go test ./internal/service/ -run TestSnapshotImmutableAfterRuleChange -v` | PASS |
| 级联删除（功能8） | `go test ./internal/api/ -run TestDeleteProjectCascadesOverHTTP -v` | PASS |
| 无浮点判分 | `go test ./internal/scoring/ -run TestNoFloatDrift -v` | PASS |

---

## 3. 变异测试取证

### 精确比较改为 `float64` → 「无浮点」测试立即失败

改动：把 `Compare` 中的 `got.Cmp(want) == 0` 换成 float64 除法比较。

```
scoring_test.go:275: 2^53 与 2^53+1 在精确比较下必须不等，但判为正确。数值精确相等
scoring_test.go:283: 1/3 与 0.3333333333333333 精确比较必须不等，但判为正确。数值精确相等
--- FAIL: TestNoFloatDrift (0.00s)
```

→ 证明 §5.5.3 的「判分路径无浮点」是被测试真实守护的，不是口头承诺。
两个用例都恰好落在 float64 无法区分、精确有理数可以区分的点上。已还原（`grep MUTATION` 无残留）。

---

## 4. 执行期发现的问题

### 4.1 设计缺陷：文本答案的 ASCII 限制自相矛盾（已修规格）

规格 §5.5.2 原文写「非有理数但为 **ASCII** 规范文本」，却在同一行用 `质数` 举例——
示例本身不是 ASCII。按字面实现会让**中文文本答案无法使用**。

已修正为「含数值字母表以外字符的非空文本」并显式允许中文，同时把文本规范化
定义为一条最小且可逐条镜像的规则（trim → 折叠空白 → ASCII 大小写折叠，
**不做任何 Unicode 形式变换**）。

### 4.2 设计缺陷：容差无处存放（已修规格 + 迁移 0002）

D3 承诺「项目级可选容差」，但规格 §6 的 schema 没有对应字段。
已新增迁移 `0002_add_tolerance.sql`，用两个整数列（而非浮点）表示容差，
使其也留在无浮点判分路径上。

**为何不放在 `cfg_json`**：`cfg_json` 是作者自由定义的配置，平台不解释其结构；
容差是平台的判分语义，必须类型化、被校验、有单一 owner。放进 cfg 会让同一个概念
有两个来源。已由 `TestToleranceComesFromColumn` 守护（在 cfg 里放 `tolerance` 字段不生效）。

### 4.3 实现缺陷：`fail()` 被当作错误值使用，导致非法 ID 静默变成 0

`fail()` 返回的是 `c.JSON()` 的结果——写出成功时是 **nil**。

```go
// 错误写法
return 0, fail(c, fiber.StatusBadRequest, CodeBadRequest, "项目 ID 不合法")
// 实际返回 (0, nil)：调用方以为没出错，继续用 id=0 查询 → 得到误导性的 404
```

由 `TestBadProjectIDReturns400` 捕获（`/api/projects/abc` 返回 404 而非 400）。

已修正：`parseProjectID` 改为返回 `(int64, bool)`，`false` 表示「响应已写出，立即 return nil」；
并在 `fail()` 上写明用法约定，防止复发。

### 4.4 测试自身的脆弱性：硬编码迁移数量

P0 的两个测试断言「已应用迁移数 = 1」。加入 0002 后它们因**与被测行为无关的原因**失败。
这类断言会诱使人「顺手改大数字」，从而削弱测试意义。

已改为由内嵌迁移文件数推导期望值（`embeddedMigrationCount`），新增迁移不再产生噪声失败。

### 4.5 我自己的测试错误：逗号分组

原把 `1,2,3` 列为应报错的笔误。但逗号由清洗表**无条件删除**，`1,2,3` → `123` 正常解析。

不校验千分位分组正确性是有意取舍：那需要让清洗表理解「逗号出现在什么位置才合法」，
即把**数据**变成**逻辑**，破坏 §5.5.4「清洗表是数据、客户端只应用数据」的跨端一致性基础。
键盘字母表本就不含逗号（功能6），逗号仅用于处理粘贴文本。
已改写为 `TestCommaGroupingIsDeliberatelyLenient` 记录该取舍。

### 4.6 顺带修正：两个历史文件未 gofmt

`internal/rules/rules_test.go` 与 `internal/service/auth_test.go` 此前提交时未格式化。
现已格式化，`gofmt -l internal cmd` 无输出。

---

## 5. 未验证项（诚实记录）

| # | 项 | 原因 | 何时可验 |
|---|---|---|---|
| 1 | A14 的「统计一律基于 `server_is_correct`」 | 统计端点在 P5 | P5 |
| 2 | 分享链接与订阅导入/退订（功能5） | 属 P6 | P6 |
| 3 | 前端判定与服务端判定的一致性 | 客户端实现属 P3.5 | P3.5（语料门禁） |
| 4 | Docker 镜像构建、签名 APK | 本机无 Docker；secrets 已建但无法本地验证 | P7 |

---

## 6. 结论

规格 §16 的 **P3 出口条件已满足**：A11 与 A14 通过，
且两项均以「实际输出 + 查库结果 + 变异测试」三类证据支撑。

**新增规模**：`internal/scoring`（4 源文件 + 1 测试）、
`store/project.go`、`store/round.go`、`store/subscription.go`、
`service/project.go`、`service/round.go`、`api/project.go`、`api/round.go`、
迁移 `0002`；以及对应测试。

**下一步**：P3.5 判分单一性与语料门禁 —— 由 `internal/scoring` 生成
≥10,000 条边界向量，Dart 侧镜像实现必须逐例一致，CI 门禁使分歧即失败。
这一步完成后，客户端本地即时判分（功能9）才有资格进入 P4。

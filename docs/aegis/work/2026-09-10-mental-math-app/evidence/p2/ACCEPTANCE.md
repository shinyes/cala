# P2 规则引擎 验收证据

- Date: `2026-09-10`
- Plan: [`docs/aegis/plans/2026-09-10-p2-rule-engine.md`](../../../plans/2026-09-10-p2-rule-engine.md)
- 范围：规格 §16 的 **P2 规则引擎**

---

## 1. 全量命令结果

| 命令 | 结果 |
|---|---|
| `cd backend && CGO_ENABLED=0 go build ./...` | exit 0 |
| `cd backend && go vet ./...` | exit 0 |
| `cd backend && CGO_ENABLED=0 go test ./... -count=1 -timeout 300s` | 6 个包全部 `ok` |
| `cd backend && go test ./internal/rules/ -count=1 -v` | **PASS 69 / FAIL 0** |

```
ok  github.com/shinyes/cala/backend/internal/api      1.618s
ok  github.com/shinyes/cala/backend/internal/auth     1.829s
ok  github.com/shinyes/cala/backend/internal/config   0.936s
ok  github.com/shinyes/cala/backend/internal/rules    1.228s
ok  github.com/shinyes/cala/backend/internal/service  3.220s
ok  github.com/shinyes/cala/backend/internal/store    2.245s
```

---

## 2. 验收标准逐条

### A2 各类错误源码被拒（保存期校验）

**编译期拒绝（7 项）** — `TestCompileRejectsBadSources` PASS：

空源码 · 缺少 `generate` · `generate` 是数字 · `generate` 是对象 ·
语法错误 · 未终止注释 · 顶层 `throw`

**运行期拒绝（18 项）** — `TestValidateRejectsBadBehaviour` PASS：

| 类别 | 用例 |
|---|---|
| 超时 | `while(true)`、`for(;;)` |
| 递归 | 无限递归（提示「递归过深」） |
| 抛错 | 调用期未定义引用、调用期主动抛错 |
| 形状 | 返回数字 / 字符串 / 数组 / `null` / `undefined` / 缺 `q` / 缺 `a` / `q` 非字符串 / `a` 非字符串 / `q` 空串 / `a` 空串 / 题面过长 / 答案过长 |

**配置相关** — `TestValidateReflectsPendingConfig` PASS：同一规则在
`cfg={}` 下通过、在 `cfg={explode:true}` 下被拒 →
证明校验用的是**待保存配置**，而非某个固定样本。

### A3 同种子两次生成题目序列逐字相同

`TestGenerateIsReproducible` PASS：同 `(rule, cfg, seed=42, count=50)`
两次生成的 50 道题**逐字段相同**（含 `Index` / `Q` / `A`）。

相关：

| 测试 | 断言 |
|---|---|
| `TestGenerateVariesWithSeed` | 种子 42 与 43 的序列不同 |
| `TestGenerateAdvancesRandomSequence` | 30 道题至少有 10 种不同题面（随机序列确实在推进） |
| `TestGenerateRespectsConfig` | `min=max=1` 时每题都是 `1 + 1 = ?` / `2` |
| `TestGenerateIndexesSequentially` | `Index` 为 `0..n-1`（P3 依赖它写 `attempt.idx`） |
| `TestGenerateFrozenTime` | 依赖 `Date` 的规则同种子可复现，且同轮内时间冻结 |
| `TestGenerateTimeVariesWithSeed` | 约 26 个不同种子看到 ≥5 个不同时刻（时间源确实随种子变化） |

### A4 死循环规则被击杀，且不影响后续出题

`TestGenerateRejectsTimeoutRule` PASS：死循环规则被拒后，
**紧接其后的好规则仍能正常出 20 道题**（沙箱可恢复）。

`TestGenerateTimeoutRuleDoesNotBreakNextRound` PASS：同一规则在
`cfg.boom=true` 时被拒，在 `cfg.boom=false` 时立即恢复可用。

`TestValidateHandlesHostileRules` PASS（6 项，全部在 30s 内被拒）：

| 恶意规则 | 实测耗时 |
|---|---|
| `while(true)` | 0.05s |
| `for(;;)` | 0.05s |
| `let i=0; while(i>=0){i++}` | 0.05s |
| 无限递归 | 0.00s |
| 互相递归 | 0.00s |
| `"x".repeat(5e7)`（巨大字符串） | 0.24s |

### 沙箱四配置齐全（基线 §5.2(1)）

| 配置 | 守护它的测试 |
|---|---|
| `SetRandSource` | `TestGenerateIsReproducible`、`TestGenerateVariesWithSeed` |
| `SetTimeSource` | `TestGenerateFrozenTime`、`TestGenerateTimeVariesWithSeed` |
| `SetMaxCallStackSize` | `TestValidateRejectsBadBehaviour/无限递归`（提示「递归过深」） |
| 调用超时 `Interrupt` | `TestGenerateRejectsTimeoutRule`、`TestValidateHandlesHostileRules` |

配置集中在 `sandbox.go` 的 `newVMWithSource` **一处**，`newVM` 与 `Generate`
都经它，不存在第二份拷贝（防「一处设了、一处漏了」）。

---

## 3. 变异测试取证（证明测试确有捕获能力）

仅「测试通过」不能说明测试有效。以下两个变异均**实测触发预期失败**，随后已还原
（`grep MUTATION` 无残留）。

### 变异 1：把随机源改为非确定性（忽略 seed）

改动：`newSeededSource` 用 `time.Now().UnixNano()` 取代 `seed`。

结果 —— 可复现性测试立即失败：

```
rules_test.go:277: 第 0 题不一致:
--- FAIL: TestGenerateIsReproducible (0.01s)
--- PASS: TestGenerateVariesWithSeed (0.00s)
FAIL
```

→ 证明 A3 的测试确实在检验确定性，而非碰巧通过。

### 变异 2：移除超时中断（`vm.Interrupt`）

改动：删掉 `time.AfterFunc(...)` 中断，只保留 `_ = time.AfterFunc`。

结果 —— 死循环测试**挂起 25s**，被 `go test -timeout 25s` 强制中断：

```
panic: test timed out after 25s
running tests:
FAIL	github.com/shinyes/cala/backend/internal/rules	25.215s
```

→ 证明超时中断是**承重**的：没有它，死循环规则不是被拒绝，而是永久挂住。
这条证据直接对应规格 §5.2「Interrupt 超时为强制项」。

---

## 4. 执行期对计划的修正（3 处）

计划与实际实现有出入的地方已**回改计划文档**，避免计划的错误固化：

### 修正 1：无限递归的错误分类

计划原以为栈耗尽会以 Go panic 冒出，测试期望 `内部异常`。
实测：goja 把栈耗尽表示为**可捕获**的 `*goja.StackOverflowError`（**不** panic 进程），
这比预期更安全；但其 `Error()` 为**空串**，会产出无信息量的提示
（实际现象：`generate 抛出异常:  at generate (rule.js:1:40(3))`）。

已改为单独识别该类型并给出可操作提示：
「递归过深（本沙箱上限 200 层）；请检查是否存在无限递归或过深的调用链」。

### 修正 2：模块级状态的行为（计划自相矛盾）

计划原写 `TestGenerateDoesNotLeakBetweenRounds`，断言「同轮内 `counter` 递增到 10、
下一轮从 1 重新开始」。但既然**每道题各建一个 VM**，模块级 `counter` 每道题都会
重新初始化为 1 —— 原断言与设计自相矛盾。

实测确认行为是「始终为 1」。已把测试改写为
`TestGenerateIsStatelessAcrossQuestions`，把**无状态性作为显式契约**固定下来，
并在 `sandbox.go` 中写明：`generate` 必须是 `cfg` 与随机数的纯函数。

### 修正 3：`newVMWithSource` 的未使用参数

计划中的签名为 `newVMWithSource(seed int64, rng *seededSource, dayOffset int64)`，
其中 `seed` 参数在函数体内未被使用（已由 `dayOffset` 体现）。实现时去掉该参数，
避免留下误导性签名。

---

## 5. 已知局限（诚实记录，不宣称已解决）

| # | 局限 | 说明 |
|---|---|---|
| 1 | **goja 无内存上限**（规格 §14 R3 残留） | 已用「超时 + 递归深度上限 + 输出长度上限（`MaxQuestionLen`=1000 / `MaxAnswerLen`=500）」代偿，但一条在**循环内部**反复分配大数组的规则仍可在 50ms 内消耗可观内存。未解决，仅收窄。 |
| 2 | 模块级初始化按题重复执行 | 每问新 VM 的必然代价。对纯计算型速算规则耗时可忽略（实测 23.7µs/题）；若日后确有昂贵预计算的规则，需改为「每轮一个 VM + 调用前 `ClearInterrupt`」并配压力测试。已在 `sandbox.go` 写明。 |
| 3 | `MaxCallStackSize=200` 对深递归的合法规则可能偏紧 | 实测合法速算规则远低于此。它是**宿主策略**而非作者契约，日后可上调。 |
| 4 | 校验只跑 20 次 | 「只在极低概率分支出错」的规则仍有漏网可能。这是概率性防护，不是证明。已由 `ValidationRuns` 具名以便调整。 |

---

## 6. 结论

规格 §16 的 **P2 出口条件已满足：A2 / A3 / A4 全部通过**，
且三项均有变异测试或恶意用例取证。

**新增规模**：`internal/rules` 6 个源文件 + 1 个测试文件（69 项测试）。
**未新增依赖之外的抽象**：无 VM 池、无多函数契约、无 fallback。

**下一步**：P3 项目与练习闭环（项目 CRUD、`/rounds/start`、`/rounds/complete`、
答案信封分类、服务端重算 `server_is_correct`、分歧告警计数），
出口条件为 A11 / A14 通过并保证报告与落库一致。

# 实施计划 —— P2 规则引擎

- Date: `2026-09-10`
- Design Spec: [`docs/aegis/specs/2026-09-10-mental-math-app-design.md`](../specs/2026-09-10-mental-math-app-design.md)
- Baseline: [`docs/aegis/baseline/2026-09-10-initial-baseline.md`](../baseline/2026-09-10-initial-baseline.md)
- 上一阶段验收: [`evidence/p0-p1/ACCEPTANCE.md`](../work/2026-09-10-mental-math-app/evidence/p0-p1/ACCEPTANCE.md)
- 覆盖阶段：规格 §16 的 **P2 规则引擎**
- 后续阶段：P3 项目与练习闭环 → P3.5 判分单一性 → P4 前端 → P5 统计 → P6 分享订阅 → P7 CI

---

## Goal

实现 `internal/rules`：编译作者的 JS 出题规则，在**可复现的沙箱**中执行，并在保存前
完成校验。交付后可回答三个问题：

1. 作者的规则能不能安全地跑？（沙箱四配置齐全，恶意/错误规则被挡住）
2. 同一轮能否复现？（同种子两次生成完全一致）
3. 一条坏规则能不能被挡在保存之前？（不连坐订阅者）

**本计划不含**：项目 CRUD、`/rounds/*` 端点、答案信封分类、判分（P3/P3.5）。

## Architecture

```
internal/rules/            出题规则编译与沙箱执行的唯一 owner
  sandbox.go               沙箱配置常量与 VM 构造
  compile.go               Compile: 语法 + 顶层执行 + generate 可调用
  validate.go              Validate: 连续 N 次调用 + 返回形状校验
  generate.go              Generate: 生产期整轮出题
  errors.go                类型化错误，供 API 映射为 rule_invalid
  rules_test.go            表驱动测试（A2/A3/A4）
```

依赖方向：`rules` 只依赖 `goja`，**不依赖** `store`/`api`/`service`。
`service` 与 `api` 依赖 `rules`（由 P3 接线）。这保证规则引擎可独立测试。

## Tech Stack

| 项 | 选择 | 依据 |
|---|---|---|
| JS 引擎 | `github.com/dop251/goja` v0.0.0-20260906210903-70ad66ec7ce4 | 纯 Go，无 cgo，与 `CGO_ENABLED=0` 一致；已由 8 组实验验证 |

**已实测的 goja API 事实**（来自 `evidence/goja-spike`，避免重复踩坑）：

- `goja.AssertFunction(v)` 返回 `(Callable, bool)`，**不是** `(Callable, error)`
- `RunProgram` 返回**脚本完成值**（函数声明 → `undefined`），取函数必须用 `vm.Get("generate")`
- `SetRandSource(func() float64)` 让 `Math.random()` 可确定化
- `SetTimeSource(func() time.Time)` 让 `new Date()` 可确定化
- `SetMaxCallStackSize(n)` **必须设置**：不设时无限递归会挂死整个进程（实测跑满 300s 未返回）
- `Interrupt` / `ClearInterrupt` 可击杀死循环，且 VM 之后仍可用

## Baseline / Authority Refs

- 规格 §5（规则引擎：契约、沙箱四配置、保存期校验、实测踩坑）
- 规格 §5.5.1（为何保存期校验是强制项：坏规则会连坐所有订阅者）
- 规格 §14 R3（goja 无内存上限，以超时 + 递归上限代偿）
- 基线 §5.2（架构不可协商项 1、2：沙箱四配置齐全性；保存前必须校验）
- 基线 §9（兼容边界 2：沙箱四配置齐全性）
- 证据：`evidence/goja-spike/OUTPUT.txt`

## Compatibility Boundary

1. **沙箱四配置必须齐全**（`SetRandSource` / `SetTimeSource` / `SetMaxCallStackSize` / 调用超时）。
   缺任一即破坏基线 §5.2(1)。由 A4 守护。
2. **作者契约是 `function generate(cfg)` 返回 `{q, a}`**（D1/D11）。
   本计划**不得**扩展契约（不加 index 参数、不加 check 函数）。
3. **保存前必须校验**（基线 §5.2(2)）。校验失败必须给出可读错误，且**不落库**。
4. `rules` 不得反向依赖 `store`/`api`/`service`。

## TDD Route

```text
TDD Route:
- Mode: off
- Decision: skipped
- Strict authority: not applicable
- Test posture: post-change regression
- Reason: 项目配置 tdd_mode = "off"，用户未要求严格 TDD；Aegis 禁止仅凭风险推断 strict
- Verification: go test ./internal/rules/ -count=1 -v
```

> 说明：本计划为**安全边界**（不可信代码执行）编写表驱动回归测试，
> 属 proportional regression，而非 RED/GREEN 循环。

## Verification（全局命令）

```powershell
cd D:\Desktop\Cala\backend
go build ./... ; go vet ./... ; CGO_ENABLED=0 go test ./... -count=1
go test ./internal/rules/ -count=1 -v -timeout 180s
```

---

## Scope Check

**Aegis Visibility**：本阶段要落地的是**不可信代码执行**这一安全边界，因此即使 TDD 路由为
`skipped`，也为「沙箱四配置齐全性」与「坏规则必须被挡在保存前」写回归测试，
并把实测得到的 goja 陷阱（`AssertFunction` 返回值、`RunProgram` 完成值、递归上限必要性）
固化为代码注释与测试，避免后来者按直觉写出挂死进程的实现。

```text
Requirement Ready Check:
- Requirement source refs: 用户功能2「内嵌微型 js 解释器，写一个 js 函数代表一道题」
- Goals and scope refs: 规格 §5 全文；规格 §16 P2 行
- User / scenario refs: 项目作者编写规则；订阅者被动承受作者规则（故校验是强制的）
- Requirement item refs: 功能2
- Acceptance / verification criteria refs: A2（7 类错误源码被拒）、A3（同种子逐字相同）、A4（死循环被击杀且不影响后续）
- Open blocker questions: 无
- Decision: ready
```

```text
Change Necessity:
- User-visible need: 作者能自定义出题规则，且坏规则不能连坐订阅者
- No-change / non-code option: 不存在——功能2 的核心即此；无第三方组件可直接满足
  （内嵌 JS 引擎 + 本项目的校验语义与错误契约）
- Why code change is necessary: 无任何既有实现可复用
- Minimum change boundary: 新增 backend/internal/rules 一个包，不触及既有包
- Decision: code-change
```

```text
Existence Check:
- Proposed new surface: ①internal/rules 包 ②类型化错误 ValidationError ③输出长度上限
- Existing owner / reuse candidate: 既有 5 个包（config/store/api/service/auth）均不涉及 JS 执行
- Why existing surface is insufficient: 无 owner 承接；把规则引擎塞进 store 或 api 会破坏分层
- Creation proof:
  ① 规格 §5 定义该子系统，且 §16 将其列为独立阶段；
  ② 类型化错误是 API 层把校验失败映射为 rule_invalid 并回传可读消息的必要载体，
     否则只能靠字符串匹配错误内容（脆弱）；
  ③ 输出长度上限针对规格 §14 R3（goja 无内存上限）：这是唯一能由宿主控制的内存增长向量，
     属对已知残留风险的最小代偿，而非新增复杂度。
- Entropy / retirement impact: 无旧路径需退役。**明确拒绝**：不引入外部规则市场、
  不支持多函数契约、不实现 VM 池（P0 阶段已用 Existence Check 判定 reject）。
- Decision: add-with-proof
```

```text
Architecture Integrity Lens:
- Invariant: 沙箱四配置齐全（基线 §5.2(1)）；作者契约不得扩张（要求 2）
- Canonical owner / contract: rules 包是规则编译与执行的唯一 owner；
  沙箱配置集中在 sandbox.go 一个常量块，不得散落
- Responsibility overlap: 无——P2 不实现校验的 HTTP 映射（P3 负责），只提供类型化错误
- Higher-level simplification: 用「每次出题用全新 VM + 共享每轮 RNG」代替
  「复用 VM + 中断竞态处理」，以少量冷启动成本（实测 23.7µs/题）换取
  一整类中断残留竞态的消失
- Retirement / falsifier: 若日后有人改成复用 VM 并按直觉写 AfterFunc，可能引入
  「上一题的超时中断误杀下一题」的间歇性 bug → 由每问独立 VM 的设计与注释防住；
  A4 保证超时后仍可继续出题
- Verdict: 无阻塞问题，进入任务分解
```

```text
Plan Pressure Test:
- Owner / contract / retirement: 契约承载点已识别（沙箱配置、可复现性、校验）
- Architecture integrity / higher-level path: 已确认用「每问新 VM + 共享 RNG」消除竞态
- Verification scope: A2/A3/A4 各有对应测试；并有「超时后仍可继续」的断言
- Task executability: 每步含完整代码与确切命令
- Pressure result: proceed
```

```text
Plan-Time Complexity Check:
- Target files: backend/internal/rules/*.go（全新包，5 个源文件 + 1 个测试）
- Existing size / shape signals: 无既有压力（新包）
- Owner fit: rules 是 JS 执行的唯一 owner
- Add-in-place risk: 若把编译/校验/生成/沙箱全塞进一个 rules.go 会超预算且职责混杂
- Better file boundary: sandbox.go（配置与 VM 构造）/ compile.go / validate.go /
  generate.go / errors.go，各按单一职责拆分
- Recommendation: add owner file（按职责拆分文件）
```

---

## Task P2.1 —— goja 依赖与沙箱配置

**Files**
- Create: `backend/internal/rules/sandbox.go`
- Create: `backend/internal/rules/errors.go`

**Why**：把规格 §5.2 的沙箱四配置集中为**一个 owner 文件**。散落配置会让
「缺一项」这类错误难以发现，而缺 `SetMaxCallStackSize` 会挂死整个进程。

**Change Necessity**：`code-change`；最小边界 = 新增 `internal/rules` 包的两个文件。

**Impact / Compatibility**：承载基线 §5.2(1) 与 §9(2)。

**Steps**

1. 写 `backend/internal/rules/errors.go`：

```go
package rules

import "fmt"

// 校验阶段。API 层据此决定错误文案，也便于测试定位。
const (
	StageCompile = "compile" // 语法错误、顶层执行抛错、无 generate 函数
	StageExecute = "execute" // 调用 generate 时抛错
	StageTimeout = "timeout" // 调用超时（死循环等）
	StageShape   = "shape"   // 返回值不符合 {q, a} 契约
)

// ValidationError 表示作者规则未通过校验。
//
// 用类型而非字符串比较：API 层需要把「规则的错」与「服务端的错」分开处理，
// 前者回 400 rule_invalid 并附可读原因，后者回 500。
type ValidationError struct {
	Stage  string
	Detail string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("规则校验失败[%s]: %s", e.Stage, e.Detail)
}

func newErr(stage, format string, args ...any) *ValidationError {
	return &ValidationError{Stage: stage, Detail: fmt.Sprintf(format, args...)}
}
```

2. 写 `backend/internal/rules/sandbox.go`：

```go
// Package rules 是出题规则编译与沙箱执行的唯一 owner。
//
// 本包执行的是**不可信代码**（用户编写的 JS）。沙箱的四项配置是安全边界，
// 集中声明在本文件，不得散落到调用方。
package rules

import (
	"time"

	"github.com/dop251/goja"
)

const (
	// MaxCallStackSize 限制 JS 递归深度。
	//
	// 这不是可选优化：实测表明**不设置时，一条无限递归的规则会把整个 Go 进程挂死**
	// （跑满 300s 未返回）。goja 默认的栈限制远高于此，且耗尽时进程无法恢复。
	MaxCallStackSize = 200

	// QuestionTimeout 是单次 generate 调用的墙钟预算。
	// 实测：计时器设为 60ms 时，while(true) 在 61ms 被击杀，且 ClearInterrupt 后
	// VM 仍可复用。生产预算取 50ms。
	QuestionTimeout = 50 * time.Millisecond

	// ValidationRuns 是保存前连续调用 generate 的次数（规格 §5.3 第 3 步）。
	// 取 20 是为了让「偶尔才出错」的规则（例如只在特定分支死循环）有较高概率暴露。
	ValidationRuns = 20

	// MaxQuestionLen / MaxAnswerLen 限制单个题面与答案的长度（按 rune 计）。
	//
	// 针对规格 §14 R3：goja 无内存上限。规则的**计算量**由超时与递归深度约束，
	// 但**返回值大小**只能由宿主控制；一条 return {q: "x".repeat(1e9)} 的规则
	// 会把额度和磁盘一起吃掉。这里给一个宽松但有限的上界。
	MaxQuestionLen = 1000
	MaxAnswerLen   = 500

	// timeBase 是冻结时间源的基准时刻。
	// 实际返回值 = timeBase + (seed mod 86400) 秒，使依赖 Date 的规则
	// 在同一轮内**可复现**、在不同轮之间**仍有变化**。
	timeBase = 1577836800 // 2020-01-01T00:00:00Z, Unix 秒
)

// newVM 构造一个沙箱化的 goja 运行时。
//
// seed 决定 Math.random() 与 Date 的取值，从而决定本轮题目的可复现性。
// 每次出题都调用本函数新建 VM（而不是复用），理由见 sandbox.go 末尾注释。
func newVM(seed int64) (*goja.Runtime, error) {
	vm := goja.New()

	vm.SetMaxCallStackSize(MaxCallStackSize)

	// 确定性随机：同种子两次生成必须逐字相同（A3）。
	// 用 math/rand 的确定性源，绝不用全局随机源。
	rng := newSeededSource(seed)
	vm.SetRandSource(func() float64 { return rng.Float64() })

	// 确定性时间：避免 new Date() 破坏可复现性（规格 §5.2）。
	offset := seed % 86400
	if offset < 0 {
		offset += 86400
	}
	frozen := time.Unix(timeBase+offset, 0).UTC()
	vm.SetTimeSource(func() time.Time { return frozen })

	return vm, nil
}

// 设计说明：为什么每次出题新建 VM，而不是复用同一个 VM 并逐题 Interrupt？
//
// 复用 VM 需要在每次调用前后管理 time.AfterFunc + Interrupt + ClearInterrupt。
// 这里存在一个难以彻底消除的竞态：AfterFunc 的 goroutine 可能在调用返回之后、
// ClearInterrupt 之后才真正执行 Interrupt，于是这次超时中断会**误杀下一道题**，
// 表现为间歇性、难以复现的失败。
//
// 新建 VM 的成本实测仅 23.7µs/题（一轮 50 题约 1.2ms，复用为 8.8µs/题），
// 差异不足以支撑上述竞态的管理复杂度。用可忽略的冷启动成本换掉一整类竞态，
// 是这里的正确取舍；同时它也顺带消除了「上一题改写全局变量影响下一题」的问题。
//
// 复现性不受影响：随机源是每轮共享的确定性序列（见 newSeededSource），
// 因此「重新建 VM」不会让每道题抽到同一个数。
```

3. 再加 `backend/internal/rules/rng.go`（存放确定性随机源）：

```go
package rules

import "math/rand"

// seededSource 是每轮共享的确定性随机源。
//
// 它被注入到**每一道题各自的 VM** 中，因此题目之间延续同一条随机序列，
// 而整轮的结果由 seed 完全决定（A3）。
type seededSource struct {
	r *rand.Rand
}

func newSeededSource(seed int64) *seededSource {
	// #nosec G404 -- 这是出题用的可复现随机，不是安全用途，必须确定性
	return &seededSource{r: rand.New(rand.NewSource(seed))}
}

func (s *seededSource) Float64() float64 { return s.r.Float64() }
```

> 注：`math/rand` 的 `NewSource` 是确定性 PRNG，正是本处所需；
> 它**不可**用于安全用途。本包不含任何安全用途。

4. 验证：

```powershell
cd D:\Desktop\Cala\backend
go get github.com/dop251/goja@v0.0.0-20260906210903-70ad66ec7ce4
go build ./... ; go vet ./...
```

5. **Commit**：`feat(rules): goja 依赖与沙箱配置`

---

## Task P2.2 —— 规则编译

**Files**
- Create: `backend/internal/rules/compile.go`
- Create: `backend/internal/rules/compile_test.go`

**Why**：把「源码是不是能跑、有没有 `generate`」这个判断集中在一处，并产出可复用的
`*goja.Program`（编译一次、多轮复用）。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：拒绝行为是功能2 的错误契约的一部分。

**Steps**

1. 写 `backend/internal/rules/compile.go`：

```go
package rules

import (
	"fmt"

	"github.com/dop251/goja"
)

// Rule 是一条已通过语法与顶层检查的出题规则。
//
// prog 可跨 VM 复用（goja.Program 是不可变的），这是「编译一次、多轮执行」的基础。
type Rule struct {
	source string
	prog   *goja.Program
}

// Source 返回作者可见的原始源码。
func (r *Rule) Source() string { return r.source }

// Compile 检查源码语法、执行顶层语句、并确认 generate 是可调用函数。
//
// 它**不**校验 generate 的返回形状——那需要真正调用，由 Validate 完成。
// 分开的理由：Compile 很便宜且无副作用，可在每次加载项目时做；
// Validate 会真的执行作者代码 20 次，只在保存时做。
func Compile(source string) (*Rule, error) {
	if source == "" {
		return nil, newErr(StageCompile, "规则源码为空")
	}

	prog, err := goja.Compile("rule.js", source, false)
	if err != nil {
		return nil, newErr(StageCompile, "语法错误: %s", firstLine(err.Error()))
	}

	// 必须在 VM 中执行一次，才能发现顶层抛错（例如 throw new Error("x")）
	// 以及确认 generate 真的存在。
	vm := goja.New()
	vm.SetMaxCallStackSize(MaxCallStackSize)
	if _, err := vm.RunProgram(prog); err != nil {
		return nil, newErr(StageCompile, "顶层执行失败: %s", firstLine(err.Error()))
	}

	// 注意：RunProgram 返回的是**脚本完成值**，函数声明语句的完成值是 undefined，
	// 因此不能拿它的返回值当函数，必须用 vm.Get 取具名绑定。
	if _, ok := goja.AssertFunction(vm.Get("generate")); !ok {
		return nil, newErr(StageCompile,
			"未找到可调用的 generate 函数；规则必须定义 function generate(cfg) { ... }")
	}

	return &Rule{source: source, prog: prog}, nil
}

// firstLine 只取错误首行，避免把 goja 的多行堆栈直接回给作者。
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

var _ = fmt.Sprintf // 保留 fmt 供后续使用
```

> 实现时删除末尾的 `var _ = fmt.Sprintf` 与其 import，它只是占位说明。

2. 写 `backend/internal/rules/compile_test.go`：

```go
package rules

import (
	"errors"
	"strings"
	"testing"
)

const goodRule = `
function generate(cfg) {
  const a = cfg.min + Math.floor(Math.random() * (cfg.max - cfg.min + 1));
  const b = cfg.min + Math.floor(Math.random() * (cfg.max - cfg.min + 1));
  return { q: a + " + " + b + " = ?", a: String(a + b) };
}`

// TestCompileRejectsBadSources 覆盖验收标准 A2 的前半部分。
// 这些是实测中确实会被 goja 拒绝的写法，逐条固化防止回归。
func TestCompileRejectsBadSources(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		stage string
	}{
		{"空源码", "", StageCompile},
		{"缺少 generate", `function gen(cfg){ return {q:"1",a:"1"} }`, StageCompile},
		{"generate 不是函数", `var generate = 42`, StageCompile},
		{"语法错误", `function generate( { return }`, StageCompile},
		{"未终止注释", `/* oops`, StageCompile},
		{"顶层抛错", `throw new Error("boom")`, StageCompile},
		{"generate 声明为对象", `const generate = { call: 1 }`, StageCompile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(tc.src)
			if err == nil {
				t.Fatalf("应被拒绝但通过了")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("错误类型应为 *ValidationError, 得到 %T", err)
			}
			if ve.Stage != tc.stage {
				t.Errorf("阶段 = %q, 期望 %q (detail=%s)", ve.Stage, tc.stage, ve.Detail)
			}
			if ve.Detail == "" {
				t.Error("Detail 不应为空——作者需要可读原因")
			}
		})
	}
}

func TestCompileAcceptsGoodRule(t *testing.T) {
	r, err := Compile(goodRule)
	if err != nil {
		t.Fatalf("合法规则应通过: %v", err)
	}
	if r.Source() != goodRule {
		t.Error("Source() 应返回原始源码")
	}
}

// TestCompileAcceptsModernSyntax 记录沙箱支持的语法面（实测 28/30 通过）。
// 作者能用这些写法，故它们是契约的一部分，需要回归保护。
func TestCompileAcceptsModernSyntax(t *testing.T) {
	features := map[string]string{
		"模板字符串":  "const s = `x${1}`;",
		"箭头函数":   "const f = (a=1) => a;",
		"解构":     "const {a,b=2} = {a:1};",
		"展开":     "const xs = [...[1,2], 3];",
		"可选链":    "const o = {a:{b:1}}; const v = o?.a?.b;",
		"空值合并":   "const z = null ?? 7;",
		"类":      "class A { get y() { return 1 } }",
		"for-of": "let n = 0; for (const v of [1,2]) n += v;",
		"BigInt": "const b = 1n;",
		"正则命名组":  `const m = /(?<n>\d+)/.exec("a12");`,
		"生成器":    "function* g(){ yield 1; }",
	}
	for name, snippet := range features {
		t.Run(name, func(t *testing.T) {
			src := snippet + "\nfunction generate(cfg){ return {q:'1',a:'1'} }"
			if _, err := Compile(src); err != nil {
				t.Errorf("语法 %s 应被支持, 得到 %v", name, err)
			}
		})
	}
}

func TestFirstLineTruncates(t *testing.T) {
	long := strings.Repeat("x", 500) + "\nsecond line"
	got := firstLine(long)
	if strings.Contains(got, "\n") {
		t.Error("firstLine 不应包含换行")
	}
	if len([]rune(got)) > 201 {
		t.Errorf("firstLine 应截断到 200 字符左右, 得到 %d", len([]rune(got)))
	}
}
```

3. 验证：

```powershell
cd D:\Desktop\Cala\backend
go test ./internal/rules/ -count=1 -v -timeout 180s
```

4. **Commit**：`feat(rules): 规则编译与顶层检查`

---

## Task P2.3 —— 保存期校验

**Files**
- Create: `backend/internal/rules/validate.go`
- Create: `backend/internal/rules/validate_test.go`

**Why**：这是**基线 §5.2(2)** 的载体，也是规格 §5.5.1 的落点：
作者改规则会**同步影响所有订阅者**，因此一条坏规则必须在保存时就被挡住，
否则连坐全员。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：校验失败必须**不产生任何副作用**（不落库、不改配置）。

**Steps**

1. 写 `backend/internal/rules/validate.go`：

```go
package rules

import (
	"time"

	"github.com/dop251/goja"
)

// Validate 连续调用 generate 共 ValidationRuns 次，检查每次调用的
// 耗时、异常与返回形状。任一次失败即返回 *ValidationError。
//
// cfg 应为**即将保存的**配置，使校验反映真实使用条件。
func (r *Rule) Validate(cfg map[string]any) error {
	for i := 0; i < ValidationRuns; i++ {
		// 每题用固定种子，使校验本身也可复现（便于把作者报的错误重现出来）
		vm, err := newVM(int64(i))
		if err != nil {
			return newErr(StageExecute, "创建运行时失败: %s", firstLine(err.Error()))
		}
		q, err := callGenerate(vm, r.prog, cfg)
		if err != nil {
			return err
		}
		if err := checkShape(q); err != nil {
			return err
		}
	}
	return nil
}

// callGenerate 在给定 VM 上执行一次 generate(cfg)，带超时与 panic 隔离。
func callGenerate(vm *goja.Runtime, prog *goja.Program, cfg map[string]any) (any, error) {
	// 已编译过的 Program 可跨 VM 复用；这里重新载入以获得本轮 VM 中的绑定。
	if _, err := vm.RunProgram(prog); err != nil {
		return nil, newErr(StageExecute, "载入规则失败: %s", firstLine(err.Error()))
	}
	fn, ok := goja.AssertFunction(vm.Get("generate"))
	if !ok {
		// Compile 已保证过；此处属于防御，说明 Program 与 VM 不匹配
		return nil, newErr(StageExecute, "generate 不可调用")
	}

	// panic 隔离：本包执行的是不可信代码。goja 通常把 JS 异常转成 error，
	// 但宿主侧的极端情况（如栈耗尽）可能以 panic 形式冒出。规则引擎作为
	// 不可信代码的边界，不应让一次规则执行杀死整个服务。
	var (
		result any
		callErr error
	)
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				callErr = newErr(StageExecute, "规则执行引发内部异常: %v", rec)
			}
		}()

		timer := time.AfterFunc(QuestionTimeout, func() {
			vm.Interrupt("rule timeout")
		})
		defer timer.Stop()

		v, err := fn(goja.Undefined(), vm.ToValue(cfg))
		if err != nil {
			callErr = classifyCallError(err)
			return
		}
		result = v.Export()
	}()

	// 无论成功与否都清掉中断标记，避免留给后续调用
	vm.ClearInterrupt()

	if callErr != nil {
		return nil, callErr
	}
	return result, nil
}

// classifyCallError 把 goja 的错误区分为「超时」与「规则自身抛错」。
//
// 区分很重要：超时说明规则可能死循环（作者需修的是性能/逻辑），
// 抛错说明规则逻辑错误（作者需修的是 bug）。两者的提示语不同。
func classifyCallError(err error) error {
	if ie, ok := err.(*goja.InterruptedError); ok {
		return newErr(StageTimeout,
			"单次出题超过 %s 未返回（原因: %v）；请检查是否存在死循环或过重的计算",
			QuestionTimeout, ie.Value())
	}
	return newErr(StageExecute, "generate 抛出异常: %s", firstLine(err.Error()))
}

// checkShape 校验返回值符合 {q, a} 契约（D11）。
func checkShape(v any) error {
	obj, ok := v.(map[string]any)
	if !ok {
		return newErr(StageShape,
			"generate 必须返回对象 {q: string, a: string}，实际返回 %T", v)
	}

	q, ok := obj["q"].(string)
	if !ok {
		return newErr(StageShape, "返回值缺少字符串字段 q（题面）")
	}
	a, ok := obj["a"].(string)
	if !ok {
		return newErr(StageShape, "返回值缺少字符串字段 a（答案）")
	}
	if q == "" {
		return newErr(StageShape, "题面 q 不能为空字符串")
	}
	if a == "" {
		return newErr(StageShape, "答案 a 不能为空字符串")
	}
	if n := len([]rune(q)); n > MaxQuestionLen {
		return newErr(StageShape, "题面 q 过长（%d 个字符，上限 %d）", n, MaxQuestionLen)
	}
	if n := len([]rune(a)); n > MaxAnswerLen {
		return newErr(StageShape, "答案 a 过长（%d 个字符，上限 %d）", n, MaxAnswerLen)
	}
	return nil
}
```

2. 写 `backend/internal/rules/validate_test.go`：

```go
package rules

import (
	"errors"
	"strings"
	"testing"
)

func mustCompile(t *testing.T, src string) *Rule {
	t.Helper()
	r, err := Compile(src)
	if err != nil {
		t.Fatalf("Compile 失败: %v", err)
	}
	return r
}

func cfgRange(min, max int) map[string]any {
	return map[string]any{"min": min, "max": max}
}

func TestValidateAcceptsGoodRule(t *testing.T) {
	if err := mustCompile(t, goodRule).Validate(cfgRange(1, 9)); err != nil {
		t.Fatalf("合法规则应通过校验: %v", err)
	}
}

// TestValidateRejectsBadBehaviour 覆盖验收标准 A2 的后半部分：
// 语法正确但运行期有问题的规则也必须被挡在保存之前。
func TestValidateRejectsBadBehaviour(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		stage  string
		substr string
	}{
		{
			"死循环",
			`function generate(cfg){ while(true){} }`,
			StageTimeout, "未返回",
		},
		{
			"无限递归",
			`function generate(cfg){ return generate(cfg) }`,
			StageExecute, "递归过深",
		},
		{
			"调用期未定义引用",
			`function generate(cfg){ return notDefined.x }`,
			StageExecute, "抛出异常",
		},
		{
			"调用期主动抛错",
			`function generate(cfg){ throw new Error("boom") }`,
			StageExecute, "抛出异常",
		},
		{
			"返回数字",
			`function generate(cfg){ return 42 }`,
			StageShape, "必须返回对象",
		},
		{
			"返回 null",
			`function generate(cfg){ return null }`,
			StageShape, "必须返回对象",
		},
		{
			"返回 undefined",
			`function generate(cfg){ }`,
			StageShape, "必须返回对象",
		},
		{
			"缺少 q",
			`function generate(cfg){ return {a:"1"} }`,
			StageShape, "q",
		},
		{
			"缺少 a",
			`function generate(cfg){ return {q:"1+1"} }`,
			StageShape, "a",
		},
		{
			"q 不是字符串",
			`function generate(cfg){ return {q:1, a:"1"} }`,
			StageShape, "q",
		},
		{
			"a 不是字符串",
			`function generate(cfg){ return {q:"1", a:2} }`,
			StageShape, "a",
		},
		{
			"q 为空串",
			`function generate(cfg){ return {q:"", a:"1"} }`,
			StageShape, "q 不能为空",
		},
		{
			"a 为空串",
			`function generate(cfg){ return {q:"1", a:""} }`,
			StageShape, "a 不能为空",
		},
		{
			"题面过长",
			`function generate(cfg){ return {q:"x".repeat(2000), a:"1"} }`,
			StageShape, "过长",
		},
		{
			"答案过长",
			`function generate(cfg){ return {q:"1", a:"x".repeat(2000)} }`,
			StageShape, "过长",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mustCompile(t, tc.src).Validate(cfgRange(1, 9))
			if err == nil {
				t.Fatal("应校验失败但通过了")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("错误类型应为 *ValidationError, 得到 %T", err)
			}
			if ve.Stage != tc.stage {
				t.Errorf("阶段 = %q, 期望 %q (detail=%s)", ve.Stage, tc.stage, ve.Detail)
			}
			if !strings.Contains(ve.Detail, tc.substr) {
				t.Errorf("Detail 应包含 %q, 实际 %q", tc.substr, ve.Detail)
			}
		})
	}
}

// TestValidateRejectsOnlySometimesFailingRule 覆盖「连续 20 次」的意义：
// 只在某一题才出错的规则也必须被拒绝，否则它会在线上的某一轮炸掉订阅者。
func TestValidateRejectsOnlySometimesFailingRule(t *testing.T) {
	// cfg 未提供 min/max 时返回合法题面；提供时抛错。
	// 校验用固定种子 i=0..19，故这条规则在校验中必然被触发。
	src := `
function generate(cfg) {
  if (cfg.explode === true) { throw new Error("boom") }
  return { q: "1+1", a: "2" };
}`
	rule := mustCompile(t, src)

	if err := rule.Validate(map[string]any{}); err != nil {
		t.Fatalf("不含 explode 的配置应通过: %v", err)
	}
	if err := rule.Validate(map[string]any{"explode": true}); err == nil {
		t.Fatal("含 explode 的配置应被拒绝——校验必须反映即将保存的配置")
	}
}

// TestValidateDoesNotLeakStateBetweenRuns 保证校验过程无副作用累积。
func TestValidateDoesNotLeakStateBetweenRuns(t *testing.T) {
	rule := mustCompile(t, goodRule)
	for i := 0; i < 3; i++ {
		if err := rule.Validate(cfgRange(1, 9)); err != nil {
			t.Fatalf("第 %d 次校验应通过: %v", i+1, err)
		}
	}
}

// TestValidateHandlesHostileRule 确认恶意规则不会把进程带走。
func TestValidateHandlesHostileRule(t *testing.T) {
	hostile := []string{
		`function generate(cfg){ while(true){} }`,
		`function generate(cfg){ for(;;){} }`,
		`function generate(cfg){ let i=0; while(i>=0){ i++ } }`,
		`function generate(cfg){ return generate(cfg) }`,
		`function generate(cfg){ const a = new Array(1e9); return {q:"1",a:"1"} }`,
	}
	for i, src := range hostile {
		t.Run(strings.Fields(src)[2], func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				done <- mustCompile(t, src).Validate(cfgRange(1, 9))
			}()
			select {
			case err := <-done:
				if err == nil {
					t.Errorf("恶意规则 #%d 应被拒绝", i)
				}
			case <-timeAfterSeconds(30):
				t.Fatalf("恶意规则 #%d 卡住超过 30s——沙箱失效", i)
			}
		})
	}
}
```

需在测试文件 import 中加 `"time"`，并加辅助函数：

```go
func timeAfterSeconds(n int) <-chan time.Time {
	return time.After(time.Duration(n) * time.Second)
}
```

3. 验证：

```powershell
cd D:\Desktop\Cala\backend
go test ./internal/rules/ -count=1 -v -timeout 180s
```

4. **Commit**：`feat(rules): 保存期校验与返回形状检查`

---

## Task P2.4 —— 整轮出题与可复现性

**Files**
- Create: `backend/internal/rules/generate.go`
- Create: `backend/internal/rules/generate_test.go`

**Why**：交付 P3 需要的生产期入口，并落地验收标准 A3（同种子逐字相同）。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：`Generate` 的返回顺序即题号顺序，P3 依赖它写入 `attempt.idx`。

**Steps**

1. 写 `backend/internal/rules/generate.go`：

```go
package rules

// Question 是一道已生成的题目。
type Question struct {
	// Index 是题号，从 0 开始，与 attempt.idx 对应。
	Index int
	// Q 是题面，A 是答案原文。
	// P3 会把两者作为快照落库（规格 §6.1(3)），使作者事后改规则不影响历史。
	Q string
	A string
}

// Generate 生成一轮的题目。
//
// seed 由调用方（P3）为每轮随机生成并落库；同一 (rule, cfg, seed, count)
// 必须产出完全相同的结果（验收标准 A3），这是「重练错题」与问题复现的基础。
func (r *Rule) Generate(cfg map[string]any, seed int64, count int) ([]Question, error) {
	if count <= 0 {
		return nil, newErr(StageShape, "题数必须为正数，得到 %d", count)
	}

	// 随机源在整轮内共享，使题目之间延续同一条确定性序列。
	// 每道题仍在各自的 VM 中执行（见 sandbox.go 的取舍说明）。
	rng := newSeededSource(seed)
	baseOffset := seed % 86400
	if baseOffset < 0 {
		baseOffset += 86400
	}

	out := make([]Question, 0, count)
	for i := 0; i < count; i++ {
		vm, err := newVMWithSource(rng, baseOffset)
		if err != nil {
			return nil, newErr(StageExecute, "创建运行时失败: %s", firstLine(err.Error()))
		}
		v, err := callGenerate(vm, r.prog, cfg)
		if err != nil {
			return nil, err
		}
		if err := checkShape(v); err != nil {
			return nil, err
		}
		obj := v.(map[string]any)
		out = append(out, Question{
			Index: i,
			Q:     obj["q"].(string),
			A:     obj["a"].(string),
		})
	}
	return out, nil
}
```

2. 在 `sandbox.go` 增加按需注入共享随机源的构造器（供 `Generate` 使用）：

```go
// newVMWithSource 是沙箱配置的**唯一实现**。
// rng 可跨多个 VM 共享（Generate 用它让整轮延续同一条随机序列）。
func newVMWithSource(rng *seededSource, dayOffset int64) (*goja.Runtime, error) {
	vm := goja.New()
	vm.SetMaxCallStackSize(MaxCallStackSize)
	vm.SetRandSource(func() float64 { return rng.Float64() })

	frozen := time.Unix(timeBase+dayOffset, 0).UTC()
	vm.SetTimeSource(func() time.Time { return frozen })
	return vm, nil
}
```

并把 `newVM` 改为复用它，避免两处配置漂移（**这是关键**：沙箱配置若有第二份拷贝，
就会出现「一处设了、一处漏了」的缺口）：

```go
func newVM(seed int64) (*goja.Runtime, error) {
	offset := seed % 86400
	if offset < 0 {
		offset += 86400
	}
	return newVMWithSource(newSeededSource(seed), offset)
}
```

3. 写 `backend/internal/rules/generate_test.go`：

```go
package rules

import (
	"strings"
	"testing"
)

// TestGenerateIsReproducible 覆盖验收标准 A3。
// 同一 (rule, cfg, seed, count) 必须产出逐字相同的结果，
// 否则「重练错题」与线上问题复现都无从谈起。
func TestGenerateIsReproducible(t *testing.T) {
	rule := mustCompile(t, goodRule)

	a, err := rule.Generate(cfgRange(10, 99), 42, 50)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	b, err := rule.Generate(cfgRange(10, 99), 42, 50)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}

	if len(a) != 50 {
		t.Fatalf("题数 = %d, 期望 50", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("第 %d 题不一致:\n  A=%+v\n  B=%+v", i, a[i], b[i])
		}
	}
}

func TestGenerateVariesWithSeed(t *testing.T) {
	rule := mustCompile(t, goodRule)

	a, err := rule.Generate(cfgRange(10, 99), 42, 50)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	b, err := rule.Generate(cfgRange(10, 99), 43, 50)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}

	same := 0
	for i := range a {
		if a[i] == b[i] {
			same++
		}
	}
	if same == len(a) {
		t.Error("不同种子产生了完全相同的题目序列——种子未生效")
	}
}

// TestGenerateAdvancesRandomSequence 保证题与题之间抽到不同的数。
// 若随机源被错误地按题重置，所有题会一模一样，这条测试会发现。
func TestGenerateAdvancesRandomSequence(t *testing.T) {
	rule := mustCompile(t, goodRule)

	qs, err := rule.Generate(cfgRange(10, 99), 7, 30)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}

	distinct := map[string]bool{}
	for _, q := range qs {
		distinct[q.Q] = true
	}
	// 30 道两位数加法，重复率不应高到只剩个位数种
	if len(distinct) < 10 {
		t.Errorf("30 道题只有 %d 种不同题面，随机序列疑似未推进", len(distinct))
	}
}

func TestGenerateRespectsConfig(t *testing.T) {
	rule := mustCompile(t, goodRule)

	qs, err := rule.Generate(cfgRange(1, 1), 1, 5)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	for _, q := range qs {
		// min=max=1 时，每道题都必须是 "1 + 1 = ?"，答案为 2
		if q.Q != "1 + 1 = ?" || q.A != "2" {
			t.Errorf("cfg 未生效: %+v", q)
		}
	}
}

func TestGenerateIndexesSequentially(t *testing.T) {
	rule := mustCompile(t, goodRule)

	qs, err := rule.Generate(cfgRange(1, 9), 3, 10)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	for i, q := range qs {
		if q.Index != i {
			t.Errorf("第 %d 项 Index = %d, 期望 %d（P3 依赖该顺序写 attempt.idx）", i, q.Index, i)
		}
	}
}

func TestGenerateRejectsNonPositiveCount(t *testing.T) {
	rule := mustCompile(t, goodRule)
	if _, err := rule.Generate(cfgRange(1, 9), 1, 0); err == nil {
		t.Error("题数为 0 应被拒绝")
	}
	if _, err := rule.Generate(cfgRange(1, 9), 1, -1); err == nil {
		t.Error("题数为负应被拒绝")
	}
}

// TestGenerateFrozenTime 验证冻结时间源：依赖 Date 的规则同种子可复现。
func TestGenerateFrozenTime(t *testing.T) {
	rule := mustCompile(t, `
function generate(cfg) {
  const d = new Date();
  return { q: "y" + d.getUTCFullYear(), a: String(d.getUTCDate()) };
}`)

	a, err := rule.Generate(nil, 12345, 3)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	b, err := rule.Generate(nil, 12345, 3)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("依赖 Date 的规则应可复现, 第 %d 题不同: %+v vs %+v", i, a[i], b[i])
		}
	}
	// 同轮内所有题应看到同一个冻结时刻
	for i := 1; i < len(a); i++ {
		if a[i].Q != a[0].Q {
			t.Errorf("同轮内时间应冻结, 第 %d 题看到 %q 而第 0 题看到 %q", i, a[i].Q, a[0].Q)
		}
	}

	// 不同种子应给出不同的日期（在一天内偏移），证明时间源确实随种子变化
	c, err := rule.Generate(nil, 99999, 1)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if a[0].A == c[0].A && a[0].Q == c[0].Q {
		t.Log("提示: 两个种子恰好落在同一天，属可接受的低概率事件")
	}
}

// TestGenerateRejectsTimeoutRule 覆盖验收标准 A4：死循环规则被击杀，
// 且**不影响后续出题**（沙箱可恢复）。
func TestGenerateRejectsTimeoutRule(t *testing.T) {
	bad := mustCompile(t, `function generate(cfg){ while(true){} }`)

	if _, err := bad.Generate(nil, 1, 3); err == nil {
		t.Fatal("死循环规则应被拒绝")
	}

	// 关键：淘汰掉坏规则后，好规则必须仍能正常出题。
	// 若沙箱中断状态泄漏到全局，这里会失败。
	good := mustCompile(t, goodRule)
	qs, err := good.Generate(cfgRange(1, 9), 5, 20)
	if err != nil {
		t.Fatalf("坏规则之后, 好规则应仍可出题: %v", err)
	}
	if len(qs) != 20 {
		t.Errorf("题数 = %d, 期望 20", len(qs))
	}
	for _, q := range qs {
		if strings.Contains(q.Q, "timeout") {
			t.Errorf("题目内容异常: %q", q.Q)
		}
	}
}

// TestGenerateIsStatelessAcrossQuestions 把「每问新建 VM」这一设计选择
// 固化为**显式契约**：规则的模块级状态不会在题目之间保留。
//
// 这不是副作用而是有意为之（见 sandbox.go 的取舍说明）：
// 它让 generate 成为 cfg 与随机数的纯函数，从而同种子必然可复现。
// 若日后改为「每轮一个 VM」，本测试会失败并强制维护者显式确认该变更。
func TestGenerateIsStatelessAcrossQuestions(t *testing.T) {
	rule := mustCompile(t, `
var counter = 0;
function generate(cfg) {
  counter++;
  return { q: String(counter), a: "0" };
}`)

	qs, err := rule.Generate(nil, 1, 5)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	for i, q := range qs {
		if q.Q != "1" {
			t.Errorf("第 %d 题 counter=%q，期望 1——模块级状态不应跨题保留", i, q.Q)
		}
	}

	// 两轮结果必须完全一致（可复现性的直接体现）
	again, err := rule.Generate(nil, 1, 5)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	for i := range qs {
		if qs[i] != again[i] {
			t.Fatalf("同种子两轮应完全一致, 第 %d 题: %+v vs %+v", i, qs[i], again[i])
		}
	}
}
```

> **执行期修正**：计划原写的是 `TestGenerateDoesNotLeakBetweenRounds`，断言
> 「同轮内 counter 递增到 10、下一轮从 1 重新开始」。但既然**每道题各建一个 VM**，
> 模块级 `counter` 会在每道题重新初始化为 1，原断言自相矛盾。
> 实测确认实现行为是「始终 1」。已把测试改写为
> `TestGenerateIsStatelessAcrossQuestions`，把无状态性作为**显式契约**固定下来，
> 而不是假装它递增。

4. 验证：

```powershell
cd D:\Desktop\Cala\backend
go build ./... ; go vet ./... ; go test ./internal/rules/ -count=1 -v -timeout 180s
```

5. **Commit**：`feat(rules): 整轮出题与可复现性`

---

## Task P2.5 —— P2 验收

**Files**：无新增（验证与证据）

**Steps**

1. 全量验证：

```powershell
cd D:\Desktop\Cala\backend
go build ./... ; go vet ./... ; CGO_ENABLED=0 go test ./... -count=1 -timeout 300s
go test ./internal/rules/ -count=1 -v -timeout 180s
```

2. 逐条核对验收标准，把实际输出写入
   `docs/aegis/work/2026-09-10-mental-math-app/evidence/p2/ACCEPTANCE.md`：

| 标准 | 证明它的确切命令 |
|---|---|
| A2 7 类错误源码被拒 | `go test ./internal/rules/ -run TestCompileRejectsBadSources -v` 与 `-run TestValidateRejectsBadBehaviour -v` |
| A3 同种子逐字相同 | `go test ./internal/rules/ -run TestGenerateIsReproducible -v` |
| A4 死循环被击杀且不影响后续 | `go test ./internal/rules/ -run TestGenerateRejectsTimeoutRule -v` 与 `-run TestValidateHandlesHostileRule -v` |
| 沙箱四配置齐全 | `-run TestPragmas` 不适用；改由 `-run TestGenerateFrozenTime`（时间源）+ 上述可复现性（随机源）+ 递归/超时测试共同覆盖 |

3. 更新工作记录 checkpoint 与证据，提交。

4. **Commit**：`test(rules): P2 验收记录`

---

## Risks

| # | 风险 | 处置 |
|---|---|---|
| R-P2-1 | 后来者改成「复用 VM + AfterFunc」而引入中断残留竞态 | 每问新建 VM，并在 `sandbox.go` 末尾以注释说明取舍与代价 |
| R-P2-2 | 沙箱配置出现第二份拷贝，导致「一处设了、一处漏了」 | `newVM` 与 `newVMWithSource` 共用同一实现，配置只写一次 |
| R-P2-3 | `MaxCallStackSize=200` 对合法递归规则偏紧 | 它是宿主策略而非作者契约，日后可上调；已设为具名常量并有注释 |
| R-P2-4 | goja 无内存上限（规格 §14 R3） | 以超时 + 递归上限 + 输出长度上限代偿；**残余风险仍存在**（一条分配巨量中间数组的规则仍可吃内存），诚实记录，不宣称已解决 |
| R-P2-5 | `math/rand` 用于出题随机被误用为安全用途 | `rng.go` 注释明确标注「不可用于安全用途」 |

## Retirement

- **Old owner / fallback**：无。P2 全部为新增，无旧路径、无兼容分支。
- **Active status**：N/A。
- **Deletion trigger**：无。

> 明确**拒绝**：VM 池（P0 已判 reject）、多函数契约、外部规则市场、规则热加载。

## ADR Signals（待 P2 完成后回填）

本计划为 **ADR-1「规则引擎契约与沙箱边界」** 的落地。P2 完成后应回填 ADR-1，
内容包括：`generate(cfg)` 契约、沙箱四配置、每问新建 VM 的取舍、
输出长度上限、以及被排除的替代方案（复用 VM、VM 池、共享 JS 源码判分）。

---

## Execution Readiness View

```text
Execution Readiness View:
- Intent Lock: 交付 internal/rules —— 编译、沙箱执行、保存期校验、整轮可复现出题
- Scope Fence: 仅 backend/internal/rules；不触及 store/api/service，不实现 /rounds 端点
- Baseline Lock: 规格 §5 全文、§14 R3；基线 §5.2(1)(2)、§9(2)；evidence/goja-spike
- Approved Behavior: 作者写 function generate(cfg) 返回 {q, a}；
  同 (rule,cfg,seed,count) 逐字可复现；坏规则在保存前被拒并给出可读原因
- Owner / Contract Constraints: rules 是 JS 执行的唯一 owner；
  沙箱配置集中一处；不依赖 store/api/service
- Compatibility Boundary: 作者契约不得扩张（D1/D11）；沙箱四配置不得缺项
- Retirement Boundary: 无退役对象；明确拒绝 VM 池与多函数契约
- Task Batches: P2.1（沙箱配置）→ P2.2（编译）→ P2.3（校验）→ P2.4（出题）→ P2.5（验收）
- Test Obligations: 编译拒绝 7 项、语法支持 11 项、行为拒绝 15 项、
  可复现性/种子生效/序列推进/cfg 生效/索引顺序/冻结时间/超时恢复/无状态泄漏
- Review Gates: 每个 Task 后 go build/vet/test；P2.5 逐条核对 A2/A3/A4
- Drift / Rewind Rules: 若实现中需要扩张作者契约、新增 owner 或改动既有包，停止并回到规格
- Evidence Required Before Completion: go build/vet/test 全绿输出；
  A2/A3/A4 对应测试通过；恶意规则 30s 内被拒
- Advisory Boundary: method-pack execution guidance only; not GateDecision,
  PolicySnapshot, or completion authority
```

## Execution Route

```text
Execution Route:
- Decision: inline
- Evidence: P2.1→P2.4 严格顺序依赖（sandbox → compile → validate → generate），
  且四个文件共享同一包与同一组常量，并行会产生写冲突而非收益
- Fallback: 无
- User confirmation required: no
```

**REQUIRED SUB-SKILL**：aegis:executing-plans

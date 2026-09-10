# 实施计划 —— P3 项目与练习闭环（后端）

- Date: `2026-09-10`
- Design Spec: [`docs/aegis/specs/2026-09-10-mental-math-app-design.md`](../specs/2026-09-10-mental-math-app-design.md)
- Baseline: [`docs/aegis/baseline/2026-09-10-initial-baseline.md`](../baseline/2026-09-10-initial-baseline.md)
- 上一阶段验收: [`evidence/p2/ACCEPTANCE.md`](../work/2026-09-10-mental-math-app/evidence/p2/ACCEPTANCE.md)
- 覆盖阶段：规格 §16 的 **P3 项目与练习闭环（后端）**
- 后续：P3.5 判分单一性语料门禁 → P4 前端 → P5 统计 → P6 分享订阅 → P7 CI

---

## Goal

让「建项目 → 出一轮题 → 交卷落库」这条主线在服务端可用：

1. 作者能创建/查看/修改/删除自己的项目；创建与修改时**规则必须通过保存期校验**。
2. 能取到一轮题目（含判分信封与服务端下发的清洗表）。
3. 交卷时服务端**独立重算** `server_is_correct`，并保留客户端判定用于分歧告警。

**本计划不含**：分享链接与订阅导入/退订（P6）、统计端点（P5）、前端（P4）、
语料生成器与 Dart 镜像实现（P3.5）。

## Architecture

```
internal/scoring/          判分语义的唯一 owner（纯函数，无外部依赖）
  envelope.go              答案信封：分类与序列化
  cleanup.go               清洗表（作为数据下发）与规范化
  rational.go              big.Int 有理数解析与交叉相乘比较
  compare.go               Compare: 判定一次作答是否正确

internal/store/project.go  项目与轮次的数据访问
internal/service/project.go 项目业务规则（所有权、校验）
internal/service/round.go   出题与交卷
internal/api/project.go     项目 HTTP 端点
internal/api/round.go       轮次 HTTP 端点
```

依赖方向：`api → service → {store, rules, scoring}`。
`scoring` **不依赖任何本仓库其他包**，因此可被 P3.5 的语料生成器独立复用。

## Tech Stack

无新增第三方依赖。判分用标准库 `math/big`。

## Baseline / Authority Refs

- 规格 §5.5 全文（答案域收敛、无浮点比较、清洗表单一 owner、分歧告警）
- 规格 §5.5.2 / §5.5.3 / §5.5.4（**含本次修正后的文本答案规则与容差列**）
- 规格 §6 / §6.1（数据模型与五条不变量）、§6.2（引导管理员）
- 规格 §7（API 契约、`/rounds/complete` 的服务端行为五条）
- 基线 §5.2（架构不可协商项 1–8）、§9（兼容边界 1–6）
- 证据：`evidence/p2/ACCEPTANCE.md`、`evidence/p0-p1/ACCEPTANCE.md`

## Compatibility Boundary

1. **判分权威**：`attempt.server_is_correct` 是唯一权威值；`client_is_correct`
   仅用于即时反馈与分歧告警，**统计一律基于前者**（基线 §5.2(8)）。
   二者不一致必须留痕，**不得静默覆盖**。
2. **判分路径不得出现浮点**（基线 §5.2(6)）：一律 `big.Int` 交叉相乘。
3. **清洗表只有一个 owner（服务端）**（基线 §5.2(7)）：客户端只应用数据，
   不得引入 NFKC 类归一化库。
4. **题面与答案以快照落库**（规格 §6.1(3)）：作者事后改规则不得改动历史错题。
5. **级联删除不变量**（规格 §6.1(2)）：删项目时该项目的 `practice_round`/`attempt`/
   `subscription` 随外键消亡；应用层**不写补偿删除逻辑**（P0 已实测并设 falsifier）。
6. 作者契约不得扩张：仍是 `function generate(cfg)` 返回 `{q, a}`（D1/D11）。

## TDD Route

```text
TDD Route:
- Mode: off
- Decision: skipped
- Strict authority: not applicable
- Test posture: post-change regression
- Reason: 项目配置 tdd_mode = "off"，用户未要求严格 TDD
- Verification: go test ./... -count=1
```

> 本计划为**判分语义**（跨端契约，P3.5 要据此生成语料）与**数据不变量**编写回归测试，
> 属 proportional regression。

## Verification（全局命令）

```powershell
cd D:\Desktop\Cala\backend
go build ./... ; go vet ./... ; CGO_ENABLED=0 go test ./... -count=1 -timeout 300s
go test ./internal/scoring/ -count=1 -v
```

---

## Scope Check

**Aegis Visibility**：本阶段落地的是**判分语义**（跨端契约，一旦错误会静默污染历史统计）
与**跨用户数据不变量**（删项目会删除他人数据）。因此即使 TDD 路由为 `skipped`，
也把「无浮点比较」「分歧必须留痕」「快照不受规则变更影响」写成回归测试，
并把判分语义集中在一个无依赖的包里，使 P3.5 能由它生成语料、由 Dart 逐例对齐。

```text
Requirement Ready Check:
- Requirement source refs: 用户功能 2/5/8/9；规格 §5.5、§6、§7
- Goals and scope refs: 规格 §16 P3 行
- User / scenario refs: 作者建项目并练习；订阅者被动承受作者的规则与配置
- Requirement item refs: 功能 2（出题）、功能 9（即时反馈依赖的判定语义）
- Acceptance / verification criteria refs: A11（畸形答案拒绝保存）、A14（分歧告警）
- Open blocker questions: 无（文本答案与容差两处设计缺陷已在本轮修正）
- Decision: ready
```

```text
Change Necessity:
- User-visible need: 没有项目与轮次端点，App 无法练习
- No-change / non-code option: 不存在——当前无任何项目/轮次实现
- Why code change is necessary: 无既有实现可复用
- Minimum change boundary: 新增 internal/scoring 包、store/service/api 的项目与轮次部分、
  一个迁移文件 0002；不改动既有包的行为
- Decision: code-change
```

```text
Existence Check:
- Proposed new surface: ①internal/scoring 包 ②容差列（迁移 0002）
  ③分歧计数器（进程内指标）
- Existing owner / reuse candidate: ①无——判分语义目前不存在；
  ②project 表已存在，但缺字段；③无
- Why existing surface is insufficient:
  ① 判分若散落在 service 或 api 中，P3.5 就无法由一个稳定入口生成语料，
     跨端一致性将失去可验证的锚点；
  ② 容差无处存放（见规格 §6 的 Design Defect 记录）；
  ③ 分歧必须可观测（D16），否则语料未覆盖的边界会静默污染统计
- Creation proof:
  ① 规格 §5.5.2 要求答案信封分类，§5.5.3 要求无浮点比较——两者都需要一个 owner；
  ② D3 明确承诺项目级可选容差；
  ③ D16/验收标准 A14 明确要求分歧可观测
- Entropy / retirement impact: 无旧路径退役。**明确拒绝**：不引入 ORM、
  不引入第三方有理数库、不做规则热加载
- Decision: add-with-proof
```

```text
Architecture Integrity Lens:
- Invariant: `server_is_correct` 是唯一权威；判分路径无浮点；清洗表单一下发
- Canonical owner / contract: `internal/scoring` 是判分语义唯一 owner，
  且不依赖其他包（保证可被 P3.5 独立复用）
- Responsibility overlap: 客户端实现由 P3.5 提供，但它必须由 scoring 生成的语料钉死，
  不是第二个 owner
- Higher-level simplification: 用「信封在出题时定型」代替「判分时反复解释原文」——
  判分只做算术，不做字符串猜测
- Retirement / falsifier: 若有人把 `cfg_json` 里的字段当容差读取，
  会出现两个来源 → 由 schema 的独立列 + `TestToleranceComesFromColumn` 防住
- Verdict: 无阻塞问题
```

```text
Plan Pressure Test:
- Owner / contract / retirement: 主要承载点已识别（scoring、容差列、分歧留痕）
- Verification scope: A11/A14 各有专项测试；并有「快照不受规则变更影响」测试
- Task executability: 每步含完整代码与确切命令
- Pressure result: proceed
```

```text
Plan-Time Complexity Check:
- Target files: internal/scoring/*（新包 4 文件）、store/project.go、
  service/project.go、service/round.go、api/project.go、api/round.go
- Owner fit: 各文件单一职责
- Add-in-place risk: 若把出题与交卷都塞进 service/project.go 会混杂两类职责
- Better file boundary: project 与 round 分文件；scoring 独立成包
- Recommendation: add owner file
```

---

## Task P3.1 —— 判分核心 `internal/scoring`

**Files**
- Create: `backend/internal/scoring/envelope.go`
- Create: `backend/internal/scoring/rational.go`
- Create: `backend/internal/scoring/cleanup.go`
- Create: `backend/internal/scoring/compare.go`
- Create: `backend/internal/scoring/scoring_test.go`

**Why**：这是 §5.5 的落点。判分语义必须有唯一 owner 且无外部依赖，
否则 P3.5 无法由一个稳定入口生成跨端语料。

**Change Necessity**：`code-change`；最小边界 = 新增一个自包含的包。

**Impact / Compatibility**：承载基线 §5.2(6)(7) 与 §9(5)。

**Steps**

1. 写 `backend/internal/scoring/envelope.go`：

```go
// Package scoring 是判分语义的唯一 owner。
//
// 本包**不依赖本仓库其他包**，也不依赖任何第三方库（只用标准库 math/big）。
// 这样 P3.5 的语料生成器可以独立复用本包，而客户端实现必须向本包对齐。
//
// 本包不出现任何浮点运算：有理数一律用 big.Int 交叉相乘比较（规格 §5.5.3）。
package scoring

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 信封种类。
const (
	KindRational = "rational"
	KindText     = "text"
)

// Envelope 是作者答案的规范化形式，随 a_snapshot 一并落库，
// 成为该次答题的判分依据。
type Envelope struct {
	Kind string `json:"kind"`
	// Num/Den 仅在 Kind == KindRational 时有效，且 Den > 0。
	Num string `json:"num,omitempty"`
	Den string `json:"den,omitempty"`
	// Value 仅在 Kind == KindText 时有效，为规范化后的文本。
	Value string `json:"value,omitempty"`
}

// numericAlphabet 是「作者想写数值」的字符集合（ASCII）。
// 仅由这些字符组成却无法解析的答案，会被判为笔误（见 §5.5.2 第三条）。
const numericAlphabet = "0123456789+-./"

// Classify 把作者返回的答案原文分类为规范信封。
//
// 规则（规格 §5.5.2）：
//  1. 清洗后能解析为有理数且分母非零 -> rational
//  2. 含数值字母表以外字符的非空文本 -> text（允许中文）
//  3. 仅由数值字母表组成但无法解析，或分母为零 -> 报错（这是笔误，不是文本答案）
//  4. 含控制字符或超长 -> 报错
func Classify(raw string) (Envelope, error) {
	if strings.TrimSpace(raw) == "" {
		return Envelope{}, fmt.Errorf("答案不能为空")
	}
	if n := len([]rune(raw)); n > MaxAnswerLen {
		return Envelope{}, fmt.Errorf("答案过长（%d 个字符，上限 %d）", n, MaxAnswerLen)
	}
	if i := indexControl(raw); i >= 0 {
		return Envelope{}, fmt.Errorf("答案含有控制字符（位置 %d）；请勿粘贴不可见字符", i)
	}

	cleaned := Clean(raw)

	// 先试数值
	if isNumericCandidate(cleaned) {
		r, err := ParseRational(cleaned)
		if err == nil {
			return Envelope{Kind: KindRational, Num: r.Num.String(), Den: r.Den.String()}, nil
		}
		// 看起来是数值却解析不了：明确报错，不静默降级为文本。
		// 静默降级会让用户在答题时永远答不对，且原因极难排查。
		return Envelope{}, fmt.Errorf(
			"答案 %q 看起来是数值但无法解析（%v）；若要作为文本答案，请包含数值以外的字符",
			raw, err)
	}

	// 文本答案：规范化后存储
	v := NormalizeText(cleaned)
	if v == "" {
		return Envelope{}, fmt.Errorf("答案不能为空")
	}
	return Envelope{Kind: KindText, Value: v}, nil
}

// isNumericCandidate 判断清洗后的字符串是否「只由数值字母表组成」。
// 空串不算数值候选（否则空答案会走到解析分支报「无法解析」而不是「不能为空」）。
func isNumericCandidate(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r == ' ' {
			continue
		}
		if !strings.ContainsRune(numericAlphabet, r) {
			return false
		}
	}
	return true
}

func indexControl(s string) int {
	for i, r := range s {
		if r < 0x20 || r == 0x7F {
			return i
		}
	}
	return -1
}

// Marshal 序列化为落库用的 JSON。
func (e Envelope) Marshal() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("序列化答案信封失败: %w", err)
	}
	return string(b), nil
}

// UnmarshalEnvelope 从落库的 JSON 还原信封。
func UnmarshalEnvelope(s string) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal([]byte(s), &e); err != nil {
		return Envelope{}, fmt.Errorf("解析答案信封失败: %w", err)
	}
	switch e.Kind {
	case KindRational, KindText:
		return e, nil
	default:
		return Envelope{}, fmt.Errorf("未知的答案信封种类 %q", e.Kind)
	}
}
```

2. 写 `backend/internal/scoring/cleanup.go`：

```go
package scoring

import (
	"sort"
	"strings"
)

// CleanupVersion 标识清洗表的内容版本。
//
// 清洗表作为**数据**随 /rounds/start 下发（规格 §5.5.4），客户端只负责应用它。
// 版本号让服务端日后调整表格时，能在分歧告警里分辨「是客户端用了旧表」还是
// 「两边算法真的不一致」。
const CleanupVersion = 1

// CleanupTable 返回字符映射表：键为单个字符，值为替换文本（空串表示删除）。
//
// 该表是**唯一 owner 在服务端**的具体体现：Go 与 Dart 都不写清洗逻辑，
// 而是共同应用这张表。表里只做「字符替换」，不含任何依赖 Unicode 数据库的运算。
func CleanupTable() map[string]string {
	return map[string]string{
		// 全角数字 ０-９ -> 0-9
		"０": "0", "１": "1", "２": "2", "３": "3", "４": "4",
		"５": "5", "６": "6", "７": "7", "８": "8", "９": "9",
		// 全角运算符
		"．": ".", "。": ".", "／": "/", "＋": "+", "－": "-", "＝": "",
		// 各类 Unicode 减号 -> ASCII '-'
		"−": "-", // U+2212 MINUS SIGN
		"‐": "-", // U+2010 HYPHEN
		"‑": "-", // U+2011 NON-BREAKING HYPHEN
		"‒": "-", // U+2012 FIGURE DASH
		"–": "-", // U+2013 EN DASH
		"—": "-", // U+2014 EM DASH
		// 千分位分隔符：删除（"1,234" -> "1234"）
		",": "", "，": "", "、": "",
		// 空白类 -> ASCII 空格（含全角空格与不换行空格）
		"\u3000": " ", "\u00A0": " ", "\u2007": " ", "\u202F": " ",
		"\t": " ", "\r": " ", "\n": " ",
	}
}

// CleanupKeys 返回排序后的键，便于客户端展示与测试断言稳定性。
func CleanupKeys() []string {
	keys := make([]string, 0, len(CleanupTable()))
	for k := range CleanupTable() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Clean 应用清洗表并去除首尾空白。
//
// 注意：这里**只做表驱动的字符替换**，随后仅做 ASCII 空白处理。
// 刻意不调用任何 Unicode 归一化（NFKC/NFKD/NFC/NFD）——
// Go 的 x/text 与 Dart 生态对应库的语义未必逐字一致，
// 那是本项目最容易忽略的跨端漂移源（规格 §5.5.4）。
func Clean(s string) string {
	table := CleanupTable()
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if repl, ok := table[string(r)]; ok {
			b.WriteString(repl)
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// NormalizeText 规范化文本答案（规格 §5.5.2）：
//  1. 去除首尾空白
//  2. 内部连续空白折叠为单个 ASCII 空格
//  3. ASCII 大写 -> 小写（仅 ASCII）
//  4. 不做任何 Unicode 形式变换
func NormalizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		// 只把 ASCII 空白视作空白：不触碰全角空格等，
		// 因为清洗表已把它们转成 ASCII 空格。
		if r == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
			b.WriteRune(' ')
			continue
		}
		prevSpace = false
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
```

3. 写 `backend/internal/scoring/rational.go`：

```go
package scoring

import (
	"fmt"
	"math/big"
	"strings"
)

// Rational 是一个精确有理数。Den 恒为正。
type Rational struct {
	Num *big.Int
	Den *big.Int
}

// MaxAnswerLen 与 rules.MaxAnswerLen 保持一致（本包不依赖 rules，故独立声明）。
const MaxAnswerLen = 500

// ParseRational 严格解析一个有理数字面量。
//
// 文法（Γ 极小，便于客户端逐例镜像）：
//
//	number := sign? ( digits | digits '.' digits | '.' digits | digits '/' digits )
//	sign   := '+' | '-'
//
// 分母必须非零。整数部分允许前导零。不接受科学计数法、不接受千分位
// （千分位已在 Clean 阶段删除）。
func ParseRational(s string) (Rational, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return Rational{}, fmt.Errorf("空字符串")
	}

	neg := false
	switch t[0] {
	case '+':
		t = t[1:]
	case '-':
		neg = true
		t = t[1:]
	}
	if t == "" {
		return Rational{}, fmt.Errorf("缺少数字")
	}

	// 整数
	if isAllDigits(t) {
		return newRational(digitsToBig(t), big.NewInt(1), neg)
	}

	// 分数 a/b
	if i := strings.IndexByte(t, '/'); i >= 0 {
		numPart, denPart := t[:i], t[i+1:]
		if !isAllDigits(numPart) || !isAllDigits(denPart) {
			return Rational{}, fmt.Errorf("分数格式应为 整数/整数")
		}
		den := digitsToBig(denPart)
		if den.Sign() == 0 {
			return Rational{}, fmt.Errorf("分母不能为零")
		}
		return newRational(digitsToBig(numPart), den, neg)
	}

	// 小数
	if i := strings.IndexByte(t, '.'); i >= 0 {
		intPart, fracPart := t[:i], t[i+1:]
		if fracPart == "" {
			return Rational{}, fmt.Errorf("小数点后缺少数字")
		}
		if !isAllDigits(fracPart) {
			return Rational{}, fmt.Errorf("小数点后应为数字")
		}
		if intPart != "" && !isAllDigits(intPart) {
			return Rational{}, fmt.Errorf("小数点前应为数字")
		}
		// 0.25 -> 25/100
		digits := intPart + fracPart
		den := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(fracPart))), nil)
		return newRational(digitsToBig(digits), den, neg)
	}

	return Rational{}, fmt.Errorf("既不是整数、小数，也不是分数")
}

func newRational(num, den *big.Int, neg bool) (Rational, error) {
	if den.Sign() == 0 {
		return Rational{}, fmt.Errorf("分母不能为零")
	}
	if neg {
		num = new(big.Int).Neg(num)
	}
	return Rational{Num: num, Den: den}, nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// digitsToBig 解析十进制数字串。前导零由 big.Int 自行处理。
func digitsToBig(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		// isAllDigits 已保证只含数字，理论上不可达
		return big.NewInt(0)
	}
	return n
}

// Cmp 比较两个有理数，返回 -1/0/1。
// 交叉相乘，全程整数（规格 §5.5.3）。
func (r Rational) Cmp(o Rational) int {
	l := new(big.Int).Mul(r.Num, o.Den)
	rr := new(big.Int).Mul(o.Num, r.Den)
	return l.Cmp(rr)
}

// String 返回可读形式，仅用于日志与错误信息（不参与判分）。
func (r Rational) String() string {
	if r.Den.Cmp(big.NewInt(1)) == 0 {
		return r.Num.String()
	}
	return r.Num.String() + "/" + r.Den.String()
}
```

4. 写 `backend/internal/scoring/compare.go`：

```go
package scoring

import (
	"fmt"
	"math/big"
	"strings"
)

// Result 是一次判分的结果。
type Result struct {
	// Correct 是判定结论。
	Correct bool
	// Reason 说明判定依据，仅用于调试与分歧排查，不参与存储。
	Reason string
}

// Tolerance 是项目级可选容差（规格 §5.5.3），表示为整数比 tNum/tDen。
// 零值表示精确比较。
type Tolerance struct {
	Num *big.Int
	Den *big.Int
}

// Exact 返回精确比较（不容差）。
func Exact() Tolerance { return Tolerance{} }

// NewTolerance 构造容差，den 必须为正。
func NewTolerance(num, den int64) (Tolerance, error) {
	if den <= 0 {
		return Tolerance{}, fmt.Errorf("容差分母必须为正，得到 %d", den)
	}
	if num < 0 {
		return Tolerance{}, fmt.Errorf("容差不能为负，得到 %d", num)
	}
	return Tolerance{Num: big.NewInt(num), Den: big.NewInt(den)}, nil
}

// IsZero 表示精确比较。
func (t Tolerance) IsZero() bool { return t.Num == nil || t.Num.Sign() == 0 }

// Compare 判定用户作答是否正确。
//
// 判定规则（规格 §5.5.3）：
//   - 文本答案：规范化后精确相等
//   - 有理数答案：|n1·d2 − n2·d1| · t_d  ≤  t_n · d1 · d2
//     其中作答 = n1/d1，正确答案 = n2/d2，容差 = t_n/t_d
//
// 全程 big.Int，**不出现任何浮点**。作答无法解析为数值时判为错误
// （未作答或乱输不应因容差而意外算对）。
func Compare(userInput string, env Envelope, tol Tolerance) Result {
	cleaned := Clean(userInput)
	if strings.TrimSpace(cleaned) == "" {
		return Result{Correct: false, Reason: "未作答"}
	}

	switch env.Kind {
	case KindText:
		got := NormalizeText(cleaned)
		if got == env.Value {
			return Result{Correct: true, Reason: "文本相等"}
		}
		return Result{Correct: false, Reason: fmt.Sprintf("文本不等: 期望 %q 得到 %q", env.Value, got)}

	case KindRational:
		got, err := ParseRational(cleaned)
		if err != nil {
			return Result{Correct: false, Reason: "作答无法解析为数值: " + err.Error()}
		}
		want, err := envelopeRational(env)
		if err != nil {
			// 落库的信封损坏属于服务端数据问题，判为错并保留原因
			return Result{Correct: false, Reason: "正确答案信封损坏: " + err.Error()}
		}

		if tol.IsZero() {
			if got.Cmp(want) == 0 {
				return Result{Correct: true, Reason: "数值精确相等"}
			}
			return Result{Correct: false, Reason: fmt.Sprintf("数值不等: 期望 %s 得到 %s", want, got)}
		}

		// |n1*d2 - n2*d1| * t_d <= t_n * d1 * d2
		lhs := new(big.Int).Mul(got.Num, want.Den)
		lhs.Sub(lhs, new(big.Int).Mul(want.Num, got.Den))
		lhs.Abs(lhs)
		lhs.Mul(lhs, tol.Den)

		rhs := new(big.Int).Mul(tol.Num, got.Den)
		rhs.Mul(rhs, want.Den)

		if lhs.Cmp(rhs) <= 0 {
			return Result{Correct: true, Reason: fmt.Sprintf("在容差内: 期望 %s 得到 %s", want, got)}
		}
		return Result{Correct: false, Reason: fmt.Sprintf("超出容差: 期望 %s 得到 %s", want, got)}

	default:
		return Result{Correct: false, Reason: "未知的信封种类 " + env.Kind}
	}
}

func envelopeRational(e Envelope) (Rational, error) {
	num, ok := new(big.Int).SetString(e.Num, 10)
	if !ok {
		return Rational{}, fmt.Errorf("num 不是整数: %q", e.Num)
	}
	den, ok := new(big.Int).SetString(e.Den, 10)
	if !ok {
		return Rational{}, fmt.Errorf("den 不是整数: %q", e.Den)
	}
	if den.Sign() == 0 {
		return Rational{}, fmt.Errorf("den 为零")
	}
	return Rational{Num: num, Den: den}, nil
}
```

5. 写测试 `scoring_test.go`（**必须覆盖**下列断言，实现时逐条写为子测试）：

- **分类**：`"42"`→rational 42/1；`"3.14"`→314/100；`"1/2"`→1/2；`"-5"`→-5/1；
  `".5"`→5/10；`"007"`→7/1；`"质数"`→text；`"  Hello World  "`→text `"hello world"`
- **笔误必须报错**（§5.5.2 第三条）：`"1/0"`、`"1.2.3"`、`"--5"`、`"1/"`、`"-"`、`"+"`、`"1..2"`
- **控制字符与超长**：`"a\x00b"` 报错；超 500 字符报错；空串与纯空白报错
- **清洗表**：`"１２３"`→`"123"`；`"1,234"`→`"1234"`；`"−5"`(U+2212)→`"-5"`；
  `"1.5"` 用全角句点；全角空格；`"\u00A0"`
- **比较（精确）**：`"0.5"` 对 `1/2` → 正确；`"1/2"` 对 `0.5` → 正确；
  `"2"` 对 `2/1` → 正确；`"3"` 对 `1/2` → 错误；空输入 → 错误
- **浮点陷阱必须正确**：`"0.3"` 对答案 `0.1+0.2`（作者写 `"0.3"`）→ 正确；
  且断言判分路径不含 float（用 `0.1+0.2` 的十进制字面量 `"0.30000000000000004"` 作反例 →
  与 `0.3` 不等，证明比较是精确的而非先转 float）
- **比较（容差）**：容差 `1/100`：`"3.14"` 对 `3.14159` → 正确；`"3.15"` 对 `3.14159` → 正确；
  `"3.2"` 对 `3.14159` → 错误；边界相等（恰好等于容差）→ 正确
- **文本比较**：`"质数"` 对 `"质数"` → 正确；`"质数 "` → 正确（trim）；
  `"质  数"` 对 `"质 数"` → 正确（空白折叠）；`"PRIME"` 对 `"prime"` → 正确（ASCII 折叠）；
  `"素数"` 对 `"质数"` → 错误
- **信封往返**：Marshal/Unmarshal 保真；未知 kind 报错
- **容差构造**：`NewTolerance(1,0)` 与负数报错

6. 验证：

```powershell
cd D:\Desktop\Cala\backend
go build ./... ; go vet ./... ; go test ./internal/scoring/ -count=1 -v
```

7. **Commit**：`feat(scoring): 判分核心（答案信封、清洗表、无浮点比较）`

---

## Task P3.2 —— 容差列迁移与项目数据访问

**Files**
- Create: `backend/internal/store/migrations/0002_add_tolerance.sql`
- Create: `backend/internal/store/project.go`
- Modify: `backend/internal/store/store_test.go`（追加）

**Why**：D3 承诺的项目级容差在原 schema 中缺失（规格 §6 的 Design Defect）。
本任务补齐字段并提供项目数据访问。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：迁移必须对既有库幂等可用（P0 的迁移机制按版本号跳过）。

**Steps**

1. 写 `0002_add_tolerance.sql`：

```sql
-- 规格 §6 的 Design Defect 修正：D3 承诺「项目级可选容差」，但 0001 未建字段。
-- 两列同时为空 = 精确比较；同时非空且 den > 0 = 容差 num/den。
-- 用两个整数而非浮点，使容差本身也留在无浮点判分路径上（§5.5.3）。
ALTER TABLE project ADD COLUMN tolerance_num INTEGER;
ALTER TABLE project ADD COLUMN tolerance_den INTEGER;
```

2. 写 `backend/internal/store/project.go`：项目 CRUD、访问权判定、容差读写。

关键点（其余按既有 `user.go` 的风格实现）：

```go
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
	ToleranceNum  *int64 `json:"toleranceNum"`
	ToleranceDen  *int64 `json:"toleranceDen"`
	ShareToken    *string `json:"shareToken"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
	// Access 说明调用者与该项目的关��，由查询填充。
	Access string `json:"access"` // "owner" | "subscriber"
}

// CanAccess 判定用户是否有权访问该项目：拥有 或 已订阅（规格 §6.1(1)）。
// 这是访问权的唯一判定入口，API 与 service 都必须经它，不得各自实现。
```

必须实现的函数：`CreateProject`、`GetProject`（含访问权判定）、
`ListProjectsForUser`（拥有 + 已订阅，分组返回）、`UpdateProject`（仅作者）、
`DeleteProject`（仅作者，级联由外键完成）、`ProjectAccess(userID, projectID) (string, error)`。

3. 追加迁移测试到 `store_test.go`：

```go
func TestMigration0002AddsToleranceColumns(t *testing.T)
func TestToleranceColumnsNullable(t *testing.T)   // 两列可同时为空
```

4. 验证：`go test ./internal/store/ -count=1 -v`

5. **Commit**：`feat(store): 容差列迁移与项目数据访问`

---

## Task P3.3 —— 项目业务与 API

**Files**
- Create: `backend/internal/service/project.go`
- Create: `backend/internal/api/project.go`
- Create: `backend/internal/service/project_test.go`
- Create: `backend/internal/api/project_test.go`
- Modify: `backend/internal/api/router.go`（接线）

**Why**：把项目能力暴露为规格 §7 的 HTTP 契约，并在创建/修改时强制规则校验。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：创建与修改都必须经 `rules.Validate`（基线 §5.2(2)），
失败返回 `400 rule_invalid`（错误码 P0.4 已定义）。

**Steps**

端点（规格 §7）：

```
GET    /api/projects               # 拥有 + 已订阅
POST   /api/projects               # 创建（含规则校验）
GET    /api/projects/:id
PUT    /api/projects/:id           # 仅作者；改后订阅者即时可见
DELETE /api/projects/:id           # 仅作者；级联删除全员记录（功能8）
```

要点：

- `POST`/`PUT` 的处理器必须把 `rules.ValidationError` 映射为
  `400 rule_invalid` 并**带上原始 Detail**，使作者知道错在哪一行。
- `PUT` 只有作者可调用；订阅者调用返回 `403 forbidden`（D4 纯只读跟随）。
- `DELETE` 返回 `204`；级联由外键完成，应用层不写补偿删除。
- 容差：请求可带 `toleranceNum`/`toleranceDen`；两者必须同时给或同时不给，
  且 `den > 0`；否则 `400 bad_request`。

测试必须覆盖：

- 创建时坏规则被拒（用 P2 的 7 类错误源码中的若干）→ 400 且 `code == rule_invalid`
- 创建成功后能在列表与详情中看到
- 非作者改项目 → 403；作者改 → 200 且 `updatedAt` 变化
- 删除后：项目消失、`practice_round`/`attempt` 随级联消失（功能8）
- 容差：只给一个 → 400；`den=0` → 400；合法 → 200
- 未登录访问任一端点 → 401

**Commit**：`feat(api): 项目 CRUD 与规则校验`

---

## Task P3.4 —— 轮次：出题与交卷

**Files**
- Create: `backend/internal/service/round.go`
- Create: `backend/internal/api/round.go`
- Create: `backend/internal/service/round_test.go`
- Create: `backend/internal/api/round_test.go`

**Why**：功能9 的即时反馈依赖客户端本地判分，其正确性最终由服务端重算保证（D13）。

**Change Necessity**：`code-change`。

**Impact / Compatibility**：承载基线 §5.2(8)（判分权威与分歧留痕）。

**Steps**

1. `POST /api/rounds/start` 返回：

```json
{
  "seed": 123456789,
  "scoring": { "version": 1, "cleanupTable": { "０": "0", "...": "..." } },
  "tolerance": { "num": 1, "den": 100 },
  "questions": [ { "idx": 0, "q": "3 + 4 = ?", "a": "7", "envelope": {"kind":"rational","num":"7","den":"1"} } ]
}
```

- `seed` 由服务端用 `crypto/rand` 生成并返回；客户端在交卷时回传。
- `questions` 的 `a` 是原文（展示用），`envelope` 供客户端判分。
- 出题数取 `project.question_count`（D12）。
- 访问权必须校验：非拥有非订阅 → 403。

2. `POST /api/rounds/complete` 请求：

```json
{ "projectId": 1, "seed": 123, "startedAt": "...", "finishedAt": "...",
  "attempts": [ { "idx": 0, "input": "7", "clientIsCorrect": true, "elapsedMs": 1200 } ] }
```

服务端行为（规格 §7 五条，逐条实现）：

1. 用**服务端判分实现**从落库信封与 `input` 重算 `server_is_correct`；
2. 原样保存客户端上报的 `clientIsCorrect`，不覆盖、不采信；
3. 二者不一致时递增分歧计数器并记录结构化日志；
4. `total_ms` 与 `correct_count` 由服务端从 attempts 派生，不采信客户端汇总；
5. 统计一律基于 `server_is_correct`。

**题面来源**：服务端不信任客户端上报的 `q`/`a`。但 `/rounds/complete` 时服务端
并未保存出题结果（D6：不持久化未完成练习）。因此**服务端在交卷时用同一 seed
重新出题**，得到题面与答案快照落库。这与 A3（同种子可复现）直接相关——
若可复现性失效，交卷会失败，这是可接受的失败方式（显式报错而非静默写错数据）。

注意：作者在「出题后、交卷前」修改了规则，会导致重放出的题目与客户端看到的不同。
处置：重放后用 `len(questions) == len(attempts)` 与 `idx` 连续性校验；
若项目 `updatedAt` 晚于该轮 `startedAt`，在响应中返回 `staleProject: true` 提示客户端。
**不静默写入可能不匹配的快照**。

3. 分歧计数器：在 `service` 包内以 `atomic.Int64` 实现，并暴露
`DiscrepancyCount() int64` 供测试与 P7 的健康端点读取。
不引入第三方 metrics 库（YAGNI）。

测试必须覆盖：

- 交卷落库的 `server_is_correct` 与 `client_is_correct` **各自独立**填写
- 人为构造不一致（客户端说 true、实际 false）→ 落库两值不同，
  且 `DiscrepancyCount()` 增加（A14）
- `correct_count` 由服务端派生：客户端上报任何汇总值都被忽略
- 题面/答案快照落库，且**作者事后改规则不影响已落库快照**（规格 §6.1(3)）
- 非授权用户 start → 403
- `seed` 缺失或 attempts 为空 → 400
- 死循环规则的项目 start → 400 `rule_invalid`（而非 500）

**Commit**：`feat(api): 轮次出题与交卷（服务端重算与分歧告警）`

---

## Task P3.5 —— P3 验收

**Files**：无新增（验证与证据）

1. 全量验证并逐条核对 A11 / A14，把实际输出写入
   `docs/aegis/work/2026-09-10-mental-math-app/evidence/p3/ACCEPTANCE.md`：

| 标准 | 证明它的确切命令 |
|---|---|
| A11 畸形答案被拒且报错可读 | `go test ./internal/scoring/ -run TestClassifyRejectsMalformed -v` 与 `-run TestProjectRejectsMalformedAnswer -v` |
| A14 分歧被告警且 `server_is_correct` 权威 | `go test ./internal/service/ -run TestDiscrepancyIsRecorded -v` |
| 快照不受规则变更影响 | `go test ./internal/service/ -run TestSnapshotImmutableAfterRuleChange -v` |
| 级联删除（功能8） | `go test ./internal/api/ -run TestDeleteProjectCascades -v` |
| 无浮点判分 | `go test ./internal/scoring/ -run TestNoFloatDrift -v` |

2. **手工端到端**：起服务 → 注册 → 建项目（含真实规则）→ start → complete →
   查库确认 `attempt` 行数、`server_is_correct`、快照内容。

3. 更新工作记录，提交。

**Commit**：`test(scoring): P3 验收记录`

---

## Risks

| # | 风险 | 处置 |
|---|---|---|
| R-P3-1 | 交卷时用 seed 重放出题，依赖 A3 可复现性 | A3 已由 P2 的变异测试取证；重放失败时显式报错，不静默写入不匹配的快照 |
| R-P3-2 | 作者在出题后、交卷前改规则 → 重放题目与客户端所见不符 | 用项目 `updatedAt` 与该轮 `startedAt` 比较，返回 `staleProject: true`，不静默写入 |
| R-P3-3 | 浮点意外混入判分 | 判分只用 `big.Int`；测试用 `0.1+0.2` 类字面量断言精确性，防止有人改回 float |
| R-P3-4 | 清洗表若被客户端复制实现则退化为两个 owner | 清洗表随 `/rounds/start` 下发且带 `version`；P3.5 以语料钉死客户端行为 |
| R-P3-5 | 容差放错位置（cfg_json）导致两个来源 | 独立列 + `TestToleranceComesFromColumn` |

## Retirement

- **Old owner / fallback**：无。全部新增；无兼容分支。
- **Deletion trigger**：无。
- 明确拒绝：ORM、第三方有理数库、第三方 metrics 库、规则热加载。

## ADR Signals

本计划为 **ADR-4「判分单一性策略」** 的落地。P3 完成后可回填：
答案信封两类、清洗表为数据且 owner 在服务端、无浮点交叉相乘、
容差为独立列、分歧留痕而非静默覆盖。

---

## Execution Readiness View

```text
Execution Readiness View:
- Intent Lock: 交付项目 CRUD 与轮次闭环；判分语义集中在 internal/scoring
- Scope Fence: 不含分享/订阅导入退订（P6）、统计端点（P5）、前端（P4）、
  语料生成器与 Dart 镜像（P3.5）
- Baseline Lock: 规格 §5.5（含本轮修正）、§6（含容差列修正）、§7；基线 §5.2、§9
- Approved Behavior: 创建/修改项目必须过规则校验；交卷由服务端重算
  server_is_correct 并保留 client_is_correct；快照不受规则变更影响
- Owner / Contract Constraints: scoring 不依赖本仓库其他包；
  访问权判定只有 ProjectAccess 一个入口
- Compatibility Boundary: 基线 §5.2(6)(7)(8)、§9(1)(5)(6)；作者契约不扩张
- Retirement Boundary: 无退役对象；拒绝 ORM/有理数库/metrics 库
- Task Batches: P3.1（scoring）→ P3.2（迁移+store）→ P3.3（项目 API）
  → P3.4（轮次）→ P3.5（验收）
- Test Obligations: 分类/笔误拒绝/清洗表/精确比较/浮点陷阱/容差边界/文本规范化/
  信封往返/迁移幂等/CRUD 权限/级联/交卷重算/分歧计数/快照不可变
- Review Gates: 每 Task 后 go build/vet/test；P3.5 核对 A11/A14 + 手工端到端
- Drift / Rewind Rules: 若需扩张作者契约或改动既有包行为，停止并回到规格
- Evidence Required Before Completion: 全量测试输出；A11/A14 专项通过；
  手工端到端 + 查库结果
- Advisory Boundary: method-pack execution guidance only; not GateDecision,
  PolicySnapshot, or completion authority
```

## Execution Route

```text
Execution Route:
- Decision: inline
- Evidence: P3.1→P3.4 严格顺序依赖（scoring → store → service → api），
  且共享同一组类型，并行会产生写冲突
- Fallback: 无
- User confirmation required: no
```

**REQUIRED SUB-SKILL**：aegis:executing-plans

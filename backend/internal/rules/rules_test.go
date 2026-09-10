package rules

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// timeAfter 是 time.After 的薄封装，让测试里的超时守卫读起来更直白。
func timeAfter(seconds int) <-chan time.Time {
	return time.After(time.Duration(seconds) * time.Second)
}

const goodRule = `
function generate(cfg) {
  const a = cfg.min + Math.floor(Math.random() * (cfg.max - cfg.min + 1));
  const b = cfg.min + Math.floor(Math.random() * (cfg.max - cfg.min + 1));
  return { q: a + " + " + b + " = ?", a: String(a + b) };
}`

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

// ---------------------------------------------------------------------------
// P2.2 编译
// ---------------------------------------------------------------------------

// TestCompileRejectsBadSources 覆盖验收标准 A2 的前半部分。
// 这些是实测中确实会被拒绝的写法，逐条固化防止回归。
func TestCompileRejectsBadSources(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		stage string
	}{
		{"空源码", "", StageCompile},
		{"缺少 generate", `function gen(cfg){ return {q:"1",a:"1"} }`, StageCompile},
		{"generate 是数字", `var generate = 42`, StageCompile},
		{"generate 是对象", `const generate = { call: 1 }`, StageCompile},
		{"语法错误", `function generate( { return }`, StageCompile},
		{"未终止注释", `/* oops`, StageCompile},
		{"顶层抛错", `throw new Error("boom")`, StageCompile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(tc.src)
			if err == nil {
				t.Fatal("应被拒绝但通过了")
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

// TestCompileAcceptsModernSyntax 记录沙箱支持的语法面（实测 30 项中 28 项通过）。
// 作者能用这些写法，故它们是契约的一部分，需要回归保护。
func TestCompileAcceptsModernSyntax(t *testing.T) {
	features := map[string]string{
		"模板字符串":             "const s = `x${1}`;",
		"箭头函数与默认参数":         "const f = (a=1) => a;",
		"解构与默认值":            "const {a,b=2} = {a:1};",
		"展开与剩余":             "const f=(...xs)=>[...xs,1].length;",
		"可选链":               "const o={a:{b:1}}; const v=o?.a?.b;",
		"空值合并":              "const z = null ?? 7;",
		"类与 getter":         "class A { get y() { return 1 } }",
		"for-of 与 Set":      "let n=0; for (const v of new Set([1,2])) n+=v;",
		"BigInt":            "const b = 1n;",
		"正则命名组":             `const m = /(?<n>\d+)/.exec("a12");`,
		"正则后行断言":            `const m = /(?<=\$)\d+/.exec("$42");`,
		"生成器":               "function* g(){ yield 1; }",
		"指数运算":              "const p = 2 ** 10;",
		"Array.flat/at":     "const v=[1,[2]].flat().at(-1);",
		"Object.entries":    "const e = Object.entries({a:1});",
		"Math.hypot":        "const h = Math.hypot(3,4);",
		"try/catch/finally": "let x=0; try{x=1}catch(e){}finally{x===1}",
		"Symbol.iterator":   "const s = typeof Symbol.iterator;",
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
		t.Errorf("firstLine 应截断到约 200 字符, 得到 %d", len([]rune(got)))
	}
}

// ---------------------------------------------------------------------------
// P2.3 保存期校验（A2 后半部分）
// ---------------------------------------------------------------------------

func TestValidateAcceptsGoodRule(t *testing.T) {
	if err := mustCompile(t, goodRule).Validate(cfgRange(1, 9)); err != nil {
		t.Fatalf("合法规则应通过校验: %v", err)
	}
}

// TestValidateRejectsBadBehaviour 覆盖 A2 后半部分：
// 语法正确但运行期有问题的规则也必须被挡在保存之前。
func TestValidateRejectsBadBehaviour(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		stage  string
		substr string
	}{
		{"死循环 while", `function generate(cfg){ while(true){} }`, StageTimeout, "未返回"},
		{"死循环 for", `function generate(cfg){ for(;;){} }`, StageTimeout, "未返回"},
		{"无限递归", `function generate(cfg){ return generate(cfg) }`, StageExecute, "递归过深"},
		{"调用期未定义引用", `function generate(cfg){ return notDefined.x }`, StageExecute, "抛出异常"},
		{"调用期主动抛错", `function generate(cfg){ throw new Error("boom") }`, StageExecute, "抛出异常"},
		{"返回数字", `function generate(cfg){ return 42 }`, StageShape, "必须返回对象"},
		{"返回字符串", `function generate(cfg){ return "x" }`, StageShape, "必须返回对象"},
		{"返回数组", `function generate(cfg){ return [1,2] }`, StageShape, "必须返回对象"},
		{"返回 null", `function generate(cfg){ return null }`, StageShape, "必须返回对象"},
		{"返回 undefined", `function generate(cfg){ }`, StageShape, "必须返回对象"},
		{"缺少 q", `function generate(cfg){ return {a:"1"} }`, StageShape, "q"},
		{"缺少 a", `function generate(cfg){ return {q:"1+1"} }`, StageShape, "a"},
		{"q 不是字符串", `function generate(cfg){ return {q:1, a:"1"} }`, StageShape, "q"},
		{"a 不是字符串", `function generate(cfg){ return {q:"1", a:2} }`, StageShape, "a"},
		{"q 为空串", `function generate(cfg){ return {q:"", a:"1"} }`, StageShape, "q 不能为空"},
		{"a 为空串", `function generate(cfg){ return {q:"1", a:""} }`, StageShape, "a 不能为空"},
		{"题面过长", `function generate(cfg){ return {q:"x".repeat(2000), a:"1"} }`, StageShape, "过长"},
		{"答案过长", `function generate(cfg){ return {q:"1", a:"x".repeat(2000)} }`, StageShape, "过长"},
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

// TestValidateReflectsPendingConfig 覆盖「校验必须反映即将保存的配置」。
// 若校验用了某个固定配置而不是待保存配置，这条规则会漏网，
// 而它上线后会在真实配置下炸掉所有订阅者。
func TestValidateReflectsPendingConfig(t *testing.T) {
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
		t.Fatal("含 explode 的配置应被拒绝——校验必须反映待保存配置")
	}
}

func TestValidateIsIdempotent(t *testing.T) {
	rule := mustCompile(t, goodRule)
	for i := 0; i < 3; i++ {
		if err := rule.Validate(cfgRange(1, 9)); err != nil {
			t.Fatalf("第 %d 次校验应通过: %v", i+1, err)
		}
	}
}

// TestValidateHandlesHostileRules 确认恶意规则不会把进程带走。
// 每条规则都在独立 goroutine 中执行，并有 30s 上限；
// 若沙箱失效（例如漏了递归上限），本测试会以超时失败而不是挂住整个测试套件。
func TestValidateHandlesHostileRules(t *testing.T) {
	hostile := []struct {
		name string
		src  string
	}{
		{"while 死循环", `function generate(cfg){ while(true){} }`},
		{"for 死循环", `function generate(cfg){ for(;;){} }`},
		{"递增到溢出", `function generate(cfg){ let i=0; while(i>=0){ i++ } }`},
		{"无限递归", `function generate(cfg){ return generate(cfg) }`},
		{"互相递归", `function a(){ return b() } function b(){ return a() }
			function generate(cfg){ return a() }`},
		{"巨大字符串", `function generate(cfg){ return {q:"x".repeat(5e7), a:"1"} }`},
	}
	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			rule, err := Compile(tc.src)
			if err != nil {
				// 顶层就拒绝也算通过（更早拦住）
				return
			}
			done := make(chan error, 1)
			go func() { done <- rule.Validate(cfgRange(1, 9)) }()

			select {
			case err := <-done:
				if err == nil {
					t.Errorf("恶意规则应被拒绝")
				}
			case <-timeAfter(30):
				t.Fatalf("恶意规则卡住超过 30s——沙箱失效")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// P2.4 整轮出题与可复现性（A3 / A4）
// ---------------------------------------------------------------------------

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
		// min=max=1 时每道题都必须是 "1 + 1 = ?"，答案为 2
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

// TestGenerateFrozenTime 验证冻结时间源：依赖 Date 的规则同种子可复现，
// 且同一轮内所有题看到同一个时刻。
func TestGenerateFrozenTime(t *testing.T) {
	rule := mustCompile(t, `
function generate(cfg) {
  const d = new Date();
  return { q: "y" + d.getUTCFullYear() + "-d" + d.getUTCDate(), a: String(d.getUTCHours()) };
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
	// 同轮内时间应冻结。
	// 注意只比较题面与答案：Index 按设计逐题递增，不参与「时间是否冻结」的判断。
	for i := 1; i < len(a); i++ {
		if a[i].Q != a[0].Q || a[i].A != a[0].A {
			t.Errorf("同轮内时间应冻结, 第 %d 题 (%q,%q) 与第 0 题 (%q,%q) 不同",
				i, a[i].Q, a[i].A, a[0].Q, a[0].A)
		}
	}
	// 同时确认 Index 确实逐题递增，避免上面的比较因 Index 相同而失去意义
	for i := range a {
		if a[i].Index != i {
			t.Errorf("第 %d 项 Index = %d, 期望 %d", i, a[i].Index, i)
		}
	}
}

// TestGenerateTimeVariesWithSeed 证明时间源随种子变化——
// 否则「冻结」会退化成「所有轮次看到同一时刻」，使依赖日期的规则失去意义。
func TestGenerateTimeVariesWithSeed(t *testing.T) {
	rule := mustCompile(t, `
function generate(cfg) {
  return { q: String(new Date().getTime()), a: "0" };
}`)

	seen := map[string]bool{}
	// 种子间隔取一个与 86400 互质的步长，遍历不同天偏移
	for seed := int64(0); seed < 200000; seed += 7919 {
		qs, err := rule.Generate(nil, seed, 1)
		if err != nil {
			t.Fatalf("Generate 失败: %v", err)
		}
		seen[qs[0].Q] = true
	}
	if len(seen) < 5 {
		t.Errorf("不同种子只看到 %d 个不同时刻，时间源疑似未随种子变化", len(seen))
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
		if strings.Contains(q.Q, "timeout") || q.A == "" {
			t.Errorf("题目内容异常: %+v", q)
		}
	}
}

// TestGenerateIsStatelessAcrossQuestions 把「每问新建 VM」这一设计选择
// 固化为**显式契约**：规则的模块级状态不会在题目之间保留。
//
// 这不是副作用而是有意为之（见 sandbox.go 的取舍说明）：
// 它让 generate 成为 cfg 与随机数的纯函数，从而同种子必然可复现。
// 若日后改为「每轮一个 VM」，本测试会失败并强制作者/维护者显式确认该变更。
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

// TestGenerateTimeoutRuleDoesNotBreakNextRound 确认坏规则的一轮失败
// 不会污染紧随其后的另一轮（用户的正常使用不应被牵连）。
func TestGenerateTimeoutRuleDoesNotBreakNextRound(t *testing.T) {
	bad := mustCompile(t, `function generate(cfg){
		if (cfg.boom) { while(true){} }
		return { q: "1", a: "1" };
	}`)

	if _, err := bad.Generate(map[string]any{"boom": true}, 1, 2); err == nil {
		t.Fatal("含死循环分支的配置应被拒绝")
	}
	// 同一规则在正常配置下应立即恢复可用
	qs, err := bad.Generate(map[string]any{"boom": false}, 1, 5)
	if err != nil {
		t.Fatalf("正常配置应可出题: %v", err)
	}
	if len(qs) != 5 {
		t.Errorf("题数 = %d, 期望 5", len(qs))
	}
}

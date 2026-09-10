package scoring

import (
	"strings"
	"testing"

	"github.com/shinyes/cala/backend/internal/rules"
)

// ---------------------------------------------------------------------------
// 分类：正规形态
// ---------------------------------------------------------------------------

func TestClassifyRationalForms(t *testing.T) {
	cases := []struct {
		in      string
		wantNum string
		wantDen string
	}{
		{"42", "42", "1"},
		{"0", "0", "1"},
		{"007", "7", "1"},
		{"-5", "-5", "1"},
		{"+5", "5", "1"},
		{"3.14", "314", "100"},
		{"0.5", "5", "10"},
		{".5", "5", "10"},
		{"-0.25", "-25", "100"},
		{"1/2", "1", "2"},
		{"-3/4", "-3", "4"},
		{"6/3", "6", "3"},
		{"1,234", "1234", "1"},
		{"１２３", "123", "1"},
		{"１．５", "15", "10"},
		{"  7  ", "7", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			env, err := Classify(tc.in)
			if err != nil {
				t.Fatalf("Classify(%q) 出错: %v", tc.in, err)
			}
			if env.Kind != KindRational {
				t.Fatalf("Kind = %q, 期望 %q", env.Kind, KindRational)
			}
			if env.Num != tc.wantNum || env.Den != tc.wantDen {
				t.Errorf("得到 %s/%s, 期望 %s/%s", env.Num, env.Den, tc.wantNum, tc.wantDen)
			}
		})
	}
}

func TestClassifyTextForms(t *testing.T) {
	cases := []struct{ in, want string }{
		{"质数", "质数"},
		{"  质数  ", "质数"},
		{"Hello World", "hello world"},
		{"hello   world", "hello world"},
		{"HÉLLO", "hÉllo"}, // 仅 ASCII 折叠：É 保持原样
		{"合数（非质数）", "合数（非质数）"},
		{"a1", "a1"}, // 含数值以外字符 -> 文本
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			env, err := Classify(tc.in)
			if err != nil {
				t.Fatalf("Classify(%q) 出错: %v", tc.in, err)
			}
			if env.Kind != KindText {
				t.Fatalf("Kind = %q, 期望 %q", env.Kind, KindText)
			}
			if env.Value != tc.want {
				t.Errorf("Value = %q, 期望 %q", env.Value, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 分类：笔误必须报错（规格 §5.5.2 第三条）
// ---------------------------------------------------------------------------

// TestClassifyRejectsMalformed 覆盖验收标准 A11。
// 「看起来是数值却写错了」必须报错，而不是静默降级成文本答案——
// 后者会让用户在答题时永远答不对，且原因极难排查。
func TestClassifyRejectsMalformed(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"分母为零", "1/0"},
		{"两个小数点", "1.2.3"},
		{"双负号", "--5"},
		{"分数缺分母", "1/"},
		{"分数缺分子", "/2"},
		{"只有负号", "-"},
		{"只有正号", "+"},
		{"只有小数点", "."},
		{"空分数", "/"},
		{"尾随小数点", "1."},
		{"多余斜杠", "1/2/3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Classify(tc.in)
			if err == nil {
				t.Fatalf("Classify(%q) 应报错（笔误），但被接受了", tc.in)
			}
			if !strings.Contains(err.Error(), "无法解析") && !strings.Contains(err.Error(), "分母不能为零") {
				t.Errorf("错误信息应说明原因，得到: %v", err)
			}
		})
	}
}

// TestCommaGroupingIsDeliberatelyLenient 记录一个**有意的宽松**：
// 清洗表无条件删除千分位逗号，因此 "1,2,3" 会被当作 123 接受。
//
// 不校验分组正确性的原因：那需要让清洗表理解「逗号出现在什么位置才合法」，
// 即把**数据**变成**逻辑**，从而破坏 §5.5.4「清洗表是数据、客户端只应用数据」
// 这一跨端一致性基础。键盘字母表本就不含逗号（功能6），
// 逗号仅用于处理粘贴/抓取来的文本，故宽松是可接受的取舍。
func TestCommaGroupingIsDeliberatelyLenient(t *testing.T) {
	env, err := Classify("1,2,3")
	if err != nil {
		t.Fatalf("逗号被删除后应能解析，得到错误: %v", err)
	}
	if env.Kind != KindRational || env.Num != "123" || env.Den != "1" {
		t.Errorf("得到 %+v，期望 123/1", env)
	}
	// 与之对照：真正无法解析的形态必须仍然报错
	if _, err := Classify("1.2.3"); err == nil {
		t.Error("1.2.3 必须报错")
	}
}

func TestClassifyRejectsEmptyControlAndOverlong(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"空串", ""},
		{"纯空白", "   "},
		{"纯全角等号", "＝"},
		{"含 NUL", "a\x00b"},
		{"含制表前的控制符", "\x01"},
		{"超长", strings.Repeat("x", MaxAnswerLen+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Classify(tc.in); err == nil {
				t.Errorf("Classify 应报错，但被接受了")
			}
		})
	}
}

// TestAnswerLenMatchesRules 防止两个包的长度上限漂移。
// 不变式：scoring 的上限必须 >= rules 的上限，否则作者能保存一个
// rules 放行、scoring 却拒绝的答案，在出题时才失败。
func TestAnswerLenMatchesRules(t *testing.T) {
	if MaxAnswerLen < rules.MaxAnswerLen {
		t.Errorf("scoring.MaxAnswerLen=%d 小于 rules.MaxAnswerLen=%d："+
			"作者可能保存成功却在出题时失败", MaxAnswerLen, rules.MaxAnswerLen)
	}
}

// ---------------------------------------------------------------------------
// 清洗表
// ---------------------------------------------------------------------------

func TestCleanAppliesTable(t *testing.T) {
	cases := []struct{ in, want string }{
		{"１２３", "123"},
		{"1,234", "1234"},
		{"1，234", "1234"},
		{"1、234", "1234"},
		{"−5", "-5"},           // U+2212
		{"–5", "-5"},           // U+2013 EN DASH
		{"—5", "-5"},           // U+2014 EM DASH
		{"‐5", "-5"},           // U+2010
		{"1．5", "1.5"},         // 全角句点
		{"1。5", "1.5"},         // 中文句号
		{"1／2", "1/2"},         // 全角斜杠
		{"\u30001\u3000", "1"}, // 全角空格
		{"\u00A01\u00A0", "1"}, // 不换行空格
		{" 1\t2 ", "1 2"},
		{"＝5", "5"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := Clean(tc.in); got != tc.want {
				t.Errorf("Clean(%q) = %q, 期望 %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCleanupTableIsDataNotLogic 确保清洗表是纯数据：
// 所有值为 ASCII，且不含任何需要 Unicode 运算的映射。
func TestCleanupTableIsDataNotLogic(t *testing.T) {
	for k, v := range CleanupTable() {
		if len([]rune(k)) != 1 {
			t.Errorf("键 %q 应为单个字符", k)
		}
		for _, r := range v {
			if r > 0x7F {
				t.Errorf("键 %q 的值 %q 含非 ASCII 字符；表应只映射到 ASCII", k, v)
			}
		}
	}
	if len(CleanupKeys()) != len(CleanupTable()) {
		t.Error("CleanupKeys 数量与表不一致")
	}
}

// ---------------------------------------------------------------------------
// 精确比较与浮点陷阱
// ---------------------------------------------------------------------------

func mustEnvelope(t *testing.T, raw string) Envelope {
	t.Helper()
	env, err := Classify(raw)
	if err != nil {
		t.Fatalf("Classify(%q) 出错: %v", raw, err)
	}
	return env
}

func TestCompareRationalExact(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		answer  string
		correct bool
	}{
		{"等价小数与分数", "0.5", "1/2", true},
		{"等价分数与小数", "1/2", "0.5", true},
		{"整数与分数", "2", "2/1", true},
		{"前导零", "07", "7", true},
		{"不同值", "3", "1/2", false},
		{"负数相等", "-5", "-5", true},
		{"负号与减号混用", "−5", "-5", true},
		{"全角输入", "１２３", "123", true},
		{"千分位输入", "1,000", "1000", true},
		{"未作答", "", "1", false},
		{"乱输", "abc", "1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.input, mustEnvelope(t, tc.answer), Exact())
			if got.Correct != tc.correct {
				t.Errorf("Compare(%q, %q) = %v (%s), 期望 %v",
					tc.input, tc.answer, got.Correct, got.Reason, tc.correct)
			}
		})
	}
}

// TestNoFloatDrift 是 §5.5.3 的核心断言：判分路径不含浮点。
//
// 两个用例都能区分「精确有理数比较」与「float64 比较」——
// 若有人把实现改回浮点，这两条会立即失败：
//
//	① 2^53+1 与 2^53 在 float64 下相等，精确比较下相差 1
//	② 1/3 与 0.3333333333333333 在 float64 下相等，精确比较下不等
func TestNoFloatDrift(t *testing.T) {
	// ① 超出 float64 精确整数范围
	big1 := "9007199254740993" // 2^53 + 1
	big2 := "9007199254740992" // 2^53
	if got := Compare(big1, mustEnvelope(t, big1), Exact()); !got.Correct {
		t.Errorf("同一大整数应判对: %s", got.Reason)
	}
	if got := Compare(big2, mustEnvelope(t, big1), Exact()); got.Correct {
		t.Errorf("2^53 与 2^53+1 在精确比较下必须不等，但判为正确。%s\n"+
			"这几乎可以确定是有人改用了 float64（两者在 float64 下相等）", got.Reason)
	}

	// ② 1/3 与它的十进制近似
	third := "1/3"
	approx := "0.3333333333333333"
	if got := Compare(approx, mustEnvelope(t, third), Exact()); got.Correct {
		t.Errorf("1/3 与 %s 精确比较必须不等，但判为正确。%s\n"+
			"（两者在 float64 下相等，说明判分用了浮点）", approx, got.Reason)
	}
	// 而同一对值在显式容差下应当算对——这正是容差存在的意义
	tol, err := NewTolerance(1, 10000000000000000)
	if err != nil {
		t.Fatalf("NewTolerance 出错: %v", err)
	}
	if got := Compare(approx, mustEnvelope(t, third), tol); !got.Correct {
		t.Errorf("在容差 1e-16 下应判对: %s", got.Reason)
	}
}

func TestCompareText(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		answer  string
		correct bool
	}{
		{"完全相同", "质数", "质数", true},
		{"首尾空白", "  质数 ", "质数", true},
		{"内部空白折叠", "质  数", "质 数", true},
		{"ASCII 大小写", "PRIME", "prime", true},
		{"不同词", "素数", "质数", false},
		{"空输入", "", "质数", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.input, mustEnvelope(t, tc.answer), Exact())
			if got.Correct != tc.correct {
				t.Errorf("Compare(%q, %q) = %v (%s), 期望 %v",
					tc.input, tc.answer, got.Correct, got.Reason, tc.correct)
			}
		})
	}
}

func TestCompareToleranceBoundaries(t *testing.T) {
	// 答案 3.14159，容差 1/100（即 0.01）
	answer := mustEnvelope(t, "3.14159")
	tol, err := NewTolerance(1, 100)
	if err != nil {
		t.Fatalf("NewTolerance 出错: %v", err)
	}

	cases := []struct {
		name    string
		input   string
		correct bool
	}{
		{"远小于容差", "3.14", true},
		{"远小于容差(另一侧)", "3.15", true},
		{"恰好等于下边界", "3.13159", true}, // 差值恰为 0.01
		{"恰好等于上边界", "3.15159", true}, // 差值恰为 0.01
		{"刚超出下边界", "3.131589", false},
		{"超出容差(上侧)", "3.2", false},
		{"精确命中", "3.14159", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.input, answer, tol)
			if got.Correct != tc.correct {
				t.Errorf("Compare(%q) = %v (%s), 期望 %v", tc.input, got.Correct, got.Reason, tc.correct)
			}
		})
	}
}

// TestToleranceZeroMeansExact 确认零容差退化为精确比较，而不是「一切皆对」。
func TestToleranceZeroMeansExact(t *testing.T) {
	tol, err := NewTolerance(0, 100)
	if err != nil {
		t.Fatalf("NewTolerance 出错: %v", err)
	}
	if !tol.IsZero() {
		t.Error("num=0 应被视为精确比较")
	}
	if got := Compare("2", mustEnvelope(t, "1"), tol); got.Correct {
		t.Error("零容差下 2 != 1，应判错")
	}
	if got := Compare("1", mustEnvelope(t, "1"), tol); !got.Correct {
		t.Error("零容差下 1 == 1，应判对")
	}
}

// TestCompareRejectsNonNumericInputUnderTolerance 保证容差不会让乱输意外算对。
func TestCompareRejectsNonNumericInputUnderTolerance(t *testing.T) {
	tol, err := NewTolerance(1, 10)
	if err != nil {
		t.Fatalf("NewTolerance 出错: %v", err)
	}
	answer := mustEnvelope(t, "5")
	for _, in := range []string{"", "   ", "abc", "不是数字", "5x"} {
		if got := Compare(in, answer, tol); got.Correct {
			t.Errorf("输入 %q 不应因容差而判对（%s）", in, got.Reason)
		}
	}
}

func TestNewToleranceValidation(t *testing.T) {
	if _, err := NewTolerance(1, 0); err == nil {
		t.Error("分母为 0 应报错")
	}
	if _, err := NewTolerance(1, -5); err == nil {
		t.Error("分母为负应报错")
	}
	if _, err := NewTolerance(-1, 5); err == nil {
		t.Error("负容差应报错")
	}
	if _, err := NewTolerance(1, 5); err != nil {
		t.Errorf("合法容差不应报错: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 信封往返
// ---------------------------------------------------------------------------

func TestEnvelopeRoundTrip(t *testing.T) {
	for _, raw := range []string{"42", "-1/3", "3.14", "质数", "hello world"} {
		t.Run(raw, func(t *testing.T) {
			env := mustEnvelope(t, raw)
			s, err := env.Marshal()
			if err != nil {
				t.Fatalf("Marshal 出错: %v", err)
			}
			back, err := UnmarshalEnvelope(s)
			if err != nil {
				t.Fatalf("UnmarshalEnvelope 出错: %v", err)
			}
			if back != env {
				t.Errorf("往返后 %+v != %+v", back, env)
			}
		})
	}
}

func TestUnmarshalEnvelopeRejectsBad(t *testing.T) {
	cases := []string{
		`{"kind":"rational"}`,           // 缺 num/den
		`{"kind":"rational","num":"1"}`, // 缺 den
		`{"kind":"text"}`,               // 缺 value
		`{"kind":"nonsense"}`,           // 未知种类
		`not json`,
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			if _, err := UnmarshalEnvelope(s); err == nil {
				t.Error("应报错但被接受")
			}
		})
	}
}

// TestCompareHandlesCorruptEnvelope 确认落库信封损坏时判错而非 panic。
func TestCompareHandlesCorruptEnvelope(t *testing.T) {
	bad := []Envelope{
		{Kind: KindRational, Num: "notanumber", Den: "1"},
		{Kind: KindRational, Num: "1", Den: "0"},
		{Kind: KindRational, Num: "1", Den: "notanumber"},
		{Kind: "unknown"},
	}
	for i, env := range bad {
		got := Compare("1", env, Exact())
		if got.Correct {
			t.Errorf("损坏信封 #%d 应判错", i)
		}
		if got.Reason == "" {
			t.Errorf("损坏信封 #%d 应给出原因", i)
		}
	}
}

// TestRationalCmp 直接验证交叉相乘的比较语义。
func TestRationalCmp(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1/2", "1/3", 1},
		{"1/3", "1/2", -1},
		{"2/4", "1/2", 0},
		{"-1/2", "1/2", -1},
		{"0", "-0", 0},
	}
	for _, tc := range cases {
		t.Run(tc.a+" vs "+tc.b, func(t *testing.T) {
			ra, err := ParseRational(tc.a)
			if err != nil {
				t.Fatalf("解析 %q 出错: %v", tc.a, err)
			}
			rb, err := ParseRational(tc.b)
			if err != nil {
				t.Fatalf("解析 %q 出错: %v", tc.b, err)
			}
			if got := ra.Cmp(rb); got != tc.want {
				t.Errorf("%s vs %s = %d, 期望 %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// TestExactToleranceIsZero 确认 Exact() 是零值。
func TestExactToleranceIsZero(t *testing.T) {
	if !Exact().IsZero() {
		t.Error("Exact() 应为零值（精确比较）")
	}
	var zero Tolerance
	if !zero.IsZero() {
		t.Error("零值 Tolerance 应为精确比较")
	}
}

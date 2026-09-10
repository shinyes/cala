// Command scorpus 生成跨端判分一致性语料。
//
// 用法：
//
//	go run ./cmd/scorpus -out ../app/test/scoring/corpus_generated.dart
//
// 语料由 **Go 侧实现**（internal/scoring）计算得出，Dart 侧必须逐例对齐。
// 输出为 Dart 源码而非 JSON，使 app/test 下的测试不依赖 dart:io 文件读取，
// 从而能被 flutter test 直接运行（浏览器与 VM 均可）。
//
// 这是规格 §5.5.5「CI 语料门禁」的生成端：任一分歧即 Dart 测试失败，进而构建失败。
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/shinyes/cala/backend/internal/scoring"
)

func main() {
	out := flag.String("out", "", "输出 Dart 文件路径")
	minVectors := flag.Int("min", 10000, "最少语料条数（不足则报错）")
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "必须指定 -out")
		os.Exit(2)
	}

	table := scoring.CleanupTable()
	keys := scoring.CleanupKeys()

	var (
		classifyCases []classifyCase
		compareCases  []compareCase
	)

	// ------------------------------------------------------------------
	// 1. 分类语料：覆盖合法形态、笔误、边界
	// ------------------------------------------------------------------
	classifyInputs := []string{}
	classifyInputs = append(classifyInputs,
		// 整数
		"0", "1", "42", "-1", "-0", "+7", "007", "000", "10", "100", "1000000",
		// 小数
		"0.5", ".5", "3.14", "-0.25", "+0.25", "1.0", "0.0", "123.456",
		"0.000001", "999999.999999", "10.10", "1.", ".", "..", "1.2.3", "1..2",
		// 分数
		"1/2", "-1/2", "+1/2", "3/4", "6/3", "0/5", "1/0", "1/", "/2", "/",
		"1/2/3", "-1/-2", "1/-2",
		// 空白与符号
		"", " ", "   ", "\t", "\n", "-", "+", "--5", "++5", "+-5", "-+5",
		// 千分位与非 ASCII 标点（由清洗表处理）
		"1,234", "1,2,3", "1，234", "1、234", ",1", "1,", ",,",
		"１２３", "１．５", "１／２", "−5", "–5", "—5", "‐5", "‑5", "‒5",
		"．5", "。5", "／", "＝5", "＋5", "－5",
		// 全角/不换行空格
		"\u30001\u3000", "\u00A01\u00A0", " 1 ", "\u20071", "1\u202F",
		// 文本答案（含中文）
		"质数", "素数", "合数", " 质数 ", "质  数", "质 数", "Hello", "hello",
		"HELLO", "Hello World", "hello   world", "HÉLLO", "hÉllo",
		"合数（非质数）", "a1", "1a", "x", "X", "是", "否", "true", "TRUE",
		// 控制字符
		"a\x00b", "\x01", "1\x02", "\x7F",
	)

	// 超长用例（长度边界两侧）
	classifyInputs = append(classifyInputs,
		strings.Repeat("x", scoring.MaxAnswerLen),
		strings.Repeat("x", scoring.MaxAnswerLen+1),
		strings.Repeat("1", scoring.MaxAnswerLen),
		strings.Repeat("1", scoring.MaxAnswerLen+1),
		strings.Repeat("质", scoring.MaxAnswerLen+1),
	)

	for _, raw := range classifyInputs {
		env, err := scoring.Classify(raw)
		if err != nil {
			classifyCases = append(classifyCases, classifyCase{Input: raw, Err: true})
			continue
		}
		classifyCases = append(classifyCases, classifyCase{
			Input: raw, Kind: env.Kind, Num: env.Num, Den: env.Den, Value: env.Value,
		})
	}

	// 组合式生成：把数值片段与符号片段组合，扩充覆盖面。
	ints := []string{"0", "1", "2", "7", "10", "99", "100", "1234", "9007199254740993",
		"-1", "-7", "-100", "007", "0"}
	decs := []string{"0.5", "0.25", "3.14", "0.001", "1.0", "-0.5", ".5", "12.75"}
	fracs := []string{"1/2", "1/3", "2/4", "3/4", "22/7", "-1/3", "0/9", "100/3"}
	for _, a := range ints {
		for _, b := range decs {
			for _, sep := range []string{"", " ", "  "} {
				classifyInputs = append(classifyInputs, a+sep+b)
			}
		}
	}
	for _, a := range fracs {
		for _, pre := range []string{"", " ", "+", "-"} {
			classifyInputs = append(classifyInputs, pre+a)
		}
	}
	for _, a := range ints {
		for _, b := range []string{"０", "１", "２"} {
			classifyInputs = append(classifyInputs, a+b)
		}
	}

	for _, raw := range classifyInputs {
		env, err := scoring.Classify(raw)
		if err != nil {
			classifyCases = append(classifyCases, classifyCase{Input: raw, Err: true})
			continue
		}
		classifyCases = append(classifyCases, classifyCase{
			Input: raw, Kind: env.Kind, Num: env.Num, Den: env.Den, Value: env.Value,
		})
	}

	// ------------------------------------------------------------------
	// 2. 比较语料：精确 + 容差，含 float64 陷阱
	// ------------------------------------------------------------------
	answers := []string{
		"0", "1", "-1", "2", "7", "10", "100", "127", "1000",
		"0.5", "0.25", "3.14", "3.14159", "0.1", "0.3", "1.5", "-2.75",
		"1/2", "1/3", "2/3", "3/4", "22/7", "-1/3", "7/2",
		"9007199254740992", "9007199254740993",
		"质数", "素数", "hello world", "prime", "是", "否",
	}
	inputs := []string{
		"", " ", "0", "1", "-1", "2", "7", "9", "10", "99", "100", "127", "1000",
		"0.5", ".5", "0.50", "0.25", "3.14", "3.15", "3.14159", "3.13159", "3.15159",
		"3.131589", "3.2", "0.1", "0.2", "0.3", "0.30000000000000004",
		"1/2", "1/3", "2/3", "2/4", "3/4", "22/7", "-1/3", "7/2", "6/3",
		"9007199254740992", "9007199254740993",
		"0.3333333333333333", "0.33333333333333331",
		"abc", "不是数字", "5x", "x5", "--1", "1.2.3", "1/0",
		"１２３", "0007", "1,234", "−5", " 质数 ", "质  数", "PRIME",
		"hello  world", "素数", "HELLO WORLD",
	}
	tolerances := []struct {
		num, den int64
	}{
		{0, 1}, {1, 100}, {1, 1000}, {1, 10}, {1, 2}, {5, 100},
		{1, 10000000000000000},
	}

	tolValues := make([]scoring.Tolerance, 0, len(tolerances))
	for _, tv := range tolerances {
		t, err := scoring.NewTolerance(tv.num, tv.den)
		if err != nil {
			fmt.Fprintf(os.Stderr, "构造容差失败 %d/%d: %v\n", tv.num, tv.den, err)
			os.Exit(1)
		}
		tolValues = append(tolValues, t)
	}

	for _, ans := range answers {
		env, err := scoring.Classify(ans)
		if err != nil {
			// 不作为答案的输入（例如笔误）跳过
			continue
		}
		for _, in := range inputs {
			for i, tol := range tolValues {
				res := scoring.Compare(in, env, tol)
				compareCases = append(compareCases, compareCase{
					Input: in, Answer: ans,
					TolNum: tolerances[i].num, TolDen: tolerances[i].den,
					Correct: res.Correct,
				})
			}
		}
	}

	// 交叉相乘的精确性：a/b 与 (a*k)/(b*k) 必须相等
	for _, a := range []int64{1, 2, 3, 5, 7, 11, 13, 100, 999999} {
		for _, b := range []int64{2, 3, 4, 7, 9, 100, 1000} {
			for _, k := range []int64{2, 3, 10, 1000} {
				ans := fmt.Sprintf("%d/%d", a, b)
				env, err := scoring.Classify(ans)
				if err != nil {
					continue
				}
				in := fmt.Sprintf("%d/%d", a*k, b*k)
				res := scoring.Compare(in, env, scoring.Exact())
				compareCases = append(compareCases, compareCase{
					Input: in, Answer: ans, TolNum: 0, TolDen: 1, Correct: res.Correct,
				})
			}
		}
	}

	// 只对**总数**设下限：分类语料天然比比较语料少一个数量级
	// （比较语料是 答案×输入×容差 的组合），单独给分类语料设阈值只会产生噪声警告。
	total := len(classifyCases) + len(compareCases)
	if total < *minVectors {
		fmt.Fprintf(os.Stderr, "语料总数不足：%d < %d\n", total, *minVectors)
		os.Exit(1)
	}

	// ------------------------------------------------------------------
	// 3. 生成 Dart 源码
	// ------------------------------------------------------------------
	var b strings.Builder
	b.WriteString("// GENERATED FILE - DO NOT EDIT BY HAND.\n")
	b.WriteString("//\n")
	b.WriteString("// Regenerate with:\n")
	b.WriteString("//   cd backend && go run ./cmd/scorpus -out ../app/test/scoring/corpus_generated.dart\n")
	b.WriteString("//\n")
	b.WriteString("// Vectors are computed by the Go implementation (backend/internal/scoring).\n")
	b.WriteString("// The Dart implementation must agree on every vector; any divergence fails\n")
	b.WriteString("// the test, which fails CI (design spec section 5.5.5).\n")
	b.WriteString("//\n")
	b.WriteString("// This file is intentionally pure ASCII: all non-ASCII characters are escaped\n")
	b.WriteString("// as \\u{XXXX} so that opening/saving it under a different source encoding\n")
	b.WriteString("// cannot silently corrupt the corpus.\n")
	b.WriteString("// ignore_for_file: prefer_single_quotes\n\n")

	// 清洗表
	b.WriteString("/// Cleanup table as served by the backend (scoring.CleanupTable).\n")
	b.WriteString("const Map<String, String> corpusCleanupTable = <String, String>{\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s: %s,\n", dartString(k), dartString(table[k]))
	}
	b.WriteString("};\n\n")

	// 分类语料
	b.WriteString("/// One classify vector. expectError=true means the Go side rejects the input.\n")
	b.WriteString("class ClassifyVector {\n")
	b.WriteString("  final String input;\n")
	b.WriteString("  final bool expectError;\n")
	b.WriteString("  final String kind;\n")
	b.WriteString("  final String num;\n")
	b.WriteString("  final String den;\n")
	b.WriteString("  final String value;\n")
	b.WriteString("  const ClassifyVector({\n")
	b.WriteString("    required this.input,\n")
	b.WriteString("    required this.expectError,\n")
	b.WriteString("    this.kind = '',\n")
	b.WriteString("    this.num = '',\n")
	b.WriteString("    this.den = '',\n")
	b.WriteString("    this.value = '',\n")
	b.WriteString("  });\n")
	b.WriteString("}\n\n")
	b.WriteString("const List<ClassifyVector> corpusClassify = <ClassifyVector>[\n")
	for _, c := range classifyCases {
		fmt.Fprintf(&b, "  ClassifyVector(input: %s, expectError: %v, kind: %s, num: %s, den: %s, value: %s),\n",
			dartString(c.Input), c.Err, dartString(c.Kind), dartString(c.Num), dartString(c.Den), dartString(c.Value))
	}
	b.WriteString("];\n\n")

	// 比较语料
	b.WriteString("/// One compare vector.\n")
	b.WriteString("class CompareVector {\n")
	b.WriteString("  final String input;\n")
	b.WriteString("  final String answer;\n")
	b.WriteString("  final int tolNum;\n")
	b.WriteString("  final int tolDen;\n")
	b.WriteString("  final bool correct;\n")
	b.WriteString("  const CompareVector({\n")
	b.WriteString("    required this.input,\n")
	b.WriteString("    required this.answer,\n")
	b.WriteString("    required this.tolNum,\n")
	b.WriteString("    required this.tolDen,\n")
	b.WriteString("    required this.correct,\n")
	b.WriteString("  });\n")
	b.WriteString("}\n\n")
	b.WriteString("const List<CompareVector> corpusCompare = <CompareVector>[\n")
	for _, c := range compareCases {
		fmt.Fprintf(&b, "  CompareVector(input: %s, answer: %s, tolNum: %d, tolDen: %d, correct: %v),\n",
			dartString(c.Input), dartString(c.Answer), c.TolNum, c.TolDen, c.Correct)
	}
	b.WriteString("];\n")

	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("已生成 %s\n", *out)
	fmt.Printf("  分类语料 %d 条（其中 %d 条期望报错）\n", len(classifyCases), countErrors(classifyCases))
	fmt.Printf("  比较语料 %d 条\n", len(compareCases))
	fmt.Printf("  合计     %d 条\n", total)
	fmt.Printf("  清洗表   %d 项\n", len(table))
}

type classifyCase struct {
	Input string
	Err   bool
	Kind  string
	Num   string
	Den   string
	Value string
}

type compareCase struct {
	Input   string
	Answer  string
	TolNum  int64
	TolDen  int64
	Correct bool
}

func countErrors(cs []classifyCase) int {
	n := 0
	for _, c := range cs {
		if c.Err {
			n++
		}
	}
	return n
}

// dartString 把 Go 字符串转成 Dart 字面量。
//
// **所有非 ASCII 字符都转义为 \u{XXXX}**，使生成的文件是纯 ASCII。
// 理由：语料里大量包含不可见字符（NBSP、全角空格、各类 Unicode 减号）
// 与中文答案，若原样写出，文件一旦被以其他编码打开并保存就会静默损坏。
// 本项目已因编码问题踩过坑（见计划 P-R9），生成物更应绝缘于此。
// Dart 支持 \u{...} 形式的任意码点转义。
func dartString(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\'':
			b.WriteString("\\'")
		case '\\':
			b.WriteString("\\\\")
		case '$':
			b.WriteString("\\$")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 || r == 0x7F || r > 0x7E {
				fmt.Fprintf(&b, "\\u{%X}", r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

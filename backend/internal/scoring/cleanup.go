package scoring

import (
	"sort"
	"strings"
)

// CleanupVersion 标识清洗表的内容版本。
//
// 清洗表作为**数据**随 /rounds/start 下发（规格 §5.5.4），客户端只负责应用它。
// 版本号让服务端日后调整表格时，能在分歧告警里分辨「客户端用了旧表」
// 还是「两边算法真的不一致」。
const CleanupVersion = 1

// CleanupTable 返回字符映射表：键为单个字符，值为替换文本（空串表示删除）。
//
// 该表是「清洗规则唯一 owner 在服务端」的具体体现：Go 与 Dart 都不写清洗逻辑，
// 而是共同应用这张表。表里只做字符替换，不含任何依赖 Unicode 数据库的运算。
func CleanupTable() map[string]string {
	return map[string]string{
		// 全角数字 ０-９ -> 0-9
		"０": "0", "１": "1", "２": "2", "３": "3", "４": "4",
		"５": "5", "６": "6", "７": "7", "８": "8", "９": "9",
		// 全角运算符与句读
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
		// 各类空白 -> ASCII 空格
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
// 只做表驱动的字符替换，随后仅做 ASCII 空白裁剪。
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

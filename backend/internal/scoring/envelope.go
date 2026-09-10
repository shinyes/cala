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

// MaxAnswerLen 是答案长度上限（按 rune 计）。
// 与 rules.MaxAnswerLen 取值一致；本包不依赖 rules，故独立声明，
// 并由 TestAnswerLenMatchesRules 保证两者不会漂移。
const MaxAnswerLen = 500

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
// 仅由这些字符组成却无法解析的答案会被判为笔误（规格 §5.5.2 第三条）。
const numericAlphabet = "0123456789+-./"

// Classify 把作者返回的答案原文分类为规范信封。
//
// 规则（规格 §5.5.2）：
//  1. 清洗后可解析为有理数且分母非零 -> rational
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
	if i, r := indexControl(raw); i >= 0 {
		return Envelope{}, fmt.Errorf("答案含有控制字符 U+%04X（位置 %d）；请勿粘贴不可见字符", r, i)
	}

	cleaned := Clean(raw)

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

	v := NormalizeText(cleaned)
	if v == "" {
		return Envelope{}, fmt.Errorf("答案不能为空")
	}
	return Envelope{Kind: KindText, Value: v}, nil
}

// isNumericCandidate 判断清洗后的字符串是否「只由数值字母表组成」。
// 空串不算数值候选，否则空答案会走到解析分支报「无法解析」而非「不能为空」。
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

// indexControl 返回首个控制字符的字节位置与码点，无则返回 -1。
func indexControl(s string) (int, rune) {
	for i, r := range s {
		if r < 0x20 || r == 0x7F {
			return i, r
		}
	}
	return -1, 0
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
	case KindRational:
		if e.Num == "" || e.Den == "" {
			return Envelope{}, fmt.Errorf("有理数信封缺少 num 或 den")
		}
	case KindText:
		if e.Value == "" {
			return Envelope{}, fmt.Errorf("文本信封缺少 value")
		}
	default:
		return Envelope{}, fmt.Errorf("未知的答案信封种类 %q", e.Kind)
	}
	return e, nil
}

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

// Tolerance 是项目级可选容差（规格 §5.5.3），表示为整数比 Num/Den。
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
			// 落库的信封损坏属于服务端数据问题：判为错并保留原因供排查
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

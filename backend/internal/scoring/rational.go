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

// ParseRational 严格解析一个有理数字面量。
//
// 文法极小，便于客户端逐例镜像：
//
//	number := sign? ( digits | digits '.' digits | '.' digits | digits '/' digits )
//	sign   := '+' | '-'
//
// 分母必须非零。允许前导零。不接受科学计数法；千分位已在 Clean 阶段删除。
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

// Cmp 比较两个有理数，返回 -1/0/1。交叉相乘，全程整数（规格 §5.5.3）。
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

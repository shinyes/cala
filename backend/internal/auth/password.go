// Package auth 拥有口令哈希与会话令牌这两项安全敏感的唯一实现。
package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// ErrPasswordTooShort 用于给用户可读的提示。
var ErrPasswordTooShort = errors.New("口令至少需要 8 个字符")

// MinPasswordLen 是口令最小长度。
const MinPasswordLen = 8

// HashPassword 返回 bcrypt 哈希。cost 使用库默认值（当前为 10）。
func HashPassword(plain string) (string, error) {
	if len([]rune(plain)) < MinPasswordLen {
		return "", ErrPasswordTooShort
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("生成口令哈希失败: %w", err)
	}
	return string(h), nil
}

// VerifyPassword 校验明文口令与哈希是否匹配。
// 使用 bcrypt 的常数时间比较，避免计时侧信道。
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// dummyHash 是一个**真实格式**的 bcrypt 哈希，用于在用户名不存在时执行等价的计算量。
//
// 必须是真实哈希：若手写一个格式非法的字符串，CompareHashAndPassword 会立即
// 返回格式错误而不做密钥派生，反而让「用户不存在」比「口令错误」快得多，
// 恰好制造出它本应消除的计时差异。
var dummyHash = mustHash("cala-login-timing-equalizer")

// BurnPasswordComparison 在用户不存在时调用，使两条路径耗时相近。
func BurnPasswordComparison(plain string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(plain))
}

func mustHash(s string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	if err != nil {
		// 仅可能因 cost 非法而失败，属编程错误
		panic("auth: 无法生成 dummy 哈希: " + err.Error())
	}
	return string(h)
}

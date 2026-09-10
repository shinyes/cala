package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPasswordRejectsShort(t *testing.T) {
	if _, err := HashPassword("short"); err != ErrPasswordTooShort {
		t.Fatalf("过短口令应返回 ErrPasswordTooShort, 得到 %v", err)
	}
}

func TestHashPasswordVerifyRoundTrip(t *testing.T) {
	const pw = "correct horse battery"
	h, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if strings.Contains(h, pw) {
		t.Fatal("哈希中不应包含明文")
	}
	if !VerifyPassword(h, pw) {
		t.Error("正确口令应当校验通过")
	}
	if VerifyPassword(h, pw+"x") {
		t.Error("错误口令不应通过")
	}
}

func TestHashIsSalted(t *testing.T) {
	const pw = "same password twice"
	h1, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	h2, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if h1 == h2 {
		t.Error("相同口令两次哈希应因随机 salt 而不同")
	}
	if !VerifyPassword(h1, pw) || !VerifyPassword(h2, pw) {
		t.Error("两个哈希都应能校验原口令")
	}
}

// TestDummyHashIsValidBcrypt 是计时抹平有效性的前提条件。
// 若 dummyHash 不是合法 bcrypt 哈希，CompareHashAndPassword 会立即返回错误
// 而不做密钥派生，抹平就失去意义 —— 本测试守住这一点。
func TestDummyHashIsValidBcrypt(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyHash))
	if err != nil {
		t.Fatalf("dummyHash 不是合法 bcrypt 哈希: %v", err)
	}
	if cost != bcrypt.DefaultCost {
		t.Errorf("dummyHash cost = %d, 期望 %d", cost, bcrypt.DefaultCost)
	}
	// 抹平函数不应 panic，且对任意输入都返回
	BurnPasswordComparison("anything")
	BurnPasswordComparison("")
}

// TestPasswordLenCountsRunes 保证多字节口令按字符而非字节计数。
func TestPasswordLenCountsRunes(t *testing.T) {
	// 8 个中文字符 = 24 字节，应按 8 个字符计，属于合法长度
	if _, err := HashPassword("口令口令口令口令"); err != nil {
		t.Errorf("8 个字符的口令应合法, 得到 %v", err)
	}
	// 7 个中文字符应被拒
	if _, err := HashPassword("口令口令口令口"); err != ErrPasswordTooShort {
		t.Errorf("7 个字符的口令应被拒, 得到 %v", err)
	}
}

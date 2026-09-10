package auth

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewTokenFormatAndUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		raw, hash, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken 失败: %v", err)
		}
		if seen[raw] {
			t.Fatal("生成了重复令牌")
		}
		seen[raw] = true

		if strings.ContainsAny(raw, "+/=") {
			t.Errorf("令牌应为 base64url 无填充, 得到 %q", raw)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			t.Fatalf("令牌不是合法 base64url: %v", err)
		}
		if len(decoded) != tokenBytes {
			t.Errorf("令牌熵 = %d 字节, 期望 %d", len(decoded), tokenBytes)
		}
		if len(hash) != 64 {
			t.Errorf("SHA-256 十六进制摘要长度 = %d, 期望 64", len(hash))
		}
		if hash != HashToken(raw) {
			t.Error("NewToken 返回的 hash 应与 HashToken(raw) 一致")
		}
	}
}

func TestHashTokenIsDeterministic(t *testing.T) {
	const raw = "some-token"
	if HashToken(raw) != HashToken(raw) {
		t.Error("HashToken 应是确定性的")
	}
	if HashToken(raw) == HashToken(raw+"x") {
		t.Error("不同令牌不应产生相同摘要")
	}
}

// TestTokenHashDoesNotContainRaw 保证入库摘要不泄露明文令牌。
// token_hash 是 session 表的唯一索引列，其内容必须无法反推会话。
func TestTokenHashDoesNotContainRaw(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken 失败: %v", err)
	}
	if strings.Contains(hash, raw) {
		t.Error("摘要中不应包含明文令牌")
	}
	if hash == raw {
		t.Error("摘要不应等于明文令牌")
	}
}

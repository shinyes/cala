package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// tokenBytes 是会话令牌的熵，32 字节 = 256 位，远超暴力破解可行范围。
const tokenBytes = 32

// NewToken 生成一个新的不透明会话令牌，返回：
//   - raw：交给客户端的明文令牌（仅此一次可见）
//   - hash：入库的 SHA-256 十六进制摘要
//
// 使用 SHA-256 而非 bcrypt：令牌本身是高熵随机值，不存在字典攻击面，
// 无需慢哈希；此处的目标是「数据库泄露不等于会话可被冒用」。
func NewToken() (raw, hash string, err error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("生成随机令牌失败: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

// HashToken 返回令牌的入库摘要。查表时对客户端提交的令牌用同一函数处理。
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

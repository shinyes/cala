// Package service 承载业务规则，是应用层逻辑的唯一 owner。
package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shinyes/cala/backend/internal/auth"
	"github.com/shinyes/cala/backend/internal/store"
)

var (
	// ErrRegistrationClosed 表示管理员已关闭注册。
	ErrRegistrationClosed = errors.New("注册已关闭")
	// ErrInvalidCredentials 表示用户名或口令错误。
	// 刻意不区分「用户不存在」与「口令错误」，避免用户名枚举。
	ErrInvalidCredentials = errors.New("用户名或口令错误")
	// ErrInvalidUsername 表示用户名不合法。
	ErrInvalidUsername = errors.New("用户名需为 3-32 个字符，仅限字母、数字、下划线、连字符")
)

// AuthService 实现注册、登录、登出。
type AuthService struct {
	store          *store.Store
	sessionTTL     time.Duration
	regOpenDefault bool
}

// NewAuthService 构造认证服务。
func NewAuthService(st *store.Store, sessionTTL time.Duration, regOpenDefault bool) *AuthService {
	return &AuthService{store: st, sessionTTL: sessionTTL, regOpenDefault: regOpenDefault}
}

// RegisterResult 是注册结果。
type RegisterResult struct {
	User  store.User
	Token string
	// BecameAdmin 为 true 表示这是引导管理员（首个用户）。
	BecameAdmin bool
}

// Register 创建用户。
//
// 关键规则（规格 §6.2）：当系统尚无任何用户时，无论注册开关如何，
// 都允许注册并将其设为管理员。否则全新部署会因「开关默认关闭」而永久无法使用。
func (s *AuthService) Register(username, password string) (RegisterResult, error) {
	username = strings.TrimSpace(username)
	if !validUsername(username) {
		return RegisterResult{}, ErrInvalidUsername
	}

	// 先校验口令，避免在口令不合法时白做一次哈希与写入
	if len([]rune(password)) < auth.MinPasswordLen {
		return RegisterResult{}, auth.ErrPasswordTooShort
	}

	count, err := s.store.CountUsers()
	if err != nil {
		return RegisterResult{}, err
	}

	bootstrap := count == 0
	if !bootstrap {
		open, err := s.store.RegistrationOpen(s.regOpenDefault)
		if err != nil {
			return RegisterResult{}, err
		}
		if !open {
			return RegisterResult{}, ErrRegistrationClosed
		}
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return RegisterResult{}, err
	}

	u, err := s.store.CreateUser(username, hash, bootstrap)
	if err != nil {
		return RegisterResult{}, err
	}

	token, err := s.issueSession(u.ID)
	if err != nil {
		// 用户已建成但会话签发失败：不掩盖错误，调用方可见
		return RegisterResult{}, err
	}
	return RegisterResult{User: u, Token: token, BecameAdmin: bootstrap}, nil
}

// Login 校验凭证并签发会话。
func (s *AuthService) Login(username, password string) (store.User, string, error) {
	username = strings.TrimSpace(username)

	u, hash, err := s.store.AuthenticateLookup(username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// 执行等价计算量，抹平「用户不存在」与「口令错误」的耗时差异
			auth.BurnPasswordComparison(password)
			return store.User{}, "", ErrInvalidCredentials
		}
		return store.User{}, "", err
	}
	// 先校验口令，再判定禁用状态：两条分支响应体相同（均为 ErrInvalidCredentials），
	// 但先校验口令可保证耗时相近，不泄露账号存在性。
	if !auth.VerifyPassword(hash, password) {
		return store.User{}, "", ErrInvalidCredentials
	}
	if u.Disabled {
		return store.User{}, "", ErrInvalidCredentials
	}

	token, err := s.issueSession(u.ID)
	if err != nil {
		return store.User{}, "", err
	}
	return u, token, nil
}

// Logout 吊销会话。
func (s *AuthService) Logout(rawToken string) error {
	return s.store.DeleteSession(auth.HashToken(rawToken))
}

// Me 解析会话令牌对应的用户。
func (s *AuthService) Me(rawToken string) (store.User, error) {
	return s.store.SessionUser(auth.HashToken(rawToken))
}

// RegistrationOpen 读取当前注册开关，供公开配置端点使用。
func (s *AuthService) RegistrationOpen() (bool, error) {
	return s.store.RegistrationOpen(s.regOpenDefault)
}

// SetRegistrationOpen 修改注册开关（仅管理员，权限判定在 API 层）。
func (s *AuthService) SetRegistrationOpen(open bool) error {
	return s.store.SetRegistrationOpen(open)
}

func (s *AuthService) issueSession(userID int64) (string, error) {
	raw, hash, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	if err := s.store.CreateSession(hash, userID, time.Now().UTC().Add(s.sessionTTL)); err != nil {
		return "", fmt.Errorf("签发会话失败: %w", err)
	}
	return raw, nil
}

func validUsername(u string) bool {
	n := len([]rune(u))
	if n < 3 || n > 32 {
		return false
	}
	for _, r := range u {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

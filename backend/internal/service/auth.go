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
	// ErrWrongCurrentPassword 表示当前口令不正确。
	//
	// 刻意与 ErrInvalidCredentials **分开**：登录失败要含糊（避免用户名枚举），
	// 而这里是已认证用户在校验自己提供的当前口令，必须明确告知，
	// 否则用户无从知道是「当前口令错了」还是「新口令不合法」。
	ErrWrongCurrentPassword = errors.New("当前口令不正确")
	// ErrSamePassword 表示新口令与当前口令相同。
	ErrSamePassword = errors.New("新口令不能与当前口令相同")
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

// ChangePassword 修改当前用户的口令，返回**新的**会话令牌。
//
// 为什么要求提供当前口令：改密端点若只凭会话令牌就放行，
// 那么任何拿到令牌的人（XSS、设备失窃、日志泄露）都能直接改密并**永久占有**账号 ——
// 令牌本会过期，而改密让攻击者把访问权延展到无限期。要求当前口令把这一步
// 重新绑定到「知道口令」上，令牌泄露不再等于账号失守。
//
// 会话轮换：成功后**吊销该用户的全部会话**，并为当前设备签发新令牌。
//   - 吊销全部（而非仅其他设备）是为了让实现简单且无遗漏 ——
//     「除当前会话外全部删除」需要一个额外的排除条件，漏写就是把攻击者的会话留下。
//   - 随即签发新令牌，使当前设备无感续用；用户不会在改密后被踢下线。
//
// 返回值是新令牌。调用方（API 层）必须把它交给客户端，否则用户在下次请求时被登出。
func (s *AuthService) ChangePassword(userID int64, currentPassword, newPassword string) (string, error) {
	// 1) 先做无副作用的新口令长度校验：明显非法的输入不必走 bcrypt 与写库
	if len([]rune(newPassword)) < auth.MinPasswordLen {
		return "", auth.ErrPasswordTooShort
	}

	// 2) 校验当前口令。
	//    刻意放在「新旧是否相同」之前：若顺序反过来，当用户把当前口令**输错**
	//    且新口令恰好等于那个错误输入时，会收到「新旧口令相同」——
	//    而真正的问题是当前口令不对，该提示会把人引向错误的修正方向。
	hash, err := s.store.PasswordHash(userID)
	if err != nil {
		return "", err
	}
	if !auth.VerifyPassword(hash, currentPassword) {
		return "", ErrWrongCurrentPassword
	}

	// 3) 新口令不得与当前口令相同。
	//    此处直接比较明文（两者都在手上），无需再做一次 bcrypt ——
	//    当前口令的正确性已在上一步验证过，因此这次比较是精确的。
	//    拒绝的理由：改成一个相同的口令几乎总是误操作（看错了输入框），
	//    而且它会返回「成功」却什么都没改变，让人误以为已经换掉了泄露的口令。
	if currentPassword == newPassword {
		return "", ErrSamePassword
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return "", err
	}

	raw, tokenHash, err := auth.NewToken()
	if err != nil {
		return "", err
	}

	if err := s.store.SetPasswordAndRotateSessions(
		userID, newHash, tokenHash, time.Now().UTC().Add(s.sessionTTL),
	); err != nil {
		return "", err
	}
	return raw, nil
}

// RegistrationOpen 读取当前注册开关，供公开配置端点使用。
func (s *AuthService) RegistrationOpen() (bool, error) {
	return s.store.RegistrationOpen(s.regOpenDefault)
}

// RegistrationStatus 返回「现在是否允许新用户注册」的**有效值**。
//
// 为什么要由服务端算这个有效值、而不是让客户端自己组合：
// 引导管理员规则（系统无用户时无视开关）是服务端的事实。若客户端自行实现
// 「开关关闭但无用户时仍显示注册入口」，同一条规则就有了两个 owner，
// 且客户端并不知道用户数——它只能猜。返回有效值使客户端只负责渲染。
//
// 返回：
//   - canRegister：是否允许注册（已计入 bootstrap）
//   - bootstrap：系统尚无用户，注册者将成为管理员
//   - settingOpen：settings 表中的原始开关值（供管理员界面显示）
func (s *AuthService) RegistrationStatus() (canRegister, bootstrap, settingOpen bool, err error) {
	count, err := s.store.CountUsers()
	if err != nil {
		return false, false, false, err
	}
	settingOpen, err = s.store.RegistrationOpen(s.regOpenDefault)
	if err != nil {
		return false, false, false, err
	}
	bootstrap = count == 0
	canRegister = bootstrap || settingOpen
	return canRegister, bootstrap, settingOpen, nil
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

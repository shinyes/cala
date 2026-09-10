package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/shinyes/cala/backend/internal/auth"
	"github.com/shinyes/cala/backend/internal/store"
)

func newAuthService(t *testing.T, regOpenDefault bool) *AuthService {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewAuthService(st, time.Hour, regOpenDefault)
}

// TestFirstUserBecomesAdminEvenWhenRegistrationClosed 是本项目最关键的引导规则。
// 若实现忽略 bootstrap 分支，全新部署将永久无法创建管理员 —— 系统自我锁死。
func TestFirstUserBecomesAdminEvenWhenRegistrationClosed(t *testing.T) {
	// 注册开关初值设为 false —— 模拟管理员关闭注册后的全新部署
	svc := newAuthService(t, false)

	res, err := svc.Register("firstuser", "password123")
	if err != nil {
		t.Fatalf("首个用户应能注册, 得到错误: %v", err)
	}
	if !res.BecameAdmin || !res.User.IsAdmin {
		t.Error("首个用户必须成为管理员")
	}
	if res.Token == "" {
		t.Error("注册应同时签发会话")
	}

	// 第二个用户在开关关闭时必须被拒
	if _, err := svc.Register("seconduser", "password123"); !errors.Is(err, ErrRegistrationClosed) {
		t.Errorf("开关关闭时第二个用户应被拒, 得到 %v", err)
	}
}

func TestSecondUserIsNotAdminWhenOpen(t *testing.T) {
	svc := newAuthService(t, true)

	first, err := svc.Register("firstuser", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if !first.BecameAdmin {
		t.Error("首个用户应为管理员")
	}

	second, err := svc.Register("seconduser", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if second.BecameAdmin || second.User.IsAdmin {
		t.Error("第二个用户不应是管理员")
	}
}

func TestRegisterValidation(t *testing.T) {
	svc := newAuthService(t, true)

	if _, err := svc.Register("ab", "password123"); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("过短用户名应被拒, 得到 %v", err)
	}
	if _, err := svc.Register("bad name", "password123"); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("含空格用户名应被拒, 得到 %v", err)
	}
	if _, err := svc.Register("bad@email", "password123"); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("含非法字符用户名应被拒, 得到 %v", err)
	}
	if _, err := svc.Register("gooduser", "short"); !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Errorf("过短口令应被拒, 得到 %v", err)
	}
	if _, err := svc.Register("gooduser", "password123"); err != nil {
		t.Errorf("合法注册不应失败: %v", err)
	}
	if _, err := svc.Register("gooduser", "password123"); !errors.Is(err, store.ErrUsernameTaken) {
		t.Errorf("重复用户名应被拒, 得到 %v", err)
	}
}

// TestRegisterTrimsUsername 保证首尾空白不会造出「看起来相同」的两个账号。
func TestRegisterTrimsUsername(t *testing.T) {
	svc := newAuthService(t, true)

	if _, err := svc.Register("  alice  ", "password123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	// 去空白后同名，应被判为重复而非新建
	if _, err := svc.Register("alice", "password123"); !errors.Is(err, store.ErrUsernameTaken) {
		t.Errorf("去空白后同名应被判重复, 得到 %v", err)
	}
	if _, _, err := svc.Login("alice", "password123"); err != nil {
		t.Errorf("应以去空白后的用户名登录成功: %v", err)
	}
}

// TestPasswordTooShortRejectedBeforeAnyWrite 保证口令不合法时不会留下半个用户。
func TestPasswordTooShortRejectedBeforeAnyWrite(t *testing.T) {
	svc := newAuthService(t, true)

	if _, err := svc.Register("someone", "short"); !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Fatalf("应因口令过短被拒, 得到 %v", err)
	}
	// 用户不应被创建 —— 否则该用户名会被一个无法登录的账号占用
	if _, _, err := svc.Login("someone", "short"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("不应存在该用户, 得到 %v", err)
	}
}

func TestLoginLogoutAndMe(t *testing.T) {
	svc := newAuthService(t, true)
	if _, err := svc.Register("alice", "password123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	u, token, err := svc.Login("alice", "password123")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("用户名 = %q", u.Username)
	}

	if _, _, err := svc.Login("alice", "wrongpassword"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("错误口令应返回 ErrInvalidCredentials, 得到 %v", err)
	}
	if _, _, err := svc.Login("nobody", "password123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("不存在用户应返回 ErrInvalidCredentials（不泄露存在性）, 得到 %v", err)
	}

	me, err := svc.Me(token)
	if err != nil {
		t.Fatalf("Me 失败: %v", err)
	}
	if me.ID != u.ID {
		t.Errorf("Me 返回用户 = %d, 期望 %d", me.ID, u.ID)
	}

	// 未知令牌不应解析出用户
	if _, err := svc.Me("never-issued-token"); err == nil {
		t.Error("未知令牌应当报错")
	}

	if err := svc.Logout(token); err != nil {
		t.Fatalf("Logout 失败: %v", err)
	}
	if _, err := svc.Me(token); err == nil {
		t.Error("登出后会话应失效")
	}
}

// TestRegisterIssuesUsableSession 保证注册返回的 token 可直接使用，
// 而非只是一个非空字符串。
func TestRegisterIssuesUsableSession(t *testing.T) {
	svc := newAuthService(t, true)

	res, err := svc.Register("alice", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	me, err := svc.Me(res.Token)
	if err != nil {
		t.Fatalf("注册返回的 token 应可解析出用户: %v", err)
	}
	if me.ID != res.User.ID {
		t.Errorf("token 对应用户 = %d, 期望 %d", me.ID, res.User.ID)
	}
}

// TestMultipleSessionsCoexist 保证多设备登录互不干扰。
func TestMultipleSessionsCoexist(t *testing.T) {
	svc := newAuthService(t, true)
	if _, err := svc.Register("alice", "password123"); err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	_, t1, err := svc.Login("alice", "password123")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	_, t2, err := svc.Login("alice", "password123")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if t1 == t2 {
		t.Fatal("两次登录应签发不同令牌")
	}

	if err := svc.Logout(t1); err != nil {
		t.Fatalf("Logout 失败: %v", err)
	}
	if _, err := svc.Me(t1); err == nil {
		t.Error("t1 应已失效")
	}
	if _, err := svc.Me(t2); err != nil {
		t.Errorf("t2 不应受 t1 登出影响: %v", err)
	}
}

func TestSetRegistrationOpenRoundTrip(t *testing.T) {
	svc := newAuthService(t, true)

	if err := svc.SetRegistrationOpen(false); err != nil {
		t.Fatalf("SetRegistrationOpen 失败: %v", err)
	}
	open, err := svc.RegistrationOpen()
	if err != nil {
		t.Fatalf("RegistrationOpen 失败: %v", err)
	}
	if open {
		t.Error("写入 false 后应为 false")
	}
}

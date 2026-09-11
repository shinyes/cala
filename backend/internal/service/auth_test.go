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

// --- 修改口令 ---

// TestChangePasswordHappyPath 覆盖基本路径与「当前设备无感续用」。
func TestChangePasswordHappyPath(t *testing.T) {
	svc := newAuthService(t, true)
	reg, err := svc.Register("alice", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	newToken, err := svc.ChangePassword(reg.User.ID, "password123", "newpassword456")
	if err != nil {
		t.Fatalf("改密失败: %v", err)
	}
	if newToken == "" {
		t.Fatal("改密应返回新令牌，否则当前设备会被登出")
	}
	if newToken == reg.Token {
		t.Error("新令牌不应与旧令牌相同")
	}

	// 新令牌必须可用 —— 当前设备无感续用
	if _, err := svc.Me(newToken); err != nil {
		t.Errorf("新令牌应立即可用: %v", err)
	}

	// 旧口令不可再登录
	if _, _, err := svc.Login("alice", "password123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("旧口令应失效, 得到 %v", err)
	}
	// 新口令可登录
	if _, _, err := svc.Login("alice", "newpassword456"); err != nil {
		t.Errorf("新口令应可登录: %v", err)
	}
}

// TestChangePasswordRevokesOtherSessions 是本功能最重要的安全不变式。
//
// 改密最常见的动机是「怀疑账号被他人登录」。若旧会话继续有效，
// 攻击者仍能访问，用户的补救措施完全落空 —— 而那正是他做这件事的原因。
func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	svc := newAuthService(t, true)
	reg, err := svc.Register("alice", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 模拟另一台设备上的会话
	_, otherDevice, err := svc.Login("alice", "password123")
	if err != nil {
		t.Fatalf("第二次登录失败: %v", err)
	}
	if _, err := svc.Me(otherDevice); err != nil {
		t.Fatalf("另一设备令牌本应有效: %v", err)
	}

	newToken, err := svc.ChangePassword(reg.User.ID, "password123", "newpassword456")
	if err != nil {
		t.Fatalf("改密失败: %v", err)
	}

	// 发起改密的那台设备：旧令牌也应失效（我们把全部会话都吊销了），
	// 但返回的新令牌可用。这正是「先全吊销、再签发」的设计意图。
	if _, err := svc.Me(reg.Token); err == nil {
		t.Error("发起改密的旧令牌也应失效（全部会话被吊销）")
	}
	if _, err := svc.Me(newToken); err != nil {
		t.Errorf("新令牌应有效: %v", err)
	}
	// 其他设备的会话必须失效 —— 这是本测试的核心
	if _, err := svc.Me(otherDevice); err == nil {
		t.Error("其他设备的会话必须被吊销，否则改密无法阻止已登录的攻击者")
	}
}

func TestChangePasswordRejectsWrongCurrent(t *testing.T) {
	svc := newAuthService(t, true)
	reg, err := svc.Register("alice", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	_, err = svc.ChangePassword(reg.User.ID, "wrongpassword", "newpassword456")
	if !errors.Is(err, ErrWrongCurrentPassword) {
		t.Errorf("当前口令错误应返回 ErrWrongCurrentPassword, 得到 %v", err)
	}

	// 口令不得被改动
	if _, _, err := svc.Login("alice", "password123"); err != nil {
		t.Errorf("当前口令错误时不应改动口令: %v", err)
	}
	// 会话也不得被吊销（失败的尝试不应把用户踢下线）
	if _, err := svc.Me(reg.Token); err != nil {
		t.Errorf("失败的改密不应吊销会话: %v", err)
	}
}

func TestChangePasswordRejectsTooShort(t *testing.T) {
	svc := newAuthService(t, true)
	reg, err := svc.Register("alice", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	_, err = svc.ChangePassword(reg.User.ID, "password123", "short")
	if !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Errorf("过短新口令应返回 ErrPasswordTooShort, 得到 %v", err)
	}
	if _, _, err := svc.Login("alice", "password123"); err != nil {
		t.Errorf("口令不应被改动: %v", err)
	}
}

// TestChangePasswordRejectsSame 覆盖「新旧相同」的拒绝。
//
// 若不拒绝，接口会返回成功却什么都没变 —— 用户以为换掉了泄露的口令，
// 实际上没有，这比直接报错危险得多。
func TestChangePasswordRejectsSame(t *testing.T) {
	svc := newAuthService(t, true)
	reg, err := svc.Register("alice", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	_, err = svc.ChangePassword(reg.User.ID, "password123", "password123")
	if !errors.Is(err, ErrSamePassword) {
		t.Errorf("新旧相同应返回 ErrSamePassword, 得到 %v", err)
	}
	// 失败时不得吊销会话
	if _, err := svc.Me(reg.Token); err != nil {
		t.Errorf("失败的改密不应吊销会话: %v", err)
	}
}

// TestChangePasswordWrongCurrentBeatsSamePassword 锁定检查**顺序**。
//
// 若先判「新旧相同」再校验当前口令，那么用户把当前口令**输错**
// 且新口令恰好等于那个错误输入时，会收到「新旧口令相同」——
// 而真正的问题是当前口令不对。该提示会把人引向错误的修正方向。
func TestChangePasswordWrongCurrentBeatsSamePassword(t *testing.T) {
	svc := newAuthService(t, true)
	reg, err := svc.Register("alice", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 当前口令错，且新口令与那个错误输入相同
	_, err = svc.ChangePassword(reg.User.ID, "typoedpassword", "typoedpassword")
	if !errors.Is(err, ErrWrongCurrentPassword) {
		t.Errorf("应优先报告「当前口令不正确」，得到 %v", err)
	}
}

// TestChangePasswordOnlyAffectsThatUser 确认不会误伤其他用户。
//
// 事务里的 DELETE 按 user_id 过滤，写错成无过滤条件会吊销**所有人**的会话 ——
// 这类错误在单用户测试里发现不了。
func TestChangePasswordOnlyAffectsThatUser(t *testing.T) {
	svc := newAuthService(t, true)
	alice, err := svc.Register("alice", "password123")
	if err != nil {
		t.Fatalf("注册 alice 失败: %v", err)
	}
	bob, err := svc.Register("bob", "password123")
	if err != nil {
		t.Fatalf("注册 bob 失败: %v", err)
	}

	if _, err := svc.ChangePassword(alice.User.ID, "password123", "alicepass456"); err != nil {
		t.Fatalf("alice 改密失败: %v", err)
	}

	// bob 的会话与口令都不应受影响
	if _, err := svc.Me(bob.Token); err != nil {
		t.Errorf("bob 的会话不应被 alice 改密影响: %v", err)
	}
	if _, _, err := svc.Login("bob", "password123"); err != nil {
		t.Errorf("bob 的口令不应被改动: %v", err)
	}
}

// TestChangePasswordCanBeRepeated 确认改密后可再次改密。
//
// 覆盖一个容易漏的路径：轮换会话后新令牌必须能通过 requireAuth，
// 否则用户只能改一次口令。
func TestChangePasswordCanBeRepeated(t *testing.T) {
	svc := newAuthService(t, true)
	reg, err := svc.Register("alice", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	second, err := svc.ChangePassword(reg.User.ID, "password123", "secondpass456")
	if err != nil {
		t.Fatalf("第一次改密失败: %v", err)
	}
	// 用返回的新令牌解析出用户，再改一次
	u, err := svc.Me(second)
	if err != nil {
		t.Fatalf("新令牌应可用: %v", err)
	}
	third, err := svc.ChangePassword(u.ID, "secondpass456", "thirdpass789")
	if err != nil {
		t.Fatalf("第二次改密失败: %v", err)
	}
	if _, err := svc.Me(third); err != nil {
		t.Errorf("第二次改密后的令牌应可用: %v", err)
	}
	if _, err := svc.Me(second); err == nil {
		t.Error("第二次改密应吊销第一次的令牌")
	}
	if _, _, err := svc.Login("alice", "thirdpass789"); err != nil {
		t.Errorf("最终口令应为 thirdpass789: %v", err)
	}
}

// TestChangePasswordForMissingUser 覆盖用户不存在时的行为。
//
// 正常路径不可达（用户来自有效会话），但 store 层必须报错而非静默成功 ——
// 静默成功会让客户端显示「口令已更改」而实际什么都没发生。
func TestChangePasswordForMissingUser(t *testing.T) {
	svc := newAuthService(t, true)
	if _, err := svc.ChangePassword(99999, "whatever123", "newpassword456"); err == nil {
		t.Error("用户不存在时应报错，而不是静默成功")
	}
}

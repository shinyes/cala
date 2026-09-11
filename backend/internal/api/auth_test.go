package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/config"
	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
	"github.com/shinyes/cala/backend/internal/version"
)

// newTestApp 构建一个使用临时数据库的完整路由。
func newTestApp(t *testing.T, regOpen bool) *fiber.App {
	app, _ := newTestAppWithStore(t, regOpen)
	return app
}

// newTestAppWithStore 同上，但一并返回存储层，便于直接查库断言。
func newTestAppWithStore(t *testing.T, regOpen bool) (*fiber.App, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := config.Config{SessionTTL: time.Hour, RegistrationOpenDefault: regOpen}
	authSvc := service.NewAuthService(st, cfg.SessionTTL, regOpen)
	projectSvc := service.NewProjectService(st)
	roundSvc := service.NewRoundService(st, projectSvc)
	return NewRouter(Deps{
		Config: cfg,
		Store:  st,
		Handlers: &Handlers{
			Auth:     authSvc,
			Projects: projectSvc,
			Rounds:   roundSvc,
		},
	}), st
}

// do 发起一次请求，返回状态码与解析后的 JSON 体（体为空时返回 nil）。
func do(t *testing.T, app *fiber.App, method, path, body, token string) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("请求 %s %s 失败: %v", method, path, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("读取响应体失败: %v", err)
	}
	if len(raw) == 0 {
		return res.StatusCode, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("响应不是合法 JSON: %v\n原文: %s", err, raw)
	}
	return res.StatusCode, out
}

// errCode 从统一错误体中取出 error.code。
func errCode(t *testing.T, body map[string]any) string {
	t.Helper()
	e, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 error 对象: %#v", body)
	}
	c, _ := e["code"].(string)
	return c
}

// register 注册一个用户并返回其 token。
func register(t *testing.T, app *fiber.App, username, password string) string {
	t.Helper()
	status, body := do(t, app, http.MethodPost, "/api/auth/register",
		`{"username":"`+username+`","password":"`+password+`"}`, "")
	if status != http.StatusCreated {
		t.Fatalf("注册 %s 应返回 201, 得到 %d: %#v", username, status, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatalf("注册 %s 未返回 token", username)
	}
	return token
}

// TestHealthz 保持 P0.4 的探活契约。
func TestHealthz(t *testing.T) {
	app := newTestApp(t, true)
	status, body := do(t, app, http.MethodGet, "/api/healthz", "", "")
	if status != http.StatusOK {
		t.Fatalf("健康检查应返回 200, 得到 %d", status)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v, 期望 ok", body["status"])
	}
}

// TestHealthzReportsVersion 确认探活端点自报版本号。
//
// 为何要测：版本号是经链接器 -X 注入的，**注入失败是静默的** ——
// 二进制照常构建、照常运行，只是自报 "dev"。
// 单元测试只能覆盖「字段存在且等于 version.Version」，
// 真正的端到端一致性（注入值 == tag）由 release.yml 的 smoke job 断言。
//
// 这里同时钉住一条契约：字段必须叫 version 且为字符串。
// 客户端与 smoke job 都按此名读取。
func TestHealthzReportsVersion(t *testing.T) {
	app := newTestApp(t, true)
	status, body := do(t, app, http.MethodGet, "/api/healthz", "", "")
	if status != http.StatusOK {
		t.Fatalf("健康检查应返回 200, 得到 %d", status)
	}

	got, ok := body["version"]
	if !ok {
		t.Fatal("healthz 响应缺少 version 字段")
	}
	s, ok := got.(string)
	if !ok {
		t.Fatalf("version 应为字符串, 得到 %T", got)
	}
	if s == "" {
		t.Error("version 不应为空（未注入时应为 dev，而不是空串）")
	}
	if s != version.Version {
		t.Errorf("version = %q, 期望 %q（应与 version.Version 一致）", s, version.Version)
	}
}

// TestVersionDefaultIsDev 锁定未注入时的默认值。
//
// 默认值必须是 "dev"：本地 go build 出来的二进制应当自报「开发构建」，
// 既不伪装成某个发布版本，也不显示为空白。
// 若有人把默认值改成空串或改成某个具体版本号，这条测试会失败。
func TestVersionDefaultIsDev(t *testing.T) {
	if version.Version != "dev" {
		t.Errorf("测试构建下 version.Version = %q, 期望 dev"+
			"（若为具体版本号，说明测试二进制被注入了版本，本断言需相应调整）", version.Version)
	}
}

// TestRegisterFirstUserIsAdminEvenWhenClosed 覆盖验收标准 A1 与基线 §4.2(2)。
func TestRegisterFirstUserIsAdminEvenWhenClosed(t *testing.T) {
	app := newTestApp(t, false) // 注册开关为 false

	status, body := do(t, app, http.MethodPost, "/api/auth/register",
		`{"username":"firstuser","password":"password123"}`, "")
	if status != http.StatusCreated {
		t.Fatalf("开关关闭时首个用户仍应可注册, 得到 %d: %#v", status, body)
	}
	if became, _ := body["becameAdmin"].(bool); !became {
		t.Error("首个用户必须标记 becameAdmin=true")
	}
	u, _ := body["user"].(map[string]any)
	if isAdmin, _ := u["isAdmin"].(bool); !isAdmin {
		t.Error("首个用户的 isAdmin 必须为 true")
	}

	// 第二个人必须被拒
	status, body = do(t, app, http.MethodPost, "/api/auth/register",
		`{"username":"seconduser","password":"password123"}`, "")
	if status != http.StatusForbidden {
		t.Fatalf("开关关闭时第二个用户应返回 403, 得到 %d", status)
	}
	if code := errCode(t, body); code != CodeRegistrationClosed {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeRegistrationClosed)
	}
}

func TestRegisterDuplicateUsernameConflicts(t *testing.T) {
	app := newTestApp(t, true)
	register(t, app, "alice", "password123")

	status, body := do(t, app, http.MethodPost, "/api/auth/register",
		`{"username":"alice","password":"password123"}`, "")
	if status != http.StatusConflict {
		t.Fatalf("重复用户名应返回 409, 得到 %d", status)
	}
	if code := errCode(t, body); code != CodeConflict {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeConflict)
	}
}

func TestLoginAndMeAndLogout(t *testing.T) {
	app := newTestApp(t, true)
	register(t, app, "alice", "password123")

	// 正确凭证
	status, body := do(t, app, http.MethodPost, "/api/auth/login",
		`{"username":"alice","password":"password123"}`, "")
	if status != http.StatusOK {
		t.Fatalf("登录应返回 200, 得到 %d: %#v", status, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("登录未返回 token")
	}

	// 错误口令
	status, body = do(t, app, http.MethodPost, "/api/auth/login",
		`{"username":"alice","password":"wrongpassword"}`, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("错误口令应返回 401, 得到 %d", status)
	}
	if code := errCode(t, body); code != CodeUnauthorized {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeUnauthorized)
	}

	// /api/me：带 token 与不带 token
	if status, body = do(t, app, http.MethodGet, "/api/me", "", token); status != http.StatusOK {
		t.Errorf("带 token 访问 /api/me 应返回 200, 得到 %d: %#v", status, body)
	}
	if status, _ = do(t, app, http.MethodGet, "/api/me", "", ""); status != http.StatusUnauthorized {
		t.Errorf("不带 token 访问 /api/me 应返回 401, 得到 %d", status)
	}

	// 伪造 token
	if status, _ = do(t, app, http.MethodGet, "/api/me", "", "forged-token"); status != http.StatusUnauthorized {
		t.Errorf("伪造 token 应返回 401, 得到 %d", status)
	}

	// 登出后原 token 失效
	if status, _ = do(t, app, http.MethodPost, "/api/auth/logout", "", token); status != http.StatusNoContent {
		t.Fatalf("登出应返回 204, 得到 %d", status)
	}
	if status, _ = do(t, app, http.MethodGet, "/api/me", "", token); status != http.StatusUnauthorized {
		t.Errorf("登出后 /api/me 应返回 401, 得到 %d", status)
	}
}

func TestRegisterValidationOverHTTP(t *testing.T) {
	app := newTestApp(t, true)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"过短用户名", `{"username":"ab","password":"password123"}`, http.StatusBadRequest},
		{"非法字符", `{"username":"bad name","password":"password123"}`, http.StatusBadRequest},
		{"过短口令", `{"username":"gooduser","password":"short"}`, http.StatusBadRequest},
		{"非法 JSON", `{not json`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := do(t, app, http.MethodPost, "/api/auth/register", tc.body, "")
			if status != tc.want {
				t.Errorf("状态码 = %d, 期望 %d: %#v", status, tc.want, body)
			}
			if code := errCode(t, body); code != CodeBadRequest {
				t.Errorf("错误码 = %q, 期望 %q", code, CodeBadRequest)
			}
		})
	}
}

func TestAdminSettingsRequiresAdmin(t *testing.T) {
	app := newTestApp(t, true)
	adminToken := register(t, app, "admin", "password123") // 首个用户 = 管理员
	userToken := register(t, app, "bob", "password123")    // 第二个 = 普通用户

	// 普通用户被拒
	status, body := do(t, app, http.MethodPut, "/api/admin/settings", `{"open":false}`, userToken)
	if status != http.StatusForbidden {
		t.Fatalf("普通用户修改设置应返回 403, 得到 %d: %#v", status, body)
	}
	if code := errCode(t, body); code != CodeForbidden {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeForbidden)
	}

	// 未登录被拒
	if status, _ = do(t, app, http.MethodPut, "/api/admin/settings", `{"open":false}`, ""); status != http.StatusUnauthorized {
		t.Errorf("未登录修改设置应返回 401, 得到 %d", status)
	}

	// 管理员可改
	if status, body = do(t, app, http.MethodPut, "/api/admin/settings", `{"open":false}`, adminToken); status != http.StatusOK {
		t.Fatalf("管理员修改设置应返回 200, 得到 %d: %#v", status, body)
	}

	// 公开配置反映新值
	status, body = do(t, app, http.MethodGet, "/api/settings/public", "", "")
	if status != http.StatusOK {
		t.Fatalf("公开配置应返回 200, 得到 %d", status)
	}
	if open, _ := body["registrationOpen"].(bool); open {
		t.Error("管理员写入 false 后 registrationOpen 应为 false")
	}

	// 关闭后新用户注册被拒（且此时已有用户，故不再走 bootstrap 分支）
	status, body = do(t, app, http.MethodPost, "/api/auth/register",
		`{"username":"carol","password":"password123"}`, "")
	if status != http.StatusForbidden {
		t.Fatalf("关闭注册后新用户应返回 403, 得到 %d", status)
	}
	if code := errCode(t, body); code != CodeRegistrationClosed {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeRegistrationClosed)
	}
}

// TestMeDoesNotLeakPasswordHash 是安全断言：任何响应都不得包含口令哈希。
func TestMeDoesNotLeakPasswordHash(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	body := string(raw)
	if strings.Contains(body, "password") {
		t.Errorf("/api/me 响应中出现了 password 字段: %s", body)
	}
	if strings.Contains(body, "$2a$") || strings.Contains(body, "$2b$") {
		t.Errorf("/api/me 响应中疑似出现 bcrypt 哈希: %s", body)
	}
}

// TestPublicSettingsDoesNotRequireAuth 保证未登录也能读到注册开关，
// 否则登录页无法决定是否显示注册入口。
func TestPublicSettingsDoesNotRequireAuth(t *testing.T) {
	app := newTestApp(t, true)
	status, body := do(t, app, http.MethodGet, "/api/settings/public", "", "")
	if status != http.StatusOK {
		t.Fatalf("公开配置应无需登录即可访问, 得到 %d", status)
	}
	if _, ok := body["registrationOpen"].(bool); !ok {
		t.Errorf("响应缺少 registrationOpen 布尔字段: %#v", body)
	}
}

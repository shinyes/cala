package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/store"
)

const apiValidRule = `
function generate(cfg) {
  const lo = cfg.min || 1, hi = cfg.max || 9;
  const a = lo + Math.floor(Math.random() * (hi - lo + 1));
  const b = lo + Math.floor(Math.random() * (hi - lo + 1));
  return { q: a + " + " + b + " = ?", a: String(a + b) };
}`

func projectBody(title string) string {
	b, _ := json.Marshal(map[string]any{
		"title":         title,
		"description":   "两位数加法",
		"questionCount": 4,
		"cfgJson":       `{"min":10,"max":99}`,
		"ruleSource":    apiValidRule,
	})
	return string(b)
}

// createProject 建一个项目并返回其 ID。
func createProject(t *testing.T, app *fiber.App, token, title string) int64 {
	t.Helper()
	status, body := do(t, app, http.MethodPost, "/api/projects", projectBody(title), token)
	if status != http.StatusCreated {
		t.Fatalf("创建项目应返回 201, 得到 %d: %#v", status, body)
	}
	p, _ := body["project"].(map[string]any)
	id, _ := p["id"].(float64)
	if id == 0 {
		t.Fatalf("创建项目未返回 id: %#v", body)
	}
	return int64(id)
}

// userID 按用户名取出用户 ID。
// 不硬编码 ID：硬编码会把测试绑死在「注册顺序恰好如此」上。
func userID(t *testing.T, st *store.Store, username string) int64 {
	t.Helper()
	u, _, err := st.AuthenticateLookup(username)
	if err != nil {
		t.Fatalf("查询用户 %s 失败: %v", username, err)
	}
	return u.ID
}

// ---------------------------------------------------------------------------
// 项目 CRUD
// ---------------------------------------------------------------------------

func TestProjectRequiresAuth(t *testing.T) {
	app := newTestApp(t, true)

	cases := []struct{ method, path, body string }{
		{http.MethodGet, "/api/projects", ""},
		{http.MethodPost, "/api/projects", projectBody("x")},
		{http.MethodGet, "/api/projects/1", ""},
		{http.MethodPut, "/api/projects/1", projectBody("x")},
		{http.MethodDelete, "/api/projects/1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			status, _ := do(t, app, tc.method, tc.path, tc.body, "")
			if status != http.StatusUnauthorized {
				t.Errorf("未登录应返回 401, 得到 %d", status)
			}
		})
	}
}

// TestCreateProjectRejectsBadRuleAsRuleInvalid 确认规则错误映射为 rule_invalid
// 且带上可读原因——作者需要知道错在哪，而不是收到笼统的「不合法」。
func TestCreateProjectRejectsBadRuleAsRuleInvalid(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")

	bad := map[string]string{
		"死循环":  `function generate(cfg){ while(true){} }`,
		"语法错误": `function generate( { return }`,
		"缺函数":  `var x = 1`,
	}
	for name, rule := range bad {
		t.Run(name, func(t *testing.T) {
			b, _ := json.Marshal(map[string]any{
				"title": "坏规则", "questionCount": 3,
				"cfgJson": `{}`, "ruleSource": rule,
			})
			status, body := do(t, app, http.MethodPost, "/api/projects", string(b), token)
			if status != http.StatusBadRequest {
				t.Fatalf("状态码 = %d, 期望 400: %#v", status, body)
			}
			if code := errCode(t, body); code != CodeRuleInvalid {
				t.Errorf("错误码 = %q, 期望 %q", code, CodeRuleInvalid)
			}
			e, _ := body["error"].(map[string]any)
			if msg, _ := e["message"].(string); msg == "" {
				t.Error("应返回可读的规则错误原因")
			}
		})
	}
}

// TestCreateProjectRejectsMalformedAnswer 覆盖 A11 的 HTTP 面。
func TestCreateProjectRejectsMalformedAnswer(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")

	b, _ := json.Marshal(map[string]any{
		"title": "坏答案", "questionCount": 3, "cfgJson": `{}`,
		"ruleSource": `function generate(cfg){ return {q:"1/0 = ?", a:"1/0"} }`,
	})
	status, body := do(t, app, http.MethodPost, "/api/projects", string(b), token)
	if status != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400: %#v", status, body)
	}
	e, _ := body["error"].(map[string]any)
	msg, _ := e["message"].(string)
	if msg == "" {
		t.Fatal("应返回可读原因")
	}
	t.Logf("实际错误信息: %s", msg)
}

func TestCreateAndGetProject(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")
	id := createProject(t, app, token, "我的口算")

	status, body := do(t, app, http.MethodGet, fmt.Sprintf("/api/projects/%d", id), "", token)
	if status != http.StatusOK {
		t.Fatalf("取项目应返回 200, 得到 %d: %#v", status, body)
	}
	p, _ := body["project"].(map[string]any)
	if p["title"] != "我的口算" {
		t.Errorf("标题 = %v", p["title"])
	}
	if p["access"] != store.AccessOwner {
		t.Errorf("access = %v, 期望 owner", p["access"])
	}
}

func TestListProjectsGroupsOwnedAndSubscribed(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	alice := register(t, app, "alice", "password123")
	bob := register(t, app, "bob", "password123")

	aliceProject := createProject(t, app, alice, "Alice 的项目")

	// Bob 订阅 Alice 的项目（P6 会有导入端点；此处直接建关系以测分组）
	if err := st.Subscribe(userID(t, st, "bob"), aliceProject); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}

	status, body := do(t, app, http.MethodGet, "/api/projects", "", bob)
	if status != http.StatusOK {
		t.Fatalf("列表应返回 200, 得到 %d", status)
	}
	owned, _ := body["owned"].([]any)
	subscribed, _ := body["subscribed"].([]any)
	if len(owned) != 0 {
		t.Errorf("bob 没有自己的项目, owned 应为空, 得到 %d", len(owned))
	}
	if len(subscribed) != 1 {
		t.Errorf("bob 应看到 1 个订阅项目, 得到 %d", len(subscribed))
	}
}

// TestSubscriberCannotModifyOrDelete 覆盖 D4：订阅者是纯只读跟随。
func TestSubscriberCannotModifyOrDelete(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	alice := register(t, app, "alice", "password123")
	bob := register(t, app, "bob", "password123")

	id := createProject(t, app, alice, "Alice 的项目")
	if err := st.Subscribe(userID(t, st, "bob"), id); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}

	t.Run("不能修改", func(t *testing.T) {
		status, body := do(t, app, http.MethodPut, fmt.Sprintf("/api/projects/%d", id),
			projectBody("被篡改"), bob)
		if status != http.StatusForbidden {
			t.Fatalf("订阅者修改应返回 403, 得到 %d: %#v", status, body)
		}
		if code := errCode(t, body); code != CodeForbidden {
			t.Errorf("错误码 = %q, 期望 %q", code, CodeForbidden)
		}
	})
	t.Run("不能删除", func(t *testing.T) {
		status, _ := do(t, app, http.MethodDelete, fmt.Sprintf("/api/projects/%d", id), "", bob)
		if status != http.StatusForbidden {
			t.Errorf("订阅者删除应返回 403, 得到 %d", status)
		}
	})
	t.Run("可以查看", func(t *testing.T) {
		status, body := do(t, app, http.MethodGet, fmt.Sprintf("/api/projects/%d", id), "", bob)
		if status != http.StatusOK {
			t.Fatalf("订阅者查看应返回 200, 得到 %d", status)
		}
		p, _ := body["project"].(map[string]any)
		if p["access"] != store.AccessSubscriber {
			t.Errorf("access = %v, 期望 subscriber", p["access"])
		}
	})
}

func TestStrangerCannotSeeProject(t *testing.T) {
	app := newTestApp(t, true)
	alice := register(t, app, "alice", "password123")
	_ = register(t, app, "stranger", "password123")
	id := createProject(t, app, alice, "私有项目")

	// 用 stranger 的 token 访问
	_, body := do(t, app, http.MethodPost, "/api/auth/login",
		`{"username":"stranger","password":"password123"}`, "")
	strangerToken, _ := body["token"].(string)
	if strangerToken == "" {
		t.Fatal("登录失败")
	}

	status, _ := do(t, app, http.MethodGet, fmt.Sprintf("/api/projects/%d", id), "", strangerToken)
	if status != http.StatusForbidden {
		t.Errorf("无关用户应返回 403, 得到 %d", status)
	}
}

func TestUpdateProjectByOwner(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")
	id := createProject(t, app, token, "旧标题")

	status, body := do(t, app, http.MethodPut, fmt.Sprintf("/api/projects/%d", id),
		projectBody("新标题"), token)
	if status != http.StatusOK {
		t.Fatalf("作者修改应返回 200, 得到 %d: %#v", status, body)
	}
	p, _ := body["project"].(map[string]any)
	if p["title"] != "新标题" {
		t.Errorf("标题 = %v, 期望 新标题", p["title"])
	}
}

func TestToleranceMustComeAsPairOverHTTP(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")

	mk := func(num, den any) string {
		m := map[string]any{
			"title": "容差", "questionCount": 3, "cfgJson": `{}`, "ruleSource": apiValidRule,
		}
		if num != nil {
			m["toleranceNum"] = num
		}
		if den != nil {
			m["toleranceDen"] = den
		}
		b, _ := json.Marshal(m)
		return string(b)
	}

	cases := []struct {
		name string
		body string
		want int
	}{
		{"只给分子", mk(1, nil), http.StatusBadRequest},
		{"只给分母", mk(nil, 100), http.StatusBadRequest},
		{"分母为零", mk(1, 0), http.StatusBadRequest},
		{"分子为负", mk(-1, 100), http.StatusBadRequest},
		{"合法成对", mk(1, 100), http.StatusCreated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := do(t, app, http.MethodPost, "/api/projects", tc.body, token)
			if status != tc.want {
				t.Errorf("状态码 = %d, 期望 %d: %#v", status, tc.want, body)
			}
		})
	}
}

// TestDeleteProjectCascadesOverHTTP 覆盖功能8 的 HTTP 面。
func TestDeleteProjectCascadesOverHTTP(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	token := register(t, app, "alice", "password123")
	id := createProject(t, app, token, "要删除的项目")

	// 走一遍 start + complete，产生真实记录
	round := startAndComplete(t, app, token, id, true)

	counts := func() (int, int) {
		var rounds, attempts int
		st.DB().QueryRow(`SELECT COUNT(*) FROM practice_round`).Scan(&rounds)
		st.DB().QueryRow(`SELECT COUNT(*) FROM attempt`).Scan(&attempts)
		return rounds, attempts
	}
	if r, a := counts(); r == 0 || a == 0 {
		t.Fatalf("应有记录, 得到 rounds=%d attempts=%d (roundID=%d)", r, a, round)
	}

	status, _ := do(t, app, http.MethodDelete, fmt.Sprintf("/api/projects/%d", id), "", token)
	if status != http.StatusNoContent {
		t.Fatalf("删除应返回 204, 得到 %d", status)
	}

	r, a := counts()
	if r != 0 || a != 0 {
		t.Errorf("级联删除不完整: rounds=%d attempts=%d, 期望全为 0", r, a)
	}
}

func TestGetMissingProjectReturns404(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")

	status, body := do(t, app, http.MethodGet, "/api/projects/999999", "", token)
	if status != http.StatusNotFound {
		t.Fatalf("不存在项目应返回 404, 得到 %d", status)
	}
	if code := errCode(t, body); code != CodeNotFound {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeNotFound)
	}
}

func TestBadProjectIDReturns400(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")

	for _, bad := range []string{"abc", "0", "-1"} {
		status, _ := do(t, app, http.MethodGet, "/api/projects/"+bad, "", token)
		if status != http.StatusBadRequest {
			t.Errorf("项目 ID %q 应返回 400, 得到 %d", bad, status)
		}
	}
}

// ---------------------------------------------------------------------------
// 轮次
// ---------------------------------------------------------------------------

// startAndComplete 走完一轮并返回 roundID。
func startAndComplete(t *testing.T, app *fiber.App, token string, projectID int64, answerCorrectly bool) int64 {
	t.Helper()
	startedAt := time.Now().UTC().Format(time.RFC3339)

	status, body := do(t, app, http.MethodPost, "/api/rounds/start",
		fmt.Sprintf(`{"projectId":%d}`, projectID), token)
	if status != http.StatusOK {
		t.Fatalf("start 应返回 200, 得到 %d: %#v", status, body)
	}
	seed, _ := body["seed"].(float64)
	questions, _ := body["questions"].([]any)
	if len(questions) == 0 {
		t.Fatal("start 未返回题目")
	}

	attempts := make([]map[string]any, 0, len(questions))
	for i, raw := range questions {
		q, _ := raw.(map[string]any)
		answer, _ := q["a"].(string)
		input := answer
		if !answerCorrectly {
			input = "999999"
		}
		attempts = append(attempts, map[string]any{
			"idx": i, "input": input, "clientIsCorrect": answerCorrectly, "elapsedMs": 500,
		})
	}

	reqBody, _ := json.Marshal(map[string]any{
		"projectId": projectID, "seed": int64(seed),
		"startedAt": startedAt, "finishedAt": time.Now().UTC().Format(time.RFC3339),
		"attempts": attempts,
	})
	status, body = do(t, app, http.MethodPost, "/api/rounds/complete", string(reqBody), token)
	if status != http.StatusCreated {
		t.Fatalf("complete 应返回 201, 得到 %d: %#v", status, body)
	}
	id, _ := body["roundId"].(float64)
	return int64(id)
}

func TestRoundStartAndComplete(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")
	id := createProject(t, app, token, "口算")

	status, body := do(t, app, http.MethodPost, "/api/rounds/start",
		fmt.Sprintf(`{"projectId":%d}`, id), token)
	if status != http.StatusOK {
		t.Fatalf("start 应返回 200, 得到 %d: %#v", status, body)
	}
	if body["seed"] == nil {
		t.Error("start 应返回 seed")
	}
	scoring, _ := body["scoring"].(map[string]any)
	if scoring == nil {
		t.Fatal("start 应返回 scoring 配置（清洗表由服务端下发）")
	}
	table, _ := scoring["cleanupTable"].(map[string]any)
	if len(table) == 0 {
		t.Error("scoring.cleanupTable 不应为空")
	}
	questions, _ := body["questions"].([]any)
	for i, raw := range questions {
		q, _ := raw.(map[string]any)
		if _, ok := q["envelope"].(map[string]any); !ok {
			t.Errorf("第 %d 题缺少 envelope（客户端判分需要它）", i)
		}
	}
}

// TestRoundCompleteReportsDiscrepancy 覆盖 A14 的 HTTP 面。
func TestRoundCompleteReportsDiscrepancy(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")
	id := createProject(t, app, token, "口算")

	// 全错却声称全对
	startAndComplete(t, app, token, id, false)

	status, body := do(t, app, http.MethodPost, "/api/rounds/start",
		fmt.Sprintf(`{"projectId":%d}`, id), token)
	if status != http.StatusOK {
		t.Fatalf("start 失败: %d", status)
	}
	seed, _ := body["seed"].(float64)
	questions, _ := body["questions"].([]any)

	attempts := make([]map[string]any, 0, len(questions))
	for i := range questions {
		attempts = append(attempts, map[string]any{
			"idx": i, "input": "999999", "clientIsCorrect": true, "elapsedMs": 100,
		})
	}
	reqBody, _ := json.Marshal(map[string]any{
		"projectId": id, "seed": int64(seed),
		"startedAt":  time.Now().UTC().Format(time.RFC3339),
		"finishedAt": time.Now().UTC().Format(time.RFC3339),
		"attempts":   attempts,
	})
	status, body = do(t, app, http.MethodPost, "/api/rounds/complete", string(reqBody), token)
	if status != http.StatusCreated {
		t.Fatalf("complete 应返回 201, 得到 %d: %#v", status, body)
	}

	disc, _ := body["discrepancies"].(float64)
	if int(disc) != len(questions) {
		t.Errorf("分歧数 = %v, 期望 %d", disc, len(questions))
	}
	correct, _ := body["correctCount"].(float64)
	if correct != 0 {
		t.Errorf("服务端重算应得 0 分, 得到 %v", correct)
	}
}

func TestRoundRequiresAccess(t *testing.T) {
	app := newTestApp(t, true)
	alice := register(t, app, "alice", "password123")
	_ = register(t, app, "bob", "password123")
	id := createProject(t, app, alice, "私有")

	_, body := do(t, app, http.MethodPost, "/api/auth/login",
		`{"username":"bob","password":"password123"}`, "")
	bobToken, _ := body["token"].(string)

	status, _ := do(t, app, http.MethodPost, "/api/rounds/start",
		fmt.Sprintf(`{"projectId":%d}`, id), bobToken)
	if status != http.StatusForbidden {
		t.Errorf("无关用户 start 应返回 403, 得到 %d", status)
	}
}

func TestRoundStartMissingProjectID(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")

	status, body := do(t, app, http.MethodPost, "/api/rounds/start", `{}`, token)
	if status != http.StatusBadRequest {
		t.Fatalf("缺少 projectId 应返回 400, 得到 %d", status)
	}
	if code := errCode(t, body); code != CodeBadRequest {
		t.Errorf("错误码 = %q, 期望 %q", code, CodeBadRequest)
	}
}

package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/store"
)

// statsBody 取统计并返回解析后的响应。
func statsBody(t *testing.T, app *fiber.App, token string, projectID int64, query string) map[string]any {
	t.Helper()
	path := fmt.Sprintf("/api/projects/%d/stats", projectID)
	if query != "" {
		path += "?" + query
	}
	status, body := do(t, app, http.MethodGet, path, "", token)
	if status != http.StatusOK {
		t.Fatalf("统计应返回 200, 得到 %d: %#v", status, body)
	}
	return body
}

func TestStatsRequiresAuthAndAccess(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	aliceToken := register(t, app, "alice", "password123")
	_ = register(t, app, "stranger", "password123")
	projectID := createProject(t, app, aliceToken, "口算")

	t.Run("未登录", func(t *testing.T) {
		status, _ := do(t, app, http.MethodGet,
			fmt.Sprintf("/api/projects/%d/stats", projectID), "", "")
		if status != http.StatusUnauthorized {
			t.Errorf("应返回 401, 得到 %d", status)
		}
	})

	t.Run("无关用户", func(t *testing.T) {
		_, body := do(t, app, http.MethodPost, "/api/auth/login",
			`{"username":"stranger","password":"password123"}`, "")
		strangerToken, _ := body["token"].(string)

		status, _ := do(t, app, http.MethodGet,
			fmt.Sprintf("/api/projects/%d/stats", projectID), "", strangerToken)
		if status != http.StatusForbidden {
			t.Errorf("应返回 403, 得到 %d", status)
		}
	})

	t.Run("项目不存在", func(t *testing.T) {
		status, _ := do(t, app, http.MethodGet,
			"/api/projects/999999/stats", "", aliceToken)
		if status != http.StatusNotFound {
			t.Errorf("应返回 404, 得到 %d", status)
		}
	})

	t.Run("订阅者可看自己的统计", func(t *testing.T) {
		bobToken := register(t, app, "bob", "password123")
		if err := st.Subscribe(userID(t, st, "bob"), projectID); err != nil {
			t.Fatalf("Subscribe 失败: %v", err)
		}
		status, _ := do(t, app, http.MethodGet,
			fmt.Sprintf("/api/projects/%d/stats", projectID), "", bobToken)
		if status != http.StatusOK {
			t.Errorf("订阅者应能查看自己的统计, 得到 %d", status)
		}
	})
}

func TestStatsEmptyProject(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")
	projectID := createProject(t, app, token, "还没练过")

	body := statsBody(t, app, token, projectID, "")

	overall, _ := body["overall"].(map[string]any)
	if overall["roundCount"].(float64) != 0 {
		t.Errorf("无练习记录时 roundCount 应为 0, 得到 %v", overall["roundCount"])
	}
	// 必须是空数组而非 null
	buckets, ok := body["buckets"].([]any)
	if !ok {
		t.Fatalf("buckets 应为数组而非 null: %#v", body["buckets"])
	}
	if len(buckets) != 0 {
		t.Errorf("应为空数组, 得到 %d 项", len(buckets))
	}
}

func TestStatsGrainValidation(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")
	projectID := createProject(t, app, token, "口算")

	t.Run("三种合法粒度", func(t *testing.T) {
		for _, g := range []string{"day", "week", "month"} {
			body := statsBody(t, app, token, projectID, "grain="+g)
			if body["grain"] != g {
				t.Errorf("grain 应回显 %q, 得到 %v", g, body["grain"])
			}
		}
	})

	t.Run("缺省为 day", func(t *testing.T) {
		body := statsBody(t, app, token, projectID, "")
		if body["grain"] != "day" {
			t.Errorf("缺省 grain 应为 day, 得到 %v", body["grain"])
		}
	})

	t.Run("非法粒度", func(t *testing.T) {
		status, body := do(t, app, http.MethodGet,
			fmt.Sprintf("/api/projects/%d/stats?grain=year", projectID), "", token)
		if status != http.StatusBadRequest {
			t.Fatalf("应返回 400, 得到 %d", status)
		}
		if code := errCode(t, body); code != CodeBadRequest {
			t.Errorf("错误码 = %q", code)
		}
	})
}

func TestStatsTZOffsetValidation(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")
	projectID := createProject(t, app, token, "口算")

	t.Run("合法偏移", func(t *testing.T) {
		for _, v := range []string{"0", "480", "-300", "840", "-840"} {
			statsBody(t, app, token, projectID, "tzOffsetMinutes="+v)
		}
	})

	t.Run("越界与非法", func(t *testing.T) {
		for _, v := range []string{"841", "-841", "9999", "abc", "1.5"} {
			status, _ := do(t, app, http.MethodGet,
				fmt.Sprintf("/api/projects/%d/stats?tzOffsetMinutes=%s", projectID, v), "", token)
			if status != http.StatusBadRequest {
				t.Errorf("tzOffsetMinutes=%s 应返回 400, 得到 %d", v, status)
			}
		}
	})
}

// TestStatsComputesMetrics 用真实落库的轮次核对 8 项指标。
func TestStatsComputesMetrics(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	token := register(t, app, "alice", "password123")
	projectID := createProject(t, app, token, "口算")

	// 直接写库，精确控制耗时与正确数（走 HTTP 的话题数受项目设置约束）
	owner := userID(t, st, "alice")
	seedRounds(t, st, projectID, owner, []simpleRound{
		{1000, 4, 4, "2026-09-10T10:00:00Z"}, // 1.0
		{4000, 1, 4, "2026-09-10T11:00:00Z"}, // 0.25
		{2000, 2, 4, "2026-09-10T12:00:00Z"}, // 0.5
		{3000, 3, 4, "2026-09-10T13:00:00Z"}, // 0.75
	})

	body := statsBody(t, app, token, projectID, "grain=day")
	overall, _ := body["overall"].(map[string]any)

	if got := overall["roundCount"].(float64); got != 4 {
		t.Fatalf("roundCount = %v, 期望 4", got)
	}
	// 耗时：1000/2000/3000/4000 -> min 1000, max 4000, avg 2500, median 2500
	checks := map[string]float64{
		"timeMinMs":      1000,
		"timeMaxMs":      4000,
		"timeAvgMs":      2500,
		"timeMedianMs":   2500,
		"accuracyMin":    0.25,
		"accuracyMax":    1.0,
		"accuracyAvg":    0.625,
		"accuracyMedian": 0.625,
	}
	for k, want := range checks {
		got, _ := overall[k].(float64)
		if diff := got - want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("overall.%s = %v, 期望 %v", k, got, want)
		}
	}

	// 同一天的 4 轮应聚合到 1 个桶
	buckets, _ := body["buckets"].([]any)
	if len(buckets) != 1 {
		t.Fatalf("同一天的 4 轮应聚合为 1 个桶, 得到 %d", len(buckets))
	}
	b0, _ := buckets[0].(map[string]any)
	if b0["key"] != "2026-09-10" {
		t.Errorf("桶键 = %v", b0["key"])
	}
	bm, _ := b0["metrics"].(map[string]any)
	if bm["roundCount"].(float64) != 4 {
		t.Errorf("桶内 roundCount = %v", bm["roundCount"])
	}
}

// TestStatsBucketsByGrain 核对三种粒度的分桶结果。
func TestStatsBucketsByGrain(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	token := register(t, app, "alice", "password123")
	projectID := createProject(t, app, token, "口算")
	owner := userID(t, st, "alice")

	// 2026-09-07 周一、09-09 周三、09-16 周三（跨周同星期几）、10-05 另一月
	seedRounds(t, st, projectID, owner, []simpleRound{
		{1000, 1, 1, "2026-09-07T10:00:00Z"},
		{1000, 1, 1, "2026-09-09T10:00:00Z"},
		{1000, 1, 1, "2026-09-16T10:00:00Z"},
		{1000, 1, 1, "2026-10-05T10:00:00Z"},
	})

	t.Run("day: 4 个自然日", func(t *testing.T) {
		body := statsBody(t, app, token, projectID, "grain=day")
		buckets, _ := body["buckets"].([]any)
		if len(buckets) != 4 {
			t.Errorf("应有 4 个桶, 得到 %d", len(buckets))
		}
	})

	t.Run("week: 周一与周三共 2 个桶", func(t *testing.T) {
		body := statsBody(t, app, token, projectID, "grain=week")
		buckets, _ := body["buckets"].([]any)
		if len(buckets) != 2 {
			t.Fatalf("应有 2 个桶（周一、周三）, 得到 %d", len(buckets))
		}
		b0, _ := buckets[0].(map[string]any)
		b1, _ := buckets[1].(map[string]any)
		if b0["label"] != "周一" || b1["label"] != "周三" {
			t.Errorf("顺序或标签不对: %v, %v", b0["label"], b1["label"])
		}
		// 两个周三应聚合到一起
		m1, _ := b1["metrics"].(map[string]any)
		if m1["roundCount"].(float64) != 2 {
			t.Errorf("两个周三应聚合为 2 轮, 得到 %v", m1["roundCount"])
		}
	})

	t.Run("month: 2 个月", func(t *testing.T) {
		body := statsBody(t, app, token, projectID, "grain=month")
		buckets, _ := body["buckets"].([]any)
		if len(buckets) != 2 {
			t.Fatalf("应有 2 个月, 得到 %d", len(buckets))
		}
		b0, _ := buckets[0].(map[string]any)
		b1, _ := buckets[1].(map[string]any)
		if b0["key"] != "2026-09" || b1["key"] != "2026-10" {
			t.Errorf("月份桶不对: %v, %v", b0["key"], b1["key"])
		}
	})
}

// TestStatsIsPrivate 是隐私边界断言：
// 作者看不到订阅者的统计，订阅者也看不到作者的。
//
// 答题记录属于做题的人。统计由这些记录派生，因此同样属于个人数据。
func TestStatsIsPrivate(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")

	projectID := createProject(t, app, aliceToken, "共享项目")
	if err := st.Subscribe(userID(t, st, "bob"), projectID); err != nil {
		t.Fatalf("Subscribe 失败: %v", err)
	}

	// Alice 做 2 轮，Bob 做 3 轮
	seedSimpleRounds(t, st, projectID, userID(t, st, "alice"), 2, "2026-09-10T10:00:00Z")
	seedSimpleRounds(t, st, projectID, userID(t, st, "bob"), 3, "2026-09-10T11:00:00Z")

	aliceStats := statsBody(t, app, aliceToken, projectID, "grain=day")
	bobStats := statsBody(t, app, bobToken, projectID, "grain=day")

	ao, _ := aliceStats["overall"].(map[string]any)
	bo, _ := bobStats["overall"].(map[string]any)

	if got := ao["roundCount"].(float64); got != 2 {
		t.Errorf("作者的统计应只含自己的 2 轮, 得到 %v", got)
	}
	if got := bo["roundCount"].(float64); got != 3 {
		t.Errorf("订阅者的统计应只含自己的 3 轮, 得到 %v", got)
	}
}

// TestStatsDisappearWithProject 验证「统计是派生态」：
// 删项目后统计随之消失（无物化聚合表需要单独清理）。
func TestStatsDisappearWithProject(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	token := register(t, app, "alice", "password123")
	projectID := createProject(t, app, token, "口算")

	seedSimpleRounds(t, st, projectID, userID(t, st, "alice"), 3, "2026-09-10T10:00:00Z")

	body := statsBody(t, app, token, projectID, "grain=day")
	overall, _ := body["overall"].(map[string]any)
	if overall["roundCount"].(float64) != 3 {
		t.Fatalf("删除前应有 3 轮, 得到 %v", overall["roundCount"])
	}

	if status, _ := do(t, app, http.MethodDelete,
		fmt.Sprintf("/api/projects/%d", projectID), "", token); status != http.StatusNoContent {
		t.Fatalf("删除项目失败: %d", status)
	}

	status, _ := do(t, app, http.MethodGet,
		fmt.Sprintf("/api/projects/%d/stats", projectID), "", token)
	if status != http.StatusNotFound {
		t.Errorf("删除后统计端点应返回 404（项目已不存在）, 得到 %d", status)
	}
}

// simpleRound 是 seedRounds 的输入。
type simpleRound struct {
	totalMs int64
	correct int
	total   int
	when    string
}

// seedRounds 直接写入若干轮记录，精确控制耗时与正确数。
//
// 不走 HTTP 是因为经过练习流程的话题数受项目设置约束，
// 而统计的正确性需要能自由构造边界数据（例如 1/4 与 4/4）。
func seedRounds(t *testing.T, st *store.Store, projectID, userID int64, rows []simpleRound) {
	t.Helper()
	for i, r := range rows {
		when := r.when
		if when == "" {
			when = time.Now().UTC().Format(time.RFC3339)
		}
		if _, err := st.CreateRound(store.NewRound{
			ProjectID:     projectID,
			UserID:        userID,
			Seed:          int64(i + 1),
			StartedAt:     when,
			FinishedAt:    when,
			TotalMs:       r.totalMs,
			QuestionCount: r.total,
			CorrectCount:  r.correct,
			Attempts: []store.NewAttempt{{
				Index:           0,
				QSnapshot:       "1+1",
				ASnapshot:       "2",
				EnvelopeJSON:    `{"kind":"rational","num":"2","den":"1"}`,
				UserInput:       "2",
				ClientIsCorrect: true,
				ServerIsCorrect: true,
				ElapsedMs:       r.totalMs,
			}},
		}); err != nil {
			t.Fatalf("写入第 %d 轮失败: %v", i, err)
		}
	}
}

// seedSimpleRounds 写入 n 轮相同的记录（用于只关心轮数的场景）。
func seedSimpleRounds(t *testing.T, st *store.Store, projectID, userID int64, n int, when string) {
	t.Helper()
	rows := make([]simpleRound, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, simpleRound{totalMs: 1000, correct: 1, total: 1, when: when})
	}
	seedRounds(t, st, projectID, userID, rows)
}

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// shareProject 为项目生成分享链接，返回 token 与完整链接。
func shareProject(t *testing.T, app *fiber.App, token string, projectID int64) (shareToken, link string) {
	t.Helper()
	status, body := do(t, app, http.MethodPost,
		fmt.Sprintf("/api/projects/%d/share", projectID), "", token)
	if status != http.StatusOK {
		t.Fatalf("分享应返回 200, 得到 %d: %#v", status, body)
	}
	st, _ := body["shareToken"].(string)
	l, _ := body["link"].(string)
	if st == "" || l == "" {
		t.Fatalf("分享应返回 token 与 link: %#v", body)
	}
	return st, l
}

// importLinks 导入一组链接，返回 results 数组。
func importLinks(t *testing.T, app *fiber.App, token string, links []string) ([]any, int) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"links": links})
	status, body := do(t, app, http.MethodPost, "/api/subscriptions/import", string(b), token)
	if status != http.StatusOK {
		t.Fatalf("导入应返回 200, 得到 %d: %#v", status, body)
	}
	results, _ := body["results"].([]any)
	succeeded, _ := body["succeeded"].(float64)
	return results, int(succeeded)
}

// ---------------------------------------------------------------------------
// P6.1 分享
// ---------------------------------------------------------------------------

func TestShareRequiresOwnership(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "项目")

	t.Run("作者可分享", func(t *testing.T) {
		_, link := shareProject(t, app, aliceToken, projectID)
		if link == "" {
			t.Error("应返回链接")
		}
	})

	t.Run("订阅者不可分享", func(t *testing.T) {
		if err := st.Subscribe(userID(t, st, "bob"), projectID); err != nil {
			t.Fatalf("Subscribe 失败: %v", err)
		}
		status, _ := do(t, app, http.MethodPost,
			fmt.Sprintf("/api/projects/%d/share", projectID), "", bobToken)
		if status != http.StatusForbidden {
			t.Errorf("订阅者分享应返回 403, 得到 %d", status)
		}
	})

	t.Run("未登录", func(t *testing.T) {
		status, _ := do(t, app, http.MethodPost,
			fmt.Sprintf("/api/projects/%d/share", projectID), "", "")
		if status != http.StatusUnauthorized {
			t.Errorf("应返回 401, 得到 %d", status)
		}
	})
}

func TestShareGeneratesDistinctTokensAndLinkFormat(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")
	projectID := createProject(t, app, token, "项目")

	t1, l1 := shareProject(t, app, token, projectID)
	t2, l2 := shareProject(t, app, token, projectID)

	if t1 == t2 {
		t.Error("两次分享应生成不同 token（第二次是重置）")
	}
	if l1 == l2 {
		t.Error("两次分享应生成不同链接")
	}
	// 链接格式（规格 §4.4）
	if want := "cala://subscribe?h="; len(l2) < len(want) || l2[:len(want)] != want {
		t.Errorf("链接应以 %q 开头, 得到 %q", want, l2)
	}
	if !containsAll(l2, "&t="+t2) {
		t.Errorf("链接应包含当前 token: %q", l2)
	}
}

// TestResetInvalidatesOldLink 覆盖「重置使旧链接失效」。
func TestResetInvalidatesOldLink(t *testing.T) {
	app := newTestApp(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "项目")

	_, oldLink := shareProject(t, app, aliceToken, projectID)

	// Bob 先用旧链接成功订阅
	results, succeeded := importLinks(t, app, bobToken, []string{oldLink})
	if succeeded != 1 {
		t.Fatalf("旧链接应可订阅: %#v", results)
	}

	// Alice 重置链接
	_, newLink := shareProject(t, app, aliceToken, projectID)
	if newLink == oldLink {
		t.Fatal("重置后链接应变化")
	}

	// 新用户用旧链接应失败
	carolToken := register(t, app, "carol", "password123")
	results, succeeded = importLinks(t, app, carolToken, []string{oldLink})
	if succeeded != 0 {
		t.Errorf("重置后旧链接应失效, 得到 %#v", results)
	}
	first, _ := results[0].(map[string]any)
	if msg, _ := first["error"].(string); msg == "" {
		t.Error("应给出可读的失败原因")
	}

	// 新链接可用
	if _, succeeded := importLinks(t, app, carolToken, []string{newLink}); succeeded != 1 {
		t.Error("新链接应可订阅")
	}
}

func TestUnshareThenLinkInvalid(t *testing.T) {
	app := newTestApp(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "项目")

	_, link := shareProject(t, app, aliceToken, projectID)

	status, _ := do(t, app, http.MethodDelete,
		fmt.Sprintf("/api/projects/%d/share", projectID), "", aliceToken)
	if status != http.StatusNoContent {
		t.Fatalf("撤销分享应返回 204, 得到 %d", status)
	}

	results, succeeded := importLinks(t, app, bobToken, []string{link})
	if succeeded != 0 {
		t.Errorf("撤销后链接应失效, 得到 %#v", results)
	}
}

// ---------------------------------------------------------------------------
// P6.2 导入
// ---------------------------------------------------------------------------

func TestImportSingleLink(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "口算")
	_, link := shareProject(t, app, aliceToken, projectID)

	results, succeeded := importLinks(t, app, bobToken, []string{link})
	if succeeded != 1 {
		t.Fatalf("应成功 1 条, 得到 %#v", results)
	}
	item, _ := results[0].(map[string]any)
	if item["ok"] != true {
		t.Errorf("应标记成功: %#v", item)
	}
	if item["title"] != "口算" {
		t.Errorf("应返回项目标题: %#v", item)
	}

	// 确认订阅关系已建立
	subscribed, err := st.IsSubscribed(userID(t, st, "bob"), projectID)
	if err != nil || !subscribed {
		t.Errorf("订阅关系应已建立 (err=%v)", err)
	}
}

// TestImportMixedLinks 是「逐条返回」的核心断言。
//
// 5 个链接中混入无效令牌、格式错误、跨实例，只有有效的那条应成功，
// 其余各自给出可读原因。整批失败会让用户不知道哪个是坏的。
func TestImportMixedLinks(t *testing.T) {
	app := newTestApp(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "口算")
	_, goodLink := shareProject(t, app, aliceToken, projectID)

	links := []string{
		goodLink, // 有效
		"cala://subscribe?h=127.0.0.1:9999&t=bogus",            // 令牌无效
		"https://example.com/whatever",                         // 格式错误
		"cala://subscribe?h=other.host&t=" + tokenOf(goodLink), // 跨实例
		"", // 空
	}

	results, succeeded := importLinks(t, app, bobToken, links)

	if len(results) != 5 {
		t.Fatalf("应逐条返回 5 条结果, 得到 %d", len(results))
	}
	if succeeded != 1 {
		t.Errorf("只有 1 条应成功, 得到 %d", succeeded)
	}

	// 每条失败都必须带可读原因
	for i, raw := range results {
		item, _ := raw.(map[string]any)
		if i == 0 {
			if item["ok"] != true {
				t.Errorf("第 1 条应成功: %#v", item)
			}
			continue
		}
		if item["ok"] == true {
			t.Errorf("第 %d 条不应成功: %#v", i+1, item)
		}
		msg, _ := item["error"].(string)
		if msg == "" {
			t.Errorf("第 %d 条应给出可读原因: %#v", i+1, item)
		}
		t.Logf("第 %d 条失败原因: %s", i+1, msg)
	}
}

// TestImportIsIdempotent 覆盖重复导入。
func TestImportIsIdempotent(t *testing.T) {
	app := newTestApp(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "口算")
	_, link := shareProject(t, app, aliceToken, projectID)

	importLinks(t, app, bobToken, []string{link})
	// 同一链接再导入两次（含一次重复出现在同一批里）
	results, succeeded := importLinks(t, app, bobToken, []string{link, link})

	if succeeded != 2 {
		t.Errorf("重复导入应视为成功（幂等）, 得到 %#v", results)
	}
	for _, raw := range results {
		item, _ := raw.(map[string]any)
		if item["alreadySubscribed"] != true {
			t.Errorf("应标记 alreadySubscribed: %#v", item)
		}
	}
}

func TestImportOwnProject(t *testing.T) {
	app := newTestApp(t, true)
	aliceToken := register(t, app, "alice", "password123")
	projectID := createProject(t, app, aliceToken, "自己的项目")
	_, link := shareProject(t, app, aliceToken, projectID)

	results, succeeded := importLinks(t, app, aliceToken, []string{link})
	if succeeded != 0 {
		t.Errorf("订阅自己的项目不应计入成功, 得到 %#v", results)
	}
	item, _ := results[0].(map[string]any)
	msg, _ := item["error"].(string)
	if msg == "" {
		t.Error("应提示这是自己的项目")
	}
}

func TestImportValidation(t *testing.T) {
	app := newTestApp(t, true)
	token := register(t, app, "alice", "password123")

	t.Run("空数组", func(t *testing.T) {
		status, _ := do(t, app, http.MethodPost, "/api/subscriptions/import",
			`{"links":[]}`, token)
		if status != http.StatusBadRequest {
			t.Errorf("应返回 400, 得到 %d", status)
		}
	})

	t.Run("未登录", func(t *testing.T) {
		status, _ := do(t, app, http.MethodPost, "/api/subscriptions/import",
			`{"links":["x"]}`, "")
		if status != http.StatusUnauthorized {
			t.Errorf("应返回 401, 得到 %d", status)
		}
	})

	t.Run("超过上限", func(t *testing.T) {
		links := make([]string, 51)
		for i := range links {
			links[i] = fmt.Sprintf("cala://subscribe?t=t%d", i)
		}
		b, _ := json.Marshal(map[string]any{"links": links})
		status, _ := do(t, app, http.MethodPost, "/api/subscriptions/import", string(b), token)
		if status != http.StatusBadRequest {
			t.Errorf("应返回 400, 得到 %d", status)
		}
	})
}

// TestSubscriberHasNoWriteAccess 验证 D4：订阅是纯只读跟随。
func TestSubscriberHasNoWriteAccess(t *testing.T) {
	app := newTestApp(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "共享项目")
	_, link := shareProject(t, app, aliceToken, projectID)

	if _, succeeded := importLinks(t, app, bobToken, []string{link}); succeeded != 1 {
		t.Fatal("订阅应成功")
	}

	t.Run("不能修改", func(t *testing.T) {
		status, _ := do(t, app, http.MethodPut,
			fmt.Sprintf("/api/projects/%d", projectID), projectBody("被篡改"), bobToken)
		if status != http.StatusForbidden {
			t.Errorf("订阅者修改应返回 403, 得到 %d", status)
		}
	})

	t.Run("不能删除", func(t *testing.T) {
		status, _ := do(t, app, http.MethodDelete,
			fmt.Sprintf("/api/projects/%d", projectID), "", bobToken)
		if status != http.StatusForbidden {
			t.Errorf("订阅者删除应返回 403, 得到 %d", status)
		}
	})

	t.Run("不能分享", func(t *testing.T) {
		status, _ := do(t, app, http.MethodPost,
			fmt.Sprintf("/api/projects/%d/share", projectID), "", bobToken)
		if status != http.StatusForbidden {
			t.Errorf("订阅者分享应返回 403, 得到 %d", status)
		}
	})

	t.Run("可以练习", func(t *testing.T) {
		status, _ := do(t, app, http.MethodPost, "/api/rounds/start",
			fmt.Sprintf(`{"projectId":%d}`, projectID), bobToken)
		if status != http.StatusOK {
			t.Errorf("订阅者应能练习, 得到 %d", status)
		}
	})
}

// TestSubscriberSeesAuthorUpdates 覆盖规格 §6.1(4)：
// 订阅者读的就是 project 同一行，作者改配置后订阅者立即看到变化。
func TestSubscriberSeesAuthorUpdates(t *testing.T) {
	app := newTestApp(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "原标题")
	_, link := shareProject(t, app, aliceToken, projectID)

	if _, succeeded := importLinks(t, app, bobToken, []string{link}); succeeded != 1 {
		t.Fatal("订阅应成功")
	}

	// 作者改标题与题数
	body, _ := json.Marshal(map[string]any{
		"title": "新标题", "questionCount": 7,
		"cfgJson": `{"min":1,"max":9}`, "ruleSource": apiValidRule,
	})
	if status, _ := do(t, app, http.MethodPut,
		fmt.Sprintf("/api/projects/%d", projectID), string(body), aliceToken); status != http.StatusOK {
		t.Fatal("作者修改失败")
	}

	// 订阅者看到的应是新配置
	status, resp := do(t, app, http.MethodGet,
		fmt.Sprintf("/api/projects/%d", projectID), "", bobToken)
	if status != http.StatusOK {
		t.Fatalf("订阅者查看失败: %d", status)
	}
	p, _ := resp["project"].(map[string]any)
	if p["title"] != "新标题" {
		t.Errorf("订阅者应看到新标题, 得到 %v", p["title"])
	}
	if p["questionCount"].(float64) != 7 {
		t.Errorf("订阅者应看到新题数, 得到 %v", p["questionCount"])
	}
}

// ---------------------------------------------------------------------------
// P6.3 退订（A6 —— 本阶段最重要的断言）
// ---------------------------------------------------------------------------

func TestUnsubscribeRequiresExistingSubscription(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "项目")

	t.Run("未订阅时退订 -> 404", func(t *testing.T) {
		status, _ := do(t, app, http.MethodDelete,
			fmt.Sprintf("/api/subscriptions/%d", projectID), "", bobToken)
		if status != http.StatusNotFound {
			t.Errorf("应返回 404, 得到 %d", status)
		}
	})

	t.Run("作者对自己项目退订 -> 400 且提示删除项目", func(t *testing.T) {
		status, body := do(t, app, http.MethodDelete,
			fmt.Sprintf("/api/subscriptions/%d", projectID), "", aliceToken)
		if status != http.StatusBadRequest {
			t.Fatalf("应返回 400, 得到 %d", status)
		}
		e, _ := body["error"].(map[string]any)
		if msg, _ := e["message"].(string); msg == "" {
			t.Error("应提示应删除项目而非退订")
		}
	})

	t.Run("未登录", func(t *testing.T) {
		status, _ := do(t, app, http.MethodDelete,
			fmt.Sprintf("/api/subscriptions/%d", projectID), "", "")
		if status != http.StatusUnauthorized {
			t.Errorf("应返回 401, 得到 %d", status)
		}
	})

	_ = st
}

// TestUnsubscribeClearsOnlyOwnHistory 是 A6 的核心断言。
//
// 退订必须：清空本人的记录、保留他人的记录、保留项目本身。
// 任何一条不满足都意味着数据所有权边界被破坏。
func TestUnsubscribeClearsOnlyOwnHistory(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	carolToken := register(t, app, "carol", "password123")

	projectID := createProject(t, app, aliceToken, "共享项目")
	_, link := shareProject(t, app, aliceToken, projectID)

	// Bob 与 Carol 订阅
	if _, ok := importLinks(t, app, bobToken, []string{link}); ok != 1 {
		t.Fatal("Bob 订阅失败")
	}
	if _, ok := importLinks(t, app, carolToken, []string{link}); ok != 1 {
		t.Fatal("Carol 订阅失败")
	}

	// 三人各做若干轮
	aliceID := userID(t, st, "alice")
	bobID := userID(t, st, "bob")
	carolID := userID(t, st, "carol")
	seedSimpleRounds(t, st, projectID, aliceID, 2, "2026-09-10T10:00:00Z")
	seedSimpleRounds(t, st, projectID, bobID, 3, "2026-09-10T11:00:00Z")
	seedSimpleRounds(t, st, projectID, carolID, 4, "2026-09-10T12:00:00Z")

	counts := func() (rounds, attempts int) {
		st.DB().QueryRow(`SELECT COUNT(*) FROM practice_round`).Scan(&rounds)
		st.DB().QueryRow(`SELECT COUNT(*) FROM attempt`).Scan(&attempts)
		return
	}
	roundsBefore, attemptsBefore := counts()
	if roundsBefore != 9 || attemptsBefore != 9 {
		t.Fatalf("种子异常: rounds=%d attempts=%d", roundsBefore, attemptsBefore)
	}

	// Bob 退订
	status, body := do(t, app, http.MethodDelete,
		fmt.Sprintf("/api/subscriptions/%d", projectID), "", bobToken)
	if status != http.StatusOK {
		t.Fatalf("退订应返回 200, 得到 %d: %#v", status, body)
	}

	// 返回的删除轮次数必须准确（界面据此提示代价）
	deleted, _ := body["deletedRounds"].(float64)
	if int(deleted) != 3 {
		t.Errorf("deletedRounds = %v, 期望 3", deleted)
	}

	// 1) 本人的记录消失（round 与 attempt 都归零）
	var bobRounds, bobAttempts int
	st.DB().QueryRow(
		`SELECT COUNT(*) FROM practice_round WHERE user_id = ? AND project_id = ?`,
		bobID, projectID).Scan(&bobRounds)
	st.DB().QueryRow(
		`SELECT COUNT(*) FROM attempt a JOIN practice_round r ON r.id = a.round_id
		 WHERE r.user_id = ? AND r.project_id = ?`, bobID, projectID).Scan(&bobAttempts)
	if bobRounds != 0 {
		t.Errorf("退订后本人轮次残留 = %d, 期望 0", bobRounds)
	}
	if bobAttempts != 0 {
		t.Errorf("退订后本人题目记录残留 = %d, 期望 0（外键级联应已清除）", bobAttempts)
	}

	// 2) 他人记录完好 —— 这是 A6 的关键
	var aliceRounds, carolRounds int
	st.DB().QueryRow(`SELECT COUNT(*) FROM practice_round WHERE user_id = ?`, aliceID).Scan(&aliceRounds)
	st.DB().QueryRow(`SELECT COUNT(*) FROM practice_round WHERE user_id = ?`, carolID).Scan(&carolRounds)
	if aliceRounds != 2 {
		t.Errorf("作者的轮次被误删: %d, 期望 2", aliceRounds)
	}
	if carolRounds != 4 {
		t.Errorf("其他订阅者的轮次被误删: %d, 期望 4", carolRounds)
	}

	// 3) 订阅行消失
	subscribed, _ := st.IsSubscribed(bobID, projectID)
	if subscribed {
		t.Error("订阅关系应已删除")
	}
	// Carol 的订阅不受影响
	if stillSub, _ := st.IsSubscribed(carolID, projectID); !stillSub {
		t.Error("他人的订阅关系不应受影响")
	}

	// 4) 项目本身仍在
	if _, err := st.GetProject(projectID); err != nil {
		t.Errorf("项目不应被删除: %v", err)
	}

	// 5) 退订后不再能练习该项目
	status, _ = do(t, app, http.MethodPost, "/api/rounds/start",
		fmt.Sprintf(`{"projectId":%d}`, projectID), bobToken)
	if status != http.StatusForbidden {
		t.Errorf("退订后练习应返回 403, 得到 %d", status)
	}
}

// TestUnsubscribeDoesNotAffectOtherProjects 确认退订只清目标项目。
func TestUnsubscribeDoesNotAffectOtherProjects(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")

	p1 := createProject(t, app, aliceToken, "项目一")
	p2 := createProject(t, app, aliceToken, "项目二")
	_, link1 := shareProject(t, app, aliceToken, p1)
	_, link2 := shareProject(t, app, aliceToken, p2)

	importLinks(t, app, bobToken, []string{link1, link2})

	bobID := userID(t, st, "bob")
	seedSimpleRounds(t, st, p1, bobID, 2, "2026-09-10T10:00:00Z")
	seedSimpleRounds(t, st, p2, bobID, 5, "2026-09-10T11:00:00Z")

	// 只退订项目一
	if status, _ := do(t, app, http.MethodDelete,
		fmt.Sprintf("/api/subscriptions/%d", p1), "", bobToken); status != http.StatusOK {
		t.Fatal("退订失败")
	}

	var p1Rounds, p2Rounds int
	st.DB().QueryRow(`SELECT COUNT(*) FROM practice_round WHERE user_id = ? AND project_id = ?`,
		bobID, p1).Scan(&p1Rounds)
	st.DB().QueryRow(`SELECT COUNT(*) FROM practice_round WHERE user_id = ? AND project_id = ?`,
		bobID, p2).Scan(&p2Rounds)

	if p1Rounds != 0 {
		t.Errorf("项目一的记录应被清除, 残留 %d", p1Rounds)
	}
	if p2Rounds != 5 {
		t.Errorf("项目二的记录不应受影响, 得到 %d, 期望 5", p2Rounds)
	}
	if stillSub, _ := st.IsSubscribed(bobID, p2); !stillSub {
		t.Error("项目二的订阅关系不应受影响")
	}
}

// TestUnsubscribeThenResubscribe 确认退订后可以重新订阅（历史不会回来，但能重新开始）。
func TestUnsubscribeThenResubscribe(t *testing.T) {
	app, st := newTestAppWithStore(t, true)
	aliceToken := register(t, app, "alice", "password123")
	bobToken := register(t, app, "bob", "password123")
	projectID := createProject(t, app, aliceToken, "项目")
	_, link := shareProject(t, app, aliceToken, projectID)

	importLinks(t, app, bobToken, []string{link})
	seedSimpleRounds(t, st, projectID, userID(t, st, "bob"), 2, "2026-09-10T10:00:00Z")

	if status, _ := do(t, app, http.MethodDelete,
		fmt.Sprintf("/api/subscriptions/%d", projectID), "", bobToken); status != http.StatusOK {
		t.Fatal("退订失败")
	}

	// 重新订阅
	if _, ok := importLinks(t, app, bobToken, []string{link}); ok != 1 {
		t.Fatal("重新订阅应成功")
	}

	var rounds int
	st.DB().QueryRow(
		`SELECT COUNT(*) FROM practice_round WHERE user_id = ? AND project_id = ?`,
		userID(t, st, "bob"), projectID).Scan(&rounds)
	if rounds != 0 {
		t.Errorf("重新订阅后不应恢复已删除的历史, 得到 %d 轮", rounds)
	}

	// 能重新练习
	status, _ := do(t, app, http.MethodPost, "/api/rounds/start",
		fmt.Sprintf(`{"projectId":%d}`, projectID), bobToken)
	if status != http.StatusOK {
		t.Errorf("重新订阅后应能练习, 得到 %d", status)
	}
}

// TestUnsubscribeOnlyTouchesTargetUser 的等价断言在 store 包中
// （见 store/subscription_test.go 的 TestUnsubscribeIsolation），
// 那里可以直接验证数据层语义，无需经过 HTTP。

// tokenOf 从分享链接中取出 token 部分。
func tokenOf(link string) string {
	const marker = "&t="
	i := strings.Index(link, marker)
	if i < 0 {
		return ""
	}
	return link[i+len(marker):]
}

func containsAll(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

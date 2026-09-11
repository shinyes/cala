package service

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shinyes/cala/backend/internal/store"
)

func newRoundSvc(t *testing.T) (*RoundService, *ProjectService, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	ps := NewProjectService(st)
	return NewRoundService(st, ps), ps, st
}

func TestStartReturnsQuestionsAndScoringConfig(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	res, err := rs.Start(owner, p.ID)
	if err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	if len(res.Questions) != p.QuestionCount {
		t.Errorf("题数 = %d, 期望 %d（应取项目设置，D12）", len(res.Questions), p.QuestionCount)
	}
	if res.Scoring.Version == 0 {
		t.Error("应下发清洗表版本")
	}
	if len(res.Scoring.CleanupTable) == 0 {
		t.Error("应下发清洗表（规格 §5.5.4）")
	}
	for i, q := range res.Questions {
		if q.Index != i {
			t.Errorf("第 %d 题 idx = %d, 期望 %d", i, q.Index, i)
		}
		if q.Q == "" || q.A == "" {
			t.Errorf("第 %d 题缺少题面或答案: %+v", i, q)
		}
		if q.Envelope.Kind == "" {
			t.Errorf("第 %d 题缺少判分信封: %+v", i, q)
		}
	}
}

// TestStartIsReproducibleBySeed 是 A3 在服务层的体现：
// 交卷时服务端要用同一 seed 重放，故 Start 的结果必须由 seed 完全决定。
func TestStartIsReproducibleBySeed(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	rule, cfg, err := ps.RuleFor(p)
	if err != nil {
		t.Fatalf("RuleFor 失败: %v", err)
	}

	a, err := rule.Generate(cfg, 12345, p.QuestionCount)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	b, err := rule.Generate(cfg, 12345, p.QuestionCount)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("同种子重放第 %d 题不一致: %+v vs %+v", i, a[i], b[i])
		}
	}
	_ = rs
}

func TestStartRequiresAccess(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	stranger := makeUser(t, st, "stranger")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	if _, err := rs.Start(stranger, p.ID); !errors.Is(err, store.ErrNoAccess) {
		t.Errorf("无关用户应被拒, 得到 %v", err)
	}
	if _, err := rs.Start(owner, 999999); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("不存在的项目应返回 ErrNotFound, 得到 %v", err)
	}
}

// completeHelper 走一遍 start→complete，返回交卷结果。
func completeHelper(t *testing.T, rs *RoundService, userID, projectID int64, answer func(i int, q string) (string, bool)) CompletedRound {
	t.Helper()
	started := time.Now().UTC()
	startedStr := started.Format(time.RFC3339)

	res, err := rs.Start(userID, projectID)
	if err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	attempts := make([]AttemptInput, 0, len(res.Questions))
	for _, q := range res.Questions {
		in, clientSays := answer(q.Index, q.A)
		attempts = append(attempts, AttemptInput{
			Index: q.Index, Input: in, ClientIsCorrect: clientSays, ElapsedMs: 1000,
		})
	}

	done, err := rs.Complete(userID, CompleteInput{
		ProjectID:  projectID,
		Seed:       res.Seed,
		StartedAt:  startedStr,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Attempts:   attempts,
	})
	if err != nil {
		t.Fatalf("Complete 失败: %v", err)
	}
	return done
}

func TestCompleteAllCorrect(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	done := completeHelper(t, rs, owner, p.ID, func(i int, a string) (string, bool) {
		return a, true // 全部作答正确
	})

	if done.CorrectCount != p.QuestionCount {
		t.Errorf("正确数 = %d, 期望 %d", done.CorrectCount, p.QuestionCount)
	}
	if done.Discrepancies != 0 {
		t.Errorf("客户端判定正确时不应有分歧, 得到 %d", done.Discrepancies)
	}
	if done.TotalMs != int64(p.QuestionCount)*1000 {
		t.Errorf("总耗时 = %d, 期望 %d（应由服务端从 elapsedMs 派生）",
			done.TotalMs, p.QuestionCount*1000)
	}
}

// TestDiscrepancyIsRecorded 覆盖验收标准 A14（D16）：
// 客户端谎报正确时，服务端重算为错误，两个值各自独立落库，并计入分歧。
func TestDiscrepancyIsRecorded(t *testing.T) {
	ResetDiscrepancyCount()
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	// 客户端说「我全对」，但故意提交错误作答
	done := completeHelper(t, rs, owner, p.ID, func(i int, a string) (string, bool) {
		return "999999", true // 明显的错答，却声称正确
	})

	if done.CorrectCount != 0 {
		t.Errorf("服务端应重算为全错, 得到正确数 %d", done.CorrectCount)
	}
	if done.Discrepancies != p.QuestionCount {
		t.Errorf("分歧数 = %d, 期望 %d（每题都应被记为分歧）", done.Discrepancies, p.QuestionCount)
	}
	if got := DiscrepancyCount(); got != int64(p.QuestionCount) {
		t.Errorf("全局分歧计数 = %d, 期望 %d", got, p.QuestionCount)
	}

	// 两个判定必须各自独立落库，不得互相覆盖
	attempts, err := st.ListAttemptsByRound(done.RoundID)
	if err != nil {
		t.Fatalf("ListAttemptsByRound 失败: %v", err)
	}
	for _, a := range attempts {
		if !a.ClientIsCorrect {
			t.Errorf("第 %d 题 client_is_correct 应为 true（原样保存客户端上报）, 得到 false", a.Index)
		}
		if a.ServerIsCorrect {
			t.Errorf("第 %d 题 server_is_correct 应为 false（权威重算）, 得到 true", a.Index)
		}
	}
}

func TestCompleteRecomputesAgainstServerTruth(t *testing.T) {
	ResetDiscrepancyCount()
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	// 奇数题答对、偶数题答错，且客户端如实上报
	done := completeHelper(t, rs, owner, p.ID, func(i int, a string) (string, bool) {
		if i%2 == 0 {
			return a, true
		}
		return "0", false
	})

	if done.Discrepancies != 0 {
		t.Errorf("如实上报不应产生分歧, 得到 %d", done.Discrepancies)
	}
	wantCorrect := (p.QuestionCount + 1) / 2
	if done.CorrectCount != wantCorrect {
		t.Errorf("正确数 = %d, 期望 %d", done.CorrectCount, wantCorrect)
	}
}

// TestSnapshotImmutableAfterRuleChange 覆盖规格 §6.1(3)：
// 作者事后改规则不得改动历史错题的题面与答案快照。
// 否则用户昨天做错的题，今天打开错题页会变成另一道题。
func TestSnapshotImmutableAfterRuleChange(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")

	in := validInput()
	in.RuleSource = `function generate(cfg){ return {q:"OLD", a:"1"} }`
	in.QuestionCount = 3
	p, err := ps.Create(owner, in)
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	done := completeHelper(t, rs, owner, p.ID, func(i int, a string) (string, bool) {
		return "0", false // 全部错，模拟错题
	})

	before, err := st.ListAttemptsByRound(done.RoundID)
	if err != nil {
		t.Fatalf("读取快照失败: %v", err)
	}
	for _, a := range before {
		if a.QSnapshot != "OLD" {
			t.Fatalf("快照题面 = %q, 期望 OLD", a.QSnapshot)
		}
	}

	// 作者把规则改成完全不同的题目
	newIn := in
	newIn.RuleSource = `function generate(cfg){ return {q:"NEW", a:"2"} }`
	if _, err := ps.Update(owner, p.ID, newIn); err != nil {
		t.Fatalf("更新项目失败: %v", err)
	}

	after, err := st.ListAttemptsByRound(done.RoundID)
	if err != nil {
		t.Fatalf("读取快照失败: %v", err)
	}
	for _, a := range after {
		if a.QSnapshot != "OLD" {
			t.Errorf("改规则后历史快照被篡改: 第 %d 题题面 = %q, 期望仍为 OLD", a.Index, a.QSnapshot)
		}
		if a.ASnapshot != "1" {
			t.Errorf("改规则后历史答案被篡改: 第 %d 题答案 = %q, 期望仍为 1", a.Index, a.ASnapshot)
		}
		if a.EnvelopeJSON == "" {
			t.Errorf("第 %d 题信封不应为空（判分依据需可重算）", a.Index)
		}
	}
}

// TestStaleProjectFlagged 覆盖 R-P3-2：出题后作者改规则，
// 重放结果可能与客户端所见不同，必须显式告知而不是静默写入。
func TestStaleProjectFlagged(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	// 用一个很早的 startedAt，使项目 updated_at 必然晚于它
	startedStr := "2020-01-01T00:00:00Z"
	res, err := rs.Start(owner, p.ID)
	if err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	attempts := make([]AttemptInput, 0, len(res.Questions))
	for _, q := range res.Questions {
		attempts = append(attempts, AttemptInput{Index: q.Index, Input: q.A, ClientIsCorrect: true, ElapsedMs: 100})
	}

	done, err := rs.Complete(owner, CompleteInput{
		ProjectID: p.ID, Seed: res.Seed,
		StartedAt: startedStr, FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Attempts: attempts,
	})
	if err != nil {
		t.Fatalf("Complete 失败: %v", err)
	}
	if !done.StaleProject {
		t.Error("项目 updated_at 晚于 startedAt 时应标记 staleProject=true")
	}
}

func TestCompleteValidatesInput(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}
	res, err := rs.Start(owner, p.ID)
	if err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	good := func() []AttemptInput {
		out := make([]AttemptInput, 0, len(res.Questions))
		for _, q := range res.Questions {
			out = append(out, AttemptInput{Index: q.Index, Input: q.A, ClientIsCorrect: true, ElapsedMs: 100})
		}
		return out
	}
	now := time.Now().UTC().Format(time.RFC3339)

	cases := []struct {
		name string
		in   CompleteInput
	}{
		{"缺 projectId", CompleteInput{Seed: res.Seed, Attempts: good(), StartedAt: now}},
		{"attempts 为空", CompleteInput{ProjectID: p.ID, Seed: res.Seed, StartedAt: now}},
		{"题数不匹配", CompleteInput{ProjectID: p.ID, Seed: res.Seed, StartedAt: now,
			Attempts: good()[:1]}},
		{"时间格式非法", CompleteInput{ProjectID: p.ID, Seed: res.Seed, StartedAt: "not-a-time",
			Attempts: good()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := rs.Complete(owner, tc.in); err == nil {
				t.Error("应被拒绝")
			}
		})
	}

	t.Run("题号重复", func(t *testing.T) {
		as := good()
		as[1].Index = as[0].Index
		_, err := rs.Complete(owner, CompleteInput{
			ProjectID: p.ID, Seed: res.Seed, StartedAt: now, Attempts: as})
		if !errors.Is(err, ErrRoundInput) {
			t.Errorf("重复题号应返回 ErrRoundInput, 得到 %v", err)
		}
	})

	t.Run("题号越界", func(t *testing.T) {
		as := good()
		as[0].Index = 9999
		if _, err := rs.Complete(owner, CompleteInput{
			ProjectID: p.ID, Seed: res.Seed, StartedAt: now, Attempts: as}); err == nil {
			t.Error("越界题号应被拒绝")
		}
	})
}

func TestCompleteRequiresAccess(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	stranger := makeUser(t, st, "stranger")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	_, err = rs.Complete(stranger, CompleteInput{
		ProjectID: p.ID, Seed: 1,
		StartedAt: store.Now(), FinishedAt: store.Now(),
		Attempts: []AttemptInput{{Index: 0, Input: "1", ElapsedMs: 1}},
	})
	if !errors.Is(err, store.ErrNoAccess) {
		t.Errorf("无关用户应被拒, 得到 %v", err)
	}
}

// TestCompleteRejectsImpossibleProject 覆盖 R-P3-3 的另一面：
// 若项目规则已被改坏（例如绕过校验直接改库），Start/Complete 应返回规则错误
// 而不是 500。这里用死循环规则模拟。
func TestStartRejectsBrokenRuleAsRuleError(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	// 绕过 service 直接改库，模拟「规则在保存后被改坏」
	if _, err := st.DB().Exec(
		`UPDATE project SET rule_source = ? WHERE id = ?`,
		`function generate(cfg){ while(true){} }`, p.ID); err != nil {
		t.Fatalf("直接改库失败: %v", err)
	}

	_, err = rs.Start(owner, p.ID)
	if err == nil {
		t.Fatal("坏规则应导致 Start 失败")
	}
	if !strings.Contains(err.Error(), "未返回") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("错误应说明是超时, 得到: %v", err)
	}
}

// TestSeedSurvivesFloat64JSONRoundTrip 是一个**跨端契约**断言。
//
// 种子经 JSON 以数字形式往返：服务端 -> 客户端 -> 服务端。
// JavaScript（以及 Flutter Web 的 dart2js，其中 int 即 double）只有 53 位尾数精度，
// 超过 2^53 的整数在往返后会变成另一个值。一旦发生，
// 服务端用回传种子重放会得到**完全不同的题目**，于是每一题都被判错。
//
// 本测试用 float64 模拟 JS 侧的解析行为，断言往返无损。
func TestSeedSurvivesFloat64JSONRoundTrip(t *testing.T) {
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	// 多轮取样：种子上限若设置不当，某些值必然出问题
	for i := 0; i < 50; i++ {
		res, err := rs.Start(owner, p.ID)
		if err != nil {
			t.Fatalf("Start 失败: %v", err)
		}

		// 模拟 JSON -> JS number 的往返
		back := int64(float64(res.Seed))
		if back != res.Seed {
			t.Fatalf("种子 %d 经 float64 往返后变为 %d —— "+
				"在 Flutter Web / JavaScript 上会导致服务端重放出不同题目、每题判错",
				res.Seed, back)
		}
		if res.Seed < 0 {
			t.Fatalf("种子不应为负: %d", res.Seed)
		}
		if res.Seed >= maxSeed {
			t.Fatalf("种子 %d 超出安全上限 %d", res.Seed, maxSeed)
		}
	}
}

// TestSeedAboveLimitWouldBreakReplay 证明上一条测试的**必要性**：
// 若刻意用一个超过 2^53 的种子，float64 往返确实会失效。
// 没有这条，上面的断言可能只是碰巧通过。
func TestSeedAboveLimitWouldBreakReplay(t *testing.T) {
	huge := int64(1)<<53 + 1 // 2^53 + 1
	back := int64(float64(huge))
	if back == huge {
		t.Skip("当前平台 float64 可精确表示该值，本测试无意义")
	}
	t.Logf("证明：%d 经 float64 往返后变为 %d（相差 %d）", huge, back, back-huge)
}

// TestAnswersUseServerSideInputNotClientSupplied 确认服务端不采信客户端上报的题面。
func TestServerDoesNotTrustClientSuppliedText(t *testing.T) {
	ResetDiscrepancyCount()
	rs, ps, st := newRoundSvc(t)
	owner := makeUser(t, st, "owner")
	p, err := ps.Create(owner, validInput())
	if err != nil {
		t.Fatalf("创建项目失败: %v", err)
	}

	res, err := rs.Start(owner, p.ID)
	if err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	attempts := make([]AttemptInput, 0, len(res.Questions))
	for _, q := range res.Questions {
		attempts = append(attempts, AttemptInput{Index: q.Index, Input: q.A, ClientIsCorrect: true, ElapsedMs: 10})
	}
	done, err := rs.Complete(owner, CompleteInput{
		ProjectID: p.ID, Seed: res.Seed,
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Attempts:   attempts,
	})
	if err != nil {
		t.Fatalf("Complete 失败: %v", err)
	}

	// 落库的题面必须等于服务端重放的结果，而不是客户端可能伪造的内容
	stored, err := st.ListAttemptsByRound(done.RoundID)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	for i, a := range stored {
		if a.QSnapshot != res.Questions[i].Q {
			t.Errorf("第 %d 题快照 = %q, 期望服务端重放的 %q", i, a.QSnapshot, res.Questions[i].Q)
		}
		if a.EnvelopeJSON == "" {
			t.Errorf("第 %d 题信封不应为空", i)
		}
	}
}

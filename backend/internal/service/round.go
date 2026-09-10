package service

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/shinyes/cala/backend/internal/scoring"
	"github.com/shinyes/cala/backend/internal/store"
)

var (
	// ErrRoundInput 表示交卷请求内容不合法。
	ErrRoundInput = errors.New("交卷请求不合法")
)

// DiscrepancyCounter 统计「客户端判定与服务端判定不一致」的次数（D16）。
//
// 为什么需要它：跨端判分一致性由 §5.5 的六重措施约束，但语料不可能穷举所有
// 真实输入。这个计数器让**语料未覆盖的分歧立刻可见**，而不是静默污染统计。
// 刻意用 atomic 计数而非引入第三方 metrics 库（YAGNI）。
var discrepancyCount atomic.Int64

// DiscrepancyCount 返回累计分歧次数。
func DiscrepancyCount() int64 { return discrepancyCount.Load() }

// ResetDiscrepancyCount 仅供测试使用。
func ResetDiscrepancyCount() { discrepancyCount.Store(0) }

// RoundService 承载出题与交卷。
type RoundService struct {
	store    *store.Store
	projects *ProjectService
}

// NewRoundService 构造轮次服务。
func NewRoundService(st *store.Store, ps *ProjectService) *RoundService {
	return &RoundService{store: st, projects: ps}
}

// Question 是下发给出题客户端的一道题。
type Question struct {
	Index    int              `json:"idx"`
	Q        string           `json:"q"`
	A        string           `json:"a"`
	Envelope scoring.Envelope `json:"envelope"`
}

// ScoringConfig 随轮次下发的判分配置。
//
// 清洗表作为**数据**下发，而不是让客户端各自实现清洗逻辑（规格 §5.5.4）。
// version 让服务端日后调整表格时，能在分歧告警里分辨「客户端用了旧表」
// 还是「两边算法真的不一致」。
type ScoringConfig struct {
	Version      int               `json:"version"`
	CleanupTable map[string]string `json:"cleanupTable"`
}

// StartedRound 是 /rounds/start 的响应。
type StartedRound struct {
	Seed      int64         `json:"seed"`
	Scoring   ScoringConfig `json:"scoring"`
	Questions []Question    `json:"questions"`
}

// Start 为一次练习生成整轮题目。
//
// 服务端此时**不落库**（D6：不持久化未完成练习）。交卷时用同一 seed 重新出题
// 得到快照——这正是 A3（同种子可复现）被当作硬要求的原因。
func (s *RoundService) Start(userID, projectID int64) (StartedRound, error) {
	p, err := s.projects.Get(userID, projectID) // 内含访问权判定
	if err != nil {
		return StartedRound{}, err
	}

	rule, cfg, err := s.projects.RuleFor(p)
	if err != nil {
		return StartedRound{}, err
	}

	seed, err := newSeed()
	if err != nil {
		return StartedRound{}, err
	}

	qs, err := rule.Generate(cfg, seed, p.QuestionCount)
	if err != nil {
		return StartedRound{}, err
	}

	out := make([]Question, 0, len(qs))
	for _, q := range qs {
		env, err := scoring.Classify(q.A)
		if err != nil {
			// 保存期已校验过答案可分类；此处失败说明规则在保存后被改动过，
			// 或保存期校验有漏洞。明确报错而不是下发一个无法判分的题目。
			return StartedRound{}, fmt.Errorf("%w: 第 %d 题的答案 %q 无法分类：%v",
				ErrInvalidProject, q.Index+1, q.A, err)
		}
		out = append(out, Question{Index: q.Index, Q: q.Q, A: q.A, Envelope: env})
	}

	return StartedRound{
		Seed: seed,
		Scoring: ScoringConfig{
			Version:      scoring.CleanupVersion,
			CleanupTable: scoring.CleanupTable(),
		},
		Questions: out,
	}, nil
}

// AttemptInput 是客户端上报的一道题的作答。
type AttemptInput struct {
	Index           int    `json:"idx"`
	Input           string `json:"input"`
	ClientIsCorrect bool   `json:"clientIsCorrect"`
	ElapsedMs       int64  `json:"elapsedMs"`
}

// CompleteInput 是交卷请求。
type CompleteInput struct {
	ProjectID  int64          `json:"projectId"`
	Seed       int64          `json:"seed"`
	StartedAt  string         `json:"startedAt"`
	FinishedAt string         `json:"finishedAt"`
	Attempts   []AttemptInput `json:"attempts"`
}

// CompletedRound 是交卷响应。
type CompletedRound struct {
	RoundID       int64 `json:"roundId"`
	TotalMs       int64 `json:"totalMs"`
	QuestionCount int   `json:"questionCount"`
	CorrectCount  int   `json:"correctCount"`
	// Discrepancies 是本轮中客户端判定与服务端判定不一致的题数。
	// 正常应为 0；非 0 说明存在跨端判分歧，需要按 §5.5.6 排查。
	Discrepancies int `json:"discrepancies"`
	// StaleProject 为 true 表示项目在出题之后被作者修改过，
	// 因此服务端重放出的题目可能与客户端所见不同。
	StaleProject bool `json:"staleProject"`
}

// Complete 落库一轮练习。
//
// 关键行为（规格 §7 五条）：
//  1. 用服务端判分实现从信封与 input 重算 server_is_correct；
//  2. 原样保存客户端上报的 clientIsCorrect，不覆盖、不采信；
//  3. 二者不一致时递增分歧计数器并记录结构化日志；
//  4. total_ms 与 correct_count 由服务端从 attempts 派生，不采信客户端汇总；
//  5. 统计一律基于 server_is_correct。
func (s *RoundService) Complete(userID int64, in CompleteInput) (CompletedRound, error) {
	if in.ProjectID <= 0 {
		return CompletedRound{}, fmt.Errorf("%w: 缺少 projectId", ErrRoundInput)
	}
	if len(in.Attempts) == 0 {
		return CompletedRound{}, fmt.Errorf("%w: attempts 不能为空", ErrRoundInput)
	}

	p, err := s.projects.Get(userID, in.ProjectID)
	if err != nil {
		return CompletedRound{}, err
	}

	rule, cfg, err := s.projects.RuleFor(p)
	if err != nil {
		return CompletedRound{}, err
	}
	tol, err := ToleranceFor(p)
	if err != nil {
		return CompletedRound{}, err
	}

	// 用同一 seed 重放出题，得到题面与答案快照。
	// 服务端不采信客户端上报的题面/答案，避免被伪造的 q/a 污染历史。
	qs, err := rule.Generate(cfg, in.Seed, p.QuestionCount)
	if err != nil {
		return CompletedRound{}, err
	}
	if len(qs) != len(in.Attempts) {
		return CompletedRound{}, fmt.Errorf("%w: 题数不匹配（服务端重放 %d 题，上报 %d 题）",
			ErrRoundInput, len(qs), len(in.Attempts))
	}
	if err := checkIndexContinuity(in.Attempts, len(qs)); err != nil {
		return CompletedRound{}, err
	}

	// 项目在出题之后被改过？重放结果可能与客户端所见不同。
	stale := p.UpdatedAt > in.StartedAt

	startedAt, finishedAt, err := normalizeTimes(in)
	if err != nil {
		return CompletedRound{}, err
	}

	// 逐题判分并组装落库行。
	rows := make([]store.NewAttempt, 0, len(qs))
	correct := 0
	discrepancies := 0
	for _, a := range in.Attempts {
		env, err := scoring.Classify(qs[a.Index].A)
		if err != nil {
			return CompletedRound{}, fmt.Errorf("%w: 第 %d 题的答案无法分类：%v",
				ErrRoundInput, a.Index+1, err)
		}
		res := scoring.Compare(a.Input, env, tol)
		if res.Correct {
			correct++
		}
		if res.Correct != a.ClientIsCorrect {
			discrepancies++
			// 结构化留痕：这是语料未覆盖的边界被真实输入触发的信号。
			log.Printf("scoring discrepancy: project=%d user=%d idx=%d input=%q "+
				"client=%v server=%v reason=%s",
				in.ProjectID, userID, a.Index, a.Input, a.ClientIsCorrect, res.Correct, res.Reason)
		}

		envJSON, err := env.Marshal()
		if err != nil {
			return CompletedRound{}, err
		}
		rows = append(rows, store.NewAttempt{
			Index:        a.Index,
			QSnapshot:    qs[a.Index].Q,
			ASnapshot:    qs[a.Index].A,
			EnvelopeJSON: envJSON,
			UserInput:    a.Input,
			// 客户端判定原样保存，不覆盖（D16）
			ClientIsCorrect: a.ClientIsCorrect,
			// 服务端判定是权威值
			ServerIsCorrect: res.Correct,
			ElapsedMs:       a.ElapsedMs,
		})
	}

	if discrepancies > 0 {
		discrepancyCount.Add(int64(discrepancies))
	}

	totalMs := sumElapsed(rows)

	roundID, err := s.store.CreateRound(store.NewRound{
		ProjectID:     in.ProjectID,
		UserID:        userID,
		Seed:          in.Seed,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		TotalMs:       totalMs,
		QuestionCount: len(rows),
		CorrectCount:  correct,
		Attempts:      rows,
	})
	if err != nil {
		return CompletedRound{}, err
	}

	return CompletedRound{
		RoundID:       roundID,
		TotalMs:       totalMs,
		QuestionCount: len(rows),
		CorrectCount:  correct,
		Discrepancies: discrepancies,
		StaleProject:  stale,
	}, nil
}

// checkIndexContinuity 确认 idx 恰好是 0..n-1，无重复、无缺口。
//
// 连续性很重要：attempt 有 UNIQUE(round_id, idx)，且有缺口的 idx 会让
// 统计与错题重练出现空洞。这里提前给出可读错误，而不是让 INSERT 抛出约束错误。
func checkIndexContinuity(attempts []AttemptInput, want int) error {
	seen := make([]bool, want)
	for _, a := range attempts {
		if a.Index < 0 || a.Index >= want {
			return fmt.Errorf("%w: 题号 %d 越界（应在 0-%d）", ErrRoundInput, a.Index, want-1)
		}
		if seen[a.Index] {
			return fmt.Errorf("%w: 题号 %d 重复", ErrRoundInput, a.Index)
		}
		seen[a.Index] = true
	}
	for i, ok := range seen {
		if !ok {
			return fmt.Errorf("%w: 缺少题号 %d", ErrRoundInput, i)
		}
	}
	return nil
}

// sumElapsed 由服务端派生总耗时，不采信客户端上报的汇总值（规格 §7 第 4 条）。
func sumElapsed(rows []store.NewAttempt) int64 {
	var total int64
	for _, r := range rows {
		if r.ElapsedMs > 0 {
			total += r.ElapsedMs
		}
	}
	return total
}

func normalizeTimes(in CompleteInput) (startedAt, finishedAt string, err error) {
	startedAt, err = normalizeTime(in.StartedAt, "startedAt")
	if err != nil {
		return "", "", err
	}
	finishedAt, err = normalizeTime(in.FinishedAt, "finishedAt")
	if err != nil {
		return "", "", err
	}
	return startedAt, finishedAt, nil
}

// normalizeTime 接受 RFC3339，缺失时用当前时间兜底。
//
// 兜底而非报错：时间只影响统计分桶，不该因客户端时钟格式问题让整轮练习白做。
// 但**耗时**不兜底——它由 elapsedMs 之和派生，与这两个时间戳无关。
func normalizeTime(s, field string) (string, error) {
	if s == "" {
		return store.Now(), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return "", fmt.Errorf("%w: %s 不是合法的 RFC3339 时间：%v", ErrRoundInput, field, err)
	}
	return t.UTC().Format(time.RFC3339), nil
}

// newSeed 生成一轮的随机种子。
//
// 用 crypto/rand 而非 math/rand：种子决定了题目序列，若可预测，
// 用户就能提前算出题目。这不是安全攸关，但预测性没有任好处，成本也相同。
func newSeed() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("生成随机种子失败: %w", err)
	}
	return int64(binary.BigEndian.Uint64(b[:]) >> 1), nil // 保证非负
}

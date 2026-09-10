package rules

import "math/rand"

// seededSource 是每轮共享的确定性随机源。
//
// 它被注入到**每一道题各自的 VM** 中，因此题目之间延续同一条随机序列，
// 而整轮的结果由 seed 完全决定（验收标准 A3）。
type seededSource struct {
	r *rand.Rand
}

func newSeededSource(seed int64) *seededSource {
	// math/rand 的 NewSource 是确定性 PRNG —— 这正是本处所需。
	// 它**不可**用于任何安全用途（口令、令牌、会话 ID）；
	// 本包不含安全用途，见 internal/auth。
	return &seededSource{r: rand.New(rand.NewSource(seed))}
}

func (s *seededSource) Float64() float64 { return s.r.Float64() }

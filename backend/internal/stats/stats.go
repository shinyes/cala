package stats

import "time"

// Round 是本包需要的一轮数据。
//
// 刻意使用本包自己的类型而非 store.Round：这样 stats 不依赖任何其它包，
// 调用方负责映射。代价是一次字段拷贝，收益是可独立测试与复用。
type Round struct {
	FinishedAt    time.Time
	TotalMs       int64
	CorrectCount  int
	QuestionCount int
}

// Accuracy 返回本轮正确率（0..1）。
//
// 题数为 0 时返回 0 而非 NaN——NaN 会污染求和与中位数，
// 让整页统计变成 NaN。
func (r Round) Accuracy() float64 {
	if r.QuestionCount <= 0 {
		return 0
	}
	return float64(r.CorrectCount) / float64(r.QuestionCount)
}

// BucketResult 是一个桶的完整结果。
type BucketResult struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Metrics Metrics `json:"metrics"`
}

// Result 是一次统计查询的结果。
type Result struct {
	Grain   Grain          `json:"grain"`
	Overall Metrics        `json:"overall"`
	Buckets []BucketResult `json:"buckets"`
}

// Compute 按粒度分桶，并计算 overall 与每桶的 8 项指标。
//
// tzOffset 是用户相对 UTC 的偏移（东八区为 +8h）。
// 分桶在**用户本地时间**下进行：若按 UTC 分桶，当地 00:30 的练习会被算到前一天。
func Compute(g Grain, rounds []Round, tzOffset time.Duration) Result {
	buckets := group(g, rounds, tzOffset)

	// 空结果返回空切片而非 nil，使 JSON 序列化为 [] 而不是 null，
	// 客户端无需额外判空。
	out := make([]BucketResult, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, BucketResult{
			Key:     b.Key,
			Label:   b.Label,
			Metrics: ComputeMetrics(b.Rounds),
		})
	}
	return Result{
		Grain:   g,
		Overall: ComputeMetrics(rounds),
		Buckets: out,
	}
}

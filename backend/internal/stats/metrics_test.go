package stats

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// ---------------------------------------------------------------------------
// 中位数（A8 的核心）
// ---------------------------------------------------------------------------

func TestMedian(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		want float64
	}{
		{"空", nil, 0},
		{"单元素", []float64{7}, 7},
		{"奇数三个", []float64{1, 2, 3}, 2},
		{"奇数五个", []float64{5, 1, 3, 2, 4}, 3},
		// 偶数必须取中间两个的平均，而非下中位数
		{"偶数两个", []float64{1, 3}, 2},
		{"偶数四个", []float64{1, 2, 3, 4}, 2.5},
		{"偶数六个", []float64{10, 20, 30, 40, 50, 60}, 35},
		{"未排序输入", []float64{4, 1, 3, 2}, 2.5},
		{"含重复值", []float64{5, 5, 5, 5}, 5},
		{"浮点值", []float64{0.1, 0.2, 0.3, 0.4}, 0.25},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := median(append([]float64(nil), tc.in...))
			if !almost(got, tc.want) {
				t.Errorf("median(%v) = %v, 期望 %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestMedianEvenIsAverageNotLowerMiddle 明确区分「取平均」与「取下中位数」。
//
// 若实现只取 xs[n/2-1]，下面这些用例都会失败。这是最容易被写错的一处，
// 而且错了以后数字看起来「挺合理」，不会引人怀疑。
func TestMedianEvenIsAverageNotLowerMiddle(t *testing.T) {
	cases := []struct {
		in          []float64
		wantAverage float64
		lowerMiddle float64
	}{
		{[]float64{1, 3}, 2, 1},
		{[]float64{1, 2, 3, 4}, 2.5, 2},
		{[]float64{10, 20, 30, 40, 50, 60}, 35, 30},
	}
	for _, tc := range cases {
		got := median(append([]float64(nil), tc.in...))
		if !almost(got, tc.wantAverage) {
			t.Errorf("median(%v) = %v, 期望 %v（中间两个的平均）", tc.in, got, tc.wantAverage)
		}
		if almost(got, tc.lowerMiddle) && !almost(tc.wantAverage, tc.lowerMiddle) {
			t.Errorf("median(%v) 取到了下中位数 %v，应为 %v", tc.in, tc.lowerMiddle, tc.wantAverage)
		}
	}
}

// ---------------------------------------------------------------------------
// 8 项指标
// ---------------------------------------------------------------------------

func TestComputeMetricsEmpty(t *testing.T) {
	m := ComputeMetrics(nil)
	if m.RoundCount != 0 {
		t.Errorf("RoundCount = %d, 期望 0", m.RoundCount)
	}
	// 关键：不得出现 NaN —— 它会污染求和与中位数，让整页显示 NaN
	vals := map[string]float64{
		"TimeAvgMs": m.TimeAvgMs, "TimeMedianMs": m.TimeMedianMs,
		"AccuracyAvg": m.AccuracyAvg, "AccuracyMedian": m.AccuracyMedian,
	}
	for name, v := range vals {
		if math.IsNaN(v) {
			t.Errorf("%s 为 NaN", name)
		}
		if v != 0 {
			t.Errorf("%s = %v, 期望 0", name, v)
		}
	}
}

func TestComputeMetricsSingle(t *testing.T) {
	rounds := []Round{rnd("2026-09-10T10:00:00Z", 5000, 3, 4)}
	m := ComputeMetrics(rounds)

	if m.RoundCount != 1 {
		t.Fatalf("RoundCount = %d", m.RoundCount)
	}
	if m.TimeMaxMs != 5000 || m.TimeMinMs != 5000 {
		t.Errorf("单轮的 max/min 应等于该值: %d/%d", m.TimeMaxMs, m.TimeMinMs)
	}
	if !almost(m.TimeAvgMs, 5000) || !almost(m.TimeMedianMs, 5000) {
		t.Errorf("单轮的 avg/median 应等于该值: %v/%v", m.TimeAvgMs, m.TimeMedianMs)
	}
	if !almost(m.AccuracyMax, 0.75) || !almost(m.AccuracyMin, 0.75) {
		t.Errorf("正确率 max/min 应为 0.75: %v/%v", m.AccuracyMax, m.AccuracyMin)
	}
	if !almost(m.AccuracyAvg, 0.75) || !almost(m.AccuracyMedian, 0.75) {
		t.Errorf("正确率 avg/median 应为 0.75: %v/%v", m.AccuracyAvg, m.AccuracyMedian)
	}
}

func TestComputeMetricsOddCount(t *testing.T) {
	// 耗时 1000/3000/2000，正确率 1.0/0.5/0.75
	rounds := []Round{
		rnd("2026-09-10T10:00:00Z", 1000, 2, 2), // 1.0
		rnd("2026-09-10T11:00:00Z", 3000, 1, 2), // 0.5
		rnd("2026-09-10T12:00:00Z", 2000, 3, 4), // 0.75
	}
	m := ComputeMetrics(rounds)

	if m.TimeMaxMs != 3000 || m.TimeMinMs != 1000 {
		t.Errorf("耗时 max/min = %d/%d, 期望 3000/1000", m.TimeMaxMs, m.TimeMinMs)
	}
	if !almost(m.TimeAvgMs, 2000) {
		t.Errorf("耗时平均 = %v, 期望 2000", m.TimeAvgMs)
	}
	// 排序后 1000,2000,3000 -> 中位是 2000
	if !almost(m.TimeMedianMs, 2000) {
		t.Errorf("耗时中位 = %v, 期望 2000", m.TimeMedianMs)
	}

	if !almost(m.AccuracyMax, 1.0) || !almost(m.AccuracyMin, 0.5) {
		t.Errorf("正确率 max/min = %v/%v, 期望 1.0/0.5", m.AccuracyMax, m.AccuracyMin)
	}
	if !almost(m.AccuracyAvg, 0.75) {
		t.Errorf("正确率平均 = %v, 期望 0.75", m.AccuracyAvg)
	}
	// 排序后 0.5,0.75,1.0 -> 中位是 0.75
	if !almost(m.AccuracyMedian, 0.75) {
		t.Errorf("正确率中位 = %v, 期望 0.75", m.AccuracyMedian)
	}
}

// TestComputeMetricsEvenCount 覆盖 A8 点名的偶数长度情形。
func TestComputeMetricsEvenCount(t *testing.T) {
	// 耗时 1000, 2000, 3000, 4000 -> 中位 = 2500
	// 正确率 1.0, 0.5, 0.75, 0.25 -> 排序 0.25,0.5,0.75,1.0 -> 中位 = 0.625
	rounds := []Round{
		rnd("2026-09-10T10:00:00Z", 1000, 2, 2), // 1.0
		rnd("2026-09-10T11:00:00Z", 4000, 1, 4), // 0.25
		rnd("2026-09-10T12:00:00Z", 2000, 1, 2), // 0.5
		rnd("2026-09-10T13:00:00Z", 3000, 3, 4), // 0.75
	}
	m := ComputeMetrics(rounds)

	if m.RoundCount != 4 {
		t.Fatalf("RoundCount = %d", m.RoundCount)
	}
	if !almost(m.TimeMedianMs, 2500) {
		t.Errorf("耗时中位 = %v, 期望 2500（中间两个 2000 与 3000 的平均）", m.TimeMedianMs)
	}
	if !almost(m.TimeAvgMs, 2500) {
		t.Errorf("耗时平均 = %v, 期望 2500", m.TimeAvgMs)
	}
	if !almost(m.AccuracyMedian, 0.625) {
		t.Errorf("正确率中位 = %v, 期望 0.625（0.5 与 0.75 的平均）", m.AccuracyMedian)
	}
	if !almost(m.AccuracyAvg, 0.625) {
		t.Errorf("正确率平均 = %v, 期望 0.625", m.AccuracyAvg)
	}
	if m.TimeMaxMs != 4000 || m.TimeMinMs != 1000 {
		t.Errorf("耗时 max/min = %d/%d", m.TimeMaxMs, m.TimeMinMs)
	}
	if !almost(m.AccuracyMax, 1.0) || !almost(m.AccuracyMin, 0.25) {
		t.Errorf("正确率 max/min = %v/%v", m.AccuracyMax, m.AccuracyMin)
	}
}

// TestAccuracyAverageIsMeanOfRoundAccuracies 固定「平均正确率」的语义。
//
// 规格 §8.1：对桶内每一轮先取轮正确率，再聚合。
// 因此是**各轮正确率的平均**，而非「总正确数 / 总题数」。
// 两者在每轮题数相同时相等；题数不同时不同。本测试用不同题数区分二者。
func TestAccuracyAverageIsMeanOfRoundAccuracies(t *testing.T) {
	rounds := []Round{
		rnd("2026-09-10T10:00:00Z", 1000, 1, 1), // 1.0（1 题）
		rnd("2026-09-10T11:00:00Z", 1000, 0, 9), // 0.0（9 题）
	}
	m := ComputeMetrics(rounds)

	// 各轮正确率的平均 = (1.0 + 0.0) / 2 = 0.5
	meanOfAccuracies := 0.5
	// 总正确数/总题数 = 1/10 = 0.1
	pooledAccuracy := 0.1

	if !almost(m.AccuracyAvg, meanOfAccuracies) {
		t.Errorf("平均正确率 = %v, 期望 %v（各轮正确率的平均）", m.AccuracyAvg, meanOfAccuracies)
	}
	if almost(m.AccuracyAvg, pooledAccuracy) {
		t.Errorf("平均正确率被算成了总正确数/总题数（%v），规格 §8.1 要求各轮平均", pooledAccuracy)
	}
	// 中位数同样基于各轮正确率：排序 0.0, 1.0 -> 0.5
	if !almost(m.AccuracyMedian, 0.5) {
		t.Errorf("正确率中位 = %v, 期望 0.5", m.AccuracyMedian)
	}
}

func TestRoundAccuracyZeroQuestions(t *testing.T) {
	r := Round{CorrectCount: 0, QuestionCount: 0}
	if got := r.Accuracy(); got != 0 {
		t.Errorf("题数为 0 时应返回 0（而非 NaN）, 得到 %v", got)
	}
	if math.IsNaN(r.Accuracy()) {
		t.Error("不得返回 NaN")
	}
}

func TestComputeMetricsUnaffectedByInputOrder(t *testing.T) {
	ordered := []Round{
		rnd("2026-09-10T10:00:00Z", 1000, 1, 2),
		rnd("2026-09-10T11:00:00Z", 2000, 1, 2),
		rnd("2026-09-10T12:00:00Z", 3000, 1, 2),
		rnd("2026-09-10T13:00:00Z", 4000, 1, 2),
	}
	shuffled := []Round{ordered[2], ordered[0], ordered[3], ordered[1]}

	a, b := ComputeMetrics(ordered), ComputeMetrics(shuffled)
	if a != b {
		t.Errorf("结果不应依赖输入顺序:\n  %+v\n  %+v", a, b)
	}
}

func TestComputeMetricsZeroTimeRound(t *testing.T) {
	// 耗时可以为 0（例如极端快速或数据异常），不应导致除零或 NaN
	rounds := []Round{
		rnd("2026-09-10T10:00:00Z", 0, 1, 1),
		rnd("2026-09-10T11:00:00Z", 1000, 1, 1),
	}
	m := ComputeMetrics(rounds)
	if m.TimeMinMs != 0 {
		t.Errorf("TimeMinMs = %d, 期望 0", m.TimeMinMs)
	}
	if math.IsNaN(m.TimeAvgMs) || math.IsNaN(m.TimeMedianMs) {
		t.Error("不得出现 NaN")
	}
	if !almost(m.TimeMedianMs, 500) {
		t.Errorf("中位 = %v, 期望 500", m.TimeMedianMs)
	}
}

// ---------------------------------------------------------------------------
// P5.3 编排
// ---------------------------------------------------------------------------

func TestComputeOverallAndBuckets(t *testing.T) {
	rounds := []Round{
		rnd("2026-09-10T10:00:00Z", 1000, 1, 1), // 9/10
		rnd("2026-09-10T11:00:00Z", 3000, 1, 2), // 9/10
		rnd("2026-09-11T10:00:00Z", 2000, 2, 2), // 9/11
	}

	res := Compute(GrainDay, rounds, 0)

	if res.Grain != GrainDay {
		t.Errorf("Grain = %q", res.Grain)
	}
	if res.Overall.RoundCount != 3 {
		t.Errorf("overall.RoundCount = %d, 期望 3", res.Overall.RoundCount)
	}
	if len(res.Buckets) != 2 {
		t.Fatalf("应有 2 个桶, 得到 %d", len(res.Buckets))
	}
	if res.Buckets[0].Metrics.RoundCount != 2 {
		t.Errorf("9/10 桶应有 2 轮, 得到 %d", res.Buckets[0].Metrics.RoundCount)
	}
	if res.Buckets[1].Metrics.RoundCount != 1 {
		t.Errorf("9/11 桶应有 1 轮, 得到 %d", res.Buckets[1].Metrics.RoundCount)
	}
	// 每桶的中位数
	if !almost(res.Buckets[0].Metrics.TimeMedianMs, 2000) {
		t.Errorf("9/10 桶耗时中位 = %v, 期望 2000", res.Buckets[0].Metrics.TimeMedianMs)
	}
}

func TestComputeEmptyReturnsEmptySlice(t *testing.T) {
	res := Compute(GrainDay, nil, 0)
	if res.Buckets == nil {
		t.Error("buckets 应为空切片而非 nil，否则 JSON 会序列化为 null")
	}
	if len(res.Buckets) != 0 {
		t.Errorf("应为 0 个桶, 得到 %d", len(res.Buckets))
	}
	if res.Overall.RoundCount != 0 {
		t.Errorf("overall.RoundCount = %d", res.Overall.RoundCount)
	}
}

package stats

import "sort"

// Metrics 是一组轮次的 8 项指标。
//
// 按规格 §8.2 的 UI 分成两组（耗时 / 正确率），各 4 项。
// 用独立字段而非数组，使 JSON 契约自解释，也避免顺序被误解。
type Metrics struct {
	RoundCount int `json:"roundCount"`

	TimeMaxMs    int64   `json:"timeMaxMs"`
	TimeMinMs    int64   `json:"timeMinMs"`
	TimeAvgMs    float64 `json:"timeAvgMs"`
	TimeMedianMs float64 `json:"timeMedianMs"`

	// 正确率为 0..1 的比例，与轮次记录中的 correctCount/questionCount 一致。
	AccuracyMax    float64 `json:"accuracyMax"`
	AccuracyMin    float64 `json:"accuracyMin"`
	AccuracyAvg    float64 `json:"accuracyAvg"`
	AccuracyMedian float64 `json:"accuracyMedian"`
}

// ComputeMetrics 计算一组轮次的 8 项指标。
//
// 空输入返回全零（RoundCount=0），调用方据此判断「无数据」，
// 而不是让平均值出现 NaN——NaN 会污染求和与中位数。
func ComputeMetrics(rounds []Round) Metrics {
	if len(rounds) == 0 {
		return Metrics{}
	}

	times := make([]float64, 0, len(rounds))
	accs := make([]float64, 0, len(rounds))

	var timeMax, timeMin int64
	var timeSum, accSum float64
	first := true

	for _, r := range rounds {
		ms := float64(r.TotalMs)
		acc := r.Accuracy()

		times = append(times, ms)
		accs = append(accs, acc)
		timeSum += ms
		accSum += acc

		if first || r.TotalMs > timeMax {
			timeMax = r.TotalMs
		}
		if first || r.TotalMs < timeMin {
			timeMin = r.TotalMs
		}
		first = false
	}

	n := float64(len(rounds))
	return Metrics{
		RoundCount: len(rounds),

		TimeMaxMs:    timeMax,
		TimeMinMs:    timeMin,
		TimeAvgMs:    timeSum / n,
		TimeMedianMs: median(times),

		AccuracyMax:    maxFloat(accs),
		AccuracyMin:    minFloat(accs),
		AccuracyAvg:    accSum / n,
		AccuracyMedian: median(accs),
	}
}

// median 返回中位数。
//
// 必须区分奇偶长度：
//   - 奇数：正中间那个
//   - 偶数：中间两个的**平均**
//
// 忽略偶数分支（只取下中位数）会让偶数样本量的中位数系统性偏小。
//
// 输入会被就地排序——调用方传入的都是本包内部构造的临时切片。
func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sort.Float64s(xs)
	n := len(xs)
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}

func maxFloat(xs []float64) float64 {
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func minFloat(xs []float64) float64 {
	m := xs[0]
	for _, x := range xs[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

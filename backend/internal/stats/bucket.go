// Package stats 是统计聚合的唯一 owner。
//
// 本包不依赖本仓库其他包，只接受轻量输入（见 Round），因此可独立测试。
//
// 关于浮点：规格 §5.5.3 禁止**判分**路径出现浮点，理由是跨端精确性
// （Go 与 Dart 必须逐例一致）。统计不满足该前提 —— 它只在服务端计算，
// 客户端仅渲染返回值，没有跨端契约。因此本包允许使用 float64：
// 中位数与平均正确率本质是除法，用有理数表示只会增加复杂度而无收益。
package stats

import (
	"fmt"
	"sort"
	"time"
)

// Grain 是时间粒度。
type Grain string

const (
	// GrainDay 按自然日分桶。
	GrainDay Grain = "day"
	// GrainWeek 按星期几（周一…周日）跨日期聚合。
	//
	// 这一粒度回答的是「我哪天最快」，而不是「哪一周最快」——
	// 规格 §8.1 的原话是「按星期几跨日期聚合」。
	GrainWeek Grain = "week"
	// GrainMonth 按自然月分桶。
	GrainMonth Grain = "month"
)

// ParseGrain 解析粒度，非法值返回错误。
func ParseGrain(s string) (Grain, error) {
	switch Grain(s) {
	case GrainDay:
		return GrainDay, nil
	case GrainWeek:
		return GrainWeek, nil
	case GrainMonth:
		return GrainMonth, nil
	default:
		return "", fmt.Errorf("未知的时间粒度 %q（应为 day / week / month）", s)
	}
}

// weekdayNames 按「周一=0 … 周日=6」排列。
var weekdayNames = [...]string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}

// weekdayOrder 把 time.Weekday 映射为「周一=0 … 周日=6」。
//
// Go 的 time.Weekday 是「周日=0」，与中文习惯的「周一是一周之始」不同，
// 直接使用会让折线图从周日开始。
func weekdayOrder(w time.Weekday) int {
	return (int(w) + 6) % 7
}

// bucketKey 返回某个时刻在给定粒度下的桶键、排序序号与显示标签。
//
// local 必须已经过时区偏移转换：调用方负责把 UTC 时间转为用户的本地时间。
// 这一点很重要——若按 UTC 分桶，用户在当地 00:30 做的练习会被算到前一天，
// 「今天的练习」在他自己看来就是错的。
func bucketKey(g Grain, local time.Time) (key string, order int, label string) {
	switch g {
	case GrainDay:
		// 用「距 1970 的天数」排序，避免依赖字符串比较在跨年时的巧合
		return local.Format("2006-01-02"),
			int(local.Unix() / 86400),
			fmt.Sprintf("%d/%d", int(local.Month()), local.Day())

	case GrainWeek:
		o := weekdayOrder(local.Weekday())
		return fmt.Sprintf("dow-%d", o), o, weekdayNames[o]

	case GrainMonth:
		return local.Format("2006-01"),
			local.Year()*12 + int(local.Month()) - 1,
			fmt.Sprintf("%d年%d月", local.Year(), int(local.Month()))

	default:
		return local.Format("2006-01-02"),
			int(local.Unix() / 86400),
			local.Format("2006-01-02")
	}
}

// Bucket 是一个已分组的桶（尚未计算指标）。
type Bucket struct {
	Key    string
	Label  string
	Order  int
	Rounds []Round
}

// group 按粒度分桶并按时间顺序排序。
//
// 只返回**有数据的桶**：空桶在折线图上会被画成 0，
// 而「那天没练习」与「那天耗时 0 秒」是两回事。
func group(g Grain, rounds []Round, tzOffset time.Duration) []Bucket {
	byKey := map[string]*Bucket{}
	for _, r := range rounds {
		local := r.FinishedAt.UTC().Add(tzOffset)
		key, order, label := bucketKey(g, local)
		b, ok := byKey[key]
		if !ok {
			b = &Bucket{Key: key, Label: label, Order: order}
			byKey[key] = b
		}
		b.Rounds = append(b.Rounds, r)
	}

	out := make([]Bucket, 0, len(byKey))
	for _, b := range byKey {
		out = append(out, *b)
	}
	// 按时间顺序排列（周一在前、日期升序、月份升序）
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Key < out[j].Key
	})
	return out
}

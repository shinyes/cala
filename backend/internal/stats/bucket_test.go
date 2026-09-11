package stats

import (
	"testing"
	"time"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func rnd(finished string, ms int64, correct, total int) Round {
	return Round{
		FinishedAt:    at(finished),
		TotalMs:       ms,
		CorrectCount:  correct,
		QuestionCount: total,
	}
}

// ---------------------------------------------------------------------------
// P5.1 分桶
// ---------------------------------------------------------------------------

func TestParseGrain(t *testing.T) {
	for _, ok := range []string{"day", "week", "month"} {
		if _, err := ParseGrain(ok); err != nil {
			t.Errorf("ParseGrain(%q) 应通过, 得到 %v", ok, err)
		}
	}
	for _, bad := range []string{"", "year", "DAY", "d", "日"} {
		if _, err := ParseGrain(bad); err == nil {
			t.Errorf("ParseGrain(%q) 应报错", bad)
		}
	}
}

func TestGroupDay(t *testing.T) {
	rounds := []Round{
		rnd("2026-09-10T01:00:00Z", 1000, 1, 1),
		rnd("2026-09-10T23:00:00Z", 2000, 1, 1),
		rnd("2026-09-11T05:00:00Z", 3000, 1, 1),
	}

	bs := group(GrainDay, rounds, 0)
	if len(bs) != 2 {
		t.Fatalf("应分成 2 个自然日, 得到 %d", len(bs))
	}
	if len(bs[0].Rounds) != 2 {
		t.Errorf("9/10 应有 2 轮, 得到 %d", len(bs[0].Rounds))
	}
	if bs[0].Key != "2026-09-10" || bs[1].Key != "2026-09-11" {
		t.Errorf("桶键或顺序不对: %q, %q", bs[0].Key, bs[1].Key)
	}
	if bs[0].Label != "9/10" {
		t.Errorf("标签应为 9/10, 得到 %q", bs[0].Label)
	}
}

// TestGroupDayRespectsTimezone 是 R-P5-2 的核心断言。
//
// UTC 16:30 在东八区是**次日** 00:30。若按 UTC 分桶，用户在当地凌晨做的练习
// 会被算到前一天，「今天的练习」在他自己看来就是错的。
func TestGroupDayRespectsTimezone(t *testing.T) {
	// 2026-09-10T16:30Z == 2026-09-11T00:30+08:00
	rounds := []Round{rnd("2026-09-10T16:30:00Z", 1000, 1, 1)}

	t.Run("UTC 下属于 9/10", func(t *testing.T) {
		bs := group(GrainDay, rounds, 0)
		if len(bs) != 1 || bs[0].Key != "2026-09-10" {
			t.Fatalf("UTC 分桶应为 2026-09-10, 得到 %+v", keys(bs))
		}
	})

	t.Run("东八区下属于 9/11", func(t *testing.T) {
		bs := group(GrainDay, rounds, 8*time.Hour)
		if len(bs) != 1 || bs[0].Key != "2026-09-11" {
			t.Fatalf("东八区分桶应为 2026-09-11, 得到 %+v", keys(bs))
		}
	})

	t.Run("西五区下属于 9/10 的更早时段", func(t *testing.T) {
		// 2026-09-10T16:30Z == 2026-09-10T11:30-05:00
		bs := group(GrainDay, rounds, -5*time.Hour)
		if len(bs) != 1 || bs[0].Key != "2026-09-10" {
			t.Fatalf("西五区分桶应为 2026-09-10, 得到 %+v", keys(bs))
		}
	})

	t.Run("时区偏移可让同一天的两轮分到两天", func(t *testing.T) {
		// 23:00Z 与次日 01:00Z 在 UTC 下是两天；在东八区是 07:00 与 09:00（同一天）
		two := []Round{
			rnd("2026-09-10T23:00:00Z", 1000, 1, 1),
			rnd("2026-09-11T01:00:00Z", 1000, 1, 1),
		}
		if bs := group(GrainDay, two, 0); len(bs) != 2 {
			t.Errorf("UTC 下应为 2 天, 得到 %d", len(bs))
		}
		if bs := group(GrainDay, two, 8*time.Hour); len(bs) != 1 {
			t.Errorf("东八区下应为 1 天（07:00 与 09:00）, 得到 %d", len(bs))
		}
	})
}

func TestGroupDayAcrossYearBoundary(t *testing.T) {
	rounds := []Round{
		rnd("2026-01-01T10:00:00Z", 1000, 1, 1),
		rnd("2025-12-31T10:00:00Z", 1000, 1, 1),
		rnd("2025-12-30T10:00:00Z", 1000, 1, 1),
	}

	bs := group(GrainDay, rounds, 0)
	if len(bs) != 3 {
		t.Fatalf("应有 3 个桶, 得到 %d", len(bs))
	}
	// 顺序必须是时间升序，而非字符串顺序
	want := []string{"2025-12-30", "2025-12-31", "2026-01-01"}
	for i, w := range want {
		if bs[i].Key != w {
			t.Errorf("第 %d 个桶应为 %s, 得到 %s", i, w, bs[i].Key)
		}
	}
}

func TestGroupWeek(t *testing.T) {
	// 2026-09-09 是周三，2026-09-16 也是周三；2026-09-11 是周五
	rounds := []Round{
		rnd("2026-09-09T10:00:00Z", 1000, 1, 1), // 周三
		rnd("2026-09-16T10:00:00Z", 2000, 1, 1), // 周三
		rnd("2026-09-11T10:00:00Z", 3000, 1, 1), // 周五
	}

	bs := group(GrainWeek, rounds, 0)
	if len(bs) != 2 {
		t.Fatalf("应分成 2 个星期几, 得到 %d", len(bs))
	}
	// 周三在周五之前
	if bs[0].Label != "周三" || bs[1].Label != "周五" {
		t.Errorf("顺序应为 周三, 周五, 得到 %q, %q", bs[0].Label, bs[1].Label)
	}
	if len(bs[0].Rounds) != 2 {
		t.Errorf("不同日期的两个周三应聚合到一起, 得到 %d 轮", len(bs[0].Rounds))
	}
}

// TestGroupWeekStartsOnMonday 保证顺序是「周一→周日」而非 Go 默认的「周日→周一」。
func TestGroupWeekStartsOnMonday(t *testing.T) {
	// 2026-09-07 周一 … 2026-09-13 周日
	days := []struct {
		date  string
		label string
	}{
		{"2026-09-13", "周日"},
		{"2026-09-09", "周三"},
		{"2026-09-07", "周一"},
		{"2026-09-12", "周六"},
		{"2026-09-08", "周二"},
		{"2026-09-11", "周五"},
		{"2026-09-10", "周四"},
	}
	rounds := make([]Round, 0, len(days))
	for _, d := range days {
		rounds = append(rounds, rnd(d.date+"T10:00:00Z", 1000, 1, 1))
	}

	bs := group(GrainWeek, rounds, 0)
	if len(bs) != 7 {
		t.Fatalf("应有 7 个桶, 得到 %d", len(bs))
	}
	want := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	for i, w := range want {
		if bs[i].Label != w {
			t.Errorf("第 %d 个桶应为 %s, 得到 %s（顺序不应受传入顺序影响）", i, w, bs[i].Label)
		}
	}
}

// TestGroupWeekOmitsEmptyDays 保证只返回有数据的桶。
//
// 「那天没练习」与「那天耗时 0 秒」是两回事，空桶会被折线图画成 0。
func TestGroupWeekOmitsEmptyDays(t *testing.T) {
	rounds := []Round{
		rnd("2026-09-07T10:00:00Z", 1000, 1, 1), // 周一
		rnd("2026-09-13T10:00:00Z", 1000, 1, 1), // 周日
	}
	bs := group(GrainWeek, rounds, 0)
	if len(bs) != 2 {
		t.Fatalf("只有周一周日有数据, 应返回 2 个桶, 得到 %d", len(bs))
	}
	if bs[0].Label != "周一" || bs[1].Label != "周日" {
		t.Errorf("得到 %q, %q", bs[0].Label, bs[1].Label)
	}
}

func TestGroupMonth(t *testing.T) {
	rounds := []Round{
		rnd("2026-09-01T10:00:00Z", 1000, 1, 1),
		rnd("2026-09-30T10:00:00Z", 1000, 1, 1),
		rnd("2026-08-15T10:00:00Z", 1000, 1, 1),
	}

	bs := group(GrainMonth, rounds, 0)
	if len(bs) != 2 {
		t.Fatalf("应有 2 个月, 得到 %d", len(bs))
	}
	if bs[0].Key != "2026-08" || bs[1].Key != "2026-09" {
		t.Errorf("桶键或顺序不对: %q, %q", bs[0].Key, bs[1].Key)
	}
	if len(bs[1].Rounds) != 2 {
		t.Errorf("9 月应有 2 轮, 得到 %d", len(bs[1].Rounds))
	}
}

func TestGroupMonthAcrossYearBoundary(t *testing.T) {
	rounds := []Round{
		rnd("2026-01-15T10:00:00Z", 1000, 1, 1),
		rnd("2025-12-15T10:00:00Z", 1000, 1, 1),
		rnd("2025-11-15T10:00:00Z", 1000, 1, 1),
	}

	bs := group(GrainMonth, rounds, 0)
	want := []string{"2025-11", "2025-12", "2026-01"}
	if len(bs) != 3 {
		t.Fatalf("应有 3 个月, 得到 %d", len(bs))
	}
	for i, w := range want {
		if bs[i].Key != w {
			t.Errorf("第 %d 个桶应为 %s, 得到 %s", i, w, bs[i].Key)
		}
	}
}

func TestGroupEmptyInput(t *testing.T) {
	for _, g := range []Grain{GrainDay, GrainWeek, GrainMonth} {
		bs := group(g, nil, 0)
		if bs == nil {
			t.Errorf("%s: 空输入应返回空切片而非 nil", g)
		}
		if len(bs) != 0 {
			t.Errorf("%s: 空输入应得到 0 个桶, 得到 %d", g, len(bs))
		}
	}
}

func keys(bs []Bucket) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, b.Key)
	}
	return out
}

package rules

// Question 是一道已生成的题目。
type Question struct {
	// Index 是题号，从 0 开始，与 attempt.idx 对应。
	Index int
	// Q 是题面，A 是答案原文。
	//
	// P3 会把两者作为**快照**落库（规格 §6.1(3)），使作者事后修改规则
	// 不会篡改历史错题，也让「重练错题」无需重新出题。
	Q string
	A string
}

// Generate 生成一轮的题目。
//
// seed 由调用方（P3）为每轮随机生成并落库。同一 (rule, cfg, seed, count)
// 必须产出完全相同的结果（验收标准 A3）——这是「重练错题」与线上问题复现的基础。
//
// 注意：每道题在各自的 VM 中执行（见 sandbox.go 的取舍说明），
// 因此规则的模块级状态不会在题目之间保留；随机序列则由共享的 rng 延续。
func (r *Rule) Generate(cfg map[string]any, seed int64, count int) ([]Question, error) {
	if count <= 0 {
		return nil, newErr(StageShape, "题数必须为正数，得到 %d", count)
	}

	// 随机源在整轮内共享，使题目之间延续同一条确定性序列；
	// 时间偏移同样由 seed 决定，保证整轮看到同一个冻结时刻。
	rng := newSeededSource(seed)
	offset := seed % secondsPerDay
	if offset < 0 {
		offset += secondsPerDay
	}

	out := make([]Question, 0, count)
	for i := 0; i < count; i++ {
		vm, err := newVMWithSource(rng, offset)
		if err != nil {
			return nil, newErr(StageExecute, "创建运行时失败: %s", firstLine(err.Error()))
		}
		v, err := callGenerate(vm, r.prog, cfg)
		if err != nil {
			return nil, err
		}
		if err := checkShape(v); err != nil {
			return nil, err
		}
		obj := v.(map[string]any)
		out = append(out, Question{
			Index: i,
			Q:     obj["q"].(string),
			A:     obj["a"].(string),
		})
	}
	return out, nil
}

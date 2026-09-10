// Package rules 是出题规则编译与沙箱执行的唯一 owner。
//
// 本包执行的是**不可信代码**（用户编写的 JS）。沙箱的各项配置是安全边界，
// 集中声明在本文件，不得散落到调用方——配置一旦出现第二份拷贝，
// 就会出现「一处设了、一处漏了」的缺口，而漏掉递归上限会挂死整个进程。
package rules

import (
	"time"

	"github.com/dop251/goja"
)

const (
	// MaxCallStackSize 限制 JS 递归深度。
	//
	// 这不是可选优化：实测表明**不设置时，一条无限递归的规则会把整个 Go 进程挂死**
	// （跑满 300s 未返回）。goja 默认的栈限制远高于此，且耗尽时进程无法恢复。
	MaxCallStackSize = 200

	// QuestionTimeout 是单次 generate 调用的墙钟预算。
	// 实测：计时器设为 60ms 时，while(true) 在 61ms 被击杀，且 ClearInterrupt 后
	// VM 仍可复用。生产预算取 50ms。
	QuestionTimeout = 50 * time.Millisecond

	// ValidationRuns 是保存前连续调用 generate 的次数（规格 §5.3 第 3 步）。
	// 取 20 是为了让「只在特定分支出错」的规则有较高概率暴露。
	ValidationRuns = 20

	// MaxQuestionLen / MaxAnswerLen 限制单个题面与答案的长度（按 rune 计）。
	//
	// 针对规格 §14 R3：goja 无内存上限。规则的**计算量**由超时与递归深度约束，
	// 但**返回值大小**只能由宿主控制；一条 return {q: "x".repeat(1e9)} 的规则
	// 会把内存与磁盘一起吃掉。这里给一个宽松但有限的上界。
	MaxQuestionLen = 1000
	MaxAnswerLen   = 500

	// timeBase 是冻结时间源的基准时刻（2020-01-01T00:00:00Z 的 Unix 秒）。
	// 实际返回值 = timeBase + (seed mod 86400) 秒，使依赖 Date 的规则
	// 在同一轮内**可复现**、在不同轮之间**仍有变化**。
	timeBase = 1577836800

	// secondsPerDay 用于把种子映射到一天之内。
	secondsPerDay = 86400
)

// newVM 构造一个沙箱化的 goja 运行时。
//
// seed 决定 Math.random() 与 Date 的取值，从而决定本轮题目的可复现性。
func newVM(seed int64) (*goja.Runtime, error) {
	offset := seed % secondsPerDay
	if offset < 0 {
		offset += secondsPerDay
	}
	return newVMWithSource(newSeededSource(seed), offset)
}

// newVMWithSource 是沙箱配置的**唯一实现**。
//
// newVM 与 Generate 都经过它，因此四项配置只写一次，不存在漂移风险。
// rng 可跨多个 VM 共享（Generate 用它让整轮延续同一条随机序列）。
func newVMWithSource(rng *seededSource, dayOffset int64) (*goja.Runtime, error) {
	vm := goja.New()

	// 配置 1/4：递归深度上限。见 MaxCallStackSize 的说明——缺此项会挂死进程。
	vm.SetMaxCallStackSize(MaxCallStackSize)

	// 配置 2/4：确定性随机。同种子两次生成必须逐字相同（A3）。
	vm.SetRandSource(func() float64 { return rng.Float64() })

	// 配置 3/4：确定性时间。避免 new Date() 破坏可复现性（规格 §5.2）。
	frozen := time.Unix(timeBase+dayOffset, 0).UTC()
	vm.SetTimeSource(func() time.Time { return frozen })

	// 配置 4/4：调用超时。由 callGenerate 在每次调用时施加（Interrupt），
	// 因为超时是**每次调用**的属性，不是 VM 的属性。

	return vm, nil
}

// 设计说明：为什么每道题新建 VM，而不是复用同一个 VM 并逐题 Interrupt？
//
// 复用 VM 需要在每次调用前后管理 time.AfterFunc + Interrupt + ClearInterrupt，
// 这里存在一个难以彻底消除的竞态：AfterFunc 的 goroutine 可能在调用返回、
// ClearInterrupt 之后才真正执行 Interrupt，于是这次超时中断会**误杀下一道题**，
// 表现为间歇性、极难复现的失败。
//
// 新建 VM 的成本实测仅 23.7µs/题（一轮 50 题约 1.2ms；复用为 8.8µs/题），
// 差异不足以支撑上述竞态的管理复杂度。用可忽略的冷启动成本换掉一整类竞态。
//
// 由此得到一个**明确的作者契约**：generate 必须是 cfg 与随机数的纯函数，
// 模块级状态不会在题目之间保留（每个 VM 都重新载入一次 Program）。
// 这与 D1 选择的最小契约（只写一个函数、不接收题号）一致，也便于作者推理。
//
// 代价（诚实记录）：模块级初始化会在每道题重复执行。对纯计算型的规则
// （速算题的全部场景）耗时可忽略；若日后确有规则需要昂贵的一次性预计算，
// 应改为「每轮一个 VM + 调用前先 ClearInterrupt」并配套压力测试，而不是在这里猜测。

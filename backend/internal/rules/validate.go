package rules

import (
	"errors"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// Validate 连续调用 generate 共 ValidationRuns 次，检查每次调用的
// 超时、异常与返回形状。任一次失败即返回 *ValidationError。
//
// 这是基线 §5.2(2) 的载体：作者改规则会**同步影响所有订阅者**（功能5），
// 因此一条坏规则必须在保存时就被挡住，否则会连坐全员。
//
// cfg 应为**即将保存的**配置，使校验反映真实使用条件。
func (r *Rule) Validate(cfg map[string]any) error {
	for i := 0; i < ValidationRuns; i++ {
		// 每题用不同但固定的种子，使校验本身可复现（便于把作者报的错误重现出来）
		vm, err := newVM(int64(i))
		if err != nil {
			return newErr(StageExecute, "创建运行时失败: %s", firstLine(err.Error()))
		}
		v, err := callGenerate(vm, r.prog, cfg)
		if err != nil {
			return err
		}
		if err := checkShape(v); err != nil {
			return err
		}
	}
	return nil
}

// callGenerate 在给定 VM 上执行一次 generate(cfg)，带超时与 panic 隔离。
//
// 返回值为 Export 后的 Go 值（JS 对象会成为 map[string]any）。
func callGenerate(vm *goja.Runtime, prog *goja.Program, cfg map[string]any) (any, error) {
	if _, err := vm.RunProgram(prog); err != nil {
		return nil, newErr(StageExecute, "载入规则失败: %s", firstLine(err.Error()))
	}
	fn, ok := goja.AssertFunction(vm.Get("generate"))
	if !ok {
		// Compile 已保证过；此处属防御，说明 Program 与 VM 不匹配
		return nil, newErr(StageExecute, "generate 不可调用")
	}

	var (
		result  any
		callErr error
	)

	// panic 隔离：本包执行的是不可信代码。goja 通常把 JS 异常转成 error，
	// 但宿主侧的极端情况（如栈耗尽）可能以 panic 形式冒出。规则引擎作为
	// 不可信代码的边界，不应让一次规则执行杀死整个服务。
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				callErr = newErr(StageExecute, "规则执行引发内部异常: %v", rec)
			}
		}()

		timer := time.AfterFunc(QuestionTimeout, func() {
			vm.Interrupt("rule timeout")
		})
		defer timer.Stop()

		v, err := fn(goja.Undefined(), vm.ToValue(cfg))
		if err != nil {
			callErr = classifyCallError(err)
			return
		}
		result = v.Export()
	}()

	// 无论成功与否都清掉中断标记。虽然本包的用法是「每题一个新 VM」，
	// 调用方不应依赖残留状态，这里显式收尾以免日后改为复用 VM 时留下隐患。
	vm.ClearInterrupt()

	if callErr != nil {
		return nil, callErr
	}
	return result, nil
}

// classifyCallError 把 goja 的错误区分为「超时」「递归过深」「规则自身抛错」。
//
// 区分很重要，因为三者对作者的行动指引不同：
//   - 超时     -> 检查死循环或过重的计算
//   - 递归过深 -> 检查无限递归（本沙箱把递归深度限制在 MaxCallStackSize）
//   - 抛错     -> 检查规则自身的 bug
func classifyCallError(err error) error {
	var ie *goja.InterruptedError
	if errors.As(err, &ie) {
		return newErr(StageTimeout,
			"单次出题超过 %s 未返回（中断原因: %v）；请检查是否存在死循环或过重的计算",
			QuestionTimeout, ie.Value())
	}

	// goja 把栈耗尽表示为 *StackOverflowError，且其 Error() 为空字符串。
	// 若不单独识别，作者只会看到「抛出异常: 」这样没有信息量的提示。
	var soe *goja.StackOverflowError
	if errors.As(err, &soe) {
		return newErr(StageExecute,
			"递归过深（本沙箱上限 %d 层）；请检查是否存在无限递归或过深的调用链",
			MaxCallStackSize)
	}

	detail := firstLine(err.Error())
	if strings.TrimSpace(detail) == "" {
		detail = "规则抛出了一个没有说明信息的错误"
	}
	return newErr(StageExecute, "generate 抛出异常: %s", detail)
}

// checkShape 校验返回值符合 {q, a} 契约（D11）。
func checkShape(v any) error {
	obj, ok := v.(map[string]any)
	if !ok {
		return newErr(StageShape,
			"generate 必须返回对象 {q: string, a: string}，实际返回 %T", v)
	}

	q, ok := obj["q"].(string)
	if !ok {
		return newErr(StageShape, "返回值缺少字符串字段 q（题面）")
	}
	a, ok := obj["a"].(string)
	if !ok {
		return newErr(StageShape, "返回值缺少字符串字段 a（答案）")
	}
	if q == "" {
		return newErr(StageShape, "题面 q 不能为空字符串")
	}
	if a == "" {
		return newErr(StageShape, "答案 a 不能为空字符串")
	}
	if n := len([]rune(q)); n > MaxQuestionLen {
		return newErr(StageShape, "题面 q 过长（%d 个字符，上限 %d）", n, MaxQuestionLen)
	}
	if n := len([]rune(a)); n > MaxAnswerLen {
		return newErr(StageShape, "答案 a 过长（%d 个字符，上限 %d）", n, MaxAnswerLen)
	}
	return nil
}

package rules

import "github.com/dop251/goja"

// Rule 是一条已通过语法与顶层检查的出题规则。
//
// prog 可跨 VM 复用（goja.Program 是不可变的），这是「编译一次、多轮执行」的基础。
type Rule struct {
	source string
	prog   *goja.Program
}

// Source 返回作者可见的原始源码。
func (r *Rule) Source() string { return r.source }

// Compile 检查源码语法、执行顶层语句、并确认 generate 是可调用函数。
//
// 它**不**校验 generate 的返回形状——那需要真正调用，由 Validate 完成。
// 分开的理由：Compile 很便宜且无副作用，可在每次加载项目时执行；
// Validate 会真的以作者代码出题 20 次，只在保存时执行一次。
func Compile(source string) (*Rule, error) {
	if source == "" {
		return nil, newErr(StageCompile, "规则源码为空")
	}

	prog, err := goja.Compile("rule.js", source, false)
	if err != nil {
		return nil, newErr(StageCompile, "语法错误: %s", firstLine(err.Error()))
	}

	// 必须在 VM 中执行一次，才能发现顶层抛错（例如 throw new Error("x")），
	// 并确认 generate 真的存在。用固定的种子，使本步骤本身也可复现。
	vm, err := newVM(0)
	if err != nil {
		return nil, newErr(StageCompile, "创建运行时失败: %s", firstLine(err.Error()))
	}
	if _, err := vm.RunProgram(prog); err != nil {
		return nil, newErr(StageCompile, "顶层执行失败: %s", firstLine(err.Error()))
	}

	// 注意：RunProgram 返回的是**脚本完成值**。函数声明语句的完成值是 undefined，
	// 因此不能拿它的返回值当函数，必须用 vm.Get 取具名绑定。
	if _, ok := goja.AssertFunction(vm.Get("generate")); !ok {
		return nil, newErr(StageCompile,
			"未找到可调用的 generate 函数；规则必须定义 function generate(cfg) { ... }")
	}

	return &Rule{source: source, prog: prog}, nil
}

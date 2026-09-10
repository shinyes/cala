package rules

import "fmt"

// 校验阶段。API 层据此决定错误文案，也便于测试定位。
const (
	// StageCompile 语法错误、顶层执行抛错、或没有可调用的 generate。
	StageCompile = "compile"
	// StageExecute 调用 generate 时抛出异常。
	StageExecute = "execute"
	// StageTimeout 调用超过预算未返回（死循环等）。
	StageTimeout = "timeout"
	// StageShape 返回值不符合 {q, a} 契约。
	StageShape = "shape"
)

// ValidationError 表示作者规则未通过校验。
//
// 用类型而非字符串比较：API 层需要把「规则的错」与「服务端的错」分开处理——
// 前者回 400 rule_invalid 并附可读原因，后者回 500 且不泄露内部细节。
type ValidationError struct {
	Stage  string
	Detail string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("规则校验失败[%s]: %s", e.Stage, e.Detail)
}

func newErr(stage, format string, args ...any) *ValidationError {
	return &ValidationError{Stage: stage, Detail: fmt.Sprintf(format, args...)}
}

// firstLine 只取错误首行并截断，避免把 goja 的多行堆栈直接回给作者。
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			s = s[:i]
			break
		}
	}
	r := []rune(s)
	if len(r) > 200 {
		return string(r[:200]) + "…"
	}
	return s
}

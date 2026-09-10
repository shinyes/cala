package api

import "github.com/gofiber/fiber/v2"

// 错误码：客户端据此分支，不解析 message。
const (
	CodeBadRequest         = "bad_request"
	CodeUnauthorized       = "unauthorized"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodeRegistrationClosed = "registration_closed"
	CodeRuleInvalid        = "rule_invalid"
	CodeInternal           = "internal"
)

// APIError 是统一的错误响应体。
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// fail 输出统一错误响应。
//
// 用法约定：只能作为处理器的**最后一条 return 语句**（`return fail(...)`），
// 此时返回 nil 是正确的——响应已经写出去了。
//
// 不要在需要「返回值 + 错误」的地方把它当错误值用，例如
// `return 0, fail(c, ...)`：JSON 写出成功时它返回 nil，
// 调用方会以为没有出错而继续执行，把错误静默吞掉。
// 需要那种形态时，请显式返回一个哨兵错误，或让辅助函数返回 bool。
func fail(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": APIError{Code: code, Message: message}})
}

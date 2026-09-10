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
func fail(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": APIError{Code: code, Message: message}})
}

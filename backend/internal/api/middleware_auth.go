package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/store"
)

// 存放在 fiber.Ctx 中的本地键
const (
	localUser  = "user"
	localToken = "token"
)

// requireAuth 校验 Authorization: Bearer <token>。
func (h *Handlers) requireAuth(c *fiber.Ctx) error {
	raw := bearerToken(c)
	if raw == "" {
		return fail(c, fiber.StatusUnauthorized, CodeUnauthorized, "缺少访问令牌")
	}
	u, err := h.Auth.Me(raw)
	if err != nil {
		return fail(c, fiber.StatusUnauthorized, CodeUnauthorized, "访问令牌无效或已过期")
	}
	c.Locals(localUser, u)
	c.Locals(localToken, raw)
	return c.Next()
}

// requireAdmin 必须在 requireAuth 之后使用。
func (h *Handlers) requireAdmin(c *fiber.Ctx) error {
	u, ok := c.Locals(localUser).(store.User)
	if !ok {
		return fail(c, fiber.StatusUnauthorized, CodeUnauthorized, "缺少访问令牌")
	}
	if !u.IsAdmin {
		return fail(c, fiber.StatusForbidden, CodeForbidden, "需要管理员权限")
	}
	return c.Next()
}

// currentUser 取出 requireAuth 注入的用户。仅在受保护路由中调用。
func currentUser(c *fiber.Ctx) store.User {
	u, _ := c.Locals(localUser).(store.User)
	return u
}

func bearerToken(c *fiber.Ctx) string {
	h := c.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

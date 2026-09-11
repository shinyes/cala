package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
)

// registerShare 注册分享相关路由（规格 §7）。
func (h *Handlers) registerShare(r fiber.Router) {
	r.Post("/projects/:id/share", h.requireAuth, h.handleShareProject)
	r.Delete("/projects/:id/share", h.requireAuth, h.handleUnshareProject)
}

func (h *Handlers) handleShareProject(c *fiber.Ctx) error {
	id, ok := parseProjectID(c)
	if !ok {
		return nil // 错误响应已写出
	}

	// host 取自请求的 Host 头：同一份代码在 localhost、局域网 IP、域名下
	// 都能给出对方可用的链接。
	link, err := h.Projects.Share(currentUser(c).ID, id, c.Hostname())
	if err != nil {
		return shareError(c, err)
	}
	return c.JSON(fiber.Map{
		"shareToken": link.Token,
		"link":       link.Link,
	})
}

func (h *Handlers) handleUnshareProject(c *fiber.Ctx) error {
	id, ok := parseProjectID(c)
	if !ok {
		return nil
	}
	if err := h.Projects.Unshare(currentUser(c).ID, id); err != nil {
		return shareError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func shareError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrNotOwner):
		return fail(c, fiber.StatusForbidden, CodeForbidden,
			"只有项目作者可以管理分享链接")
	case errors.Is(err, store.ErrNoAccess):
		return fail(c, fiber.StatusForbidden, CodeForbidden, "无权访问该项目")
	case errors.Is(err, store.ErrNotFound):
		return fail(c, fiber.StatusNotFound, CodeNotFound, "项目不存在")
	default:
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "分享操作失败")
	}
}

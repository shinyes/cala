package api

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
)

// registerSubscriptions 注册订阅相关路由（规格 §7）。
func (h *Handlers) registerSubscriptions(r fiber.Router) {
	r.Post("/subscriptions/import", h.requireAuth, h.handleImportSubscriptions)
	r.Delete("/subscriptions/:projectId", h.requireAuth, h.handleUnsubscribe)
}

type importReq struct {
	Links []string `json:"links"`
}

// handleImportSubscriptions 一次导入多个分享链接。
//
// 逐条返回结果：用户粘贴 5 个链接时，其中一个失效不应让其余 4 个也失败。
// 整批回滚会让用户不知道哪个是坏的，只能反复试错。
func (h *Handlers) handleImportSubscriptions(c *fiber.Ctx) error {
	var req importReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}
	if len(req.Links) == 0 {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "links 不能为空")
	}
	if len(req.Links) > maxImportLinks {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest,
			"一次最多导入 50 条链接")
	}

	results := h.Projects.ImportSubscriptions(currentUser(c).ID, req.Links, c.Hostname())

	succeeded := 0
	for _, r := range results {
		if r.OK {
			succeeded++
		}
	}
	return c.JSON(fiber.Map{
		"results":   results,
		"succeeded": succeeded,
		"total":     len(results),
	})
}

// maxImportLinks 限制单次导入的链接数，避免一次请求做过多工作量。
const maxImportLinks = 50

// handleUnsubscribe 退订并清空本人历史。
func (h *Handlers) handleUnsubscribe(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("projectId"), 10, 64)
	if err != nil || id <= 0 {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "项目 ID 不合法")
	}

	deleted, e := h.Projects.Unsubscribe(currentUser(c).ID, id)
	if e != nil {
		return subscriptionError(c, e)
	}
	return c.JSON(fiber.Map{"deletedRounds": deleted})
}

func subscriptionError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrNotSubscribed):
		return fail(c, fiber.StatusNotFound, CodeNotFound, err.Error())
	case errors.Is(err, service.ErrNotOwner):
		return fail(c, fiber.StatusBadRequest, CodeBadRequest,
			"你是该项目作者，请直接删除项目而不是退订")
	case errors.Is(err, store.ErrNotFound):
		return fail(c, fiber.StatusNotFound, CodeNotFound, "项目不存在")
	case errors.Is(err, store.ErrNoAccess):
		return fail(c, fiber.StatusForbidden, CodeForbidden, "无权访问该项目")
	default:
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "订阅操作失败")
	}
}

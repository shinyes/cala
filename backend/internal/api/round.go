package api

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/rules"
	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
)

// registerRounds 注册轮次相关路由（规格 §7）。
func (h *Handlers) registerRounds(r fiber.Router) {
	r.Post("/rounds/start", h.requireAuth, h.handleStartRound)
	r.Post("/rounds/complete", h.requireAuth, h.handleCompleteRound)
	r.Get("/rounds/:id/attempts", h.requireAuth, h.handleRoundAttempts)
}

type startRoundReq struct {
	ProjectID int64 `json:"projectId"`
}

func (h *Handlers) handleStartRound(c *fiber.Ctx) error {
	var req startRoundReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}
	if req.ProjectID <= 0 {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "缺少 projectId")
	}

	res, err := h.Rounds.Start(currentUser(c).ID, req.ProjectID)
	if err != nil {
		return roundError(c, err)
	}
	return c.JSON(res)
}

func (h *Handlers) handleCompleteRound(c *fiber.Ctx) error {
	var in service.CompleteInput
	if err := c.BodyParser(&in); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}

	res, err := h.Rounds.Complete(currentUser(c).ID, in)
	if err != nil {
		return roundError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// handleRoundAttempts 返回某轮已落库的答题记录（错题页数据源）。
//
// 仅轮次所属用户可读：答题记录是个人数据，他人（包括项目作者）不应看到。
func (h *Handlers) handleRoundAttempts(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, fiber.StatusBadRequest, CodeBadRequest, "轮次 ID 不合法")
		return nil
	}

	attempts, err := h.Rounds.Attempts(currentUser(c).ID, id)
	if err != nil {
		return roundError(c, err)
	}
	return c.JSON(fiber.Map{"attempts": attempts})
}

// roundError 把轮次相关错误映射为统一错误契约。
//
// 规则校验失败返回 rule_invalid（而非 internal）：一个死循环规则是**作者的问题**，
// 不是服务端故障，客户端应据此提示「这个项目的规则有问题」。
func roundError(c *fiber.Ctx, err error) error {
	var ve *rules.ValidationError
	switch {
	case errors.As(err, &ve):
		return fail(c, fiber.StatusBadRequest, CodeRuleInvalid, ve.Error())
	case errors.Is(err, service.ErrRoundInput):
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
	case errors.Is(err, service.ErrInvalidProject):
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
	case errors.Is(err, service.ErrNotOwner):
		return fail(c, fiber.StatusForbidden, CodeForbidden, err.Error())
	case errors.Is(err, store.ErrNoAccess):
		return fail(c, fiber.StatusForbidden, CodeForbidden, "无权访问该项目")
	case errors.Is(err, store.ErrNotFound):
		return fail(c, fiber.StatusNotFound, CodeNotFound, "项目不存在")
	default:
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "轮次操作失败")
	}
}

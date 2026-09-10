package api

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/rules"
	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
)

// projectReq 是创建/修改项目的请求体。
type projectReq struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	QuestionCount int    `json:"questionCount"`
	CfgJSON       string `json:"cfgJson"`
	RuleSource    string `json:"ruleSource"`
	ToleranceNum  *int64 `json:"toleranceNum"`
	ToleranceDen  *int64 `json:"toleranceDen"`
}

func (r projectReq) toInput() service.ProjectInput {
	return service.ProjectInput{
		Title:         r.Title,
		Description:   r.Description,
		QuestionCount: r.QuestionCount,
		CfgJSON:       r.CfgJSON,
		RuleSource:    r.RuleSource,
		ToleranceNum:  r.ToleranceNum,
		ToleranceDen:  r.ToleranceDen,
	}
}

// registerProjects 注册项目相关路由（规格 §7）。
func (h *Handlers) registerProjects(r fiber.Router) {
	r.Get("/projects", h.requireAuth, h.handleListProjects)
	r.Post("/projects", h.requireAuth, h.handleCreateProject)
	r.Get("/projects/:id", h.requireAuth, h.handleGetProject)
	r.Put("/projects/:id", h.requireAuth, h.handleUpdateProject)
	r.Delete("/projects/:id", h.requireAuth, h.handleDeleteProject)
}

// projectError 把 service/store 的错误映射为统一错误契约。
//
// 规则校验失败必须带上原始 Detail：作者需要知道错在哪一行，
// 而不是收到一句笼统的「规则不合法」。
func projectError(c *fiber.Ctx, err error) error {
	var ve *rules.ValidationError
	switch {
	case errors.As(err, &ve):
		return fail(c, fiber.StatusBadRequest, CodeRuleInvalid, ve.Error())
	case errors.Is(err, service.ErrInvalidProject):
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
	case errors.Is(err, service.ErrNotOwner):
		return fail(c, fiber.StatusForbidden, CodeForbidden, err.Error())
	case errors.Is(err, store.ErrNoAccess):
		return fail(c, fiber.StatusForbidden, CodeForbidden, "无权访问该项目")
	case errors.Is(err, store.ErrNotFound):
		return fail(c, fiber.StatusNotFound, CodeNotFound, "项目不存在")
	default:
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "项目操作失败")
	}
}

// parseProjectID 解析路径参数 :id。
//
// 第二个返回值为 false 表示**错误响应已写出**，调用方必须立即 `return nil`。
// 不返回 error 是刻意的：fail() 在 JSON 写出成功时返回 nil，
// 若签名是 (int64, error)，`return 0, fail(...)` 会把错误吞掉，
// 使非法 ID 退化成 id=0 并产生误导性的 404。
func parseProjectID(c *fiber.Ctx) (int64, bool) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, fiber.StatusBadRequest, CodeBadRequest, "项目 ID 不合法")
		return 0, false
	}
	return id, true
}

func (h *Handlers) handleListProjects(c *fiber.Ctx) error {
	me := currentUser(c)
	owned, subscribed, err := h.Projects.List(me.ID)
	if err != nil {
		return projectError(c, err)
	}
	return c.JSON(fiber.Map{"owned": owned, "subscribed": subscribed})
}

func (h *Handlers) handleCreateProject(c *fiber.Ctx) error {
	var req projectReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}
	p, err := h.Projects.Create(currentUser(c).ID, req.toInput())
	if err != nil {
		return projectError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"project": p})
}

func (h *Handlers) handleGetProject(c *fiber.Ctx) error {
	id, ok := parseProjectID(c)
	if !ok {
		return nil // 错误响应已写出
	}
	p, err := h.Projects.Get(currentUser(c).ID, id)
	if err != nil {
		return projectError(c, err)
	}
	return c.JSON(fiber.Map{"project": p})
}

func (h *Handlers) handleUpdateProject(c *fiber.Ctx) error {
	id, ok := parseProjectID(c)
	if !ok {
		return nil // 错误响应已写出
	}
	var req projectReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}
	p, err := h.Projects.Update(currentUser(c).ID, id, req.toInput())
	if err != nil {
		return projectError(c, err)
	}
	return c.JSON(fiber.Map{"project": p})
}

func (h *Handlers) handleDeleteProject(c *fiber.Ctx) error {
	id, ok := parseProjectID(c)
	if !ok {
		return nil // 错误响应已写出
	}
	if err := h.Projects.Delete(currentUser(c).ID, id); err != nil {
		return projectError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

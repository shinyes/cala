package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/auth"
	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
)

// Handlers 持有各 API handler 的依赖。
type Handlers struct {
	Auth     *service.AuthService
	Projects *service.ProjectService
	Rounds   *service.RoundService
}

// 请求体
type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type setRegistrationReq struct {
	Open bool `json:"open"`
}

// registerAuth 注册认证相关路由。
func (h *Handlers) registerAuth(r fiber.Router) {
	r.Post("/auth/register", h.handleRegister)
	r.Post("/auth/login", h.handleLogin)
	r.Post("/auth/logout", h.requireAuth, h.handleLogout)

	r.Get("/me", h.requireAuth, h.handleMe)

	// 公开配置：客户端据此决定是否显示注册入口
	r.Get("/settings/public", h.handlePublicSettings)

	// 管理员
	r.Put("/admin/settings", h.requireAuth, h.requireAdmin, h.handleSetSettings)
}

func (h *Handlers) handleRegister(c *fiber.Ctx) error {
	var req registerReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}

	res, err := h.Auth.Register(req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRegistrationClosed):
			return fail(c, fiber.StatusForbidden, CodeRegistrationClosed, "管理员已关闭注册")
		case errors.Is(err, service.ErrInvalidUsername):
			return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
		case errors.Is(err, auth.ErrPasswordTooShort):
			return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
		case errors.Is(err, store.ErrUsernameTaken):
			return fail(c, fiber.StatusConflict, CodeConflict, "用户名已被占用")
		default:
			return fail(c, fiber.StatusInternalServerError, CodeInternal, "注册失败")
		}
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user":        res.User,
		"token":       res.Token,
		"becameAdmin": res.BecameAdmin,
	})
}

func (h *Handlers) handleLogin(c *fiber.Ctx) error {
	var req loginReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}
	u, token, err := h.Auth.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return fail(c, fiber.StatusUnauthorized, CodeUnauthorized, "用户名或口令错误")
		}
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "登录失败")
	}
	return c.JSON(fiber.Map{"user": u, "token": token})
}

func (h *Handlers) handleLogout(c *fiber.Ctx) error {
	token, _ := c.Locals(localToken).(string)
	if err := h.Auth.Logout(token); err != nil {
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "登出失败")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handlers) handleMe(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"user": currentUser(c)})
}

func (h *Handlers) handlePublicSettings(c *fiber.Ctx) error {
	open, err := h.Auth.RegistrationOpen()
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "读取配置失败")
	}
	return c.JSON(fiber.Map{"registrationOpen": open})
}

func (h *Handlers) handleSetSettings(c *fiber.Ctx) error {
	var req setRegistrationReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}
	if err := h.Auth.SetRegistrationOpen(req.Open); err != nil {
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "写入配置失败")
	}
	return c.JSON(fiber.Map{"registrationOpen": req.Open})
}

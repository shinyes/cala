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

type changePasswordReq struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// registerAuth 注册认证相关路由。
func (h *Handlers) registerAuth(r fiber.Router) {
	r.Post("/auth/register", h.handleRegister)
	r.Post("/auth/login", h.handleLogin)
	r.Post("/auth/logout", h.requireAuth, h.handleLogout)
	// 改密归入 /auth/ 而非 /me/：它操作的是**凭证**，与登录/登出同类；
	// /me 读的是用户资料。按语义分组，而不是按「都在当前用户身上」分组。
	r.Post("/auth/password", h.requireAuth, h.handleChangePassword)

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

// handleChangePassword 修改当前用户的口令。
//
// 响应返回**新令牌**：成功时全部旧会话被吊销（含发起本次请求的那个），
// 因此客户端必须用新令牌替换本地保存的值，否则下一次请求就会被登出。
func (h *Handlers) handleChangePassword(c *fiber.Ctx) error {
	var req changePasswordReq
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, "请求体不是合法 JSON")
	}

	token, err := h.Auth.ChangePassword(
		currentUser(c).ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWrongCurrentPassword):
			// **刻意不用 401。**
			//
			// 客户端把 401 解释为「令牌失效」并据此清除登录态
			//（见 ApiException.isUnauthorized）。若这里回 401，
			// 用户仅仅输错一次当前口令就会被**登出**，且被踢回登录页 ——
			// 而登录页正是他试图避免去的地方（他本来已经登录着）。
			//
			// 请求本身是已认证的（令牌有效），错的只是请求体里的一个字段，
			// 因此 400 才是语义正确的状态码。
			return fail(c, fiber.StatusBadRequest, CodeBadRequest, service.ErrWrongCurrentPassword.Error())
		case errors.Is(err, service.ErrSamePassword):
			return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
		case errors.Is(err, auth.ErrPasswordTooShort):
			return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
		case errors.Is(err, store.ErrNotFound):
			// 用户来自有效会话，正常路径下不可达；显式处理以免落入 500。
			return fail(c, fiber.StatusUnauthorized, CodeUnauthorized, "访问令牌无效或已过期")
		default:
			return fail(c, fiber.StatusInternalServerError, CodeInternal, "修改口令失败")
		}
	}
	return c.JSON(fiber.Map{"token": token})
}

func (h *Handlers) handlePublicSettings(c *fiber.Ctx) error {
	canRegister, bootstrap, _, err := h.Auth.RegistrationStatus()
	if err != nil {
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "读取配置失败")
	}
	// registrationOpen 是**有效值**（已计入引导管理员规则），客户端只需据此
	// 决定是否显示注册入口，不必自己组合判断——否则同一条规则会有两个 owner。
	// bootstrap 为 true 时注册者将成为管理员，界面据此给出提示。
	return c.JSON(fiber.Map{
		"registrationOpen": canRegister,
		"bootstrap":        bootstrap,
	})
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

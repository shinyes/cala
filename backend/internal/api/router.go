// Package api 是 HTTP 传输层的唯一 owner：路由、中间件、请求/响应形状。
package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/shinyes/cala/backend/internal/config"
	"github.com/shinyes/cala/backend/internal/store"
)

// Deps 是路由所需的依赖集合。
type Deps struct {
	Config   config.Config
	Store    *store.Store
	Handlers *Handlers
}

// NewRouter 装配 Fiber 应用。
func NewRouter(d Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "cala",
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if fe, ok := err.(*fiber.Error); ok {
				return fail(c, fe.Code, CodeBadRequest, fe.Message)
			}
			return fail(c, fiber.StatusInternalServerError, CodeInternal, "服务内部错误")
		},
	})

	// panic 不应杀死进程
	app.Use(recover.New())

	// D9：后端不内嵌前端，开发期 flutter run -d chrome 是独立来源，必须显式放行。
	// 不使用通配符 + 凭据的组合。
	if len(d.Config.DevCORSOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins:     strings.Join(d.Config.DevCORSOrigins, ","),
			AllowHeaders:     "Content-Type, Authorization",
			AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
			AllowCredentials: true,
		}))
	}

	api := app.Group("/api")
	registerHealth(api)
	if d.Handlers != nil {
		d.Handlers.registerAuth(api)
	}

	return app
}

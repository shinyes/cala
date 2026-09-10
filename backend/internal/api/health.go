package api

import "github.com/gofiber/fiber/v2"

// registerHealth 注册健康检查。用于容器 HEALTHCHECK 与本地探活。
func registerHealth(r fiber.Router) {
	r.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
}

package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/version"
)

// registerHealth 注册健康检查。用于容器探活、客户端「测试连接」与本地探活。
//
// 同时返回 version：镜像的 tag 是**外部**元数据，
// 一旦容器经过 docker save/load、被重新打标签，或只是有人记错了跑的是哪个，
// tag 就不可信了。让服务端自报版本，才能回答「现在跑的是哪个版本」——
// 这正是排查部署问题时第一个要问的问题。
//
// 取舍：该端点无需认证，因此版本号对外可见。
// 对本项目（自部署、局域网）这是可接受的；若日后要对外暴露，
// 应把 version 移到一个需要认证的端点，而不是在这里加鉴权 ——
// 探活端点必须保持无依赖，否则容器编排系统会因鉴权失败而误判服务已死。
func registerHealth(r fiber.Router) {
	r.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": version.Version,
		})
	})
}

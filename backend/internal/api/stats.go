package api

import (
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/shinyes/cala/backend/internal/stats"
	"github.com/shinyes/cala/backend/internal/store"
)

// maxTZOffsetMinutes 是时区偏移的上限（±14 小时，实际最东的 UTC+14）。
const maxTZOffsetMinutes = 14 * 60

// registerStats 注册统计路由（规格 §7）。
func (h *Handlers) registerStats(r fiber.Router) {
	r.Get("/projects/:id/stats", h.requireAuth, h.handleProjectStats)
}

// handleProjectStats 返回某项目下**当前用户自己**的统计。
//
// 查询参数：
//   - grain=day|week|month   默认 day
//   - tzOffsetMinutes=N      相对 UTC 的分钟偏移（东八区为 480），默认 0
//
// 时区参数的存在理由：分桶按自然日/自然月进行，而「自然日」是用户本地的概念。
// 若固定按 UTC 分桶，当地 00:30 做的练习会被算到前一天，
// 用户看到「今天的练习」在昨天那一列。
func (h *Handlers) handleProjectStats(c *fiber.Ctx) error {
	id, ok := parseProjectID(c)
	if !ok {
		return nil // 错误响应已写出
	}

	grainRaw := c.Query("grain", string(stats.GrainDay))
	grain, err := stats.ParseGrain(grainRaw)
	if err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
	}

	tzOffset, err := parseTZOffset(c.Query("tzOffsetMinutes", "0"))
	if err != nil {
		return fail(c, fiber.StatusBadRequest, CodeBadRequest, err.Error())
	}

	res, err := h.Projects.Stats(currentUser(c).ID, id, grain, tzOffset)
	if err != nil {
		return statsError(c, err)
	}
	return c.JSON(res)
}

func parseTZOffset(raw string) (time.Duration, error) {
	minutes, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("tzOffsetMinutes 必须是整数（相对 UTC 的分钟数）")
	}
	if minutes < -maxTZOffsetMinutes || minutes > maxTZOffsetMinutes {
		return 0, errors.New("tzOffsetMinutes 超出范围（应为 -840 到 840，即 ±14 小时）")
	}
	return time.Duration(minutes) * time.Minute, nil
}

func statsError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, store.ErrNoAccess):
		return fail(c, fiber.StatusForbidden, CodeForbidden, "无权访问该项目")
	case errors.Is(err, store.ErrNotFound):
		return fail(c, fiber.StatusNotFound, CodeNotFound, "项目不存在")
	default:
		return fail(c, fiber.StatusInternalServerError, CodeInternal, "读取统计失败")
	}
}

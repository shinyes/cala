package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 是服务端全部可调参数。所有值来自环境变量，便于容器化部署。
type Config struct {
	// Addr 是 HTTP 监听地址。
	Addr string
	// DBPath 是 SQLite 数据库文件路径。
	DBPath string
	// SessionTTL 是会话有效期。
	SessionTTL time.Duration
	// DevCORSOrigins 是开发期允许的浏览器来源（D9：后端不内嵌前端，
	// 因此 flutter run -d chrome 必须显式放行）。
	DevCORSOrigins []string
	// RegistrationOpenDefault 仅在 setting 表中尚无 registration_open 时使用。
	RegistrationOpenDefault bool
}

// Load 从环境变量读取配置，未设置时使用适合本地开发的默认值。
func Load() (Config, error) {
	c := Config{
		Addr:                    env("CALA_ADDR", ":8080"),
		DBPath:                  env("CALA_DB", "cala.db"),
		RegistrationOpenDefault: envBool("CALA_REGISTRATION_OPEN", true),
	}

	ttl, err := time.ParseDuration(env("CALA_SESSION_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("CALA_SESSION_TTL 不是合法时长: %w", err)
	}
	c.SessionTTL = ttl

	// CORS 白名单。
	//
	// 这里刻意**不用** env()，而要区分「未设置」与「显式设为空」：
	//   未设置        -> 使用开发期默认来源
	//   显式设为 ""   -> 空列表，完全不启用 CORS 放行（生产环境应如此）
	//
	// 原因：env() 内部有 `v != ""` 判断，空字符串会静默回退到默认值，
	// 与本段声明的意图相反 —— 即 `CALA_DEV_CORS_ORIGINS=""` 会变成
	// 「放行 localhost:3000」，而非「不放行任何来源」。
	// 生产部署依赖后者，故用 LookupEnv 实现该语义。
	corsRaw, corsSet := os.LookupEnv("CALA_DEV_CORS_ORIGINS")
	if !corsSet {
		corsRaw = "http://localhost:3000,http://127.0.0.1:3000"
	}
	for _, o := range strings.Split(corsRaw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.DevCORSOrigins = append(c.DevCORSOrigins, o)
		}
	}
	return c, nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

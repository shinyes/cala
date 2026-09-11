// Package version 承载构建版本号。
//
// 为什么单独成包，而不是各处在 main 里读：版本号有两个读者 ——
// 启动日志（main）与 HTTP 层（api 包的 /healthz）。
// 放在这里让两者读同一个值，且**api 包不必依赖 main**（那会形成反向依赖）。
package version

// Version 是构建版本号，由**构建时**经链接器注入，默认 "dev"：
//
//	go build -ldflags "-X github.com/shinyes/cala/backend/internal/version.Version=0.0.4"
//
// 默认值刻意取 "dev" 而不是空串：本地直接 go build 出来的二进制
// 应当自报「开发构建」，而不是伪装成某个发布版本，也不是显示为空白。
//
// 不要在这个变量上做版本比较。它的用途是**标识**（「跑的是哪个版本」），
// 不是**判断**（「是否该迁移」）。schema 版本由迁移表自己的计数负责。
var Version = "dev"

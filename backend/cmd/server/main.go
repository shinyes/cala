// Command server 是 Cala 后端入口。
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shinyes/cala/backend/internal/api"
	"github.com/shinyes/cala/backend/internal/config"
	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
	"github.com/shinyes/cala/backend/internal/version"
)

func main() {
	// --version 必须在读配置、开数据库**之前**处理。
	//
	// 原因：运行镜像是 distroless，里面没有 shell，因此检查版本只能靠
	//   docker run --rm <镜像> --version
	// 若它要先连上数据库才能回答，那么在「数据库挂载有问题」这类
	// 恰恰最需要确认版本的场景里，这个检查反而用不了。
	//
	// 输出**只有版本号本身**（不含前缀），便于脚本直接取用。
	showVersion := flag.Bool("version", false, "打印版本号并退出")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer st.Close()

	authSvc := service.NewAuthService(st, cfg.SessionTTL, cfg.RegistrationOpenDefault)
	projectSvc := service.NewProjectService(st)
	roundSvc := service.NewRoundService(st, projectSvc)
	app := api.NewRouter(api.Deps{
		Config: cfg,
		Store:  st,
		Handlers: &api.Handlers{
			Auth:     authSvc,
			Projects: projectSvc,
			Rounds:   roundSvc,
		},
	})

	// 优雅关闭：容器收到 SIGTERM 时先停止接收新请求
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("正在关闭服务…")
		if err := app.Shutdown(); err != nil {
			log.Printf("关闭出错: %v", err)
		}
	}()

	// 启动即打印版本：容器日志里能直接看到跑的是哪个版本，
	// 不必再进容器执行命令（distroless 里也没有 shell 可用）。
	log.Printf("Cala 后端 %s 监听 %s，数据库 %s", version.Version, cfg.Addr, cfg.DBPath)
	if err := app.Listen(cfg.Addr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}

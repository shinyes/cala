// Command server 是 Cala 后端入口。
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/shinyes/cala/backend/internal/api"
	"github.com/shinyes/cala/backend/internal/config"
	"github.com/shinyes/cala/backend/internal/service"
	"github.com/shinyes/cala/backend/internal/store"
)

func main() {
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
	app := api.NewRouter(api.Deps{
		Config:   cfg,
		Store:    st,
		Handlers: &api.Handlers{Auth: authSvc},
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

	log.Printf("Cala 后端监听 %s，数据库 %s", cfg.Addr, cfg.DBPath)
	if err := app.Listen(cfg.Addr); err != nil {
		log.Fatalf("HTTP 服务退出: %v", err)
	}
}

package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}
	if c.Addr != ":8080" {
		t.Errorf("默认 Addr = %q, 期望 :8080", c.Addr)
	}
	if c.SessionTTL != 720*time.Hour {
		t.Errorf("默认 SessionTTL = %v, 期望 720h", c.SessionTTL)
	}
	if !c.RegistrationOpenDefault {
		t.Error("默认 RegistrationOpenDefault 应为 true")
	}
	if len(c.DevCORSOrigins) == 0 {
		t.Error("默认应放行至少一个开发期来源")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("CALA_ADDR", "127.0.0.1:9000")
	t.Setenv("CALA_SESSION_TTL", "1h")
	t.Setenv("CALA_REGISTRATION_OPEN", "false")
	t.Setenv("CALA_DEV_CORS_ORIGINS", "http://a.test , http://b.test,")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}
	if c.Addr != "127.0.0.1:9000" {
		t.Errorf("Addr = %q", c.Addr)
	}
	if c.SessionTTL != time.Hour {
		t.Errorf("SessionTTL = %v", c.SessionTTL)
	}
	if c.RegistrationOpenDefault {
		t.Error("RegistrationOpenDefault 应为 false")
	}
	if len(c.DevCORSOrigins) != 2 || c.DevCORSOrigins[0] != "http://a.test" || c.DevCORSOrigins[1] != "http://b.test" {
		t.Errorf("DevCORSOrigins = %#v, 期望去除空白与空项后的两个来源", c.DevCORSOrigins)
	}
}

func TestLoadRejectsBadTTL(t *testing.T) {
	t.Setenv("CALA_SESSION_TTL", "不是时长")
	if _, err := Load(); err == nil {
		t.Fatal("非法 CALA_SESSION_TTL 应当报错")
	}
}

// TestExplicitEmptyCORSDisablesIt 锁定「显式设为空 = 不放行任何来源」这一语义。
// 生产部署依赖它关闭 CORS：后端不提供网页前端，Android 原生客户端不受 CORS 约束。
//
// 这条测试是针对一个真实缺陷的回归防护：此前该变量经 env() 读取，
// 而 env() 的 `v != ""` 判断会让空字符串**回退到默认的 localhost:3000**，
// 于是「设为空以关闭 CORS」实际变成了「放行开发期来源」——
// 与代码自己的注释相反，且部署者不会察觉。
func TestExplicitEmptyCORSDisablesIt(t *testing.T) {
	t.Setenv("CALA_DEV_CORS_ORIGINS", "")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}
	if len(c.DevCORSOrigins) != 0 {
		t.Errorf("显式设为空时 DevCORSOrigins 应为空, 得到 %#v", c.DevCORSOrigins)
	}
}

// TestUnsetCORSFallsBackToDevDefaults 确保区分「未设置」与「显式设为空」：
// 未设置时仍应放行开发期来源，否则本地 flutter run -d chrome 会调试不了。
func TestUnsetCORSFallsBackToDevDefaults(t *testing.T) {
	// t.Setenv 无法「取消设置」，故用 LookupEnv 语义直接验证默认分支：
	// 通过 os.Unsetenv 并依赖 t.Setenv 的清理机制恢复。
	if err := os.Unsetenv("CALA_DEV_CORS_ORIGINS"); err != nil {
		t.Fatalf("Unsetenv 失败: %v", err)
	}
	t.Cleanup(func() { os.Unsetenv("CALA_DEV_CORS_ORIGINS") })

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() 出错: %v", err)
	}
	if len(c.DevCORSOrigins) == 0 {
		t.Error("未设置时应有开发期默认来源")
	}
}

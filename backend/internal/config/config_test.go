package config

import (
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

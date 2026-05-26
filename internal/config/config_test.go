package config

import (
	"strings"
	"testing"
)

// 生产环境 JWT_SECRET 必须为强随机值；开发环境缺省回退占位。
func TestLoad_JWTSecretValidation(t *testing.T) {
	strong := strings.Repeat("a", minJWTSecretLen) // 恰好达标的长度

	cases := []struct {
		name    string
		env     string
		secret  string
		wantErr bool
	}{
		{"dev 缺省回退占位", "dev", "", false},
		{"prod 缺 secret 报错", "prod", "", true},
		{"prod 用开发默认报错", "prod", devJWTSecret, true},
		{"prod secret 过短报错", "prod", strings.Repeat("x", minJWTSecretLen-1), true},
		{"prod 强 secret 通过", "prod", strong, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENV", tc.env)
			t.Setenv("JWT_SECRET", tc.secret)

			cfg, err := Load()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错，但成功（secret=%q）", tc.secret)
				}
				return
			}
			if err != nil {
				t.Fatalf("期望成功，却报错：%v", err)
			}
			if tc.env == "dev" && tc.secret == "" && cfg.JWTSecret != devJWTSecret {
				t.Fatalf("dev 缺省应回退占位 secret，got %q", cfg.JWTSecret)
			}
		})
	}
}

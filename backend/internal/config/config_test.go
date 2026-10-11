// 文件说明：验证 .env 的读取、环境变量优先级、默认关闭和配置错误处理。
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		name, file, environment string
		missing, envSet, want   bool
	}{
		{name: "无文件且无环境变量", missing: true},
		{name: "无文件但环境变量为 dev", missing: true, envSet: true, environment: "dev", want: true},
		{name: "空文件"},
		{name: "文件未配置 APP_ENV", file: "# 本地配置\nOTHER=value\n"},
		{name: "文件 dev", file: "APP_ENV=dev\n", want: true},
		{name: "文件 prod", file: "APP_ENV=prod\n"},
		{name: "文件空值", file: "APP_ENV=\n"},
		{name: "文件其他值", file: "APP_ENV=test\n"},
		{name: "大小写精确匹配", file: "APP_ENV=DEV\n"},
		{name: "带引号的值", file: "# 配置说明\r\nAPP_ENV='dev' # 开发模式\r\n", want: true},
		{name: "值内空格不算 dev", file: "APP_ENV=' dev '\n"},
		{name: "进程 prod 覆盖文件 dev", file: "APP_ENV=dev\n", envSet: true, environment: "prod"},
		{name: "进程 dev 覆盖文件 prod", file: "APP_ENV=prod\n", envSet: true, environment: "dev", want: true},
		{name: "进程空值优先", file: "APP_ENV=dev\n", envSet: true},
		{name: "进程其他值优先", file: "APP_ENV=dev\n", envSet: true, environment: "test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 先让 t.Setenv 记录原值，测试退出时自动恢复，避免依赖本机环境。
			t.Setenv("APP_ENV", tc.environment)
			if !tc.envSet {
				if err := os.Unsetenv("APP_ENV"); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(t.TempDir(), ".env")
			if !tc.missing {
				if err := os.WriteFile(path, []byte(tc.file), 0600); err != nil {
					t.Fatal(err)
				}
			}
			settings, err := Load(path)
			if err != nil || settings.DevMode != tc.want {
				t.Fatalf("期望开发模式=%t，实际配置=%+v，错误=%v", tc.want, settings, err)
			}
			if value, exists := os.LookupEnv("APP_ENV"); exists != tc.envSet || value != tc.environment {
				t.Fatal("读取文件不应修改进程环境变量")
			}
		})
	}
}

// TestLoadError 确保显式环境变量不能掩盖已存在文件的读取或格式错误。
func TestLoadError(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Run("格式错误", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(path, []byte("APP_ENV='dev\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("配置格式错误时应返回错误")
		}
	})
	t.Run("无法作为文件读取", func(t *testing.T) {
		// 目录在不同操作系统上都无法作为 .env 文本正常读取。
		if _, err := Load(t.TempDir()); err == nil {
			t.Fatal("读取失败时应返回错误")
		}
	})
}

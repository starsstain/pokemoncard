// 文件说明：读取本地 .env 和进程环境变量，确定两个后端服务的开发模式配置。
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config 保存启动配置；服务启动后不再自动读取文件。
type Config struct {
	DevMode bool
}

// Load 读取指定配置文件；文件缺失允许启动，读取或解析失败则返回错误。
// 只读取配置，不把文件中的变量写入进程环境。
func Load(path string) (Config, error) {
	values, err := godotenv.Read(path)
	if err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("读取配置文件 %s 失败：%w", path, err)
	}

	// LookupEnv 可区分“未设置”和“明确设置为空”；后者也应优先于文件。
	environment, exists := os.LookupEnv("APP_ENV")
	if !exists {
		environment = values["APP_ENV"]
	}
	return Config{DevMode: environment == "dev"}, nil
}

// 文件说明：启动 battle 服务，将攻击请求转发给 attack，并仅返回 HTTP 状态码。
package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"school-gitlab.xsolla.dev/yijun_gao/pokemoncard/backend/internal/config"
)

const validationPath = "/api/v1/attack-validations"

// main 启动独立的 battle 进程，attack 需另行启动。
func main() {
	settings, err := config.Load(".env")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("battle 开发模式配置=%t；本版本尚未注册开发接口", settings.DevMode)
	const address = "127.0.0.1:8080"
	const attackURL = "http://127.0.0.1:8081/internal/v1/attack-validations"
	log.Printf("battle 正在监听 http://%s%s", address, validationPath)
	log.Fatal(http.ListenAndServe(address, newHandler(attackURL)))
}

// newHandler 创建转发处理器；传入 attack 地址，方便测试使用临时服务。
func newHandler(attackURL string) http.Handler {
	client := &http.Client{
		Timeout: 500 * time.Millisecond,
		// 不跟随重定向，保持 attack 返回的状态码，不额外发送请求。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusInternalServerError
		setCORSHeaders(w)
		defer func() {
			if problem := recover(); problem != nil {
				status = http.StatusInternalServerError
				log.Printf("battle 内部异常：%v", problem)
			}
			w.WriteHeader(status)
			log.Printf("battle method=%s path=%s status=%d", r.Method, r.URL.Path, status)
		}()
		if r.URL.Path != validationPath {
			status = http.StatusNotFound
			return
		}
		if r.Method == http.MethodOptions {
			status = http.StatusNoContent
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			status = http.StatusMethodNotAllowed
			return
		}

		// 原样转发请求体，不解析 JSON，也不重复检查攻击规则。
		request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, attackURL, r.Body)
		if err != nil {
			log.Printf("battle 创建转发请求失败：%v", err)
			return
		}
		request.ContentLength = r.ContentLength
		request.Header["Content-Type"] = r.Header.Values("Content-Type")
		// 每次只调用一次，不设置可重放的请求体，也不自动重试。
		response, err := client.Do(request)
		if err != nil {
			status = forwardingStatus(err)
			log.Printf("battle 调用 attack 失败：%v", err)
			return
		}
		defer response.Body.Close()
		// 收到合法 HTTP 响应后原样返回状态码；不读取或转发响应体。
		status = response.StatusCode
	})
}

// forwardingStatus 区分调用超时、连接失败及无法取得合法 HTTP 响应的情况。
func forwardingStatus(err error) int {
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return http.StatusGatewayTimeout
	}
	var connection *net.OpError
	if errors.As(err, &connection) && connection.Op == "dial" {
		return http.StatusServiceUnavailable
	}
	return http.StatusBadGateway
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", http.MethodPost+", "+http.MethodOptions)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

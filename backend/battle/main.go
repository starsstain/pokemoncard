// 文件说明：启动 battle 服务，将攻击请求转发给 attack，并仅返回 HTTP 状态码。
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	ddhttp "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"school-gitlab.xsolla.dev/yijun_gao/pokemoncard/backend/internal/config"
)

const validationPath = "/api/v1/attack-validations"

// main 启动独立的 battle 进程，attack 需另行启动。
func main() {
	settings, err := config.Load(".env")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("battle 开发接口开启=%t", settings.DevMode)
	const address = "127.0.0.1:8080"
	const attackURL = "http://127.0.0.1:8081/internal/v1/attack-validations"
	// APM：启动 Datadog 追踪器，与 attack 串成一条分布式 trace。
	// 显式写 IPv4 地址：Windows 上默认 localhost 先解析为 ::1，Agent 只监听 IPv4。
	tracer.Start(
		tracer.WithService("battle"),
		tracer.WithEnv("school"),
		tracer.WithAgentAddr("127.0.0.1:8126"),
		tracer.WithGlobalTag("version", "1.0.0"),
		tracer.WithGlobalTag("env", "school"),
		tracer.WithGlobalTag("service", "pokemoncard"),
		tracer.WithGlobalTag("team", "bj_xsolla-school-group-dongxue"),
	)
	defer tracer.Stop()
	log.Printf("battle 正在监听 http://%s%s", address, validationPath)
	wrapped := ddhttp.WrapHandler(newHandler(attackURL, settings.DevMode), "battle", validationPath)
	// log.Fatal 会跳过 defer，须先手动 Stop 刷出缓冲中的 trace 再退出。
	if err := http.ListenAndServe(address, wrapped); err != nil {
		tracer.Stop()
		log.Fatal(err)
	}
}

// logWithTrace 输出带 dd.trace_id 的结构化日志，实现 log↔trace 互跳。
func logWithTrace(r *http.Request, level, msg string, args ...any) {
	span, ok := tracer.SpanFromContext(r.Context())
	var traceID string
	if ok {
		traceID = fmt.Sprintf("%d", span.Context().TraceID())
	}
	prefix := fmt.Sprintf("level=%s service=battle dd.trace_id=%s ", level, traceID)
	log.Printf(prefix+msg, args...)
}

// newHandler 创建转发处理器；传入 attack 地址，方便测试使用临时服务。
func newHandler(attackURL string, devMode bool) http.Handler {
	client := ddhttp.WrapClient(&http.Client{
		Timeout: 500 * time.Millisecond,
		// 不跟随重定向，保持 attack 返回的状态码，不额外发送请求。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})
	// 所有路由共用方法检查、日志、空响应体和同一个 500ms HTTP 客户端。
	routes := map[string]func(*http.Request) int{
		validationPath: func(r *http.Request) int { return forwardValidation(client, attackURL, r) },
	}
	if devMode {
		registerDevRoutes(routes, client, attackURL)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusInternalServerError
		setCORSHeaders(w)
		defer func() {
			if problem := recover(); problem != nil {
				status = http.StatusInternalServerError
				logWithTrace(r, "ERROR", "battle 内部异常：%v", problem)
			}
			w.WriteHeader(status)
			logWithTrace(r, "INFO", "battle method=%s path=%s status=%d", r.Method, r.URL.Path, status)
		}()
		handler, exists := routes[r.URL.Path]
		if !exists {
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

		status = handler(r)
	})
}

// forwardValidation 原样转发正常攻击的请求体，不解析 JSON 或重复校验伤害。
func forwardValidation(client *http.Client, attackURL string, r *http.Request) int {
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, attackURL, r.Body)
	if err != nil {
		log.Printf("battle 创建转发请求失败：%v", err)
		return http.StatusInternalServerError
	}
	request.ContentLength = r.ContentLength
	request.Header["Content-Type"] = r.Header.Values("Content-Type")
	return forwardRequest(client, request)
}

// forwardRequest 供正常和开发请求共用：不重试、不跟随重定向，只保留实际状态码。
func forwardRequest(client *http.Client, request *http.Request) int {
	request.GetBody = nil // 禁用请求体重放，开发请求也不自动重试。
	response, err := client.Do(request)
	if err != nil {
		log.Printf("battle 调用 attack 失败：%v", err)
		return forwardingStatus(err)
	}
	defer response.Body.Close()
	return response.StatusCode
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

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
)

const (
	validationPath     = "/api/v1/attack-validations"
	debugFaultHeader   = "X-Pokemon-Debug-Fault"
	faultBattle502     = "battle-502"
	faultBattle503     = "battle-503"
	faultAttack500     = "attack-500"
	faultAttackTimeout = "attack-timeout"
)

// main 启动独立的 battle 进程，attack 需另行启动。
func main() {
	const address = "127.0.0.1:8080"
	const attackURL = "http://127.0.0.1:8081/internal/v1/attack-validations"
	// APM：启动 Datadog 追踪器，与 attack 串成一条分布式 trace。
	// 显式写 IPv4 地址：Windows 上默认 localhost 先解析为 ::1，Agent 只监听 IPv4。
	tracer.Start(
		tracer.WithService("battle"),
		tracer.WithEnv("school"),
		tracer.WithAgentAddr("127.0.0.1:8126"),
		tracer.WithGlobalTag("version", "1.0.0"),
		tracer.WithGlobalTag("team", "bj-xsolla-school-group-2"),
	)
	defer tracer.Stop()
	log.Printf("battle 正在监听 http://%s%s", address, validationPath)
	wrapped := ddhttp.WrapHandler(newHandler(attackURL), "battle", validationPath)
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
func newHandler(attackURL string) http.Handler {
	client := ddhttp.WrapClient(&http.Client{
		Timeout: 500 * time.Millisecond,
		// 不跟随重定向，保持 attack 返回的状态码，不额外发送请求。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})
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
		fault := r.Header.Get(debugFaultHeader)
		switch fault {
		case faultBattle502:
			status = http.StatusBadGateway
			logWithTrace(r, "ERROR", "battle 故障注入：返回 502")
			return
		case faultBattle503:
			status = http.StatusServiceUnavailable
			logWithTrace(r, "ERROR", "battle 故障注入：返回 503")
			return
		}

		request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, attackURL, r.Body)
		if err != nil {
			logWithTrace(r, "ERROR", "battle 创建转发请求失败：%v", err)
			return
		}
		request.ContentLength = r.ContentLength
		request.Header["Content-Type"] = r.Header.Values("Content-Type")
		// 每次只调用一次，不设置可重放的请求体，也不自动重试。
		if fault == faultAttack500 || fault == faultAttackTimeout {
			request.Header.Set(debugFaultHeader, fault)
		}
		response, err := client.Do(request)
		if err != nil {
			status = forwardingStatus(err)
			logWithTrace(r, "ERROR", "battle 调用 attack 失败：%v", err)
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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+debugFaultHeader)
}

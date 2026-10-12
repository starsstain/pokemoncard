// 文件说明：处理仅在开发模式注册的内部 500 和延迟响应接口，供 battle 测试。
package main

import (
	"fmt"
	"net/http"
	"time"

	"school-gitlab.xsolla.dev/yijun_gao/pokemoncard/backend/internal/observability"
)

const (
	dev500Path     = "/internal/v1/dev/500"
	devTimeoutPath = "/internal/v1/dev/timeout"
	devDelay       = 600 * time.Millisecond
)

// handleDev 不需要请求体；500 直接返回，timeout 等待 600ms 后返回 200。
func handleDev(w http.ResponseWriter, r *http.Request) {
	//只允许post
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeDevStatus(w, r, http.StatusMethodNotAllowed)
		return
	}
	//默认500
	status := http.StatusInternalServerError
	//访问timeout
	if r.URL.Path == devTimeoutPath {
		timer := time.NewTimer(devDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
			status = http.StatusOK
		case <-r.Context().Done():
			// battle 超时或断开后停止等待，不记录成一次成功响应。
			logWithTrace(r, "WARN", "request cancelled")
			return
		}
	}
	writeDevStatus(w, r, status)
}

// writeDevStatus 只写状态码并记录日志，保持响应体为空。
func writeDevStatus(w http.ResponseWriter, r *http.Request, status int) {
	if status >= http.StatusInternalServerError {
		observability.RecordRequestError(r.Context(), "development_fault", fmt.Errorf("development endpoint returned HTTP %d", status))
	}
	w.WriteHeader(status)
	logRequest(r, status)
}

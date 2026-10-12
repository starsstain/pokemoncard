// 文件说明：验证七个开发入口、环境开关、实际状态透传、超时及跨域预检。
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var devPaths = []string{"/api/v1/dev/200", "/api/v1/dev/400", "/api/v1/dev/422",
	"/api/v1/dev/500", "/api/v1/dev/502", "/api/v1/dev/503", "/api/v1/dev/504"}

// TestDevRouting 关闭时不调用上游；开启时预检不触发故障，错误方法返回空的 405。
func TestDevRouting(t *testing.T) {
	var calls atomic.Int32
	attack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(500)
	}))
	t.Cleanup(attack.Close)
	for _, enabled := range []bool{false, true} {
		handler := newHandler(attack.URL, enabled)
		for _, path := range devPaths {
			for _, method := range []string{"GET", "HEAD", "OPTIONS", "POST"} {
				if enabled && method == "POST" {
					continue // 七个 POST 场景分别在后续测试中验证。
				}
				request := httptest.NewRequest(method, path, nil)
				request.Header.Set("Origin", "http://localhost:5173")
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				want := 404
				if method == "OPTIONS" {
					want = 204
				} else if enabled {
					want = 405
				}
				checkResponse(t, response, want)
				if want == 405 && response.Header().Get("Allow") != "POST" {
					t.Error("错误方法应声明 Allow: POST")
				}
				if response.Header().Get("Access-Control-Allow-Origin") != "*" {
					t.Error("错误响应和预检也应包含跨域响应头")
				}
			}
		}
		for _, path := range []string{"/api/v1/dev/501", "/api/v1/dev/500/", "/api/v1/dev/0500"} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("POST", path, nil))
			checkResponse(t, response, 404)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("关闭的接口、错误方法和预检都不应调用 attack")
	}
}

// TestDevForwarding 检查后端生成的路径和请求体，前端内容不能改变测试场景。
func TestDevForwarding(t *testing.T) {
	for _, tc := range []struct {
		path, upstreamPath, body string
		status                   int
	}{
		{"/api/v1/dev/200", "/internal/v1/attack-validations", `{"attack_type":"throw","expected_damage":20}`, 200},
		{"/api/v1/dev/400", "/internal/v1/attack-validations", `{"attack_type":"invalid","expected_damage":20}`, 400},
		{"/api/v1/dev/422", "/internal/v1/attack-validations", `{"attack_type":"throw","expected_damage":999}`, 422},
		{"/api/v1/dev/500", "/internal/v1/dev/500", "", 500},
	} {
		t.Run(tc.path, func(t *testing.T) {
			var calls atomic.Int32
			attack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil || r.Method != "POST" || r.URL.Path != tc.upstreamPath || string(body) != tc.body {
					t.Errorf("上游请求不符：%s %s body=%q err=%v", r.Method, r.URL.Path, body, err)
				}
				contentType := ""
				if tc.body != "" {
					contentType = "application/json"
				}
				if r.Header.Get("Content-Type") != contentType || r.Header.Get("X-Pokemon-Debug-Fault") != "" {
					t.Error("不应传递前端的媒体类型或旧故障指令")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("不应转发上游响应体"))
			}))
			t.Cleanup(attack.Close)
			request := httptest.NewRequest("POST", tc.path, strings.NewReader("frontend body"))
			request.Header.Set("Content-Type", "text/plain")
			request.Header.Set("X-Pokemon-Debug-Fault", "attack-timeout")
			response := httptest.NewRecorder()
			newHandler(attack.URL+"/internal/v1/attack-validations", true).ServeHTTP(response, request)
			checkResponse(t, response, tc.status)
			if calls.Load() != 1 {
				t.Fatal("每次开发请求只应调用 attack 一次")
			}
		})
	}
}

func TestDevLocalStatus(t *testing.T) {
	// 不可用的上游地址不能影响本地模拟的 502、503。
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/api/v1/dev/502", 502}, {"/api/v1/dev/503", 503},
	} {
		response := httptest.NewRecorder()
		newHandler(":invalid", true).ServeHTTP(response, httptest.NewRequest("POST", tc.path, nil))
		checkResponse(t, response, tc.status)
	}
}

// TestDevActualStatus 不用路径中的数字覆盖实际上游结果，也不跟随重定向。
func TestDevActualStatus(t *testing.T) {
	for _, status := range []int{200, 404, 307} {
		var calls atomic.Int32
		attack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Location", "/redirect")
			w.WriteHeader(status)
		}))
		handler := newHandler(attack.URL+"/internal/v1/attack-validations", true)
		for _, path := range []string{"/api/v1/dev/500", "/api/v1/dev/504"} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("POST", path, nil))
			checkResponse(t, response, status)
		}
		attack.Close()
		if calls.Load() != 2 {
			t.Fatal("不应重试或跟随重定向")
		}
	}
}

func TestDevTimeout(t *testing.T) {
	var calls atomic.Int32
	canceled := make(chan struct{})
	attack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/internal/v1/dev/timeout" {
			t.Error("超时场景内部路径错误")
		}
		<-r.Context().Done()
		close(canceled)
	}))
	t.Cleanup(attack.Close)
	response := httptest.NewRecorder()
	started := time.Now()
	newHandler(attack.URL+"/internal/v1/attack-validations", true).ServeHTTP(response,
		httptest.NewRequest("POST", "/api/v1/dev/504", nil))
	checkResponse(t, response, 504)
	if time.Since(started) < 400*time.Millisecond {
		t.Fatal("应通过真实等待产生超时")
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("超时后应取消上游请求")
	}
	if calls.Load() != 1 {
		t.Fatal("超时不应重试")
	}
}

func TestDevUnavailable(t *testing.T) {
	attack := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	attack.Close()
	response := httptest.NewRecorder()
	newHandler(attack.URL, true).ServeHTTP(response, httptest.NewRequest("POST", "/api/v1/dev/500", nil))
	checkResponse(t, response, 503)
}

// 文件说明：验证 battle 的原样转发、空响应体、路由检查及上游调用失败处理。
package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestLegacyFaultIgnored 检查旧故障请求头既不在 battle 生效，也不转发到 attack。
func TestLegacyFaultIgnored(t *testing.T) {
	var calls atomic.Int32
	attack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("X-Pokemon-Debug-Fault") != "" {
			t.Error("旧故障指令不应传给 attack")
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(attack.Close)
	handler := newHandler(attack.URL, true)
	for _, fault := range []string{"battle-502", "battle-503", "attack-500", "attack-timeout"} {
		t.Run(fault, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, validationPath,
				strings.NewReader(`{"attack_type":"throw","expected_damage":20}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Pokemon-Debug-Fault", fault)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			checkResponse(t, response, http.StatusOK)
		})
	}
	if calls.Load() != 4 {
		t.Fatalf("四次请求都应正常转发，实际调用 %d 次", calls.Load())
	}
}

// TestPreflight 普通 JSON 请求的跨域预检仍可完成，且不会调用 attack。
func TestPreflight(t *testing.T) {
	request := httptest.NewRequest(http.MethodOptions, validationPath, nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "content-type")
	response := httptest.NewRecorder()
	newHandler(":invalid", false).ServeHTTP(response, request)
	checkResponse(t, response, http.StatusNoContent)
	if response.Header().Get("Access-Control-Allow-Origin") != "*" ||
		response.Header().Get("Access-Control-Allow-Methods") != "POST, OPTIONS" ||
		response.Header().Get("Access-Control-Allow-Headers") != corsAllowedHeaders {
		t.Fatalf("跨域响应头不符合约定：%v", response.Header())
	}
}

// TestForwarding 用真实 HTTP 连接检查请求内容和 attack 各类状态码的透传。
func TestForwarding(t *testing.T) {
	for _, code := range []int{200, 400, 422, 500, 502, 503, 504, 307} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			const body = `{"attack_type":"throw","expected_damage":20}`
			const contentType = "application/json; charset=utf-8"
			//记录attack调用次数的
			var calls atomic.Int32
			attack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				data, err := io.ReadAll(r.Body)
				//检查battle传给attack的请求是否正确
				if err != nil || string(data) != body || r.Method != http.MethodPost ||
					r.URL.Path != "/internal/v1/attack-validations" || r.Header.Get("Content-Type") != contentType {
					t.Errorf("转发请求不一致：method=%s path=%s body=%q err=%v", r.Method, r.URL.Path, data, err)
				}
				w.Header().Set("Location", "/should-not-follow")
				w.WriteHeader(code)
				_, _ = w.Write([]byte("这段上游响应体不应转发给前端"))
			}))
			defer attack.Close()
			battle := httptest.NewServer(newHandler(attack.URL+"/internal/v1/attack-validations", false))
			defer battle.Close()
			response, err := battle.Client().Post(battle.URL+validationPath, contentType, strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != code || len(data) != 0 {
				t.Fatalf("期望 %d 和空响应体，实际 status=%d body=%q err=%v", code, response.StatusCode, data, err)
			}
			if calls.Load() != 1 {
				t.Errorf("每次请求只应调用 attack 一次，实际 %d 次", calls.Load())
			}
		})
	}
}

// TestInvalidInputStillForwarded 确认格式校验由 attack 完成，battle 不提前拦截。
func TestInvalidInputStillForwarded(t *testing.T) {
	attack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		if string(data) != "broken json" || r.Header.Get("Content-Type") != "" {
			t.Error("请求体和缺失的 Content-Type 应保留原样")
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer attack.Close()
	response := httptest.NewRecorder()
	newHandler(attack.URL, false).ServeHTTP(response, httptest.NewRequest(http.MethodPost, validationPath, strings.NewReader("broken json")))
	checkResponse(t, response, http.StatusUnprocessableEntity)
}

// TestRouting 路径或方法错误时直接返回，不应调用 attack。
func TestRouting(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/unknown", http.StatusNotFound},
		{http.MethodPost, "/api/v1/dev/500", http.StatusNotFound},
		{http.MethodGet, validationPath, http.StatusMethodNotAllowed},
		{http.MethodHead, validationPath, http.StatusMethodNotAllowed},
	} {
		response := httptest.NewRecorder()
		// 无效上游地址可验证路由错误不会继续进入转发逻辑。
		newHandler(":invalid", false).ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		checkResponse(t, response, tc.want)
		if tc.want == http.StatusMethodNotAllowed && response.Header().Get("Allow") != http.MethodPost {
			t.Error("405 应声明 Allow: POST")
		}
	}
}

// TestTimeout 验证实际的 500ms 调用超时，且不重试。
func TestTimeout(t *testing.T) {
	var calls atomic.Int32
	attack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-r.Context().Done()
	}))
	defer attack.Close()
	started := time.Now()
	response := httptest.NewRecorder()
	newHandler(attack.URL, false).ServeHTTP(response, httptest.NewRequest(http.MethodPost, validationPath, nil))
	checkResponse(t, response, http.StatusGatewayTimeout)
	if elapsed := time.Since(started); elapsed < 400*time.Millisecond || elapsed > 2*time.Second {
		t.Errorf("500ms 超时耗时异常：%s", elapsed)
	}
	if calls.Load() != 1 {
		t.Errorf("超时后不应重试，实际调用 %d 次", calls.Load())
	}
}

// TestUnavailable 使用已关闭的本地服务验证连接失败返回 503。
func TestUnavailable(t *testing.T) {
	attack := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	attack.Close()
	response := httptest.NewRecorder()
	newHandler(attack.URL, false).ServeHTTP(response, httptest.NewRequest(http.MethodPost, validationPath, nil))
	checkResponse(t, response, http.StatusServiceUnavailable)
}

// TestMalformedResponse 用临时 TCP 服务返回损坏的 HTTP 响应，验证 502。
func TestMalformedResponse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
		buffer := make([]byte, 4096)
		_, _ = connection.Read(buffer)
		_, _ = io.WriteString(connection, "not-an-http-response\r\n\r\n")
	}()
	response := httptest.NewRecorder()
	newHandler("http://"+listener.Addr().String(), false).ServeHTTP(response, httptest.NewRequest(http.MethodPost, validationPath, nil))
	checkResponse(t, response, http.StatusBadGateway)
}

// TestInternalError 验证 battle 自身无法创建转发请求时返回空的 500。
func TestInternalError(t *testing.T) {
	response := httptest.NewRecorder()
	newHandler(":invalid", false).ServeHTTP(response, httptest.NewRequest(http.MethodPost, validationPath, nil))
	checkResponse(t, response, http.StatusInternalServerError)
}

// checkResponse 统一检查状态码以及响应体为空的约定。
func checkResponse(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want || response.Body.Len() != 0 {
		t.Errorf("期望 %d 和空响应体，实际 status=%d body=%q", want, response.Code, response.Body.String())
	}
}

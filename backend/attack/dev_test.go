// 文件说明：验证开发接口开关、路径和方法、延迟响应、请求取消及正常攻击不受影响。
package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDevRouting(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		dev                bool
		want               int
	}{
		{"关闭时 500 路径不存在", "POST", dev500Path, false, 404},
		{"关闭时延迟路径不存在", "POST", devTimeoutPath, false, 404},
		{"关闭时 GET 也返回 404", "GET", dev500Path, false, 404},
		{"开启时模拟 500", "POST", dev500Path, true, 500},
		{"500 接口拒绝 GET", "GET", dev500Path, true, 405},
		{"延迟接口拒绝 GET", "GET", devTimeoutPath, true, 405},
		{"拒绝 HEAD", "HEAD", dev500Path, true, 405},
		{"未知开发路径", "POST", "/internal/v1/dev/503", true, 404},
		{"路径不接受尾部斜线", "POST", dev500Path + "/", true, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			response := httptest.NewRecorder()
			newHandler(tc.dev).ServeHTTP(response, request)
			checkResponse(t, response, tc.want)
			if tc.want == 405 && response.Header().Get("Allow") != "POST" {
				t.Error("405 应声明 Allow: POST")
			}
		})
	}
}

// TestDevTimeoutResponse 通过真实 HTTP 请求确认等待至少 600ms 后返回空的 200。
func TestDevTimeoutResponse(t *testing.T) {
	server := httptest.NewServer(newHandler(true))
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 3 * time.Second}
	started := time.Now()
	response, err := client.Post(server.URL+devTimeoutPath, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || len(body) != 0 {
		t.Fatalf("期望空的 200，实际 status=%d body=%q err=%v", response.StatusCode, body, err)
	}
	if elapsed := time.Since(started); elapsed < 600*time.Millisecond {
		t.Fatalf("响应过早：%s", elapsed)
	}
}

// TestDevTimeoutCancellation 确认 HTTP 调用方取消后，服务不会继续等满 600ms。
func TestDevTimeoutCancellation(t *testing.T) {
	started, finished := make(chan struct{}), make(chan struct{})
	handler := newHandler(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		defer close(finished)
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "POST", server.URL+devTimeoutPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		response, err := server.Client().Do(request)
		if response != nil {
			response.Body.Close()
		}
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("请求未到达测试服务")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("请求取消后没有及时结束等待")
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("期望调用被取消，实际 %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("客户端未结束")
	}
}

// TestValidationWithDevMode 检查开关开发模式均不改变正常攻击，也不恢复旧故障指令。
func TestValidationWithDevMode(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		handler := newHandler(enabled)
		for _, tc := range []struct {
			body string
			want int
		}{
			{`{"attack_type":"throw","expected_damage":20}`, 200},
			{`{"attack_type":"throw","expected_damage":999}`, 422},
			{`{"attack_type":"invalid","expected_damage":20}`, 400},
		} {
			request := httptest.NewRequest("POST", validationPath, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Pokemon-Debug-Fault", "attack-500")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			checkResponse(t, response, tc.want)
		}
	}
}

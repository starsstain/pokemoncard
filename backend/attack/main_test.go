// 文件说明：验证 attack 的伤害校验、请求错误处理和所有响应体为空的约定。
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestLegacyFaultIgnored 确认旧请求头不再模拟 500，也不会增加延迟导致 battle 超时。
func TestLegacyFaultIgnored(t *testing.T) {
	server := httptest.NewServer(newHandler(true))
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for _, fault := range []string{"attack-500", "attack-timeout"} {
		t.Run(fault, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, server.URL+validationPath,
				strings.NewReader(`{"attack_type":"throw","expected_damage":20}`))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Pokemon-Debug-Fault", fault)
			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("普通请求不应被旧请求头延迟或中断：%v", err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != http.StatusOK || len(body) != 0 {
				t.Fatalf("期望空的 200，实际 status=%d body=%q err=%v", response.StatusCode, body, err)
			}
		})
	}
}

// TestAttackValidation 用不同请求检查三种正常攻击、数值不匹配及基本格式错误。
func TestAttackValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"enemy_collision 为 7", `{"attack_type":"enemy_collision","expected_damage":7}`, http.StatusOK},
		{"collision 为 18", `{"attack_type":"collision","expected_damage":18}`, http.StatusOK},
		{"throw 为 20", `{"attack_type":"throw","expected_damage":20}`, http.StatusOK},
		{"enemy_collision 数值不匹配", `{"attack_type":"enemy_collision","expected_damage":18}`, http.StatusUnprocessableEntity},
		{"collision 数值不匹配", `{"attack_type":"collision","expected_damage":7}`, http.StatusUnprocessableEntity},
		{"throw 数值不匹配", `{"attack_type":"throw","expected_damage":999}`, http.StatusUnprocessableEntity},
		{"零伤害", `{"attack_type":"throw","expected_damage":0}`, http.StatusUnprocessableEntity},
		{"负伤害", `{"attack_type":"throw","expected_damage":-1}`, http.StatusUnprocessableEntity},
		{"未知攻击类型", `{"attack_type":"fire","expected_damage":20}`, http.StatusBadRequest},
		{"类型大小写错误", `{"attack_type":"Throw","expected_damage":20}`, http.StatusBadRequest},
		{"空类型", `{"attack_type":"","expected_damage":20}`, http.StatusBadRequest},
		{"缺少类型", `{"expected_damage":20}`, http.StatusBadRequest},
		{"缺少伤害", `{"attack_type":"throw"}`, http.StatusBadRequest},
		{"类型为 null", `{"attack_type":null,"expected_damage":20}`, http.StatusBadRequest},
		{"伤害为 null", `{"attack_type":"throw","expected_damage":null}`, http.StatusBadRequest},
		{"类型不是字符串", `{"attack_type":20,"expected_damage":20}`, http.StatusBadRequest},
		{"伤害为字符串", `{"attack_type":"throw","expected_damage":"20"}`, http.StatusBadRequest},
		{"伤害为小数", `{"attack_type":"throw","expected_damage":20.5}`, http.StatusBadRequest},
		{"伤害为布尔值", `{"attack_type":"throw","expected_damage":true}`, http.StatusBadRequest},
		{"空请求体", ``, http.StatusBadRequest},
		{"JSON 损坏", `{"attack_type":`, http.StatusBadRequest},
		{"顶层为数组", `[]`, http.StatusBadRequest},
		{"顶层为 null", `null`, http.StatusBadRequest},
		{"多个 JSON 对象", `{"attack_type":"throw","expected_damage":20}{}`, http.StatusBadRequest},
		{"尾部无效字符", `{"attack_type":"throw","expected_damage":20}x`, http.StatusBadRequest},
		{"尾部空白允许", "{\"attack_type\":\"throw\",\"expected_damage\":20}\n ", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, validationPath, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handleValidation(response, req)
			checkResponse(t, response, tc.want)
		})
	}
}

// TestHTTPContract 检查地址、方法及请求媒体类型；路由错误也不返回文字。
func TestHTTPContract(t *testing.T) {
	cases := []struct {
		name, method, path, contentType string
		want                            int
	}{
		{"带字符集的 JSON", http.MethodPost, validationPath, "application/json; charset=utf-8", http.StatusOK},
		{"缺少 Content-Type", http.MethodPost, validationPath, "", http.StatusBadRequest},
		{"错误 Content-Type", http.MethodPost, validationPath, "text/plain", http.StatusBadRequest},
		{"未知路径", http.MethodPost, "/unknown", "application/json", http.StatusNotFound},
		{"开发模式关闭", http.MethodPost, "/internal/v1/dev/500", "", http.StatusNotFound},
		{"错误方法", http.MethodGet, validationPath, "application/json", http.StatusMethodNotAllowed},
		{"HEAD 方法", http.MethodHead, validationPath, "application/json", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"attack_type":"throw","expected_damage":20}`))
			req.Header.Set("Content-Type", tc.contentType)
			response := httptest.NewRecorder()
			newHandler(false).ServeHTTP(response, req)
			checkResponse(t, response, tc.want)
			if tc.want == http.StatusMethodNotAllowed && response.Header().Get("Allow") != http.MethodPost {
				t.Error("405 应通过 Allow 响应头声明 POST")
			}
		})
	}
}

// panicReader 只在测试中模拟内部异常，不给服务增加故障模拟接口。
type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("测试内部异常") }

func TestInternalErrorHasEmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, validationPath, panicReader{})
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handleValidation(response, req)
	checkResponse(t, response, http.StatusInternalServerError)
}

// TestHTTPServer 通过真实本地 HTTP 连接确认状态码与空响应体，而不只直接调用函数。
func TestHTTPServer(t *testing.T) {
	server := httptest.NewServer(newHandler(false))
	t.Cleanup(server.Close)
	response, err := server.Client().Post(server.URL+validationPath, "application/json", strings.NewReader(`{"attack_type":"throw","expected_damage":20}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || len(body) != 0 {
		t.Fatalf("期望 200 和空响应体，实际 status=%d body=%q", response.StatusCode, body)
	}
}

// checkResponse 同时检查状态码和空响应体，防止错误分支意外返回 JSON 或文字。
func checkResponse(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Errorf("期望状态码 %d，实际 %d", want, response.Code)
	}
	if response.Body.Len() != 0 {
		t.Errorf("响应体必须为空，实际为 %q", response.Body.String())
	}
}

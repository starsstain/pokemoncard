// 文件说明：注册七个开发测试入口，由 battle 准备测试请求或直接模拟状态码。
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"school-gitlab.xsolla.dev/yijun_gao/pokemoncard/backend/internal/observability"
)

// registerDevRoutes 只在开发模式调用；路由表在启动后不再修改。
func registerDevRoutes(routes map[string]func(*http.Request) int, client *http.Client, attackURL string) {
	for _, scenario := range []struct {
		path, attackPath, body string
		localStatus            int
	}{
		{path: "/api/v1/dev/200", body: `{"attack_type":"throw","expected_damage":20}`},
		{path: "/api/v1/dev/400", body: `{"attack_type":"invalid","expected_damage":20}`},
		{path: "/api/v1/dev/422", body: `{"attack_type":"throw","expected_damage":999}`},
		{path: "/api/v1/dev/500", attackPath: "/internal/v1/dev/500"},
		{path: "/api/v1/dev/502", localStatus: http.StatusBadGateway},
		{path: "/api/v1/dev/503", localStatus: http.StatusServiceUnavailable},
		{path: "/api/v1/dev/504", attackPath: "/internal/v1/dev/timeout"},
	} {
		routes[scenario.path] = func(r *http.Request) int {
			if scenario.localStatus != 0 {
				observability.RecordRequestError(r.Context(), "development_fault", fmt.Errorf("development endpoint returned HTTP %d", scenario.localStatus))
				return scenario.localStatus // 502、503 不调用 attack。
			}
			target := attackURL
			if scenario.attackPath != "" {
				base, err := url.Parse(attackURL)
				if err != nil {
					logWithTrace(r, "ERROR", "could not parse attack URL: %v", err)
					return http.StatusInternalServerError
				}
				// 使用配置的 attack 主机，替换为内部开发路径。
				target = base.ResolveReference(&url.URL{Path: scenario.attackPath}).String()
			}
			var body io.Reader
			if scenario.body != "" {
				body = strings.NewReader(scenario.body)
			}
			// 请求内容由后端固定准备，不使用前端传来的 body 或故障请求头。
			request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, body)
			if err != nil {
				logWithTrace(r, "ERROR", "could not create development request: %v", err)
				return http.StatusInternalServerError
			}
			if scenario.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			return forwardRequest(client, request)
		}
	}
}

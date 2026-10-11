// 文件说明：启动 attack 服务，校验三种攻击的固定伤害，并仅返回 HTTP 状态码。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"

	"github.com/DataDog/datadog-go/v5/statsd"
	ddhttp "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"school-gitlab.xsolla.dev/yijun_gao/pokemoncard/backend/internal/config"
)

const validationPath = "/internal/v1/attack-validations"

// attackRequest 对应请求 JSON；指针用于区分缺失或 null 与明确提交的整数 0。
type attackRequest struct {
	AttackType     string `json:"attack_type"`
	ExpectedDamage *int   `json:"expected_damage"`
}

// statsd 客户端：发业务指标（DogStatsD）。
var statsdClient = mustStatsd()

func mustStatsd() *statsd.Client {
	// Agent 默认监听 8125/UDP；与 trace 上报一样显式写 IPv4。
	c, err := statsd.New("127.0.0.1:8125")
	if err != nil {
		// 指标上报失败不影响服务启动，降级为 no-op。
		log.Printf("attack statsd 初始化失败，指标不上报：%v", err)
		return nil
	}
	return c
}

func main() {
	settings, err := config.Load(".env")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("attack 开发接口开启=%t", settings.DevMode)
	const address = "127.0.0.1:8081"
	// APM：启动 Datadog 追踪器，trace 数据经本机 Agent（8126 端口）上报。
	// 显式写 IPv4 地址：Windows 上默认 localhost 先解析为 ::1，Agent 只监听 IPv4。
	tracer.Start(
		tracer.WithService("attack"),
		tracer.WithEnv("school"),
		tracer.WithAgentAddr("127.0.0.1:8126"),
		tracer.WithGlobalTag("version", "1.0.0"),
		tracer.WithGlobalTag("team", "bj-xsolla-school-group-2"),
	)
	defer tracer.Stop()
	log.Printf("attack 正在监听 http://%s%s", address, validationPath)
	wrapped := ddhttp.WrapHandler(newHandler(settings.DevMode), "attack", validationPath)
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
	prefix := fmt.Sprintf("level=%s service=attack dd.trace_id=%s ", level, traceID)
	log.Printf(prefix+msg, args...)
}

// newHandler 在启动时建立路由表；非开发模式不注册开发路径。
func newHandler(devMode bool) http.Handler {
	routes := map[string]http.HandlerFunc{validationPath: handleValidation}
	//如果是开发者模式，加两个模拟故障的路由
	if devMode {
		routes[dev500Path] = handleDev
		routes[devTimeoutPath] = handleDev
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//查找当前请求路径在不在路由表里
		if handler, exists := routes[r.URL.Path]; exists {
			handler(w, r)
			return
		}
		// 未注册路径直接返回空的 404，不使用会附带文字的默认错误页。
		w.WriteHeader(http.StatusNotFound)
		log.Printf("attack method=%s path=%s status=%d", r.Method, r.URL.Path, http.StatusNotFound)
	})
}

// handleValidation 读取请求并返回状态码；路径、方法和内部错误的响应体也保持为空。
func handleValidation(w http.ResponseWriter, r *http.Request) {
	status := http.StatusInternalServerError
	// 所有分支统一在退出时写响应，不调用会附带错误文字的 http.Error。
	defer func() {
		if problem := recover(); problem != nil {
			status = http.StatusInternalServerError
			logWithTrace(r, "ERROR", "attack 内部异常：%v", problem)
		}
		w.WriteHeader(status)
		logWithTrace(r, "INFO", "attack method=%s path=%s status=%d", r.Method, r.URL.Path, status)
		// 业务指标：按结果计数，result=success|failure。
		if statsdClient != nil {
			result := "success"
			if status >= 400 {
				result = "failure"
			}
			_ = statsdClient.Incr("pokemon.attack.validation.count", []string{"result:" + result}, 1)
		}
	}()
	//请求校验
	if r.URL.Path != validationPath {
		status = http.StatusNotFound
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		status = http.StatusMethodNotAllowed
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		status = http.StatusBadRequest
		logWithTrace(r, "WARN", "attack Content-Type 无效：%v", err)
		return
	}

	var input attackRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		status = http.StatusBadRequest
		logWithTrace(r, "WARN", "attack JSON 解析失败：%v", err)
		return
	}
	// 一个请求只能有一个 JSON 对象；拒绝对象后拼接另一份 JSON 或无效字符。
	if err := decoder.Decode(new(any)); err != io.EOF {
		status = http.StatusBadRequest
		logWithTrace(r, "WARN", "attack 请求体多余内容：%v", err)
		return
	}
	status = validateAttack(r, input)
}

// validateAttack 先确认字段与类型，再比较固定伤害
func validateAttack(r *http.Request, input attackRequest) int {
	// 自定义 span tag：把关键业务实体打到 trace 上，方便在 APM 里筛选。
	if span, ok := tracer.SpanFromContext(r.Context()); ok {
		span.SetTag("attack.type", input.AttackType)
	}

	if input.ExpectedDamage == nil {
		logWithTrace(r, "WARN", "attack 缺少 expected_damage 字段，attack_type=%s", input.AttackType)
		return http.StatusBadRequest
	}

	var damage int
	switch input.AttackType {
	case "enemy_collision":
		damage = 7
	case "collision":
		damage = 18
	case "throw":
		damage = 20
	default:
		// 空字符串、未知类型及大小写错误统一返回 400。
		logWithTrace(r, "WARN", "attack 未知 attack_type=%s", input.AttackType)
		return http.StatusBadRequest
	}
	if *input.ExpectedDamage != damage {
		// 业务校验失败：HTTP 不是 5xx，但业务不健康，打 ERROR。
		logWithTrace(r, "ERROR", "attack 伤害不匹配：type=%s expected=%d actual=%d", input.AttackType, *input.ExpectedDamage, damage)
		return http.StatusUnprocessableEntity
	}
	return http.StatusOK
}

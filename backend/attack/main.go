// 文件说明：启动 attack 服务，校验三种攻击的固定伤害，并仅返回 HTTP 状态码。
package main

import (
	"encoding/json"
	"io"
	"log"
	"mime"
	"net/http"
)

const validationPath = "/internal/v1/attack-validations"

// attackRequest 对应请求 JSON；指针用于区分缺失或 null 与明确提交的整数 0。
type attackRequest struct {
	AttackType     string `json:"attack_type"`
	ExpectedDamage *int   `json:"expected_damage"`
}

// main 启动独立的 attack 进程；battle 将在后续通过 HTTP 调用此服务。
func main() {
	const address = "127.0.0.1:8081"
	log.Printf("attack 正在监听 http://%s%s", address, validationPath)
	log.Fatal(http.ListenAndServe(address, http.HandlerFunc(handleValidation)))
}

// handleValidation 读取请求并返回状态码；路径、方法和内部错误的响应体也保持为空。
func handleValidation(w http.ResponseWriter, r *http.Request) {
	status := http.StatusInternalServerError
	// 所有分支统一在退出时写响应，不调用会附带错误文字的 http.Error。
	defer func() {
		if problem := recover(); problem != nil {
			status = http.StatusInternalServerError
			log.Printf("attack 内部异常：%v", problem)
		}
		w.WriteHeader(status)
		log.Printf("attack method=%s path=%s status=%d", r.Method, r.URL.Path, status)
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
		return
	}

	var input attackRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		status = http.StatusBadRequest
		return
	}
	// 一个请求只能有一个 JSON 对象；拒绝对象后拼接另一份 JSON 或无效字符。
	if err := decoder.Decode(new(any)); err != io.EOF {
		status = http.StatusBadRequest
		return
	}
	status = validateAttack(input)
}

// validateAttack 先确认字段与类型，再比较固定伤害
func validateAttack(input attackRequest) int {
	if input.ExpectedDamage == nil {
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
		return http.StatusBadRequest
	}
	//校验的结果不对
	if *input.ExpectedDamage != damage {
		return http.StatusUnprocessableEntity
	}
	return http.StatusOK
}

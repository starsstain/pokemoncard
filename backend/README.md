# PokemonCard 后端：attack 与 battle 服务

attack 校验固定伤害；battle 接收前端请求，将请求体和 Content-Type 转发给 attack，并将状态码返回前端。两个服务独立运行，响应体均为空。

## 启动与测试

安装 Go 1.26 或兼容版本，在本目录执行：

```powershell
go run ./attack
```

attack 监听 `127.0.0.1:8081`。另开一个终端，同样进入本目录，启动 battle：

```powershell
go run ./battle
```

battle 监听 `127.0.0.1:8080`，调用本机 8081 上的 attack。各终端按 Ctrl+C 停止对应服务。只使用 Go 标准库，无须安装其他依赖或准备数据库。

运行测试与静态检查：

```powershell
go test ./...
go vet ./...
```

## 接口约定

前端接口：`POST http://127.0.0.1:8080/api/v1/attack-validations`。

内部接口：`POST http://127.0.0.1:8081/internal/v1/attack-validations`，供 battle 调用，也可单独测试 attack。

请求头：`Content-Type: application/json`。请求体仅约定两个字段：

```json
{"attack_type":"throw","expected_damage":20}
```

| attack_type | 固定伤害 |
| --- | ---: |
| enemy_collision | 7 |
| collision | 18 |
| throw | 20 |

`expected_damage` 为上报的名义伤害，不是目标实际减少的 HP。类型名称区分大小写。

| 状态码 | 含义 |
| --- | --- |
| 200 | 类型合法，伤害匹配 |
| 400 | 未知或空攻击类型、字段缺失或 null、字段类型错误、无效 JSON，或 Content-Type 不是 application/json |
| 422 | 类型合法，整数伤害不匹配；包括 0 和负数 |
| 500 | 请求处理过程中发生被捕获的内部异常 |
| 502 | battle 未能取得合法的上游 HTTP 响应，例如响应格式损坏 |
| 503 | battle 无法连接 attack |
| 504 | battle 调用 attack 超时（500ms） |
| 404 / 405 | 请求路径不存在 / 方法不是 POST |

**所有响应体为空。** 不返回 JSON、文本或错误消息。终端记录请求状态码，内部异常和转发失败额外记录错误原因。前端通过 battle 调用，只读取 `response.status`，不要调用 `response.json()`；422 按接口约定提示对局失败并退出战斗，技术错误不能当作伤害不匹配判负。

battle 不解析 JSON、不校验伤害、不自动重试，也不跟随上游重定向。收到合法 HTTP 响应后原样返回其状态码，包括 attack 返回的 500；仅在调用失败时由 battle 产生 502、503 或 504。battle 创建转发请求失败时返回 500。

本地前端通过 Vite 将 `/api` 代理至 `http://127.0.0.1:8080`，浏览器使用相对路径 `/api/v1/attack-validations`。代理由前端项目配置，两个后端服务默认仅监听本机。

## 手动验证

保持两个服务运行，在另一个 PowerShell 终端逐条执行，验证 battle → attack 的完整链路。使用 `curl.exe` 避免与 PowerShell 的别名混淆；`-i` 显示响应头和状态码。

匹配，预期 200：

```powershell
'{"attack_type":"throw","expected_damage":20}' | curl.exe -i -X POST 'http://127.0.0.1:8080/api/v1/attack-validations' -H 'Content-Type: application/json' --data-binary '@-'
```

数值不匹配，预期 422：

```powershell
'{"attack_type":"throw","expected_damage":999}' | curl.exe -i -X POST 'http://127.0.0.1:8080/api/v1/attack-validations' -H 'Content-Type: application/json' --data-binary '@-'
```

未知类型，预期 400：

```powershell
'{"attack_type":"fire","expected_damage":20}' | curl.exe -i -X POST 'http://127.0.0.1:8080/api/v1/attack-validations' -H 'Content-Type: application/json' --data-binary '@-'
```

停止 attack、保持 battle 运行后，重复上述请求应返回 503。恢复 attack 后可以继续请求。超时、损坏响应及状态码透传由自动化测试使用临时上游验证，正式服务不提供故障模拟接口。

## 文件用途

- `go.mod`：Go 模块定义。
- `attack/main.go`：启动服务、解析请求、判断固定伤害并返回状态码。
- `attack/main_test.go`：验证状态码、空响应体和实际 HTTP 调用。
- `battle/main.go`：转发请求、透传状态码并处理上游调用失败。
- `battle/main_test.go`：验证原样转发、不重试、超时和异常状态码。

固定伤害直接写在代码中。不使用规则文件，不保存对局、血量或攻击历史，不提供故障模拟接口。

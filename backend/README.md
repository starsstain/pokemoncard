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

battle 监听 `127.0.0.1:8080`，调用本机 8081 上的 attack。各终端按 Ctrl+C 停止对应服务。首次运行时 Go 自动下载 `.env` 解析依赖 `github.com/joho/godotenv`，无需准备数据库。

### 开发模式配置

**当前版本已提供 battle 的七个开发测试入口和 attack 的两个内部开发接口。完整测试需两个服务都开启开发模式，前端测试按钮需调用新的 battle 路径。**

首次在 `backend` 目录复制配置示例（已有 `.env` 时不要覆盖）：

```powershell
Copy-Item .env.example .env
```

示例默认 `APP_ENV=prod`；需要开发模式时，将 `.env` 中这一行改为 `APP_ENV=dev`。文件保存在本机，只需复制一次，不提交 Git；`.env.example` 随仓库提交。无 `.env` 时也可以正常启动。

- 两个服务均从 `backend` 目录启动，读取同一份 `.env`。
- 已有进程环境变量优先，包括明确设置为空值的情况；只有精确的 `dev` 启用开发模式配置，其余值均关闭。
- 希望使用文件配置时，在对应终端执行 `Remove-Item Env:APP_ENV -ErrorAction SilentlyContinue`，清除旧的终端设置。
- 配置仅在启动时读取，修改后需重启服务；文件缺失且环境变量未设置时默认关闭，文件无法读取或格式错误时启动失败。

正常攻击接口不再识别 `X-Pokemon-Debug-Fault`，battle 也不转发此请求头。前端测试按钮应改用下面的专用开发路径。真实异常、服务不可用和超时仍按原约定返回状态码。

### 自动化检查

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

停止 attack、保持 battle 运行后，重复上述请求应返回 503。恢复 attack 后可以继续请求。正常接口的超时、损坏响应及状态码透传由自动化测试使用临时上游验证。

### attack 内部开发接口

在 `backend/.env` 设置 `APP_ENV=dev`，确认终端无冲突的 `APP_ENV`，重启 attack。启动日志显示 `attack 开发接口开启=true`。

| 接口 | 响应 |
| --- | --- |
| POST /internal/v1/dev/500 | 500，响应体为空 |
| POST /internal/v1/dev/timeout | 等待 600ms 后返回 200，响应体为空；调用方取消时提前结束等待 |

无需请求体或 Content-Type。以下命令供后端直接验证；前端通过 battle 调用：

```powershell
curl.exe -i -X POST 'http://127.0.0.1:8081/internal/v1/dev/500'
curl.exe -i -X POST 'http://127.0.0.1:8081/internal/v1/dev/timeout'
```

开发模式关闭时，两条路径均返回空的 404；开启时，错误方法返回空的 405 和 `Allow: POST`，未知路径及带尾部斜线的路径返回空的 404。

直接访问延迟接口时，等待完成后得到的是 200；battle 使用 500ms 调用预算，超时后向前端返回 504。开发接口不修改血量或对局状态。

### battle 开发接口联调

两个服务都开启开发模式并重启后，前端发送 `POST /api/v1/dev/{status}`，无需请求体、Content-Type 或特殊请求头。

| status | battle 处理方式 | 正常测试条件下的响应 |
| --- | --- | --- |
| 200 | 向 attack 提交 throw、20 | 200 |
| 400 | 向 attack 提交 invalid、20 | 400 |
| 422 | 向 attack 提交 throw、999 | 422 |
| 500 | 调用 attack 内部开发 500 接口 | 500 |
| 502 | 本地模拟，不调用 attack | 502 |
| 503 | 本地模拟，不调用 attack | 503 |
| 504 | 调用 attack 延迟接口，触发真实超时 | 504 |

在 PowerShell 验证七种状态码（`size_download=0` 表示响应体为空）：

```powershell
foreach ($statusCode in 200,400,422,500,502,503,504) {
    curl.exe -sS -o NUL -w "HTTP %{http_code}, size_download=%{size_download}\n" -X POST "http://127.0.0.1:8080/api/v1/dev/$statusCode"
}
```

返回的是实际 HTTP 状态码，响应体均为空，不自动重试。只开启 battle 时，500、504 场景会透传 attack 的 404；attack 停止时，需要上游的入口返回 503，两个本地模拟入口仍可用。

battle 开发模式关闭时，上述路径均返回空的 404；开启后，未知路径返回空的 404，错误业务方法返回空的 405 和 `Allow: POST`。支持路径的 OPTIONS 预检返回空的 204，不调用 attack、不触发模拟故障。

前端只读取 `response.status`，不要调用 `response.json()`。测试结果在 Network 面板查看，不触发扣血、判负或重开；生产模式隐藏测试按钮的工作由前端配合完成。

## 文件用途

- `go.mod`：Go 模块定义。
- `go.sum`：依赖校验信息。
- `.env.example`：本地配置模板，默认关闭开发模式配置。
- `internal/config/config.go`：读取配置，统一开发模式判断和优先级。
- `internal/config/config_test.go`：验证配置读取、优先级及错误处理。
- `attack/main.go`：启动服务、解析请求、判断固定伤害并返回状态码。
- `attack/main_test.go`：验证状态码、空响应体和实际 HTTP 调用。
- `attack/dev.go`：模拟内部 500 和可取消的延迟响应。
- `attack/dev_test.go`：验证开发接口路由、开关、延迟和请求取消。
- `battle/main.go`：转发请求、透传状态码并处理上游调用失败。
- `battle/main_test.go`：验证原样转发、不重试、超时和异常状态码。
- `battle/dev.go`：注册七个开发入口，生成测试请求或模拟本地状态码。
- `battle/dev_test.go`：验证路由开关、测试数据、实际状态透传和超时取消。

固定伤害直接写在代码中。不使用规则文件，不保存对局、血量或攻击历史。故障模拟仅通过开发模式下的专用接口提供。

# PokemonCard 后端：attack 服务

当前交付 attack 伤害校验服务。battle 转发服务将在后续独立变更中实现；本阶段通过内部接口直接验证 attack，不代表前端调用链已完成。

## 启动与测试

安装 Go 1.26 或兼容版本，在本目录执行：

```powershell
go run ./attack
```

服务监听 `127.0.0.1:8081`。按 Ctrl+C 停止。只使用 Go 标准库，无须安装其他依赖或准备数据库。

运行测试与静态检查：

```powershell
go test ./...
go vet ./...
```

## 接口约定

内部接口：`POST http://127.0.0.1:8081/internal/v1/attack-validations`。

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
| 404 / 405 | 请求路径不存在 / 方法不是 POST |

**所有响应体为空。** 不返回 JSON、文本或错误消息；具体错误记录在终端日志中。前端最终通过 battle 调用，并按状态码处理；422 按接口约定提示对局失败并退出战斗，技术错误不能当作伤害不匹配判负。

## 手动验证

保持服务运行，在另一个 PowerShell 终端逐条执行。使用 `curl.exe` 避免与 PowerShell 的别名混淆；`-i` 显示响应头和状态码。

匹配，预期 200：

```powershell
'{"attack_type":"throw","expected_damage":20}' | curl.exe -i -X POST 'http://127.0.0.1:8081/internal/v1/attack-validations' -H 'Content-Type: application/json' --data-binary '@-'
```

数值不匹配，预期 422：

```powershell
'{"attack_type":"throw","expected_damage":999}' | curl.exe -i -X POST 'http://127.0.0.1:8081/internal/v1/attack-validations' -H 'Content-Type: application/json' --data-binary '@-'
```

未知类型，预期 400：

```powershell
'{"attack_type":"fire","expected_damage":20}' | curl.exe -i -X POST 'http://127.0.0.1:8081/internal/v1/attack-validations' -H 'Content-Type: application/json' --data-binary '@-'
```

## 文件用途

- `go.mod`：Go 模块定义。
- `attack/main.go`：启动服务、解析请求、判断固定伤害并返回状态码。
- `attack/main_test.go`：验证状态码、空响应体和实际 HTTP 调用。

固定伤害直接写在代码中。不使用规则文件，不保存对局、血量或攻击历史，不提供故障模拟接口。

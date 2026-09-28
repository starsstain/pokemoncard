# 可观测性文档（OBSERVABILITY）

## 架构

```
浏览器（前端）
    │  POST /api/v1/attack-validations
    ▼
battle（:8080）───HTTP───▶ attack（:8081）
    │                        │
    └── dd-trace-go ─────────┘
         （分布式 trace：battle→attack 一条链路）
```

- **前端**：浏览器里的宝可梦对战游戏，玩家点击投掷球触发攻击。
- **battle**：对外网关，接收前端请求，转发给 attack，原样返回状态码。
- **attack**：校验攻击类型与固定伤害是否匹配，返回 200/400/422。

## 四大支柱埋点位置

| 支柱 | 在哪里埋的 |
|------|-----------|
| **Trace（APM）** | `attack/main.go`、`battle/main.go`：`ddhttp.WrapHandler` 包裹入口，`ddhttp.WrapClient` 包裹转发客户端，battle→attack 上下文自动透传。attack 校验时打自定义 span tag `attack.type`。 |
| **日志（Log）** | `logWithTrace()` 函数输出 `level= service= dd.trace_id=` 结构化日志，实现 log↔trace 互跳。失败路径（422/500/转发失败）打 ERROR，参数错误打 WARN。 |
| **指标（Metric）** | attack 每次校验后通过 DogStatsD 发 `pokemon.attack.validation.count{result:success|failure}` 业务指标。系统资源（CPU/内存/磁盘）由 Datadog Agent 自动采集。 |
| **RUM** | 前端接入（孟令奇负责），上报页面加载、JS 错误、用户操作。 |

统一标签：`env:school`、`service`、`version:1.0.0`、`team:bj-xsolla-school-group-2`。

## 监控表

| 监控器 | 类别 | 阈值 | 理由 | 通知 |
|--------|------|------|------|------|
| battle p95 延迟 | 延迟 SLO | > 500ms | battle 业务调用预算 500ms，p95 超阈值说明用户明显卡顿 | Slack |
| battle 5xx 错误 | 错误 | > 0 | battle 出现任何 5xx 都意味着服务异常，需立即排查 | Slack |
| attack 校验失败率 | 业务结果 | > 10% | HTTP 不是 5xx，但伤害不匹配率过高意味着业务逻辑异常或被篡改 | Slack |
| CPU 使用率 | 资源饱和 | > 80% | 持续高 CPU 可能导致请求积压、延迟上升 | Slack |
| attack 5xx 错误 | 错误 | > 0 | attack 内部错误需独立监控，与 battle 区分 | Slack |
| battle 流量中断 | 无数据/心跳 | 5 分钟 < 1 请求 | 流量骤停通常意味着服务挂了或前端入口断了 | Slack |

## 文件结构

```
backend/
  attack/main.go          # APM + 日志 + 业务指标
  battle/main.go          # APM + 日志
  datadog/
    http_check.d/conf.yaml    # battle/attack 健康检查
    pokemon.d/conf.yaml       # 日志采集
monitors/                   # 监控器定义（as-code）
  apm-latency-p95-monitor.json
  http-5xx-monitor.json
  validation-failure-rate-monitor.json
  cpu-monitor.json
  attack-5xx-monitor.json
  no-data-monitor.json
```

## 应用监控器

```bash
export DD_API_KEY=... DD_APP_KEY=... DD_SITE=us5.datadoghq.com
for f in monitors/*.json; do
  curl -sX POST "https://api.${DD_SITE}/api/v1/monitor" \
    -H "DD-API-KEY: ${DD_API_KEY}" -H "DD-APPLICATION-KEY: ${DD_APP_KEY}" \
    -H "Content-Type: application/json" -d @"$f"; echo
done
```

## 截图（待补）

- （a）RUM 会话 → 后端 trace → 关联日志 → 业务指标
- （b）Slack 告警触发与恢复

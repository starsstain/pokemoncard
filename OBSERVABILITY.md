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

> 状态说明：前 6 个已在线上 Datadog 网页中运行（UI 创建、实战验证过告警闭环）；
> 后 3 个为 as-code 定义（attack 专属监控与类别补充），用下方"应用监控器"命令上线。

| 监控器 | 类别 | 阈值 | 理由 | 通知 |
|--------|------|------|------|------|
| battle 服务健康检查 | 可用性/心跳 | can_connect 5分钟均值 < 0.9（Agent 每 60 秒探测真实业务请求） | 探测贯穿 battle→attack 全链路，任一服务挂掉 1~2 分钟内发现，无需真实流量 | Slack |
| battle 出现 5xx 错误 | 错误 | 5 分钟内 5xx 日志 > 3 条 | 容忍零星波动，错误扎堆才是故障信号 | Slack |
| 程序内部异常 | 错误（严重） | 5 分钟内"内部异常"日志 > 0 条 | panic 是崩溃路径，比 5xx 更严重，出现即报 | Slack |
| CPU 持续过高 | 资源饱和 | > 95% 持续 15 分钟 | 防误报设计：个人电脑有游戏/编译等短时飙高，持续超标才算异常 | Slack |
| 内存可用不足 | 资源饱和 | 可用率 < 10% 持续 15 分钟 | Windows 上用 system.mem.pct_usable（无 system.mem.pct） | Slack |
| P95 延迟过高 | 延迟 SLO | p95 > 500ms | battle 调 attack 的客户端超时即 500ms，P95 破线说明大量请求在等超时（典型：attack 变慢） | Slack |
| attack 5xx 错误率过高 | 错误 | > 0（attack 独立监控） | attack 内部错误需与 battle 区分定位，不能只靠 battle 间接发现 | Slack |
| attack 校验失败率过高 | 业务结果 | 422 占比 > 10% | HTTP 非 5xx 但业务不健康：伤害不匹配率过高说明数值逻辑异常或被篡改 | Slack |
| battle 流量中断 | 无数据/心跳 | 5 分钟 < 1 请求 | 流量骤停通常意味着服务挂了或前端入口断了 | Slack |

## 文件结构

```
backend/
  attack/main.go          # APM + 日志 + 业务指标
  battle/main.go          # APM + 日志
  datadog/
    http_check.d/conf.yaml    # battle/attack 健康检查
    pokemon.d/conf.yaml       # 日志采集
monitors/                   # 监控器定义（as-code，与线上监控器一一对应）
  battle-health-monitor.json          # ↔ 线上：battle服务健康检查
  battle-5xx-monitor.json             # ↔ 线上：battle出现5xx错误
  internal-error-monitor.json         # ↔ 线上：程序内部异常
  cpu-monitor.json                    # ↔ 线上：CPU持续过高
  memory-monitor.json                 # ↔ 线上：内存可用不足
  battle-latency-p95-monitor.json     # ↔ 线上：P95延迟过高
  attack-5xx-monitor.json            # as-code：attack 专属 5xx（待应用）
  validation-failure-rate-monitor.json # as-code：attack 校验失败率·业务结果（待应用）
  no-data-monitor.json                # as-code：battle 流量心跳（待应用）
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

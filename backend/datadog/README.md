# 宝可梦后端 Datadog 监控部署指南

本项目对 battle（端口 8080）和 attack（端口 8081）两个 Go 服务做全链路监控。
监控栈全部配置在 `backend/datadog/` 目录下，随仓库走，任何组员 15 分钟可复刻。

## 监控覆盖一览

| 需求 | 实现方式 | Datadog 上的证据 |
|------|----------|------------------|
| 系统资源 | Agent 自动采集 | 监控器：CPU持续过高 / 内存可用不足 / 磁盘空间不足 |
| health check / 功能正常 | Agent http_check 每 60 秒发真实业务请求，断言返回 200 | 监控器：battle服务健康检查（指标 network.http.can_connect，标签 instance:battle-health） |
| 程序 error | 日志采集 + 日志监控器 | 监控器：battle出现5xx错误（5 分钟超 3 条报警）、程序内部异常（panic 日志） |
| p95 / p99 + trace | 代码内嵌 Datadog APM（dd-trace-go） | 监控器：P95/P99延迟过高；APM → Services 里 battle/attack 两个服务、火焰图展示 battle→attack 全链路 |

监控器统一命名 `[宝可梦] xxx`，统一标签 `team:bj-xsolla-school-group-2`（Monitor List 里按标签筛选即可只看我们的）。

## 组员复刻步骤（Windows，约 15 分钟）

### 1. 安装 Datadog Agent（管理员 PowerShell）

从 Datadog 网页 **Integrations → Agent → Windows** 复制安装命令运行，安装时：

- **API key**：用组织里的共享 key（问老师或在 Organization Settings → API Keys 复制）
- **site 必须填 `us5.datadoghq.com`**（学校组织所在站点，填错数据全部发丢且无报错）

### 2. 安装本目录的配置

右键管理员运行 `install-datadog-config.bat`（做三件事：开 logs_enabled、
把 http_check.d 和 pokemon.d 两个配置装进 `C:\ProgramData\Datadog\conf.d\`、重启 Agent）。

### 3. 启动后端服务（Git Bash）

```bash
mkdir -p /c/dd-logs
cd backend
go run ./attack 2>&1 | tee -a /c/dd-logs/attack.log
go run ./battle 2>&1 | tee -a /c/dd-logs/battle.log
```

> trace 上报地址（127.0.0.1:8126）已在代码里显式指定，无需环境变量。
> 历史包袱：Windows 上默认 localhost 会先走 IPv6（::1），而 Agent 只监听
> IPv4 的 127.0.0.1——不显式指定的话 trace 会被静默丢弃。

> 主机名如需正式化，在 `C:\ProgramData\Datadog\datadog.yaml` 加一行
> `hostname: <你的名字>-pokemon-backend` 后重启 Agent。注意：改完后监控器里
> 用 host 过滤的要跟着改（健康检查用的是 instance:battle-health，不受影响）。

### 4. 验证

- Infrastructure → Host Map：能看到自己的机器
- Logs：能搜到 `battle method=POST ... status=200`
- APM → Services：能看到 battle / attack

## 踩坑记录（都是真实发生过的，演示可讲）

1. **site 配错（ap1 → us5）**：DNS 解析失败，数据一条都发不出去，Agent 日志里全是 `no such host`。
2. **API key 无效**：网络通了但服务器回 `403 API key is missing or invalid`，换组织内有效 key 解决。
3. **http_check 请求体参数名**：必须是 `data` 不是 `body`（写错被静默忽略，探测一直返回 400）。
4. **Windows 无 system.mem.pct 指标**：内存监控要用 `system.mem.pct_usable`（可用率，below 0.1 报警）。
5. **主机改名后监控器查询失配**：from 过滤写死 host 名，改名后变 No Data；健康检查改用 `instance:battle-health` 标签过滤，换机器都不怕。
6. **IPv6 localhost 陷阱**：trace 上报地址已在代码中显式写为 127.0.0.1:8126（见 main.go），无需再操心；若将来移除该配置，需在启动时带 `DD_AGENT_HOST=127.0.0.1`。
7. **Agent 重启可能重放历史日志**：曾导致 5xx 监控器收到重复旧日志而误报一次（红一下又恢复）。

## 演示剧本（约 5 分钟）

1. **正常态**：Monitor List 按 team:bj-xsolla-school-group-2 筛选，7 个监控器全绿。
2. **注入错误**：Git Bash 打 6 个 502（或点前端 debug 按钮）：
   ```bash
   for i in $(seq 1 6); do curl -s -o /dev/null -X POST http://127.0.0.1:8080/api/v1/attack-validations \
     -H "Content-Type: application/json" -H "X-Pokemon-Debug-Fault: battle-502" \
     -d '{"attack_type":"collision","expected_damage":18}'; done
   ```
   → 2 分钟内「battle出现5xx错误」变红 + 告警邮件，5 分钟后自动恢复绿。
3. **杀死服务**：`taskkill //PID <battle的PID> //F`（PID 用 `netstat -ano | findstr :8080` 查）
   → 「battle服务健康检查」变红 + 邮件；重启服务后恢复绿 + 恢复邮件。
4. **慢请求**：打几个 `X-Pokemon-Debug-Fault: attack-timeout` 请求（响应 504、耗时 600ms+）
   → P95 监控器可能变红；APM → Traces 里能看到慢请求的火焰图（battle 等待 attack 超时）。
5. **trace 展示**：APM → battle → Traces，点开任意一条，展示 battle → attack 的调用链与耗时分布。

## 文件说明

- `http_check.d/conf.yaml` —— 健康检查（POST 真实业务请求，断言 200）
- `pokemon.d/conf.yaml` —— battle/attack 日志文件采集
- `install-datadog-config.ps1` / `.bat` —— 一键安装上述配置（需管理员）
- 本 README

## 待办（老师/管理员）

- Slack 集成需要组织管理员在 Integrations → Slack 安装，之后每个监控器通知栏加 `@slack-频道名` 即可切换告警渠道（当前用邮件）。

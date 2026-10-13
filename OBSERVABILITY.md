# 可观测性文档（OBSERVABILITY）

> 一句话版本：这是一个跑在自己电脑上的宝可梦对战小游戏。我们用 Datadog 把"玩家玩的时候到底发生了什么"看住——请求链路、日志、指标、前端体验四路信号，出了问题发 Slack 告诉我们。

## 架构：一次投掷请求的旅程

```
浏览器（前端）
    │  POST /api/v1/attack-validations
    ▼
battle（:8080）───HTTP───▶ attack（:8081）
    │                        │
    └── dd-trace-go ─────────┘
         （分布式 trace：battle→attack 一条链路）
```

- **前端**：浏览器里的对战游戏。玩家点击投掷道具，前端就把"这次攻击的类型和伤害"发给后端校验。
- **battle（8080）**：唯一入口。它只做一件事：把请求原样转给 attack，再把状态码原样带回来。
- **attack（8081）**：校验"攻击类型和伤害是否匹配"。200 = 正常，400 = 请求格式不对，422 = 伤害对不上（判负，这局结束）。

为什么拆成两个服务？为了演示分布式追踪：一次投掷要穿过两个服务，trace 能把它们串成一条完整的链路。

## 四路信号分别埋在哪

| 信号 | 埋在哪 | 说人话 |
| --- | --- | --- |
| Trace（链路） | `attack/main.go`、`battle/main.go` | 每个请求进来自动记一条链路，battle→attack 自动串起来；attack 会给 span 打上 `attack.type` 标签，方便按攻击类型筛选。 |
| Log（日志） | 两个服务里的 `logWithTrace()` | 结构化日志，每条带 `dd.trace_id`——在 Datadog 里从日志能跳到 trace，从 trace 也能跳回日志。出错的请求打 ERROR/WARN。 |
| Metric（指标） | `attack/main.go` | 每次校验发一条计数 `pokemon.attack.validation.count`（带 result:success/failure 标签），走 DogStatsD 进 Datadog。失败率 = `result:failure` 请求数（按 HTTP 400–599 以及无响应码 0）÷ 总请求数。CPU/内存/磁盘不用我们埋，Datadog Agent 自己采。 |
| RUM（前端） | `src/monitoring/rum.js` | 在浏览器里初始化 Datadog RUM，采集页面浏览、点击、JS 报错、接口请求——出问题能直接看到用户视角。开发面板的演练按钮会额外上报 `dev_*` 事件。 |

所有信号带统一标签：`env:school`、`version:1.0.0`、`team:bj-xsolla-school-group-2`。

## 监控：10 条，全部只发 Slack

> monitors/ 目录下 10 个 JSON 和线上一一对应，跑 `py sync_datadog_monitor.py` 就能从 git 复现。
> 只发 Slack 不打电话：课程项目，看 Slack 就够了，也免得半夜被叫醒。

| 监控 | 盯什么 | 什么时候报警 |
| --- | --- | --- |
| battle 服务健康检查 | 入口活没活（探测会走完整条链路） | 5 分钟里有 1 次探测失败，或 10 分钟完全没有探测数据 |
| battle 出现 5xx | 网关自身或转发出错 | 5 分钟内 5xx 日志 > 3 条 |
| attack 出现 5xx | 校验服务自身出错 | 5 分钟内 5xx 日志 > 3 条 |
| CPU 持续过高 | 机器算力饱和 | system.cpu.user > 95% 持续 15 分钟 |
| 内存可用不足 | 机器内存见底 | 可用率 < 10% 持续 15 分钟 |
| P95 延迟过高 | 请求变慢 | battle P95 > 500ms（trace 指标单位是秒，查询里写 0.5） |
| attack 校验失败率过高 | 业务在失败（422 扎堆） | 5 分钟内失败率 > 20% |
| 前端 RUM 错误 | 页面 JS 崩了 | 5 分钟内 error 事件 > 4 条 |
| 前端非预期 4xx | 前端把请求发坏了（400/405 等，已排除 404/422） | 5 分钟内 > 5 条（只发警告） |
| supervisor 进程故障级别 | 进程崩没崩、自动重启有没有放弃 | 级别 ≥1 发警告，≥2 正式报警 |

### 阈值为什么这么定

三个背景，所有阈值都是从这里推出来的：

1. **一台机器装所有东西**：前端、两个后端、supervisor、Datadog Agent 全挤在同一台平时还用来办公的 Windows 电脑上；
2. **流量很小**：玩家就是组员和评审，每分钟个位数请求，大厂那种"按百分比"的阈值在这里没有意义；
3. **游戏规则**：伤害不匹配返回 422、这局判负；battle 等 attack 最多 500ms；玩家操作问题只会产生 400/422，**5xx 跟玩家无关，只跟我们的代码有关**。

逐条说：

- **battle 健康检查**：Agent 每 60 秒真的发一次校验请求（会走完 battle→attack 全链路，任何一环挂都会失败）。5 分钟里只要失败 1 次就报警——入口挂了所有人都进不了游戏，宁可灵敏。10 分钟完全没探测数据也报警：万一是 Agent 或整台机器没了，也能发现。
- **5xx 两条（>3 条/5 分钟）**：正常情况 5 分钟内一条 5xx 都不该有，3 条足够说明出事；不设成 1 条是给 attack 重启那几十秒留余地，免得刷屏。分成 battle/attack 两条是为了定位：只有 attack 报 → 校验服务自己坏了；两个一起报 → 多半是 attack 挂了，battle 转发不动跟着报 503。
- **CPU >95% 持续 15 分钟**：这台机器平时还开浏览器、编译，短时飙高太正常了，按瞬时值报警会天天误报。但连续 15 分钟打满，说明校验请求已经在排队（P95 那条会一起红），这时候才值得人过来看。
- **内存 <10% 持续 15 分钟**：内存见底后 Windows 开始狂换页，前端、两个服务、Agent 一起变慢。同样用"持续 15 分钟"过滤掉开大型软件的瞬时波动。
- **P95 > 500ms**：500ms 就是 battle 等 attack 的超时上限，一到点这局直接判负。P95 破线意味着每 20 次投掷至少 1 次卡在超时边缘——游戏基本没法玩了。所以阈值直接对齐超时值，不是另拍的数字。
- **校验失败率 >20%**：正常玩家永远不该收到 422（前端伤害表和后端一致）。个位数失败多半是有人改包作弊；到 20% 只可能是我们自己出了系统性问题（比如前端发了错的版本），这时所有正在玩的玩家都会被误判输。健康检查发现不了它（服务不是 5xx），所以要单独盯。
- **前端 JS 报错 >4 条/5 分钟**：JS 一崩玩家直接白屏，是最直观的"坏了"。平时没几个人在线，4 条扎堆只能是新版本有问题。
- **前端非预期 4xx >5 条/5 分钟（只警告）**：422 是游戏机制内的正常结果、404 已排除，剩下的 400/405 只可能是前端自己把请求发坏了。这种时候健康检查还是绿的、5xx 也不响，是个盲区，所以补一条。critical 阈值故意放到一百万，等于"只提醒、不叫人"。
- **supervisor 故障级别**：supervisor 每 3 秒看一眼三个进程，崩了自动拉起；1 小时内崩第二次就放弃重启。=1（警告）：崩过一次但已经自己好了；≥2（报警）：反复崩、自动恢复没用了，游戏实际下线。

### 怎么看"玩家是不是真的玩起来了"

不用额外建监控，四层数据现查就有（RUM Explorer / Metrics Explorer），哪层断了一目了然：

| 层 | 意思 | 查询 |
| --- | --- | --- |
| L1 打开页面 | 来了多少人 | RUM：`@type:view service:pokemon-battle-frontend` |
| L2 真的玩了 | 打开后有交互的人 | RUM：`@type:view @view.action.count:>0` |
| L3 发起校验 | 投掷触发了后端请求 | RUM：`@type:resource @http.url:*attack-validations*`，或指标 `trace.http.request.hits{service:battle}` |
| L4 校验成功 | 投掷被判定有效 | 指标：`pokemon.attack.validation.count{result:success}` |

L2 低 → 可能白屏/JS 报错；L3 比 L2 少很多 → 玩家点了但请求没到后端；L4 比 L3 少很多 → 422 判负扎堆（就是校验失败率那条监控的另一种看法）。

## 文件结构：哪些文件和可观测性有关

```
backend/
  attack/main.go          # APM + 日志 + 业务指标
  battle/main.go          # APM + 日志
  datadog/
    http_check.d/conf.yaml    # battle/attack 健康检查
    pokemon.d/conf.yaml       # 日志采集
monitors/                   # 监控器定义（as-code，与线上一一对应）
  battle-health-monitor.json          # ↔ battle服务健康检查
  battle-5xx-monitor.json             # ↔ battle出现5xx错误
  attack-5xx-monitor.json             # ↔ attack出现5xx错误
  cpu-monitor.json                    # ↔ CPU持续过高
  memory-monitor.json                 # ↔ 内存可用不足
  battle-latency-p95-monitor.json     # ↔ P95延迟过高
  attack-validation-failure-rate-monitor.json  # ↔ attack校验失败率过高
  pokemoncard-supervisor-failure-level.json  # ↔ 进程故障级别监控
  rum-frontend-error-monitor.json     # ↔ 前端JS异常
  rum-frontend-unexpected-4xx-monitor.json  # ↔ 前端非预期4xx
```

> 注：Datadog 上同 team 标签下其余监控不是本项目的。

## Datadog Agent 是怎么配的

Agent 是装在这台电脑上的一个 Windows 服务（MSI 安装），负责四件事：采系统指标（CPU/内存/磁盘）、收 DogStatsD 指标、按配置探测 battle/attack、收集日志文件。配置就两处：

1. `C:\ProgramData\DataDog\datadog.yaml`：主配置。我们把主机名固定成 `zhang-pokemon-backend`（监控器里按 host 过滤靠它，换机器要记得改），并开了 `logs_enabled: true`。
2. `C:\ProgramData\DataDog\conf.d\`：把仓库里 `backend/datadog/` 的两份配置复制进去（目录名必须一致，Agent 按目录名认集成）：

| 复制什么 | 到哪 | 干什么 |
| --- | --- | --- |
| `backend/datadog/http_check.d/conf.yaml` | `conf.d\http_check.d\conf.yaml` | 每 60 秒发一次真实业务请求探活 |
| `backend/datadog/pokemon.d/conf.yaml` | `conf.d\pokemon.d\conf.yaml` | 收集 `C:/dd-logs/` 下三个服务的日志 |

日志从哪来：`run_supervisor.py` 启动时自动把前端、battle、attack 的输出写到 `C:\dd-logs\*.log`，Agent 只管读这些文件——所以日志能被采集的前提是 supervisor 在跑。

改完配置要重启才生效（管理员 PowerShell）：

```powershell
Restart-Service datadogagent                                     # 重启 Agent
& "C:\Program Files\Datadog\Datadog Agent\bin\agent.exe" status  # 看整体状态
& "C:\Program Files\Datadog\Datadog Agent\bin\agent.exe" check http_check  # 单跑一次探测验证
```

一个容易踩的坑：attack 和 supervisor 的指标走 UDP 发到本机 8125，**发出去就没了**（Agent 不在跑就丢、不会补），所以先起 Agent、再起后端。`.env`（参考 `.env.example`）里的相关变量：

| 变量 | 用途 |
| --- | --- |
| `DATADOG_AGENT_HOST` / `DD_DOGSTATSD_PORT` | supervisor 发指标到哪（默认 127.0.0.1:8125） |
| `DD_API_KEY` / `DD_APP_KEY` / `DD_SITE` | `sync_datadog_monitor.py` 同步监控器用 |
| `VITE_DD_APPLICATION_ID` / `VITE_DD_CLIENT_TOKEN` | 前端 RUM 初始化（只放 client token，绝不放 API/App key） |

## 应用监控器（as-code）

```bash
# .env 里配好 DD_API_KEY / DD_APP_KEY / DD_SITE
py sync_datadog_monitor.py --dry-run   # 先预览会改什么
py sync_datadog_monitor.py             # 实际应用
```

## 复现方式：从零到有告警

新队友按顺序照抄即可：

```powershell
# 1. 根目录 .env 按 .env.example 填好（变量用途见上一节表格）
# 2. 把 backend/datadog/ 下两份 conf.yaml 复制进 C:\ProgramData\DataDog\conf.d\
# 3. 按依赖顺序启动：Agent → supervisor → 浏览器
Restart-Service datadogagent           # 重启 Agent 加载新 conf
py run_supervisor.py                   # 拉起前端 + battle + attack，日志写 C:\dd-logs\
# 4. 浏览器打开 http://localhost:5173，开始战斗并投掷几次，让指标/日志/RUM 先有数据
# 5. 同步监控器（上一节两条命令）
```

然后验证四件事：`agent.exe status` 里 http_check / pokemon 两个检查在跑；Metrics Explorer 查得到 `pokemon.attack.validation.count`；Logs Explorer 有 `service:battle` 的日志且带 `dd.trace_id`；RUM Explorer 能看到 `pokemon-battle-frontend`。最后用前端开发面板的演练按钮制造几个 5xx/422，Slack 频道应该收到对应告警。

## 截图

### （a）RUM → trace → log

![RUM 会话关联 APM trace](./pictures/rum1.png)

从 RUM 里的一次投掷请求点进 APM trace，能看到请求穿过前端、battle、attack 的完整链路。

![按服务与状态码过滤日志](./pictures/rum2.png)

在日志页按 `service:battle @http.status_code:502` 过滤，一眼定位网关转发失败的日志。

### （b）业务指标

![业务指标 pokemon.attack.validation.count](./pictures/rum3.png)

业务指标 `pokemon.attack.validation.count` 按 result 分组，成功/失败一目了然；21:29 前后的尖峰是一波集中投掷，Watchdog 自动标了异常点。它就是「attack 校验失败率过高」监控器的数据源。查询：

```
sum:pokemon.attack.validation.count{env:school,service:attack,team:bj-xsolla-school-group-2} by {result}.as_count()
```

### （c）Slack 告警触发与恢复

![Slack 告警触发与恢复](./pictures/slack-alert-trigger-recovery.png)

- **触发**：battle 5 分钟内出现超过 3 条 5xx 日志，告警变红，通知到 Slack 频道
- **恢复**：错误数回落后，自动发送恢复通知

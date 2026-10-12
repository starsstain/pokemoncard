# 前端监控（Datadog RUM）

依据 `EX5_capstone_ZH.md`，前端监控以下四类资源：

| 监控资源 | RUM 信号 | 用途 |
| --- | --- | --- |
| 页面加载 | View / 页面视图 | 观察页面访问量与加载体验 |
| 用户操作 | Action / 用户交互 | 还原用户点击和操作路径 |
| JavaScript 异常 | Error / 前端错误 | 发现并定位运行时异常 |
| 前端到后端请求 | Resource / 网络资源 | 观察战斗校验请求、HTTP 错误与耗时，并与后端 trace 关联 |

SDK 在 `src/monitoring/rum.js` 初始化。复制 `.env.example` 为 `.env.local`，填写 Datadog RUM Application ID 与 **Client Token** 后重启 Vite。`site` 默认 `us5.datadoghq.com`；若组织使用其它站点请改成对应站点域名。浏览器只使用 RUM Client Token，不能使用 API key 或 Application key。

本地开发时打开右上角 `dev` 面板，可分别触发一个捕获后继续运行的 JavaScript 异常和一个失败的后端资源请求。Datadog 有效配置后，这两项会出现在 RUM 的 Errors 与 Resources 中；按钮交互也会作为 RUM Action 记录。

本项目后端地址是 `http://127.0.0.1:8080`。要让资源请求跨服务关联 trace，请在 Datadog RUM 应用配置该 origin，并在后端启用 Datadog APM 接收和继续传播 `tracecontext` / `datadog` 请求头。浏览器访问源也应在 Client Token 的允许来源中配置。

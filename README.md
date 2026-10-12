# Pokemon Fight

[poke.yijungao.com](https://poke.yijungao.com)

一个像素风的宝可梦对战演示项目。玩家点击画面投掷道具协助伊布对战妙蛙种子；前端会按采样频率将攻击伤害提交给后端校验，发现伤害被篡改时会结束本局。

## 技术与功能

- React 18 + Vite：对战画面、血条、投掷动画与开发调试面板。
- Go 服务：`battle`（`127.0.0.1:8080`）转发请求给 `attack`（`127.0.0.1:8081`）校验三种固定伤害。
- 可观测性：Datadog RUM、APM Trace、结构化日志、DogStatsD 指标和 Monitor-as-Code。

## 本地启动

需要 Node.js、npm 与 Go 1.26（或兼容版本）。分别打开三个终端：

```powershell
# 终端 1：前端（项目根目录）
npm install
npm run dev
```

```powershell
# 终端 2：攻击校验服务
cd backend
go run ./attack
```

```powershell
# 终端 3：网关服务
cd backend
go run ./battle
```

在 Windows 上运行 `py run_supervisor.py` 可同时监管三个服务。

浏览器打开 Vite 输出的本地地址（通常是 `http://localhost:5173`）。点击对话框开始战斗，再点击左键投掷道具。

`backend/.env.example` 中的 `APP_ENV=dev` 可开启后端故障演练接口；前端开发模式会显示对应调试面板。

## 验证

```powershell
npm test
cd backend
go test ./...
go vet ./...
```

## 可观测性

将 Datadog 配置写入根目录 `.env`（可参考 `.env.example`）即可启用 RUM；服务端 trace、日志、指标、监控定义及同步方式见 [OBSERVABILITY.md](./OBSERVABILITY.md)。后端 API 契约与故障演练说明见 [backend/README.md](./backend/README.md)。

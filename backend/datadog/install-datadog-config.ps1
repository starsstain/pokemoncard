# Datadog Agent 配置一键安装脚本（需要管理员权限运行）
# 作用：1) 打开日志采集开关  2) 安装健康检查和日志采集配置  3) 建日志文件夹  4) 重启 Agent
$ErrorActionPreference = "Stop"
$src = $PSScriptRoot
$confDir = "C:\ProgramData\Datadog"

Write-Host "== 1/4 打开日志采集开关 + 设置全局 team 标签 =="
$ddYaml = Join-Path $confDir "datadog.yaml"
$content = Get-Content $ddYaml -Raw
if ($content -match '(?m)^\s*#?\s*logs_enabled:') {
    $content = $content -replace '(?m)^\s*#?\s*logs_enabled:.*', 'logs_enabled: true'
} else {
    $content += "`r`nlogs_enabled: true`r`n"
}
# 全局 team 标签：让 CPU/内存/磁盘等所有指标都带上 team 标签，监控器可按 team 筛选
if ($content -match '(?m)^\s*#?\s*tags:') {
    $content = $content -replace '(?m)^\s*#?\s*tags:.*', "tags:`r`n  - team:bj-xsolla-school-group-2"
} else {
    $content += "`r`ntags:`r`n  - team:bj-xsolla-school-group-2`r`n"
}
Set-Content -Path $ddYaml -Value $content
Write-Host "   OK"

Write-Host "== 2/4 安装健康检查 + 日志采集配置 =="
New-Item -ItemType Directory -Force (Join-Path $confDir "conf.d\http_check.d") | Out-Null
Copy-Item (Join-Path $src "http_check.d\conf.yaml") (Join-Path $confDir "conf.d\http_check.d\conf.yaml") -Force
New-Item -ItemType Directory -Force (Join-Path $confDir "conf.d\pokemon.d") | Out-Null
Copy-Item (Join-Path $src "pokemon.d\conf.yaml") (Join-Path $confDir "conf.d\pokemon.d\conf.yaml") -Force
Write-Host "   OK"

Write-Host "== 3/4 创建日志文件夹 C:\dd-logs =="
New-Item -ItemType Directory -Force "C:\dd-logs" | Out-Null
Write-Host "   OK"

Write-Host "== 4/4 重启 Datadog Agent =="
Restart-Service datadogagent
Start-Sleep -Seconds 5
Write-Host "   OK"

Write-Host ""
Write-Host "全部完成！现在可以：1) 用 Git Bash 以 tee 方式启动两个服务  2) 回到 ZCode 继续验证"

@echo off
rem 右键此文件，选择“以管理员身份运行”即可完成 Datadog 配置安装
powershell -ExecutionPolicy Bypass -File "%~dp0install-datadog-config.ps1"
pause

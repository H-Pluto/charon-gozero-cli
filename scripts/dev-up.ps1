#Requires -Version 5.1
<#
.SYNOPSIS
    一键拉起 charon 本地开发栈（Windows PowerShell 路径）。

.DESCRIPTION
    依次完成：起 Postgres 容器 → 等它就绪 → 跑数据库迁移 → 前台启动 api 与 rpc。

    这是 Windows 上推荐的开发入口。Makefile 里的目标（make dev 等）依赖
    Unix shell 语义（$(MAKE)、行尾续行、for 循环），在 Windows 上必须用
    Git Bash 或 WSL 执行；纯 PowerShell 请用本脚本。

.NOTES
    前置工具：
      - Docker Desktop（docker compose 可用）
      - golang-migrate CLI（go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest）
      - goreman（go install github.com/mattn/goreman@latest），用于同时拉起 api 与 rpc
    数据库连接串取自环境变量 DB_DATA_SOURCE，未设置时用本地默认值。
#>

$ErrorActionPreference = "Stop"

# 切到仓库里 charon 的根目录（脚本在 scripts/ 下，上一级即根）。
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

# 迁移用的 DSN：优先读环境变量，否则用本地默认（与 Makefile 的 DB_DSN 一致）。
$Dsn = $env:DB_DATA_SOURCE
if (-not $Dsn) {
    $Dsn = "postgres://postgres:postgres@127.0.0.1:5432/charon?sslmode=disable"
}

$PgUser = if ($env:POSTGRES_USER) { $env:POSTGRES_USER } else { "postgres" }
$PgDb   = if ($env:POSTGRES_DB)   { $env:POSTGRES_DB }   else { "charon" }

# 1. 起 Postgres。
Write-Host ">>> 启动 Postgres 容器..."
docker compose up -d postgres

# 2. 等 Postgres 就绪。
#    只用端口探测不够——端口通了不代表能接受查询，所以用容器自带的 pg_isready。
Write-Host ">>> 等待 Postgres 就绪..."
$ready = $false
for ($i = 1; $i -le 30; $i++) {
    docker compose exec -T postgres pg_isready -U $PgUser -d $PgDb | Out-Null
    if ($LASTEXITCODE -eq 0) { $ready = $true; break }
    Start-Sleep -Seconds 2
}
if (-not $ready) {
    Write-Error "Postgres 在 60 秒内未就绪，请查看 'docker compose logs postgres'。"
    exit 1
}

# 3. 跑迁移。
#    migrate 的 -database 用本机 DSN（宿主机映射端口 5432），而不是容器内网名。
Write-Host ">>> 应用数据库迁移..."
migrate -path migrations -database $Dsn up

# 4. 前台启动 api + rpc。
#    用 goreman（读取 Procfile）同时拉起两个进程；Ctrl+C 一并停止。
#    注意：api 配置里 DB.DataSource 留空时接口会返回 503，但服务仍能起；
#    配了 DSN 却连不上则进程直接退出（这是设计行为，不是 bug）。
Write-Host ">>> 启动 api 与 rpc（Ctrl+C 停止）..."
goreman start

#!/usr/bin/env bash
# 一键拉起 charon 本地开发栈（Git Bash / WSL / Linux / macOS）。
#
# 依次完成：起 Postgres 容器 → 等它就绪 → 跑数据库迁移 → 前台启动 api 与 rpc。
#
# 说明：Makefile 里的目标依赖 Unix shell 语义，在 Windows 上必须用
# Git Bash 或 WSL 执行（纯 PowerShell 请用 scripts/dev-up.ps1）。
# 本脚本与 `make dev` 等价，单独提供是为了不依赖 PATH 里有 make。
#
# 前置工具：
#   - docker compose
#   - golang-migrate CLI（go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest）
#   - goreman（go install github.com/mattn/goreman@latest）

set -euo pipefail

# 切到仓库里 charon 的根目录（脚本在 scripts/ 下，上一级即根）。
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# 迁移用的 DSN：优先读环境变量，否则用本地默认（与 Makefile 的 DB_DSN 一致）。
DB_DSN="${DB_DATA_SOURCE:-postgres://postgres:postgres@127.0.0.1:5432/charon?sslmode=disable}"
PG_USER="${POSTGRES_USER:-postgres}"
PG_DB="${POSTGRES_DB:-charon}"

# 1. 起 Postgres。
echo ">>> 启动 Postgres 容器..."
docker compose up -d postgres

# 2. 等 Postgres 就绪。
#    只用端口探测不够——端口通了不代表能接受查询，所以用容器自带的 pg_isready。
echo ">>> 等待 Postgres 就绪..."
ready=0
for _ in $(seq 1 30); do
	if docker compose exec -T postgres pg_isready -U "$PG_USER" -d "$PG_DB" >/dev/null 2>&1; then
		ready=1
		break
	fi
	sleep 2
done
if [ "$ready" -ne 1 ]; then
	echo "!! Postgres 在 60 秒内未就绪，请查看 'docker compose logs postgres'。" >&2
	exit 1
fi

# 3. 跑迁移。
#    migrate 的 -database 用本机 DSN（宿主机映射端口 5432），而不是容器内网名。
echo ">>> 应用数据库迁移..."
migrate -path migrations -database "$DB_DSN" up

# 4. 前台启动 api + rpc。
#    用 goreman（读取 Procfile）同时拉起两个进程；Ctrl+C 一并停止。
#    注意：api 配置里 DB.DataSource 留空时接口会返回 503，但服务仍能起；
#    配了 DSN 却连不上则进程直接退出（这是设计行为，不是 bug）。
echo ">>> 启动 api 与 rpc（Ctrl+C 停止）..."
exec goreman start

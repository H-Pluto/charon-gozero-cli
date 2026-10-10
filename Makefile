# charon 的版本与镜像变量。
# 改动这里会同时影响本机构建与 CI，请谨慎。

# go-zero 运行时版本。
GO_ZERO_VERSION := v1.10.3

# goctl 的版本。
#
# 注意 goctl 已从 go-zero 主模块拆分为独立 module
# （github.com/zeromicro/go-zero/tools/goctl），版本号与运行时不一定一致。
# 当前最新的 goctl 是 v1.10.2，没有 v1.10.3。
GOCTL_VERSION := v1.10.2

# 本地 Postgres 镜像 tag。若 postgres:18-alpine 拉不到，改成 17-alpine。
POSTGRES_VERSION := 18-alpine

# 应用镜像名与 tag
IMAGE ?= charon
TAG   ?= dev

# 本地开发数据库 DSN。敏感值请放到 .env，不要提交。
DB_DSN ?= postgres://postgres:postgres@127.0.0.1:5432/charon?sslmode=disable

OUTPUT_DIR := output

# Windows 下可执行文件带 .exe 后缀
ifeq ($(OS),Windows_NT)
	EXE := .exe
else
	EXE :=
endif

.PHONY: help tools init deps gen build run run-rpc dev migrate migrate-down \
        docker docker-build tidy vet lint fmt test fmt-check check clean

help:
	@echo "charon 常用命令："
	@echo "  make init          首次准备：安装工具 + 下载依赖"
	@echo "  make dev           本地起 Postgres + 迁移 + 同时跑 api 和 rpc"
	@echo "  make run           只跑 api"
	@echo "  make migrate       应用数据库迁移"
	@echo "  make gen           由 api/desc/*.api 生成代码到 .gen/ 供比对"
	@echo "  make test          跑全部单测"
	@echo "  make check         fmt + vet + test 一把过"
	@echo "  make docker-build  构建镜像"

## tools: 安装代码生成与迁移工具
tools:
	@echo ">>> 安装 goctl $(GOCTL_VERSION)"
	go install github.com/zeromicro/go-zero/tools/goctl@$(GOCTL_VERSION)
	@echo ">>> 安装 protoc 插件"
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo ">>> 安装迁移工具"
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo ">>> protoc 本体需单独安装（本机已有则跳过）"
	@protoc --version || echo "!! 未找到 protoc，请先安装 protobuf 编译器"

## deps: 下载 Go 依赖
deps:
	go mod download

## init: 首次准备
init: tools deps
	@test -f .env || (cp .env.example .env && echo ">>> 已生成 .env，请按需修改")

## gen: 由 .api 契约生成 HTTP 代码
##
## 重要：internal/handler、internal/logic、internal/types 当前是手写维护的，
## 保证不依赖 goctl 也能构建。因此本目标**不直接覆盖**这些文件，
## 而是生成到临时目录供人工比对。
##
## 原因：本机 goctl 版本可能与 GOCTL_VERSION 不一致，生成的样板会有出入，
## 直接覆盖会把手工改动冲掉。等验证 goctl 输出稳定后可改为直接生成。
gen:
	@echo ">>> 校验 goctl（期望 $(GOCTL_VERSION)）"
	@goctl --version || (echo "!! 未安装 goctl，先跑 make tools" && exit 1)
	@rm -rf .gen && mkdir -p .gen
	goctl api go -api api/desc/charon.api -dir .gen --style goZero
	@echo ">>> 生成结果在 .gen/，请与 internal/ 下的手写实现比对后手动合并。"
	@echo ">>> 完成后可删除 .gen/"
	@echo ">>> 新增 @handler 后必须手工补 internal/handler/<group>/ 与"
	@echo ">>> internal/logic/<group>/，并在 internal/handler/routes.go 注册路由。"

## build: 编译两个服务到 output/
build:
	@mkdir -p $(OUTPUT_DIR)
	go build -o $(OUTPUT_DIR)/charon-api$(EXE) ./cmd/api
	go build -o $(OUTPUT_DIR)/charon-rpc$(EXE) ./cmd/rpc
	@echo ">>> 产物在 $(OUTPUT_DIR)/"

## run: 只启动 api
run:
	go run ./cmd/api -f etc/charon-api.yaml

## run-rpc: 只启动 rpc
run-rpc:
	go run ./cmd/rpc -f etc/charon-rpc.yaml

## dev: 起数据库 + 迁移 + 用 goreman 同时跑 api 和 rpc
dev:
	docker compose up -d postgres
	@echo ">>> 等待 Postgres 就绪..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do docker compose exec -T postgres pg_isready -U postgres >/dev/null 2>&1 && break; sleep 2; done
	$(MAKE) migrate
	goreman start

## migrate: 应用迁移
migrate:
	migrate -path migrations -database "$(DB_DSN)" up

## migrate-down: 回滚一步
migrate-down:
	migrate -path migrations -database "$(DB_DSN)" down 1

## tidy: 整理依赖
tidy:
	go mod tidy

## vet: 静态检查
vet:
	go vet ./...

## lint: 更全面的静态检查（需要本机已装 golangci-lint）
##
## 刻意不放进 check：golangci-lint 不是必需工具，CI 环境可能没有。
## 配置见 .golangci.yml（里面剔除了 go-zero tag 引起的 SA5008 误报）。
lint:
	golangci-lint run --timeout 5m

## fmt: 格式化
fmt:
	gofmt -w .

## fmt-check: 检查格式（CI 用）
##
## 刻意写成单行：Windows 上 make 对「反斜杠续行 + 中文」的处理不稳定
## （会把续行当成参数传给 echo），单行可以绕开。
fmt-check:
	@out="$$(gofmt -l .)"; test -z "$$out" || { echo "以下文件未格式化:"; echo "$$out"; exit 1; }

## test: 跑单测
test:
	go test ./... -count=1

## check: 提交前的完整检查
check: fmt-check vet test

## docker-build: 构建镜像
docker-build:
	docker build -t $(IMAGE):$(TAG) .

## docker: 构建并推送
docker: docker-build
	docker push $(IMAGE):$(TAG)

## clean: 清理产物
clean:
	rm -rf $(OUTPUT_DIR) .gen

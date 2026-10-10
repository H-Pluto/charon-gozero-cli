# charon

`charon` 是 `primary-star` 里承载业务后端的 Go 微服务，基于 **go-zero v1.10.3**。

**当前状态：脚手架，还没有真实业务。** 里面的接口都是「自证」用的——用来证明配置、路由、日志、追踪、鉴权、数据库这条链路是通的。业务开工时按下面「新增一个接口」的步骤往里面加。

> 这份文档的目标是让第一次接触 go-zero 的人也能看懂。所以会先解释「这是干什么的、在哪被调用」，再讲为什么这么设计。

---

## 目录

- [1. 快速开始](#1-快速开始)
- [2. 一个接口长什么样（完整示例）](#2-一个接口长什么样完整示例)
- [3. 一个请求是怎么走完全程的](#3-一个请求是怎么走完全程的)
- [4. 目录地图](#4-目录地图)
- [5. pkg/ 里每个包是干什么的](#5-pkg-里每个包是干什么的)
- [6. 环境配置](#6-环境配置)
- [7. 数据库](#7-数据库)
- [8. 新增一个接口](#8-新增一个接口)
- [9. 架构约束](#9-架构约束)
- [10. 与参考文档的偏离](#10-与参考文档的偏离)
- [11. 常见问题](#11-常见问题)

---

## 1. 快速开始

### 前置：装工具

```bash
make tools     # 装 goctl / protoc 插件 / migrate
```

需要本机已有 `protoc`（本项目用 protoc 36.2 验证过）。Windows 上 **Makefile 里的命令需要在 Git Bash 或 WSL 里跑**，PowerShell 不认 `$$` 这类写法。

### 首次准备

```bash
cp .env.example .env    # 或 make init 会自动做
make deps               # 下载 Go 依赖
```

### 起服务

推荐（自动起 Postgres + 迁移 + 同时跑 api 和 rpc）：

```bash
make dev
```

Windows 也可以：

```bash
powershell -File scripts\dev-up.ps1
```

只想跑 api，且已经有可用数据库：

```bash
DB_DATA_SOURCE="postgres://postgres:postgres@127.0.0.1:5432/charon?sslmode=disable" \
  make run
```

不想装数据库也可以跑——把 `etc/charon-api.yaml` 里的 `DB.DataSource` 留空即可，数据库相关接口会返回 503，其余功能正常。这一点在 [第 7 节](#7-数据库) 有解释。

### 验证它真的起来了

```bash
curl -i -H "X-Trace-Id: demo-trace" "http://127.0.0.1:8888/api/v1/hello?name=charon"
curl "http://127.0.0.1:8888/health"
```

---

## 2. 一个接口长什么样（完整示例）

`GET /api/v1/hello?name=charon` 是最简单的完整例子。下面是**实际运行**的输出（不是手写的示意）：

```bash
$ curl -i -H "X-Trace-Id: demo-trace" "http://127.0.0.1:8888/api/v1/hello?name=charon"

HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
X-Trace-Id: demo-trace
Content-Length: 118

{"code":0,"message":"ok","data":{"message":"hello, charon","time":"2026-10-10T15:25:16+08:00"},"traceId":"demo-trace"}
```

响应外壳 `{code, message, data, traceId}` 是**所有接口统一的**，由 `pkg/response` 自动套上——业务代码只负责填 `data`。

### 出错了会是什么样

```bash
# 路由不存在
$ curl -s "http://127.0.0.1:8888/api/v1/nope"
{"code":1004,"message":"资源不存在","traceId":"da233e97a6dcd6aa0de068e32746ca7d"}

# 没配置数据库时访问数据库接口
$ curl -s "http://127.0.0.1:8888/api/v1/demo"
{"code":501,"message":"服务不可用","traceId":"833129a3ae2be83baa303a39a60bfed3"}

# 开启鉴权后不带 token
$ curl -s -i "http://127.0.0.1:8888/api/v1/hello"
HTTP/1.1 401 Unauthorized
{"code":1002,"message":"未认证或凭证已失效","traceId":"f48d7d881edf8f24b1e8e5ba7eb724c1"}
```

注意每种错误都带了 `traceId`，**报错时把它给我，我就能在日志里定位到那一次请求**。错误码表见 [第 5 节](#5-pkg-里每个包是干什么的)。

### 这个接口的代码在哪

一个接口的代码分成 4 个地方，各管一件事：

| 文件 | 角色 | 负责什么 |
|------|------|----------|
| [api/desc/hello.api](api/desc/hello.api) | 契约 | 声明 URL、方法、请求参数、返回结构。**这是唯一真源**，改接口先改它 |
| [internal/types/hello.go](internal/types/hello.go) | 数据结构 | 上面 `.api` 的 Go 表达，请求/响应的结构体 |
| [internal/handler/hello/hello.go](internal/handler/hello/hello.go) | HTTP 层 | 解析请求 → 调 logic → 写响应。**不写业务判断** |
| [internal/logic/hello/hello.go](internal/logic/hello/hello.go) | 业务层 | 真正的业务规则。**不碰 HTTP，也不拼 SQL** |

外加 [internal/handler/routes.go](internal/handler/routes.go) 负责把 URL 绑定到 handler。

**为什么要分成 handler 和 logic 两层？** 因为「解析 HTTP」和「业务规则」是两种变化原因不同的东西。业务规则将来可能被定时任务、消息队列、gRPC 复用，那时它不该被 HTTP 的细节绑住；反过来改 HTTP 传输方式也不该动业务逻辑。所以 handler 很薄，logic 很纯。

handler 的写法固定就这么几步：

```go
func HelloHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 解析请求参数
		var req types.HelloReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		// 2. 调 logic
		l := hello.NewHelloLogic(r.Context(), svcCtx)
		resp, err := l.Hello(&req)

		// 3. 写响应
		if err != nil {
			response.Fail(r.Context(), w, err)
			return
		}
		response.Ok(r.Context(), w, resp)
	}
}
```

logic 里返回错误有一个**必须遵守的规矩**：用 `pkg/errors` 构造，不要直接 `return err`。

```go
// 对：客户端拿到 400 + 错误码 1001
return nil, bizerr.NewFromContext(l.ctx, bizerr.CodeInvalidParam,
	bizerr.Newf(bizerr.CodeInvalidParam, "name 长度 %d 超过上限 64", n))

// 错：客户端拿到 500，且可能把数据库内部信息暴露出去
return nil, err
```

因为裸 `error` 会被统一归为「内部错误 500」，而数据库报错的原文（表名、SQL 片段、连接信息）**不会**泄漏给客户端——它只进日志。这是有意设计的安全边界。

`demo` 系列接口（`POST/GET/DELETE /api/v1/demo`）是更完整的例子，包含读写数据库、分页、以及「查不到要翻译成 404」的写法，见 [internal/logic/demo/demo.go](internal/logic/demo/demo.go)。

---

## 3. 一个请求是怎么走完全程的

以 `GET /api/v1/hello?name=charon` 为例，从进入进程到返回的完整路径：

```
curl
 │
 ├─ 1. 中间件链（go-zero 挂的 + 我们挂的）
 │     trace.Middleware        ← 取/生成 X-Trace-Id，放进 context、写回响应头
 │     auth.Middleware         ← 校验 Authorization: Bearer 非空（仅业务路由）
 │     go-zero 自带：日志、超时、限流、熔断、recover ...
 │
 ├─ 2. internal/handler/hello.HelloHandler
 │     httpx.Parse(r, &req)    ← 把 query string 解析成 types.HelloReq
 │
 ├─ 3. internal/logic/hello.HelloLogic.Hello
 │     业务规则：name 为空则兜底为 "world"；超过 64 字符返回 1001 错误
 │
 ├─ 4. internal/handler/hello  ← response.Ok(ctx, w, resp)
 │
 └─ 5. pkg/response 的全局 handler
       套上 {code, message, data, traceId} 外壳，序列化成 JSON
```

第 5 步值得单独说明：**业务代码里没有一处写着 `code` 或 `message`**，但响应里一定有。这是因为 `pkg/response.Install()` 在启动时往 go-zero 注册了一个全局的「响应加工函数」，所有走 `response.Ok` / `httpx.OkJsonCtx` 的响应都会被它套上外壳。这样做的价值是：改一次外壳格式，所有接口一起生效，不用改 N 个地方。

涉及数据库的接口在第 3 步里还会往下走一层：

```
logic  →  internal/repo（拼 SQL、读写）  →  internal/model（数据结构）  →  PostgreSQL
```

`repo` 只负责「取/存」，不知道什么算 404；`logic` 负责把 `sql.ErrNoRows` 翻译成 `CodeNotFound`。这样分工的好处是将来对外错误码要改（比如查不到改成返回空对象而不是 404），只动 logic，不用碰 SQL。

---

## 4. 目录地图

```
charon/
│
├── api/                        【接口契约】
│   ├── desc/                   .api 和 .proto 文件 —— 接口的唯一真源
│   │   ├── charon.api          入口，只做 import 汇总
│   │   ├── common.api          跨模块复用的类型（如 PageQuery）
│   │   ├── hello.api           自证接口
│   │   ├── demo.api            自证接口（读写数据库的完整例子）
│   │   └── hello.proto         gRPC 自证接口
│   ├── pb/                     protoc 由 .proto 生成，勿手改
│   └── sdk-interfaces/         Phase 2 的 SDK 占位
│
├── cmd/                        【程序入口】
│   ├── api/main.go             HTTP 服务入口。★运行时真源★
│   └── rpc/main.go             gRPC 服务入口。★运行时真源★
│
├── internal/                   【业务代码】Go 规定 internal/ 不能被外部模块 import
│   ├── config/config.go        配置结构体，与 etc/*.yaml 一一对应
│   ├── svc/servicecontext.go   依赖装配：启动时建好配置、数据库连接池、各 repo
│   ├── handler/                HTTP 层：解析请求 → 调 logic → 写响应
│   │   ├── routes.go           路由表 + 中间件挂载范围。★改路由看这里★
│   │   ├── hello/hello.go
│   │   └── demo/demo.go
│   ├── logic/                  业务层：业务规则都在这
│   │   ├── hello/hello.go
│   │   └── demo/demo.go
│   ├── types/                  请求/响应结构体（.api 的 Go 表达）
│   ├── model/                  数据库表对应的结构体
│   ├── repo/                   数据访问：拼 SQL、读写
│   └── svcerpc/hello.go        gRPC 服务实现（避开与 svc/ 混淆）
│
├── pkg/                        【可复用基础库】不含业务，可被其他服务抄走
│   ├── errors/                 业务错误码 + 多语言文案
│   ├── response/               统一响应外壳 {code,message,data,traceId}
│   ├── trace/                  X-Trace-Id 的读取、生成、透传
│   ├── logger/                 日志初始化 + trace id 注入
│   ├── auth/                   鉴权中间件（none / require / sdk）
│   ├── health/                 /health 与 /readyz 探针
│   └── grpcx/                  gRPC 拦截器（trace 透传、调用日志）
│
├── etc/                        【配置文件】
│   ├── charon-api.yaml         api 的开发配置
│   └── charon-rpc.yaml         rpc 的开发配置
│
├── config/                     预留：按环境拆分的配置（当前未用，见其 README）
├── migrations/                 【数据库迁移】golang-migrate 格式
│   ├── 000001_init.up.sql      建表
│   └── 000001_init.down.sql    回滚
├── templates/                  goctl 自定义代码模板 + .in 约定说明
├── deploy/
│   ├── k8s/                    Kubernetes 部署清单（模板）
│   └── secret.example.yaml     密钥示例
├── scripts/
│   ├── dev-up.ps1 / dev-up.sh  一键起本地开发环境
│   └── cloud/                  云库跳板机隧道说明
│
├── Makefile                    常用命令，先跑 make help
├── Procfile                    goreman 用：同时起 api 和 rpc
├── Dockerfile                  多阶段构建
├── docker-compose.yml          本地开发：postgres + api + rpc
├── docker-compose.full.yml     叠加 primary-star 其他服务
├── go.mod                      module charon / go 1.26.5 / go-zero v1.10.3
└── .env.example                环境变量示例，复制成 .env 用
```

**几个容易混淆的点：**

- `internal/svc`（HTTP 侧的依赖装配）和 `internal/svcerpc`（gRPC 服务实现）是两个完全不同的东西，名字像但职责无关。
- `internal/model`（数据库结构）和 `internal/types`（HTTP 结构）**故意分开**。合成一个的话，数据库加个字段就会变成 API 的破坏性变更。
- `pkg/` 和 `internal/` 的区别：`pkg/` 里不该有任何业务概念，理论上可以直接复制到别的服务；`internal/` 是本服务专属。

---

## 5. pkg/ 里每个包是干什么的

这 7 个包是「基础设施」，每个都解决一个具体问题。下面按「什么时候会用到它」来说明。

### `pkg/errors` — 业务错误码

**问题**：接口出错时要告诉客户端「错在哪」，但又不能把数据库报错原文吐出去。

**做法**：预定义一批错误码（`CHARON-1001` 这种形式），每个码绑定一个 HTTP 状态码和一组按语言分的文案。

```go
// 最常用：按请求语言构造错误
bizerr.NewFromContext(ctx, bizerr.CodeNotFound, nil)

// 带内部原因（只进日志，不给客户端看）
bizerr.Newf(bizerr.CodeInvalidParam, "name 长度 %d 超过上限", n)

// 把 repo 返回的 sql.ErrNoRows 翻译成 404
if errors.Is(err, sql.ErrNoRows) {
    return nil, bizerr.NewFromContext(ctx, bizerr.CodeNotFound, err)
}
```

**错误码表**（新增码前先读 [error-code-spec.md](pkg/errors/error-code-spec.md)）：

| 码 | 常量 | HTTP | 含义 |
|----|------|------|------|
| 1001 | `CodeInvalidParam` | 400 | 参数不合法 |
| 1002 | `CodeUnauthorized` | 401 | 未认证 |
| 1003 | `CodeForbidden` | 403 | 无权限 |
| 1004 | `CodeNotFound` | 404 | 资源不存在 |
| 1005 | `CodeConflict` | 409 | 资源冲突 |
| 1006 | `CodeTooManyRequests` | 429 | 请求过频 |
| 1007 | `CodeMethodNotAllowed` | 405 | 方法不允许 |
| 1999 | `CodeInternal` | 500 | 内部错误 |
| 500 | `CodeCommonInternal` | 500 | 通用内部错误 |
| 501 | `CodeCommonUnavailable` | 503 | 服务不可用 |
| 502 | `CodeCommonTimeout` | 504 | 超时 |

多语言：`Accept-Language` 头决定文案，在 handler 里把请求头塞进 context

```go
ctx = bizerr.WithLang(ctx, r.Header.Get("Accept-Language"))
```

**关键行为**：`From(err)` 会把任何「非业务错误」统一变成 500，原始内容只留在 `Cause`（进日志）。所以**永远不会**出现数据库报错原文返回给客户端的情况。

### `pkg/response` — 统一响应外壳

**问题**：每个接口都手写 `{code, message, data}` 太啰嗦，而且格式容易不一致。

**做法**：往 go-zero 注册一个全局加工函数，响应自动套壳。

```go
response.Install()                     // 启动时调一次（在 cmd/api/main.go）
response.Ok(ctx, w, data)              // 成功
response.OkPage(ctx, w, list, total, page, size)  // 分页
response.Fail(ctx, w, err)             // 失败
```

**注意**：`Install()` 是进程级全局状态，只能调一次，且要在开始处理请求前。它还会顺带处理一件事——只有 5xx 才打 error 日志，4xx 不打（否则客户端传错参数就能把日志刷满，真正的问题反而看不见）。

### `pkg/trace` — 调用链追踪

**问题**：用户说「我刚才那个请求报错了」，怎么在几万行日志里找到那一次？

**做法**：每个请求有一个 `X-Trace-Id`，贯穿日志和响应。客户端可以自带这个头，我们就沿用它（这样跨服务的调用链能串起来）；不自带就生成一个随机值。

```bash
curl -H "X-Trace-Id: my-debug-123" ...   # 自带的话日志里就是 my-debug-123
curl ...                                  # 不自带的话响应头会回写一个生成的
```

**安全处理**：这个头是客户端可控的，直接写进日志会有「日志注入」风险（塞换行符伪造日志行），所以做了过滤——只保留字母数字和 `-_.`，且截断到 64 字符。

**和 go-zero 自带 trace 的关系**：go-zero 的中间件走的是 W3C `traceparent`（给 otel 采样上报用），不认 `X-Trace-Id`。两套并存、互不干扰——**我们的业务日志串联不依赖 otel 是否开启**，这是刻意的，本地开发关掉 otel 也能正常追日志。

### `pkg/logger` — 日志

基于 go-zero 的 `logx`，额外做一件事：把 trace id 注入日志字段，让每条日志都带 traceId。

```go
// 请求入口处包一层，之后 logx.WithContext(ctx).Info(...) 就会带上 traceId
ctx = logger.WithTrace(ctx)
```

必须在入口显式调用，因为 go-zero 的 `WithContext` 只从 otel span 里取 trace id，拿不到我们的。

### `pkg/auth` — 鉴权中间件

**现状是「门闩」而不是真鉴权**，这点必须清楚：

| 模式 | 行为 |
|------|------|
| `none` | 全部放行。仅本地开发 |
| `require` | 只校验 `Authorization: Bearer <非空>`，**不验签、不解析** |
| `sdk` | 预留给 sdk-auth，当前放行并打警告 |

`require` **挡不住伪造的 token**，它只是拦住没有凭证的探测流量，并为将来换成真鉴权留好位置。因此：

> **`Auth.Mode=require` 期间，该服务禁止暴露到公网 Ingress**，只能经集群内网访问。

### `pkg/health` — 探针

| 路径 | 类型 | 行为 |
|------|------|------|
| `/health` | 存活（liveness） | 进程还能处理请求就返回 200。**不检查任何依赖** |
| `/readyz` | 就绪（readiness） | 检查数据库连通性，不通返回 503 |

**为什么不合并成一个？** 因为存活探针失败会让 kubelet **重启容器**。如果存活探针也去检查数据库，那数据库抖动就会导致所有实例被重启——而重启解决不了数据库的问题，只会让雪崩更严重。所以：依赖检查放就绪探针，重启决策才不会被依赖故障污染。

两个探针都**不走鉴权中间件**（见下一节），否则鉴权配置问题会让编排系统误判实例不健康。

### `pkg/grpcx` — gRPC 拦截器

go-zero 的 zrpc 已经内置了 trace/recover/限流等拦截器。这里补的是它不做的两件事：

1. **trace 透传**：把 `X-Trace-Id` 放进 gRPC metadata，这样 HTTP 入口的 trace id 能一路带到 gRPC 下游服务，日志能串起来。
2. **调用日志**：记录每个 gRPC 方法的方法名、状态码、耗时（不记录请求体——可能含敏感信息且日志量会失控）。

---

## 6. 环境配置

### 配置文件

| 文件 | 用途 |
|------|------|
| [etc/charon-api.yaml](etc/charon-api.yaml) | HTTP 服务的配置 |
| [etc/charon-rpc.yaml](etc/charon-rpc.yaml) | gRPC 服务的配置 |
| [.env.example](.env.example) | 环境变量示例，复制成 `.env` |

配置里可以写 `$VAR` 引用环境变量（由 `conf.UseEnv()` 展开），这样密码不用写进仓库：

```yaml
DB:
  DataSource: $DB_DATA_SOURCE
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DB_DATA_SOURCE` | 空 | PostgreSQL 连接串。**留空 = 该实例不用数据库** |
| `POSTGRES_USER` | `postgres` | 本地 Postgres 容器用 |
| `POSTGRES_PASSWORD` | `postgres` | 同上 |
| `POSTGRES_DB` | `charon` | 同上 |
| `POSTGRES_PORT` | `5432` | 同上 |

### 常用配置项

| 配置 | 位置 | 说明 |
|------|------|------|
| `Port` | `charon-api.yaml` | HTTP 端口，**8888**（8080 被 q-bot 占了，别改回去） |
| `Mode` | `charon-api.yaml` | `dev` 本地 / `pro` 生产。dev 会关掉自适应限流并输出更详细的日志 |
| `Auth.Mode` | `charon-api.yaml` | `none` / `require` / `sdk`，见 [第 5 节](#pkg-auth--鉴权中间件) |
| `Health.RequireToken` | `charon-api.yaml` | 打开后探针必须带 `X-Health-Token` 头 |
| `DB.MaxOpenConns` | `charon-api.yaml` | 连接池上限。**别留 0**（0 = 无上限，流量突增会打满数据库） |
| `Telemetry.Disabled` | `charon-api.yaml` | 本地没有 OTLP 后端时设为 `true`，否则启动会刷连接失败日志 |

### 日志

配置在 `etc/charon-api.yaml` 的 `Log` 段：

```yaml
Log:
  Mode: console      # console 输出终端 / file 输出文件 / volume（k8s）
  Encoding: plain    # plain 可读文本 / json 便于日志系统采集
  Level: debug       # debug / info / error / severe
  Stat: false        # 是否打印每秒请求数等统计
```

生产环境建议 `Encoding: json` + `Mode: volume`，便于采集和按 Pod 分文件。

---

## 7. 数据库

引擎是 **PostgreSQL**，驱动用 **pgx**（`jackc/pgx/v5/stdlib`），操作通过 go-zero 的 `sqlx`。

### 一条重要的设计决定

**`DB.DataSource` 一旦配置，启动时就会真的去连；连不上则启动失败，不会回退到内存实现。**

```bash
$ DB_DATA_SOURCE="postgres://...@127.0.0.1:59999/nope" ./charon-api -f etc/charon-api.yaml
{"level":"fatal","content":"failed to connect to `user=postgres database=nope`... dial error"}
$ echo $?
1
```

**为什么不做兜底？** 因为「连不上就用内存」会让一个配置错误在生产上表现成「服务起来了、探针正常、但数据全是空的」——这种故障比启动失败难排查得多。启动就失败能让编排系统立刻停下这次部署。

这条规则只有一个例外，是有意留的口子：**`DataSource` 留空**表示这个实例确实不用数据库。这时：

- 启动正常，并打一条明确日志 `未配置 DB.DataSource：数据库相关接口将不可用`
- 数据库相关接口返回 503（不是 panic）
- `/readyz` 仍然通过

### 迁移

用 [golang-migrate](https://github.com/golang-migrate/migrate)，文件在 `migrations/`，命名 `<版本>_<名字>.up.sql` / `.down.sql`。

```bash
make migrate          # 应用全部迁移
make migrate-down     # 回滚一步
```

**规矩：已经应用过的迁移文件不要修改**，否则各环境的 schema 会不一致。要改就新增一个迁移。

### 分层

```
logic  →  repo  →  model  →  PostgreSQL
```

| 层 | 只做 | 不做 |
|----|------|------|
| `model` | 描述表结构（`db` tag 绑定列名） | 发起查询、业务规则 |
| `repo` | 拼 SQL、读写 | 业务判断（如「名字是否重复」）、翻译错误码 |
| `logic` | 业务规则、把 `sql.ErrNoRows` 翻译成 `CodeNotFound` | 拼 SQL、碰 HTTP |

`repo` 返回原始错误而不自作主张包装成业务错误码，是为了让数据层不知道对外契约——将来改对外错误码只需要动 logic。

### 连接池

在 [internal/svc/servicecontext.go](internal/svc/servicecontext.go) 里设置：

```go
rawDB.SetMaxOpenConns(c.DB.MaxOpenConns)   // 必须设，默认 0 = 无上限
rawDB.SetMaxIdleConns(c.DB.MaxIdleConns)
rawDB.SetConnMaxLifetime(c.DB.MaxLifetime) // 定期重建连接，避免用到已被中间件掐断的死连接
```

启动时会主动 `Ping` 一次，在启动期就暴露连通性问题——而不是等第一个请求进来才发现。

### 自证用的表

`demo_item` 表纯粹用来证明「配置 → svc → repo → model → 迁移」这条链路是通的。**业务开工后请删除它**，删除清单见 [api/desc/demo.api](api/desc/demo.api) 顶部的注释。

---

## 8. 新增一个接口

以加一个 `GET /api/v1/products` 为例，一共 5 步：

**1. 写契约** —— 新建 `api/desc/product.api`，并在 [api/desc/charon.api](api/desc/charon.api) 里 import 它：

```go
type ProductListReq {
    PageQuery                      // 复用分页参数
}

@server (
    prefix: /api/v1
    group:  product                // 决定代码放在 internal/handler/product
)
service charon-api {
    @handler ProductList
    get /products (ProductListReq) returns (ProductListResp)
}
```

**2. 写结构体** —— 在 `internal/types/product.go` 里定义 `ProductListReq` / `ProductListResp`，与上面对应。

**3. 写业务** —— 在 `internal/logic/product/list.go` 里实现。日志用 `l.Infof`，错误用 `bizerr`：

```go
func (l *ListLogic) List(req *types.ProductListReq) (*types.PageResult, error) {
    items, total, err := l.svcCtx.ProductRepo.List(l.ctx, req.Page, req.PageSize)
    if err != nil {
        return nil, bizerr.NewFromContext(l.ctx, bizerr.CodeInternal, err)
    }
    return &types.PageResult{List: items, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}
```

**4. 写 handler** —— 在 `internal/handler/product/list.go` 里，照抄 [hello handler](internal/handler/hello/hello.go) 的三步结构。

**5. 注册路由** —— 在 [internal/handler/routes.go](internal/handler/routes.go) 的 `bizRoutes` 里加一行：

```go
{Method: http.MethodGet, Path: apiPrefix + "/products", Handler: product.ListHandler(svcCtx)},
```

如果要用新的 repo，在 [internal/svc/servicecontext.go](internal/svc/servicecontext.go) 里挂上：

```go
ctx.ProductRepo = repo.NewProductRepo(ctx.DB)
```

**关于 `make gen`**：它会把 goctl 生成的结果输出到 `.gen/` 供比对，**不会直接覆盖上面这些手写文件**。原因是本机 goctl 版本可能与 `GOCTL_VERSION`（v1.10.2）不一致，生成的样板会有出入，直接覆盖会冲掉手工改动。等验证 goctl 输出稳定后可以改为直接生成。详见 [templates/README.md](templates/README.md)。

---

## 9. 架构约束

> **现行口径：业务能力走 HTTP API；服务间先不实现业务 RPC。**

| 做 | 不做 |
|----|------|
| 业务加 `api/desc/*.api` + HTTP | 不为业务建 `.proto` / 往 hello rpc 加领域方法 |
| 服务间调用走 HTTP（超时、Trace、统一错误） | 把业务契约做成服务间 gRPC |
| 保留 `cmd/rpc` + hello 仅作脚手架自证 | 以 hello 为模板扩展领域 RPC |

| 边界 | 现行行为 | 何时关闭 |
|------|----------|----------|
| `Auth.Mode=require` | 非空 Bearer 门闩（**非 JWT**）；**禁止公网 Ingress** | `sdk-auth` 合入后切 `Mode=sdk` |
| `cmd/rpc` hello | 明文 gRPC，靠 NetworkPolicy 隔离 | 若上业务 RPC：补 mTLS + 鉴权 |
| `/health` 探针 | 不经过鉴权中间件 | 若探针暴露到不可信网络：打开 `Health.RequireToken` |

`api/desc/hello.proto` 存在**只为自证** gRPC 链路（含 trace 透传）是通的。更新它用 `protoc`：

```bash
protoc --proto_path=api/desc \
  --go_out=. --go_opt=module=charon \
  --go-grpc_out=. --go-grpc_opt=module=charon \
  api/desc/hello.proto
```

**不要**用 `goctl rpc --zrpc_out` 覆盖 `cmd/rpc` —— 它生成的样板与本项目的中间件约定不一致。

---

## 10. 与参考文档的偏离

本脚手架参考了内部《业务微服务框架搭建实施方案》，但有几处**有意偏离**，落地时请以本文件为准：

| 项 | 参考文档 | 本项目 | 原因 |
|----|----------|--------|------|
| 模块名 | `pms` | `charon` | 归属 primary-star |
| 错误码域 | `PMS` / `COM` | `CHARON` / `COM` | 同上，`COM` 通用域沿用 |
| go-zero 版本 | v1.7.3 | **v1.10.3** | 更新且本地已缓存 |
| goctl | 随 go-zero | **独立 module v1.10.2** | goctl 已拆分；没有 v1.10.3 |
| Memory 演示仓 | 有 | **没有** | 生产上「配置错→内存兜底→数据全空」比启动失败难排查得多 |
| 代码生成 | goctl 直出 | **手写 + `.gen/` 比对** | 本机 goctl 版本落后，直出会冲掉手工改动 |
| `.in` 逻辑保留 | 已启用 | **约定已写但未启用** | 未验证 goctl 的确切语义，不把正确性押在上面 |

---

## 11. 常见问题

**Q：启动报 `listen tcp 0.0.0.0:8888: bind: Only one usage of each socket address`**

端口被占了。多半是上一次的进程没退干净：

```bash
taskkill //F //IM charon-api.exe     # Windows
pkill -f charon-api                   # Linux / Git Bash
```

**Q：启动直接 fatal 退出，报 `failed to connect to ...`**

按设计如此——配了 `DB.DataSource` 就连真的连，连不上就启动失败。检查：数据库是否在跑（`docker compose ps`）、DSN 主机名对不对（容器内要用服务名 `postgres`，不能用 `127.0.0.1`）。不需要数据库就把 `DataSource` 留空。

**Q：`/api/v1/demo` 返回 `{"code":501}`**

该实例没配置数据库，属预期行为。配好 `DB.DataSource` 并跑 `make migrate` 即可。

**Q：日志里没有 traceId**

确认请求入口调了 `logger.WithTrace(ctx)`，且用的是 `logx.WithContext(ctx)` 而不是 `logx.Info`。

**Q：`/health` 返回 401**

不该发生——探针刻意不走鉴权中间件。如果真出现，检查是否有人把 `auth.Middleware` 挂到了 `server.Use()` 上（那会作用于所有路由，包括探针）。正确做法是像 [routes.go](internal/handler/routes.go) 那样用 `rest.WithMiddlewares` 只包业务路由。

**Q：`make gen` 之后我的改动丢了**

不应该发生——`gen` 只写 `.gen/`。如果丢了，说明有人手动把它改成了直接生成，请检查 Makefile 的 `gen` 目标。

**Q：`postgres:18-alpine` 拉不下来**

把 `Makefile` 里的 `POSTGRES_VERSION` 改成 `17-alpine`，或设环境变量 `POSTGRES_VERSION=17-alpine`。

**Q：Windows 上 `make` 命令报语法错误**

Makefile 用了 `$$`、`test -f`、`rm -rf` 等 POSIX 写法，需要在 **Git Bash 或 WSL** 里跑。纯 PowerShell 环境可以用 `scripts/dev-up.ps1`。

---

## 更多文档

| 文档 | 内容 |
|------|------|
| [docs/本地启动.md](docs/本地启动.md) | 标准本地 / Windows / 云库隧道 |
| [docs/开发与运维详解.md](docs/开发与运维详解.md) | goctl、配置、Trace/鉴权、Health、ORM、K8s、FAQ |
| [docs/云资源对接清单.md](docs/云资源对接清单.md) | 云资源交接 |
| [pkg/errors/error-code-spec.md](pkg/errors/error-code-spec.md) | 错误码与前缀审批 |
| [templates/README.md](templates/README.md) | goctl 模板与 `.in` 约定 |
| [deploy/k8s/README.md](deploy/k8s/README.md) | Kubernetes 部署 |

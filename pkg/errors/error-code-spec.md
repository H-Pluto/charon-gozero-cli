# charon 错误码规范

本文件是 `pkg/errors` 的配套说明。**新增错误码前请先读完。**

## 格式

错误码对外展示为 `域-码`，例如 `CHARON-1001`。日志里也是这个形式，便于一眼看出错误归属哪个服务。

`域`（Domain）：

| 域 | 用途 |
|----|------|
| `CHARON` | charon 自己的业务错误 |
| `COM` | 跨服务通用的错误（内部错误、不可用、超时） |

## 分段

| 区间 | 归属 | 说明 |
|------|------|------|
| 0 | — | 成功。不作为错误返回 |
| 500-599 | `COM` | 通用域，**所有服务共用同一套语义**，不要在这里加业务含义 |
| 1000-1999 | `CHARON` | charon 业务域 |

新服务接入时，向 `COM` 域申请一段独立区间，避免各服务码值撞车。

## 现行码表

### COM 通用域

| 码 | 常量 | HTTP | 含义 |
|----|------|------|------|
| 500 | `CodeCommonInternal` | 500 | 服务内部错误 |
| 501 | `CodeCommonUnavailable` | 503 | 服务不可用（依赖未就绪等） |
| 502 | `CodeCommonTimeout` | 504 | 请求超时 |

### CHARON 业务域

| 码 | 常量 | HTTP | 含义 |
|----|------|------|------|
| 1001 | `CodeInvalidParam` | 400 | 参数不合法 |
| 1002 | `CodeUnauthorized` | 401 | 未认证或凭证失效 |
| 1003 | `CodeForbidden` | 403 | 无权限 |
| 1004 | `CodeNotFound` | 404 | 资源不存在 |
| 1005 | `CodeConflict` | 409 | 资源冲突 |
| 1006 | `CodeTooManyRequests` | 429 | 请求过于频繁 |
| 1007 | `CodeMethodNotAllowed` | 405 | 请求方法不被允许 |
| 1999 | `CodeInternal` | 500 | 服务内部错误 |

`1001`-`1007` 与 HTTP 标准状态码一一对应，是刻意对齐的：这样客户端、网关、监控都能直接理解，不需要额外查表。`1999` 用作业务域的内部兜底，与 `COM-500` 语义相同但便于区分错误来源。

## HTTP 状态码与业务码的关系

两者并存，**不要混用**：

- **HTTP 状态码**给基础设施看——网关路由、重试策略、监控告警、CDN 缓存。
- **业务码**给业务客户端看——分支处理、文案展示。

因此出现「路径对但方法不对」时用 `405` 而不是 `400`：对客户端来说两者处理方式不同，对监控来说也该分开统计。这也是为什么 `CodeMethodNotAllowed` 单独占一个码，而不是复用 `CodeInvalidParam`。

## 新增错误码的流程

1. **确认真的需要新码**。如果只是文案不同，用 `NewWithMsg` 覆盖描述即可，不必新增码。
2. **选好码值**。加在业务域内，不要占用 `COM` 域。
3. **必须在 `registry` 里注册**（`pkg/errors/code.go`），同时提供 `LangZH` 与 `LangEN` 两种文案。

   未注册的码不会崩溃——`Info()` 会保留原始码值并当作 500 处理。这是有意的：漏注册一个码不该让 4xx 被误报成 500 而丢掉排障线索，但也不要依赖这个兜底。

4. **域前缀变更需要评审**。新增 `域` 属于跨服务约定变更，要同步通知使用 `COM` 域的其他服务。
5. **加测试**。在 `pkg/errors/errors_test.go` 的 `TestCodeHTTPAndDomain` 里补一行。

## 使用约定

### 业务代码里不要返回裸 error

```go
// 对
return nil, bizerr.NewFromContext(l.ctx, bizerr.CodeInvalidParam, cause)

// 错 —— 客户端会收到 500，且丢失「这是参数问题」的语义
return nil, err
```

裸 error 经 `From()` 会被归为 `CodeInternal`，原始内容只进日志、不返回给客户端。**这层过滤是有意的安全边界**：数据库报错原文可能含表名、SQL 片段、连接信息。

### 语言

`NewFromContext` 按 context 中的语言选文案。语言在 handler 入口从 `Accept-Language` 解析：

```go
ctx = bizerr.WithLang(ctx, r.Header.Get("Accept-Language"))
```

解析只做前缀匹配（`zh-CN` 命中 `zh`），不实现完整的 RFC 7231 协商——微服务不需要那个复杂度。

### Cause 只进日志

`BizError` 有三层信息，用途不同：

| 字段 | 去向 | 说明 |
|------|------|------|
| `Code` | 响应 + 日志 | 对外契约 |
| `Msg` | 响应 + 日志 | 已本地化的对外文案 |
| `Cause` | **仅日志** | 内部原因，绝不序列化给客户端 |

写日志时把 `Cause` 带上，响应时不要带。

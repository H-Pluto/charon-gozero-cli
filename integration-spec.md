# charon · Phase 2 集成说明（占位）

**本文件是占位，Phase 2 尚未开始。** 这里记录将来接入 `primary-star` 其他服务时的对接点，避免接手的人重新摸索一遍。

## 当前状态：Phase 1（脚手架）

charon 目前**不参与任何实际业务**。它只有自证接口：

| 接口 | 用途 |
|------|------|
| `GET /api/v1/hello` | 验证路由、中间件、统一响应 |
| `/api/v1/demo` CRUD | 验证配置 → svc → repo → model → 迁移整链 |
| `hello.Hello/Say`（gRPC） | 验证 gRPC 链路含 trace 透传 |

`api/sdk-interfaces/` 目录为空，是给 Phase 2 预留的位置。

## 已探明的对接点（来自现有服务，供 Phase 2 使用）

以下信息是从 `q-bot` 与 `pluto-ai` 现有代码里读出来的**实际契约**，不是设计意图。接入前请复核一遍，因为它们可能已变化。

### RabbitMQ

| 项 | 值 |
|----|----|
| 交换机 | 默认交换机（`exchange_name=""`），**routing key 即队列名** |
| 主队列 | `message.create` |
| 死信队列 | `message.create.dlq` |
| 死信配置 | `x-dead-letter-exchange: ""`，`x-dead-letter-routing-key: message.create.dlq` |
| 队列属性 | durable，消息 persistent |
| 重入队 | `pluto-ai` 侧 `requeue_on_error=False` |

**消息体**（q-bot 的 `MQMessagePayload` 与 pluto-ai 的 `AgentMessage` 字段一致）：

```json
{
  "group_id": 123456,
  "user_id":  null,
  "message":  "文本内容"
}
```

`group_id` 与 `user_id` 均为 `int | str | null`，二者必有一个非空（群消息给 `group_id`，私聊给 `user_id`）。

**生产者**：`q-bot`（`src/message.py` → `producer.send("message.create", payload, dlq="message.create.dlq")`）
**消费者**：`pluto-ai`（`agent/mq/consumer.py`）

### HTTP 调用（q-bot 的发送接口）

pluto-ai 通过 HTTP 调 q-bot 发消息，基址 `BOT_API_BASE_URL`（默认 `http://q-bot:8080`）：

| 方法 | 路径 | 请求体 |
|------|------|--------|
| POST | `/api/v1/send/group` | `{"groupId": 123, "text": "..."}` |
| POST | `/api/v1/send/user` | `{"userId": 123, "text": "..."}` |

两者都返回 `{"code": int, "data": str}`。

**注意**：这个响应格式与 charon 的统一外壳（`{code,message,data,traceId}`）不同——q-bot 是既有的 Python 服务，不打算改。charon 若调用它，需要在客户端侧按它的格式解析，不要假设两边的响应结构一致。

### 环境变量名（现有服务的约定）

| 服务 | 变量 |
|------|------|
| q-bot | `MQ_HOST` `MQ_PORT` `MQ_USER` `MQ_PASSWORD` `MQ_VHOST` `HOST` `PORT` `LOG_LEVEL` |
| pluto-ai | `MQ_HOST` `MQ_PORT` `MQ_USER` `MQ_PASSWORD` `MQ_VHOST` `BOT_API_BASE_URL` `NAP_API_BASE_URL` `LOG_LEVEL` |

charon 目前用的是 `DB_DATA_SOURCE` 等前缀不同的名字。**接入时统一命名风格需要一次讨论**——建议 charon 侧沿用现有服务的 `MQ_*` 命名，减少拼接错误。

### 部署网络

仓库根目录 `docker-compose.yml` 定义了共享网络 **`primary-net`**（bridge），现有服务都在上面。charon 若要互相调用，需接入同一网络——见 [docker-compose.full.yml](../docker-compose.full.yml)。

### 端口占用

| 端口 | 占用 |
|------|------|
| 8080 | q-bot |
| 8888 | **charon-api** |
| 9090 | charon-rpc |
| 5432 | Postgres（charon 的 compose 起的） |
| 5672 / 15672 | RabbitMQ |
| 6099 / 3000 / 3001 | napcat |

## Phase 2 待决问题

留给真正开工时决策，此处仅登记：

1. **charon 的角色是什么？** 它消费 `message.create` 还是生产？还是提供 HTTP 业务接口给其他服务调用？这决定要引入 MQ 客户端还是只做 HTTP 服务端。
2. **是否引入 RabbitMQ 客户端**。若引入，需要选库（`amqp091-go` 还是封装过的），并决定消息结构放 `internal/model` 还是 `pkg/`。
3. **`api/sdk-interfaces/` 放什么**。原参考文档里这是给 SDK 对接预留的，具体形态未定。
4. **服务间 HTTP 客户端的封装**。需要一个带超时、trace 透传、统一错误处理的 HTTP 客户端，对应 `pkg/` 下的一个新包（目前 `pkg/grpcx` 只覆盖 gRPC）。
5. **鉴权何时切到 `sdk`**。依赖 `sdk-auth` 合入。

## 变更记录

| 日期 | 内容 |
|------|------|
| 2026-10-10 | 创建占位，登记 Phase 1 现状与已探明的对接点 |

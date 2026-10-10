# charon Kubernetes 部署草图

本目录是 charon 的 k8s 部署**起点模板**，用于说明「两个服务怎么组、探针怎么配、凭据放哪」。它**不是**可直接投产的清单，请先通读再按实际环境改。

## 包含什么

| 文件 | 内容 |
| --- | --- |
| `api.yaml` | `charon-api` 的 ConfigMap + Deployment + Service（HTTP 8888） |
| `rpc.yaml` | `charon-rpc` 的 ConfigMap + Deployment + Service（gRPC 9090） |
| `../secret.example.yaml` | `DB_DATA_SOURCE` 的 Secret 示例（**不是** `kubectl apply` 的一部分，见下） |

## 应用顺序

1. **先建 Secret**（缺了它 Deployment 会起不来）：

   ```bash
   kubectl create secret generic charon-db \
     --from-literal=DB_DATA_SOURCE='postgres://user:pass@host:5432/charon?sslmode=require'
   ```

   或复制 `../secret.example.yaml` 为 `../secret.yaml` 填好后再 `kubectl apply -f`。`deploy/secret.yaml` 已在 `.gitignore` 里。

2. 再应用两个清单：

   ```bash
   kubectl apply -f deploy/k8s/api.yaml
   kubectl apply -f deploy/k8s/rpc.yaml
   ```

## 关键设计点

### 配置来源

`etc/charon-api.yaml`（仓库里）与集群里 ConfigMap 里的那份是**同一套配置**，只是按环境覆盖了 `Mode`、日志格式等。`DataSource` 一律写成 `$DB_DATA_SOURCE`，由 `conf.UseEnv()` 在启动时从环境变量展开——这样**密文只存在于 Secret**，ConfigMap 里只有变量名。

### 探针

- api：`livenessProbe → /health`（只查进程存活，永不查依赖），`readinessProbe → /readyz`（会 ping Postgres）。探针路径**不经过鉴权中间件**（见 `pkg/health`）——kubelet 不带 Bearer，若被鉴权挡住会一直失败、实例被误判不健康。
- rpc：用原生 `grpc` 探针，调用 zrpc 的健康检查服务（配置里 `Health: true` 才会开）。原生 grpc 探针需要 **k8s >= 1.24**；老集群改用 `grpc_health_probe` + exec 探针（清单里有注释说明）。

### 鉴权与暴露面（**最重要**）

当前 `Auth.Mode: require` 只是「Bearer 非空」的临时门闩，**不验签、挡不住伪造 token**（见 `pkg/auth`）。因此：

- api 的 Service 必须保持 `ClusterIP`；
- **不得**用公网 Ingress 前置它。

等到真正接入 sdk-auth / 验签之后，再谈对外暴露。rpc 目前是明文 gRPC，同样只能走集群网络。

## 上生产前请复核

- [ ] 镜像 tag 换成不可变版本/摘要（清单里是 `charon:dev` 占位）
- [ ] `resources.requests/limits` 按压测结果调整（现在是开发量级）
- [ ] 副本数与更新策略；按需补 PDB、HPA
- [ ] Secret 的获取方式（`--from-literal` / sealed-secrets / external-secrets），确认密文不入库
- [ ] NetworkPolicy：限制谁能访问 api/rpc、只放行到数据库的出向流量
- [ ] ServiceAccount / RBAC（当前用默认 SA）

## 有意未包含

Ingress 网关配置、HPA、PDB、NetworkPolicy、ServiceAccount、命名空间划分——这些依部署环境而定，留给接入时按需补，不在草图里预置以免误导。

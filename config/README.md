# 按环境拆分的配置目录（预留，当前为空）

**本目录目前没有使用。** 配置暂时只放在 [../etc/](../etc/) 下：

- `etc/charon-api.yaml`
- `etc/charon-rpc.yaml`

## 为什么保留这个空目录

`../.gitignore` 里已经有一条 `config/config-local.yaml`，说明最初的设想是把
「按环境拆分的本地配置」放在这里。保留目录是为了让那个约定有个明确的落点。

## 将来怎么用（如果要做多环境配置）

当前做法是**单文件 + 环境变量覆盖**：

```yaml
# etc/charon-api.yaml
DB:
  DataSource: $DB_DATA_SOURCE     # 由 conf.UseEnv() 从环境变量展开
```

同一个文件在本地、测试、生产都能用，差异靠环境变量注入。好处是不会出现
「某个环境漏改了配置文件」这类问题，代价是无法在文件层面看出各环境的差异。

如果将来差异大到环境变量表达不了（比如不同环境走完全不同的中间件组合），
再改成按环境拆文件：

```
config/
├── config-local.yaml      本地：DB 指向 127.0.0.1，日志 debug，Auth none
├── config-dev.yaml        开发环境
└── config-prod.yaml       生产：日志 json，Auth sdk，Telemetry 开
```

启动时用 `-f` 指定：

```bash
./charon-api -f config/config-prod.yaml
```

**注意**：走这条路之前先确认真的有必要。多环境配置文件容易出现「改了
local 忘了改 prod」的问题，而且配置的重复内容会随时间漂移。

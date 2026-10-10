// Package config 定义服务的配置结构，与 etc/*.yaml 一一对应。
//
// 映射规则由 go-zero 决定：**结构体字段的 json tag 就是配置文件的键名**。
// 所以每个字段都必须带 json tag（含 default / optional / options 等选项），
// 否则配置文件里的同名键不会生效。这与标准 encoding/json 只是形似——
// tag 的解析由 go-zero 的 core/mapping 包负责。详见 docs/开发与运维详解.md。
//
// 内嵌 rest.RestConf / zrpc.RpcServerConf 提供框架的所有配置项（Host / Port /
// Timeout / Log / Telemetry / Middlewares ...）。**不要自己重复定义同名字段**，
// 那会覆盖掉框架的默认值。
package config

import (
	"time"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"

	"charon/pkg/auth"
)

// Config 是整个 api 服务的配置树，与 etc/charon-api.yaml 一一对应。
//
// go-zero 用「json tag 决定 yaml 字段名」的约定来解析配置：
// 结构体字段必须带 json tag，否则配置文件里的同名键不会生效。
type Config struct {
	// rest.RestConf 内嵌了 go-zero 的所有 HTTP 服务配置：
	// Name / Host / Port / Timeout / MaxBytes / Middlewares / Log / Telemetry ...
	// 这些字段名不要自己重复定义，否则会覆盖掉框架的默认值。
	rest.RestConf

	// Auth 是鉴权配置。见 pkg/auth —— 当前 Mode=require 只是非空 Bearer 门闩。
	Auth auth.Config

	// Health 是探针配置。
	Health HealthConfig

	// DB 是数据库配置。
	DB DBConfig
}

// HealthConfig 是探针配置，对应 yaml 的 Health 段。
type HealthConfig struct {
	// RequireToken 为 true 时，/health 与 /readyz 必须带 X-Health-Token 头。
	// 默认 false：k8s 的 httpGet 探针来自 kubelet，来源可判定。
	RequireToken bool `json:",default=false"`

	// Token 是探针 token，仅在 RequireToken=true 时有意义。
	Token string `json:",optional"`
}

// DBConfig 是数据库配置，对应 yaml 的 DB 段。
//
// 重要：DataSource 一旦配置，服务启动时就会真的去连；
// 连不上则**启动失败**。这里刻意不做「连不上就回退到内存」的兜底——
// 那样会让配置错误在生产环境表现成「服务正常但数据全空」，比直接启动失败危险得多。
type DBConfig struct {
	// Driver 是 database/sql 的驱动名。我们用 pgx，
	// 它在 pkg/svc 里通过 jackc/pgx/v5/stdlib 注册，名字就是 "pgx"。
	Driver string `json:",default=pgx,options=pgx|postgres"`

	// DataSource 是 PostgreSQL 连接串（DSN），形如：
	//   postgres://user:pass@host:5432/dbname?sslmode=disable
	// 留空表示这个服务不使用数据库。
	DataSource string `json:",optional"`

	// MaxOpenConns 是连接池最大连接数。
	MaxOpenConns int `json:",default=20"`

	// MaxIdleConns 是连接池最大空闲连接数，应不大于 MaxOpenConns。
	MaxIdleConns int `json:",default=10"`

	// MaxLifetime 是单个连接的最长存活时间。
	// 设上限是为了让连接定期重建，避免被中间件（如云上 PgBouncer）单方面掐断后
	// 客户端还在用一个已死的连接。
	MaxLifetime time.Duration `json:",default=1h"`
}

// Enabled 表示是否配置了数据库。
func (c DBConfig) Enabled() bool {
	return c.DataSource != ""
}

// RPCConfig 是 cmd/rpc 的配置树，与 etc/charon-rpc.yaml 一一对应。
//
// 单独一个结构体而不是复用 Config：RPC 服务不需要 HTTP 的 Host/Port/Timeout，
// 混在一起会让配置文件的必填项无从判断。
type RPCConfig struct {
	// zrpc.RpcServerConf 提供 ListenOn / Etcd / Timeout / Middlewares / Log 等。
	// 注意它的 Mode 字段（dev/pro/...）由内嵌的 service.ServiceConf 提供，
	// cmd/rpc/main.go 用它来决定是否开启 gRPC 反射。
	zrpc.RpcServerConf
}

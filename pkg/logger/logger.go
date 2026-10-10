// Package logger 封装 go-zero 的 logx，并解决一个具体问题：
// 让业务日志带上 trace id，且这个能力不依赖 otel 是否开启。
//
// 必须在请求入口显式包一层：
//
//	ctx = logger.WithTrace(ctx)
//	logx.WithContext(ctx).Info("...")   // 输出里就会带 traceId
//
// 为什么不能自动：go-zero 的 WithContext 只从 otel span 里取 trace id，
// 拿不到 pkg/trace 放在 context 里的那个。两套 trace 是刻意分开的（见 pkg/trace）。
//
// 另外转发 Field / WithFields，让业务包统一从本包引入日志能力，
// 不必每处都 import go-zero 的 logx。
package logger

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"charon/pkg/trace"
)

// traceFieldKey 是日志里 trace id 的字段名。
// 与 X-Trace-Id 保持一致的语义，方便在日志系统里按同一个词检索。
const traceFieldKey = "traceId"

// SetUp 初始化日志。整个进程只需调用一次，重复调用会覆盖之前的配置。
//
// 直接用 logx.MustSetup 而不是自己包一层 logx.SetUp：
// 配置错误属于启动期就应该暴露的问题，不该让调用方去处理 error。
func SetUp(conf logx.LogConf) {
	logx.MustSetup(conf)
}

// WithTrace 返回携带 trace id 字段的 context。
//
// 用法是在请求入口拿到 ctx 后立刻包一层：
//
//	ctx = logger.WithTrace(ctx)
//	logx.WithContext(ctx).Info("...")   // 输出里就会带上 traceId
//
// 之所以要显式调用而不是让 logx 自动带上：go-zero 的 WithContext 只会从
// otel span 里取 trace id，而我们的 trace id 走的是 pkg/trace 的 context key，
// 两套是独立的。这样设计是为了让业务日志的串联不依赖 otel 是否开启。
func WithTrace(ctx context.Context) context.Context {
	id := trace.FromContext(ctx)
	if id == "" {
		return ctx
	}
	return logx.ContextWithFields(ctx, logx.Field(traceFieldKey, id))
}

// WithFields 是 ContextWithFields 的转发，方便业务包统一从 logger 引入日志能力，
// 不必同时 import go-zero 的 logx。
func WithFields(ctx context.Context, fields ...logx.LogField) context.Context {
	return logx.ContextWithFields(ctx, fields...)
}

// Field 转发 logx.Field，避免业务包直接依赖 logx。
func Field(key string, value any) logx.LogField {
	return logx.Field(key, value)
}

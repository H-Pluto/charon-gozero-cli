// Package trace 用 X-Trace-Id 为每个请求建立可追踪的标识。
//
// 解决的问题：用户说「我刚才那个请求报错了」，怎么在几万行日志里找到那一次？
// 每个请求分配一个 trace id，贯穿日志与响应，报错时把它贴出来就能定位。
//
// 与 go-zero 自带 trace 的关系：go-zero 的中间件走 W3C traceparent（给 otel
// 采样上报用），不认 X-Trace-Id。两套并存、互不干扰——这样做的价值是
// **业务日志的串联不依赖 otel 是否开启**，本地开发关掉 otel 也能正常追日志。
//
// 入站值是客户端可控的，因此会做 sanitize（见该函数），防日志注入与头注入。
package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// HeaderTraceID 是网关与客户端传递调用链标识用的头。
//
// 注意：go-zero 自带的 trace 中间件走的是 W3C traceparent，不认这个头。
// 两者并不冲突——traceparent 供 otel 采样上报，X-Trace-Id 供业务日志串联，
// 这个中间件负责后者。
const HeaderTraceID = "X-Trace-Id"

// maxTraceIDLen 限制入站 trace id 长度。
// 入站值是客户端可控的，会写进日志与响应头，必须设上限，
// 否则可以被用来撑爆日志或做响应头注入。
const maxTraceIDLen = 64

type ctxKey struct{}

// FromContext 取出当前请求的 trace id，没有则返回空串。
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKey{}).(string); ok {
		return id
	}
	return ""
}

// WithTraceID 把 trace id 放进 context。
// 服务间走 HTTP/gRPC 调用时需要显式传递，接收端再放回 context。
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// Middleware 为每个请求确定一个 trace id 并放进 context。
//
// 优先复用入站的 X-Trace-Id，这样跨服务的调用链能串起来；
// 入站没有（或经过 sanitize 后不可用）时生成一个新的。
// 无论哪种情况都会把最终值写回响应头，方便调用方对账。
//
// 这是挂给 go-zero 中间件链用的形式（rest.Middleware 的签名）。
func Middleware(next http.HandlerFunc) http.HandlerFunc {
	wrapped := Handler(next)

	return func(w http.ResponseWriter, r *http.Request) {
		wrapped.ServeHTTP(w, r)
	}
}

// Handler 是 Middleware 的 http.Handler 版本。
//
// 需要它的原因：go-zero 的 WithNotFoundHandler / WithNotAllowedHandler
// 接收的是 http.Handler，且这两个 handler 在**中间件链之外**被调用——
// 也就是说 server.Use(trace.Middleware) 管不到它们。
// 不额外包一层的话，404/405 的响应体里就不会有 traceId，
// 用户报「接口 404」时反而最难定位。
func Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := sanitize(r.Header.Get(HeaderTraceID))
		if id == "" {
			id = NewID()
		}

		w.Header().Set(HeaderTraceID, id)
		next.ServeHTTP(w, r.WithContext(WithTraceID(r.Context(), id)))
	})
}

// NewID 生成一个随机 trace id（16 字节，hex 编码）。
func NewID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand 在现代系统上不会失败；真失败了也不该让请求挂掉，
		// 退化成空 id，中间件会走到这里说明系统已经不正常了。
		return ""
	}
	return hex.EncodeToString(buf[:])
}

// sanitize 过滤入站 trace id。
//
// 保留可打印且不含分隔符的字符，截断到 maxTraceIDLen。
// 丢掉换行与逗号是为了防日志注入和响应头注入——入站值不可信。
func sanitize(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range id {
		if b.Len() >= maxTraceIDLen {
			break
		}
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			b.WriteRune(r)
		}
	}
	return b.String()
}

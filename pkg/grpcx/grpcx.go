// Package grpcx 提供 go-zero zrpc 没有覆盖的 gRPC 拦截器。
//
// zrpc 已经内置了 trace（otel span）、recover、限流、prometheus 等拦截器，
// 本包补的是它不做的两件事：
//
//  1. trace id 透传：把 pkg/trace 的 X-Trace-Id 放进 gRPC metadata。
//     这样 HTTP 入口拿到的 trace id 能一路带到下游，日志跨服务能串起来。
//  2. 调用日志：记录方法名、状态码、耗时。不记录请求体——可能含敏感信息，
//     且日志量会失控。
//
// 架构约束：cmd/rpc 的 hello 是**明文 gRPC**，只做脚手架自证，靠集群
// NetworkPolicy 隔离。若将来要承载业务 RPC，必须先补 mTLS 与服务端鉴权。
package grpcx

import (
	"context"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"charon/pkg/trace"
)

// MetadataTraceID 是 gRPC metadata 里传递 trace id 的键。
//
// gRPC 的 metadata 键必须小写，与 HTTP 的 X-Trace-Id 是同一个语义
// 的不同传输形式，因此 pkg/trace 的中间件和这里的拦截器可以互相衔接。
const MetadataTraceID = "x-trace-id"

// UnaryServerTraceInterceptor 从入站 metadata 取 trace id 放进 context，
// 并回写到响应 metadata，让调用方能在日志里对上号。
//
// zrpc 自带的 UnaryTracingInterceptor 处理的是 otel span，
// 这个拦截器处理的是我们自己的 trace id，两者互补不重复。
func UnaryServerTraceInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (any, error) {
		id := inboundTraceID(ctx)
		if id == "" {
			id = trace.NewID()
		}

		// 回写响应头。失败不影响业务——这只是一个便利，
		// 调用方拿不到照样能用自己生成的 trace id 排障。
		_ = grpc.SetHeader(ctx, metadata.Pairs(MetadataTraceID, id))

		return handler(trace.WithTraceID(ctx, id), req)
	}
}

// UnaryServerLogInterceptor 记录每个 gRPC 调用的方法与耗时。
//
// 只记录方法与状态码，不记录请求体——gRPC 的请求体可能包含敏感信息，
// 而且日志量会失控。
func UnaryServerLogInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logx.WithContext(ctx).Infof("grpc %s code=%s duration=%s",
			info.FullMethod, code, time.Since(start))

		return resp, err
	}
}

// UnaryClientTraceInterceptor 把 context 里的 trace id 注入出站 metadata，
// 使服务间调用链不断。
func UnaryClientTraceInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if id := trace.FromContext(ctx); id != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, MetadataTraceID, id)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// NewClient 建立一个带 trace 透传的 gRPC 客户端连接。
//
// 关于超时：grpc.NewClient 是惰性连接，不提供拨号超时——连接在首次调用时
// 才真正建立。所以超时必须逐调用通过 context 控制：
//
//	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
//	defer cancel()
//	resp, err := client.Say(ctx, req)
//
// 这里不接受 timeout 参数，就是为了不在签名上暗示一个并不存在的保证。
//
// 安全边界：当前是明文连接，靠集群 NetworkPolicy 隔离。
// 若后续上真实业务 RPC，这里需要补 mTLS 与服务端鉴权凭据。
func NewClient(target string) (*grpc.ClientConn, error) {
	return grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(UnaryClientTraceInterceptor()),
	)
}

// inboundTraceID 从入站 metadata 里取 trace id（取第一个值）。
// 同时兼容 HTTP 头透传过来的大小写变体。
func inboundTraceID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	for _, key := range []string{MetadataTraceID, strings.ToLower(trace.HeaderTraceID)} {
		if vals := md.Get(key); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}

	return ""
}

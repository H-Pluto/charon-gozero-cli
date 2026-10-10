package svcerpc

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"charon/api/pb"
	"charon/pkg/grpcx"
	"charon/pkg/trace"
)

// 用 bufconn 起一个内存内的 gRPC 服务，不占端口、不受网络环境影响。
func startTestServer(t *testing.T) (pb.HelloClient, func()) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(
		// 这两个拦截器就是 cmd/rpc 里挂的那两个，保证测的是真实链路。
		grpc.ChainUnaryInterceptor(
			grpcx.UnaryServerTraceInterceptor(),
			grpcx.UnaryServerLogInterceptor(),
		),
	)
	pb.RegisterHelloServer(server, NewHelloServer())

	go func() {
		_ = server.Serve(lis)
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpcx.UnaryClientTraceInterceptor()),
	)
	if err != nil {
		t.Fatalf("建连失败: %v", err)
	}

	return pb.NewHelloClient(conn), func() {
		_ = conn.Close()
		server.Stop()
	}
}

func TestSay(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	resp, err := client.Say(context.Background(), &pb.SayReq{Name: "charon"})
	if err != nil {
		t.Fatalf("Say 失败: %v", err)
	}

	if want := "hello, charon"; resp.GetMessage() != want {
		t.Errorf("Message = %q, want %q", resp.GetMessage(), want)
	}
}

// 不传 name 时应该有兜底值，不能返回空问候。
func TestSayEmptyNameFallsBack(t *testing.T) {
	client, cleanup := startTestServer(t)
	defer cleanup()

	resp, err := client.Say(context.Background(), &pb.SayReq{})
	if err != nil {
		t.Fatalf("Say 失败: %v", err)
	}

	if want := "hello, world"; resp.GetMessage() != want {
		t.Errorf("Message = %q, want %q", resp.GetMessage(), want)
	}
}

// 这是 grpcx 存在的核心理由：客户端 context 里的 trace id
// 必须真的到达服务端 metadata，服务间调用链才不会断。
func TestClientInterceptorPropagatesTraceID(t *testing.T) {
	var gotTraceID string
	lis := bufconn.Listen(1024 * 1024)

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcx.UnaryServerTraceInterceptor(),
			// 在业务 handler 之前把服务端看到的 trace id 抓出来
			func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
				handler grpc.UnaryHandler) (any, error) {
				if md, ok := metadata.FromIncomingContext(ctx); ok {
					if vals := md.Get(grpcx.MetadataTraceID); len(vals) > 0 {
						gotTraceID = vals[0]
					}
				}
				return handler(ctx, req)
			},
		),
	)
	pb.RegisterHelloServer(server, NewHelloServer())

	go func() { _ = server.Serve(lis) }()
	defer server.Stop()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpcx.UnaryClientTraceInterceptor()),
	)
	if err != nil {
		t.Fatalf("建连失败: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx := trace.WithTraceID(context.Background(), "trace-from-http")
	if _, err := pb.NewHelloClient(conn).Say(ctx, &pb.SayReq{Name: "x"}); err != nil {
		t.Fatalf("Say 失败: %v", err)
	}

	if gotTraceID != "trace-from-http" {
		t.Errorf("服务端收到的 trace id = %q, want %q", gotTraceID, "trace-from-http")
	}
}

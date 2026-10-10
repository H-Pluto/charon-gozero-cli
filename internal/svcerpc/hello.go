// Package svcerpc 是 gRPC 服务的实现层，对应 internal/handler 在 HTTP 侧的角色。
//
// 包名用 svcerpc 而不是 svc，是为了与 internal/svc 区分开：后者是 HTTP 侧的
// 依赖装配（配置、数据库连接、repo），与本包职责完全不同，名字太像会看错。
//
// 当前只实现 hello 自证方法。架构约束：服务间业务调用走 HTTP，
// 不要以本包为模板扩展领域 RPC（见 README.md 的架构约束一节）。
package svcerpc

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"charon/api/pb"
	"charon/pkg/trace"
)

// HelloServer 实现 api/desc/hello.proto 里定义的 Hello 服务。
//
// 服务名是 svcerpc 而不是 svc，是为了与 internal/svc（HTTP 侧的依赖装配）
// 区分开：这两个包职责完全不同，名字太像容易看错。
type HelloServer struct {
	// 必须内嵌 UnimplementedHelloServer，这样将来 proto 加了新方法，
	// 旧代码不会因为没实现而编译失败（gRPC 的前向兼容要求）。
	pb.UnimplementedHelloServer
}

// NewHelloServer 构造服务实现。
//
// 当前没有依赖需要注入。将来若需要数据库或配置，从 internal/svc 取，
// 保持与 HTTP 侧同一套装配方式。
func NewHelloServer() *HelloServer {
	return &HelloServer{}
}

// Say 返回一句问候。仅用于自证 gRPC 链路（含 trace 透传）是通的。
func (s *HelloServer) Say(ctx context.Context, req *pb.SayReq) (*pb.SayResp, error) {
	name := req.GetName()
	if name == "" {
		name = "world"
	}

	logx.WithContext(ctx).Infof("grpc Say called: trace=%s", trace.FromContext(ctx))

	return &pb.SayResp{Message: "hello, " + name}, nil
}

// charon-rpc 是 gRPC 服务的启动入口。
//
// 架构约束：这个服务当前只承载 hello 自证方法，不带业务。
// 服务间的业务调用走 HTTP，不要以本服务为模板扩展领域 RPC。
// 若将来确实需要业务 RPC，先补 mTLS 与鉴权，再考虑开放（见 README.md）。
package main

import (
	"flag"
	"fmt"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"charon/api/pb"
	"charon/internal/config"
	"charon/internal/svcerpc"
	"charon/pkg/grpcx"
)

var configFile = flag.String("f", "etc/charon-rpc.yaml", "配置文件路径")

func main() {
	flag.Parse()

	var c config.RPCConfig
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// zrpc 已经内置了 trace / recover / stat / prometheus 拦截器
	// （见 zrpc 的 RpcServerConf.Middlewares 开关）。
	// 这里额外补的是两件 zrpc 不做的事：
	//   1. 把我们的 X-Trace-Id 在 metadata 里透传（zrpc 只管 otel span）
	//   2. 记录每个方法的调用日志
	svr := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterHelloServer(grpcServer, svcerpc.NewHelloServer())

		// 开发环境开启反射，方便用 grpcurl 直接调试。
		// 生产环境务必关掉：反射会把所有方法签名暴露出去。
		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	svr.AddUnaryInterceptors(
		grpcx.UnaryServerTraceInterceptor(),
		grpcx.UnaryServerLogInterceptor(),
	)

	fmt.Printf("starting rpc server at %s...\n", c.ListenOn)

	svr.Start()
}

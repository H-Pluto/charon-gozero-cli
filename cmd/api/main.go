// charon-api 是 HTTP 服务的启动入口。
//
// 本文件是「运行时真源」：读配置、装配依赖、挂中间件、注册路由、启动。
// templates/ 下的模板仅供参考，不要以模板为准。
package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"

	bizerr "charon/pkg/errors"

	"charon/internal/config"
	"charon/internal/handler"
	"charon/internal/svc"
	"charon/pkg/health"
	"charon/pkg/response"
	"charon/pkg/trace"
)

// configFile 是配置路径。部署时用 -f 指定，默认指向 etc 下的开发配置。
var configFile = flag.String("f", "etc/charon-api.yaml", "配置文件路径")

func main() {
	flag.Parse()

	// 1. 读配置。UseEnv 让配置里可以用 $VAR 引用环境变量，
	//    这样同一个 etc 文件能适配不同环境（见 .env.example）。
	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// 2. 提前校验鉴权模式。配错了要在启动期就失败，
	//    而不是等第一个请求进来才发现「鉴权被静默跳过」。
	if err := c.Auth.Validate(); err != nil {
		panic(fmt.Sprintf("Auth 配置非法: %v", err))
	}

	// 3. 装配依赖（配置、数据库连接池、各 repo）。
	//    数据库配了但连不上时，这里会直接终止进程——不做内存回退。
	svcCtx := svc.NewServiceContext(c)

	// 4. 注册统一响应格式。必须在 server 处理请求前调用一次。
	response.Install()

	// 5. 创建 server 并挂全局中间件。
	server := rest.MustNewServer(c.RestConf,
		// WithNotFoundHandler / WithNotAllowedHandler 收的是 http.Handler，要显式转换。
		//
		// 关键：这两个 handler 在 go-zero 的**中间件链之外**被调用
		// （404 由 engine.notFoundHandler 自己组链，405 直接交给 router），
		// 所以 server.Use(trace.Middleware) 覆盖不到它们。
		// 这里手工包一层 trace.Handler，否则 404/405 的响应体里没有 traceId。
		rest.WithNotFoundHandler(trace.Handler(http.HandlerFunc(notFoundHandler))),
		rest.WithNotAllowedHandler(trace.Handler(http.HandlerFunc(notAllowedHandler))),
	)

	// 全局只挂 trace：它作用于所有路由（含探针），让每个请求都有 trace id。
	// 鉴权不在这里挂——否则会连带挡住探针，见 internal/handler/routes.go。
	server.Use(trace.Middleware)

	// 6. 注册探针路由。探针不经过鉴权中间件，
	//    避免鉴权配置问题导致编排系统误判实例不健康。
	for _, route := range health.Routes(health.Config{
		RequireToken: c.Health.RequireToken,
		Token:        c.Health.Token,
	}, svcCtx.DBReady) {
		server.AddRoutes([]rest.Route{route})
	}

	// 7. 注册业务路由（含鉴权中间件）。
	handler.RegisterHandlers(server, svcCtx)

	server.PrintRoutes()
	server.Start()
}

// notFoundHandler 让 404 也走统一响应格式。
//
// 不这么处理的话，路径写错时会返回纯文本 "404 page not found"，
// 客户端按 JSON 解析会直接报错，掩盖掉真正的问题。
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	response.Fail(r.Context(), w,
		bizerr.NewFromContext(r.Context(), bizerr.CodeNotFound,
			bizerr.Newf(bizerr.CodeNotFound, "路由不存在: %s %s", r.Method, r.URL.Path)))
}

// notAllowedHandler 让 405 也走统一响应格式（路径对但方法不对）。
func notAllowedHandler(w http.ResponseWriter, r *http.Request) {
	response.Fail(r.Context(), w,
		bizerr.NewFromContext(r.Context(), bizerr.CodeMethodNotAllowed,
			bizerr.Newf(bizerr.CodeMethodNotAllowed, "方法不允许: %s %s", r.Method, r.URL.Path)))
}

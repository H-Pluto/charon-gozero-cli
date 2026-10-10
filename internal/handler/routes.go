// Package handler 负责把 URL 路由绑定到 handler 函数。
//
// 本文件是路由的唯一定义处。新增接口时在这里加一行，
// 手写与 goctl 生成的结构保持一致，便于将来切回自动生成。
package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest"

	"charon/internal/handler/demo"
	"charon/internal/handler/hello"
	"charon/internal/svc"
	"charon/pkg/auth"
)

// apiPrefix 是所有业务接口的路径前缀，与 api/desc/*.api 里的 prefix 保持一致。
const apiPrefix = "/api/v1"

// RegisterHandlers 注册业务路由。
//
// 中间件的挂载范围是本文件最重要的设计点，分三层：
//
//  1. 全局（在 cmd/api/main.go 里用 server.Use 挂）：只挂 trace。
//     它会作用于**所有**路由，包括探针——这是需要的，探针也要能串日志。
//
//  2. 业务组（下面 WithMiddlewares 包住的一组）：鉴权。
//     刻意不用 server.Use：那样会把 /health 也一起挡在鉴权后面，
//     导致 kubelet 拿不到探针响应而误判实例不健康。
//     go-zero 的 server.Use 是全局的，没有「按路由挂」的开关，
//     所以这里用 rest.WithMiddlewares 手工给业务路由分组。
//
//  3. 探针路由（在 cmd/api/main.go 里单独注册）：不加任何中间件。
//
// 顺序说明：WithMiddlewares 传进来的中间件按传入顺序由外到内包裹，
// 所以 trace 在 auth 外层——鉴权失败时也能拿到 trace id 排障。
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	// 业务路由：经过 trace + 鉴权。
	bizRoutes := []rest.Route{
		{
			Method:  http.MethodGet,
			Path:    apiPrefix + "/hello",
			Handler: hello.HelloHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    apiPrefix + "/demo",
			Handler: demo.CreateDemoHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    apiPrefix + "/demo",
			Handler: demo.ListDemoHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    apiPrefix + "/demo/:id",
			Handler: demo.GetDemoHandler(svcCtx),
		},
		{
			Method:  http.MethodDelete,
			Path:    apiPrefix + "/demo/:id",
			Handler: demo.DeleteDemoHandler(svcCtx),
		},
	}

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{auth.Middleware(svcCtx.Config.Auth)},
			bizRoutes...,
		),
	)
}

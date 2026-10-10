// Package hello 是 hello 接口的 HTTP 层。
//
// handler 层只做三件事：解析请求 → 调 logic → 写响应。
// 不要在这里写业务判断——那样 logic 就无法被复用和单测。
package hello

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"charon/internal/logic/hello"
	"charon/internal/svc"
	"charon/internal/types"
	"charon/pkg/response"
)

// HelloHandler 处理 GET /api/v1/hello。
//
// 返回 http.HandlerFunc 是 go-zero 的约定；serverCtx 由 routes.go 注入。
func HelloHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. 解析请求参数。失败说明客户端传参不合法。
		var req types.HelloReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		// 2. 交给 logic 处理业务。
		l := hello.NewHelloLogic(r.Context(), svcCtx)
		resp, err := l.Hello(&req)

		// 3. 写响应。用 response.Ok / response.Fail 保证格式统一
		//    （外层会被 pkg/response 的全局 handler 套上 {code,message,data,traceId}）。
		if err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		response.Ok(r.Context(), w, resp)
	}
}

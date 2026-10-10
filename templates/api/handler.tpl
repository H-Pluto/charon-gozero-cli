// Code scaffolded by goctl. Safe to edit.
// 本文件是 charon 定制的 handler 模板，详见同目录 README.md。
// goctl {{.version}}

// Package {{.PkgName}} 是 {{.PkgName}} 接口的 HTTP 层。
//
// handler 只做三件事：解析请求 → 调 logic → 写响应。
// 不要在这里写业务判断——那样 logic 就无法被复用和单测。
package {{.PkgName}}

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	{{.ImportPackages}}

	"charon/pkg/response"
)

{{if .HasDoc}}{{.Doc}}{{end}}
func {{.HandlerName}}(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		{{if .HasRequest}}// 1. 解析请求参数。失败说明客户端传参不合法。
		var req types.{{.RequestType}}
		if err := httpx.Parse(r, &req); err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		{{end}}// 2. 交给 logic 处理业务。
		l := {{.LogicName}}.New{{.LogicType}}(r.Context(), svcCtx)
		{{if .HasResp}}resp, {{end}}err := l.{{.Call}}({{if .HasRequest}}&req{{end}})

		// 3. 写响应。统一走 response.Ok / response.Fail，
		//    由 pkg/response 的全局 handler 套上 {code,message,data,traceId} 外壳。
		if err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		{{if .HasResp}}response.Ok(r.Context(), w, resp){{else}}response.Ok(r.Context(), w, nil){{end}}
	}
}

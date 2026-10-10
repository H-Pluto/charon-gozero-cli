// Package demo 是 demo 接口的 HTTP 层。
//
// 脚手架自证用，业务开工后删除。
package demo

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"

	"charon/internal/logic/demo"
	"charon/internal/svc"
	"charon/internal/types"
	"charon/pkg/response"
)

// CreateDemoHandler 处理 POST /api/v1/demo。
func CreateDemoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateDemoReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		l := demo.NewDemoLogic(r.Context(), svcCtx)
		resp, err := l.Create(&req)
		if err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		response.Ok(r.Context(), w, resp)
	}
}

// GetDemoHandler 处理 GET /api/v1/demo/:id。
func GetDemoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetDemoReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		l := demo.NewDemoLogic(r.Context(), svcCtx)
		resp, err := l.Get(&req)
		if err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		response.Ok(r.Context(), w, resp)
	}
}

// ListDemoHandler 处理 GET /api/v1/demo。
func ListDemoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListDemoReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		l := demo.NewDemoLogic(r.Context(), svcCtx)
		resp, err := l.List(&req)
		if err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		response.Ok(r.Context(), w, resp)
	}
}

// DeleteDemoHandler 处理 DELETE /api/v1/demo/:id。
func DeleteDemoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DeleteDemoReq
		if err := httpx.Parse(r, &req); err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		l := demo.NewDemoLogic(r.Context(), svcCtx)
		if err := l.Delete(&req); err != nil {
			response.Fail(r.Context(), w, err)
			return
		}

		// 删除成功返回 null data 的 200 响应。
		response.Ok(r.Context(), w, nil)
	}
}

// Package response 提供统一的 HTTP 响应外壳 {code, message, data, traceId}。
//
// 解决的问题：如果每个接口各自手写响应格式，格式很容易不一致，客户端要写
// 一堆兼容分支；而改一次格式又要动所有接口。
//
// 做法是往 go-zero 注册全局的响应加工函数（Install 只调一次），之后业务代码
// 只管填 data，外壳自动套上。业务代码里不会出现 code 或 message 字段。
//
// 用法（handler 里的固定三步的最后一步）：
//
//	response.Ok(ctx, w, resp)          // 成功
//	response.Fail(ctx, w, err)         // 失败，err 会被规整成 BizError
//	response.OkPage(ctx, w, list, total, page, size)  // 分页
//
// 只有 5xx 才打 error 日志——4xx 是客户端的问题，每次都打会把日志淹掉。
package response

import (
	"context"
	"net/http"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"

	bizerr "charon/pkg/errors"
	"charon/pkg/trace"
)

// Body 是所有 HTTP 接口统一的响应外壳。
//
// code 遵循 pkg/errors 的约定：0 表示成功，非 0 是业务错误码。
// 注意 HTTP 状态码与 code 并存：HTTP 码给基础设施（网关、监控、重试策略）看，
// code 给业务客户端看，两者不要混用。
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	TraceID string `json:"traceId,omitempty"`
}

// CodeOK 是成功响应的业务码。
const CodeOK = 0

// PageData 是分页响应的统一结构，列表接口都应该用它。
type PageData struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// Install 把统一响应格式注册进 go-zero 的全局 handler。
//
// 必须在创建 server 之后、开始处理请求之前调用一次；它是进程级全局状态。
// 注册之后 handler 里只要调 httpx.OkJsonCtx / httpx.ErrorCtx，
// 输出就会自动套上 Body 外壳，业务代码不需要关心序列化格式。
func Install() {
	httpx.SetOkHandler(func(ctx context.Context, v any) any {
		return Body{
			Code:    CodeOK,
			Message: "ok",
			Data:    v,
			TraceID: trace.FromContext(ctx),
		}
	})

	httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
		biz := bizerr.From(err)

		// 只有 5xx 才记 error 级日志。4xx 是客户端的问题，
		// 每次都打 error 会把日志淹掉，出故障时反而看不见真问题。
		if biz.HTTPStatus() >= http.StatusInternalServerError {
			logx.WithContext(ctx).Errorf("request failed: code=%s msg=%s cause=%v",
				biz.Code, biz.Msg, biz.Cause)
		}

		return biz.HTTPStatus(), Body{
			Code:    int(biz.Code),
			Message: biz.Msg,
			TraceID: trace.FromContext(ctx),
		}
	})
}

// Ok 返回成功响应。
func Ok(ctx context.Context, w http.ResponseWriter, v any) {
	httpx.OkJsonCtx(ctx, w, v)
}

// OkPage 返回分页成功响应。
func OkPage(ctx context.Context, w http.ResponseWriter, list any, total int64, page, pageSize int) {
	httpx.OkJsonCtx(ctx, w, PageData{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// Fail 返回错误响应。传 nil 时是空操作。
//
// err 会被 bizerr.From 规整：非业务错误统一变成 CodeInternal，
// 原始内容落到 Cause 只进日志，不会泄漏给客户端。
func Fail(ctx context.Context, w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	httpx.ErrorCtx(ctx, w, err)
}

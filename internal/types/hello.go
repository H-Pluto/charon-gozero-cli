// Package types 放 HTTP 请求/响应结构体。
//
// 这些结构体与 api/desc/*.api 里的定义一一对应，是 .api 契约的 Go 表达。
// 改了 .api 就要同步改这里，反之亦然。
//
// 注意：响应结构体（如 HelloResp）只包含业务字段，
// 不含 code/message/traceId——最外层外壳由 pkg/response 统一套上。
package types

// HelloReq 对应 hello.api 的 HelloReq。
type HelloReq struct {
	// form tag 表示从 query string 取参数（?name=xxx）。
	// optional 表示可以不传；缺省值由 logic 处理。
	Name string `form:"name,optional"`
}

// HelloResp 对应 hello.api 的 HelloResp。
type HelloResp struct {
	Message string `json:"message"`
	Time    string `json:"time"`
}

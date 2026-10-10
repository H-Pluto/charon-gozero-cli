package types

// PageResult 是分页列表接口的通用返回结构。
//
// 放在 types 而不是复用 pkg/response.PageData：
// logic 层不应该依赖 HTTP 响应格式，否则换个传输协议（比如将来加 gRPC）
// 就要改业务代码。pkg/response 会在 handler 层把它套上统一外壳。
type PageResult struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

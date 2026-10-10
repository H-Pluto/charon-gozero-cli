package types

// CreateDemoReq 对应 demo.api 的 CreateDemoReq，从 JSON body 解析。
type CreateDemoReq struct {
	Name string `json:"name"`
}

// CreateDemoResp 返回新建记录的 id。
type CreateDemoResp struct {
	ID int64 `json:"id"`
}

// GetDemoReq 从路径参数取 id（/demo/:id）。
type GetDemoReq struct {
	ID int64 `path:"id"`
}

// DemoItem 是 demo_item 的对外形状。
//
// 它和 internal/model.DemoItem 是分开的两个结构体，不是冗余：
// model 贴数据库（字段用 db tag、时间用 time.Time），
// types 贴 HTTP（字段是 json tag、时间格式化成字符串）。
// 合成一个会让数据库变更直接泄漏成 API 破坏性变更。
type DemoItem struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// ListDemoReq 是分页列表请求，内嵌 PageQuery 复用分页参数。
type ListDemoReq struct {
	Page     int `form:"page,optional,default=1"`
	PageSize int `form:"pageSize,optional,default=20,range=[1:100]"`
}

// DeleteDemoReq 从路径参数取 id。
type DeleteDemoReq struct {
	ID int64 `path:"id"`
}

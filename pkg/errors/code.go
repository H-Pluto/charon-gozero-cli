// Package errors 是 charon 的业务错误体系。
//
// 解决的问题：接口出错时要告诉客户端「错在哪」，但又不能把数据库报错原文
// 之类的内部信息泄漏出去。做法是预定义一批错误码（对外渲染为 CHARON-1001
// 这种形式），每个码绑定一个 HTTP 状态码和一组按语言分的文案。
//
// 三个层次的信息分开走：
//
//	Code   对外契约，客户端据此分支处理  → 响应 + 日志
//	Msg    已本地化的对外文案            → 响应 + 日志
//	Cause  内部原因（如 sql.ErrNoRows）  → 仅日志，绝不返回给客户端
//
// 最常用的构造入口是 NewFromContext（按请求语言选文案）。
// 错误码的分段与新增流程见 error-code-spec.md。
package errors

import "fmt"

// Lang 是错误消息的语言标识。
type Lang string

const (
	LangZH Lang = "zh"
	LangEN Lang = "en"

	// DefaultLang 是无法从请求中判定语言时的兜底。
	DefaultLang = LangZH
)

// 错误码域前缀。对外展示为 "CHARON-1001" 这种形式，
// 便于从日志里一眼看出错误归属哪个服务。
const (
	DomainCharon = "CHARON"
	DomainCommon = "COM"
)

// Code 是业务错误码。
//
// 分段约定：
//
//	0            成功，不作为错误返回
//	500-599      COM 通用域，跨服务共用
//	1000-1999    CHARON 业务域
//
// 新增错误码前请先读 error-code-spec.md，域前缀需要评审。
type Code int

// COM 通用域。
const (
	CodeCommonInternal    Code = 500
	CodeCommonUnavailable Code = 501
	CodeCommonTimeout     Code = 502
)

// CHARON 业务域。
const (
	CodeInvalidParam    Code = 1001
	CodeUnauthorized    Code = 1002
	CodeForbidden       Code = 1003
	CodeNotFound        Code = 1004
	CodeConflict        Code = 1005
	CodeTooManyRequests Code = 1006
	// CodeMethodNotAllowed 对应 HTTP 405：路径存在但方法不对。
	// 单独一个码而不是复用 CodeInvalidParam，是因为 405 与 400 在
	// 客户端缓存、网关路由、监控告警里的处理方式都不同，不该混为一谈。
	CodeMethodNotAllowed Code = 1007
	CodeInternal         Code = 1999
)

// ErrInfo 描述一个错误码的静态属性。
type ErrInfo struct {
	Code     Code
	Domain   string
	HTTP     int
	Messages map[Lang]string
}

// Message 返回指定语言的描述，缺失时回退到 DefaultLang，
// 再缺失时回退到英语，最后退化为错误码本身，保证永不返回空串。
func (i ErrInfo) Message(lang Lang) string {
	if msg, ok := i.Messages[lang]; ok && msg != "" {
		return msg
	}
	if msg, ok := i.Messages[DefaultLang]; ok && msg != "" {
		return msg
	}
	if msg, ok := i.Messages[LangEN]; ok && msg != "" {
		return msg
	}
	return i.String()
}

// String 返回 "域-码" 形式，例如 "CHARON-1001"。
func (i ErrInfo) String() string {
	return fmt.Sprintf("%s-%d", i.Domain, i.Code)
}

var registry = map[Code]ErrInfo{
	CodeCommonInternal: {
		Code: CodeCommonInternal, Domain: DomainCommon, HTTP: 500,
		Messages: map[Lang]string{
			LangZH: "服务内部错误",
			LangEN: "internal error",
		},
	},
	CodeCommonUnavailable: {
		Code: CodeCommonUnavailable, Domain: DomainCommon, HTTP: 503,
		Messages: map[Lang]string{
			LangZH: "服务不可用",
			LangEN: "service unavailable",
		},
	},
	CodeCommonTimeout: {
		Code: CodeCommonTimeout, Domain: DomainCommon, HTTP: 504,
		Messages: map[Lang]string{
			LangZH: "请求超时",
			LangEN: "request timeout",
		},
	},
	CodeInvalidParam: {
		Code: CodeInvalidParam, Domain: DomainCharon, HTTP: 400,
		Messages: map[Lang]string{
			LangZH: "参数不合法",
			LangEN: "invalid parameter",
		},
	},
	CodeUnauthorized: {
		Code: CodeUnauthorized, Domain: DomainCharon, HTTP: 401,
		Messages: map[Lang]string{
			LangZH: "未认证或凭证已失效",
			LangEN: "unauthorized",
		},
	},
	CodeForbidden: {
		Code: CodeForbidden, Domain: DomainCharon, HTTP: 403,
		Messages: map[Lang]string{
			LangZH: "无权限访问",
			LangEN: "forbidden",
		},
	},
	CodeNotFound: {
		Code: CodeNotFound, Domain: DomainCharon, HTTP: 404,
		Messages: map[Lang]string{
			LangZH: "资源不存在",
			LangEN: "not found",
		},
	},
	CodeConflict: {
		Code: CodeConflict, Domain: DomainCharon, HTTP: 409,
		Messages: map[Lang]string{
			LangZH: "资源冲突",
			LangEN: "conflict",
		},
	},
	CodeTooManyRequests: {
		Code: CodeTooManyRequests, Domain: DomainCharon, HTTP: 429,
		Messages: map[Lang]string{
			LangZH: "请求过于频繁",
			LangEN: "too many requests",
		},
	},
	CodeMethodNotAllowed: {
		Code: CodeMethodNotAllowed, Domain: DomainCharon, HTTP: 405,
		Messages: map[Lang]string{
			LangZH: "请求方法不被允许",
			LangEN: "method not allowed",
		},
	},
	CodeInternal: {
		Code: CodeInternal, Domain: DomainCharon, HTTP: 500,
		Messages: map[Lang]string{
			LangZH: "服务内部错误",
			LangEN: "internal error",
		},
	},
}

// Info 返回错误码的静态描述。未注册的码会被视为内部错误，
// 但保留原始码值，避免为了漏注册一个码就把 4xx 误报成 500 而丢失排障线索。
func (c Code) Info() ErrInfo {
	if info, ok := registry[c]; ok {
		return info
	}
	return ErrInfo{
		Code:   c,
		Domain: DomainCharon,
		HTTP:   500,
		Messages: map[Lang]string{
			LangZH: fmt.Sprintf("未注册的错误码 %d", int(c)),
			LangEN: fmt.Sprintf("unregistered error code %d", int(c)),
		},
	}
}

// HTTP 返回该错误码应使用的 HTTP 状态码。
func (c Code) HTTP() int { return c.Info().HTTP }

// Domain 返回错误码所属域前缀。
func (c Code) Domain() string { return c.Info().Domain }

// Message 返回指定语言下的默认描述。
func (c Code) Message(lang Lang) string { return c.Info().Message(lang) }

// String 返回 "域-码"，例如 "CHARON-1001"。
func (c Code) String() string { return c.Info().String() }

package errors

import (
	"context"
	"fmt"
	"strings"
)

// langCtxKey 是 context 中携带语言的私有键。用私有类型避免与其他包的键冲突。
type langCtxKey struct{}

// WithLang 把语言标识放进 context，后续 NewFromContext 会据此本地化。
//
// 典型用法是在 handler 入口处：
//
//	ctx = bizerr.WithLang(ctx, r.Header.Get("Accept-Language"))
func WithLang(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, langCtxKey{}, ParseLang(lang))
}

// LangFromContext 取出 context 中的语言，没有则返回 DefaultLang。
func LangFromContext(ctx context.Context) Lang {
	if lang, ok := ctx.Value(langCtxKey{}).(Lang); ok && lang != "" {
		return lang
	}
	return DefaultLang
}

// ParseLang 从 HTTP Accept-Language 头的原始值解析出受支持的语言。
//
// 只做前缀匹配，够用即可：一个微服务不需要完整的 RFC 7231 协商。
// 形如 "zh-CN,zh;q=0.9,en;q=0.8" 会命中 zh。
func ParseLang(header string) Lang {
	header = strings.TrimSpace(header)
	if header == "" {
		return DefaultLang
	}

	// 取第一个语言标签，丢掉 q 权重与后续备选。
	first := header
	if idx := strings.IndexByte(first, ','); idx >= 0 {
		first = first[:idx]
	}
	if idx := strings.IndexByte(first, ';'); idx >= 0 {
		first = first[:idx]
	}
	first = strings.ToLower(strings.TrimSpace(first))

	switch {
	case strings.HasPrefix(first, string(LangZH)):
		return LangZH
	case strings.HasPrefix(first, string(LangEN)):
		return LangEN
	default:
		return DefaultLang
	}
}

// NewFromContext 按 context 中的语言构造错误。
// 这是业务代码里最常用的构造入口。
func NewFromContext(ctx context.Context, code Code, cause error) *BizError {
	return &BizError{
		Code:  code,
		Msg:   code.Message(LangFromContext(ctx)),
		Cause: cause,
	}
}

// NewfFromContext 按 context 中的语言构造错误，并把 format 作为内部原因。
func NewfFromContext(ctx context.Context, code Code, format string, args ...any) *BizError {
	return &BizError{
		Code:  code,
		Msg:   code.Message(LangFromContext(ctx)),
		Cause: fmt.Errorf(format, args...),
	}
}

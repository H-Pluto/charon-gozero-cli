package errors

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestCodeHTTPAndDomain(t *testing.T) {
	cases := []struct {
		code   Code
		http   int
		domain string
	}{
		{CodeInvalidParam, 400, DomainCharon},
		{CodeUnauthorized, 401, DomainCharon},
		{CodeForbidden, 403, DomainCharon},
		{CodeNotFound, 404, DomainCharon},
		{CodeConflict, 409, DomainCharon},
		{CodeTooManyRequests, 429, DomainCharon},
		{CodeInternal, 500, DomainCharon},
		{CodeCommonInternal, 500, DomainCommon},
		{CodeCommonUnavailable, 503, DomainCommon},
		{CodeCommonTimeout, 504, DomainCommon},
	}

	for _, c := range cases {
		if got := c.code.HTTP(); got != c.http {
			t.Errorf("%d: HTTP() = %d, want %d", c.code, got, c.http)
		}
		if got := c.code.Domain(); got != c.domain {
			t.Errorf("%d: Domain() = %q, want %q", c.code, got, c.domain)
		}
	}
}

// 未注册的错误码不能退化成「丢失原始码值」，否则排障时会失去线索。
func TestUnregisteredCodeKeepsValue(t *testing.T) {
	const unknown Code = 9876
	info := unknown.Info()

	if info.Code != unknown {
		t.Errorf("Code = %d, want %d", info.Code, unknown)
	}
	if info.HTTP != 500 {
		t.Errorf("HTTP = %d, want 500", info.HTTP)
	}
}

// 语言协商：zh 命中中文，未知语言回退到默认（中文）。
func TestParseLang(t *testing.T) {
	cases := map[string]Lang{
		"":                        LangZH,
		"zh-CN":                   LangZH,
		"zh-CN,zh;q=0.9,en;q=0.8": LangZH,
		"en-US":                   LangEN,
		"en":                      LangEN,
		"fr-FR":                   LangZH,
		"ZH-cn":                   LangZH,
	}

	for header, want := range cases {
		if got := ParseLang(header); got != want {
			t.Errorf("ParseLang(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestNewFromContextLocalizes(t *testing.T) {
	ctx := WithLang(context.Background(), "en-US")
	biz := NewFromContext(ctx, CodeNotFound, nil)
	if biz.Msg != "not found" {
		t.Errorf("Msg = %q, want %q", biz.Msg, "not found")
	}
	if biz.HTTPStatus() != 404 {
		t.Errorf("HTTPStatus() = %d, want 404", biz.HTTPStatus())
	}

	// 不带 lang 的 context 走默认语言。
	plain := NewFromContext(context.Background(), CodeNotFound, nil)
	if plain.Msg != "资源不存在" {
		t.Errorf("Msg = %q, want %q", plain.Msg, "资源不存在")
	}
}

// From 必须把非业务错误一律归为 CodeInternal，且不泄漏原因到客户端可见字段。
func TestFromHidesUnknownCause(t *testing.T) {
	secret := errors.New("pq: password authentication failed for user \"postgres\"")
	biz := From(secret)

	if biz.Code != CodeInternal {
		t.Fatalf("Code = %d, want %d", biz.Code, CodeInternal)
	}
	if biz.Msg != CodeInternal.Message(DefaultLang) {
		t.Errorf("Msg = %q, 应使用错误码默认文案", biz.Msg)
	}
	// 内部原因要保留在 Cause 里供日志使用。
	if !errors.Is(biz, secret) {
		t.Error("Cause 应保留原始错误以便日志排查")
	}
}

func TestFromNilAndIdempotent(t *testing.T) {
	if From(nil) != nil {
		t.Error("From(nil) 应为 nil")
	}

	original := Newf(CodeInvalidParam, "name 为空")
	got := From(original)
	if got != original {
		t.Error("已包装的 BizError 应原样返回，不应被二次包装")
	}
}

func TestWrapKeepsExistingBizError(t *testing.T) {
	original := New(CodeUnauthorized, nil)
	// 已经确定好对外契约的错误，不该被 Wrap 覆盖成 CodeNotFound。
	if got := Wrap(original, CodeNotFound); got != original {
		t.Errorf("Wrap 不应覆盖已有 BizError，got %+v", got)
	}

	wrapped := Wrap(errors.New("sql: no rows in result set"), CodeNotFound)
	if wrapped.Code != CodeNotFound {
		t.Errorf("Code = %d, want %d", wrapped.Code, CodeNotFound)
	}
}

func TestIs(t *testing.T) {
	err := fmt.Errorf("上层包装: %w", New(CodeConflict, nil))
	if !Is(err, CodeConflict) {
		t.Error("Is 应能穿透 fmt.Errorf 的包装")
	}
	if Is(err, CodeNotFound) {
		t.Error("不应匹配不同的错误码")
	}
	if Is(nil, CodeNotFound) {
		t.Error("nil 不应匹配任何错误码")
	}
}

func TestErrorStringIncludesCause(t *testing.T) {
	biz := New(CodeInternal, errors.New("boom"))
	want := "[CHARON-1999] 服务内部错误: boom"
	if got := biz.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

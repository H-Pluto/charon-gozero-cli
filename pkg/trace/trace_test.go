package trace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSanitizeBlocksInjection(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"正常值原样保留", "abc-123_XYZ.9", "abc-123_XYZ.9"},
		{"去掉换行防日志注入", "abc\r\ninjected: 1", "abcinjected1"},
		{"去掉逗号与空格", "abc, def", "abcdef"},
		{"超长截断", strings.Repeat("a", 100), strings.Repeat("a", maxTraceIDLen)},
		{"纯空白视为空", "   ", ""},
		{"空串", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitize(c.in); got != c.want {
				t.Errorf("sanitize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// 入站 trace id 可用时必须复用，否则跨服务的调用链会断。
func TestMiddlewareReusesInboundID(t *testing.T) {
	var seen string
	next := func(w http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	handler := Middleware(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderTraceID, "inbound-trace")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if seen != "inbound-trace" {
		t.Errorf("context trace id = %q, want %q", seen, "inbound-trace")
	}
	if got := rec.Header().Get(HeaderTraceID); got != "inbound-trace" {
		t.Errorf("响应头 trace id = %q, want %q", got, "inbound-trace")
	}
}

// 入站没有 trace id 时要生成一个，且要能回写到响应头。
func TestMiddlewareGeneratesID(t *testing.T) {
	var seen string
	next := func(w http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	handler := Middleware(next)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if seen == "" {
		t.Fatal("应生成 trace id")
	}
	if len(seen) != 32 {
		t.Errorf("生成的 trace id 长度 = %d, want 32", len(seen))
	}
	if rec.Header().Get(HeaderTraceID) != seen {
		t.Error("响应头应回写生成的 trace id")
	}
}

// 入站 trace id 全是非法字符时不能复用空串，必须退化成新生成的。
func TestMiddlewareIgnoresUnusableInboundID(t *testing.T) {
	var seen string
	next := func(w http.ResponseWriter, r *http.Request) {
		seen = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	handler := Middleware(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderTraceID, ":::")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if seen == "" {
		t.Fatal("非法入站值应退化为新生成的 trace id")
	}
}

func TestFromContextEmpty(t *testing.T) {
	if got := FromContext(context.Background()); got != "" {
		t.Errorf("FromContext(empty) = %q, want empty", got)
	}
}

func TestNewIDUnique(t *testing.T) {
	a, b := NewID(), NewID()
	if a == b {
		t.Error("NewID 应产生不同的值")
	}
}

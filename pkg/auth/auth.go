// Package auth 提供鉴权中间件。
//
// **现状是「门闩」而不是真鉴权**，用之前务必清楚这一点：
//
//	none     放行。仅本地开发可用
//	require  只校验 Authorization: Bearer <非空>，不验签、不解析、不校验有效期
//	sdk      预留给 sdk-auth，当前放行并打警告
//
// ModeRequire 挡不住伪造的 token——任何字符串都能通过。它的作用只是拦住
// 没有凭证的探测流量，并为将来换成真鉴权留好挂载位置。因此：
//
//	Auth.Mode=require 期间，该服务禁止暴露到公网 Ingress，只能经集群内网访问。
//
// 中间件对非法的 Mode 值采取**默认拒绝**（返回 403）而不是默认放行，
// 不允许出现「配置写错 = 不鉴权」。
package auth

import (
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	bizerr "charon/pkg/errors"
	"charon/pkg/response"
)

// Mode 决定鉴权中间件的行为。
type Mode string

const (
	// ModeNone 完全放行，仅供本地开发。
	ModeNone Mode = "none"

	// ModeRequire 只校验 Bearer token 非空，不验签、不解析。
	//
	// 这是一个有意的临时状态：真实鉴权能力在 sdk-auth 合入后才有。
	// 当前它起到的作用是「门闩」——挡住没有凭证的探测流量，
	// 并为后续替换成真鉴权留好中间件位置。
	//
	// 安全边界：它挡不住伪造的 token。因此 ModeRequire 期间
	// 该服务禁止暴露到公网 Ingress，只能经集群内网访问。
	ModeRequire Mode = "require"

	// ModeSDK 预留给 sdk-auth 接入。当前实现为放行并打警告，
	// 避免配置了这个值却静默失效。
	ModeSDK Mode = "sdk"
)

// Config 是鉴权中间件的配置。
type Config struct {
	Mode Mode
}

// Validate 校验配置合法性，在启动期调用，早失败早暴露。
func (c Config) Validate() error {
	switch c.Mode {
	case ModeNone, ModeRequire, ModeSDK:
		return nil
	case "":
		return bizerr.Newf(bizerr.CodeInvalidParam, "Auth.Mode 不能为空，可选 none/require/sdk")
	default:
		return bizerr.Newf(bizerr.CodeInvalidParam, "Auth.Mode 取值非法: %q", c.Mode)
	}
}

// Middleware 返回鉴权中间件。
//
// 401 走 response.Fail 输出，保证与业务错误的响应结构一致；
// 客户端只需要处理一种错误格式。
func Middleware(conf Config) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		switch conf.Mode {
		case ModeNone:
			return next

		case ModeRequire:
			return func(w http.ResponseWriter, r *http.Request) {
				token, ok := bearerToken(r)
				if !ok || token == "" {
					response.Fail(r.Context(), w,
						bizerr.NewFromContext(r.Context(), bizerr.CodeUnauthorized, nil))
					return
				}
				next(w, r)
			}

		case ModeSDK:
			// sdk-auth 尚未合入。显式告警而不是静默放行：
			// 配置成 sdk 却得到「不鉴权」的行为，如果没日志会非常危险。
			logx.Error("Auth.Mode=sdk 但 sdk-auth 尚未接入，当前请求全部放行。请勿在生产环境使用。")
			return next

		default:
			// Validate 会在启动期拦住非法值，走到这里说明调用方绕过了 Validate。
			// 默认拒绝，不允许「配置错误 = 不鉴权」。
			return func(w http.ResponseWriter, r *http.Request) {
				response.Fail(r.Context(), w,
					bizerr.NewFromContext(r.Context(), bizerr.CodeForbidden,
						bizerr.Newf(bizerr.CodeForbidden, "鉴权模式未正确配置: %q", conf.Mode)))
			}
		}
	}
}

// bearerToken 从 Authorization 头里取出 Bearer 之后的原始 token。
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}

	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}

	return strings.TrimSpace(header[len(prefix):]), true
}

// Package health 提供 /health（存活）与 /readyz（就绪）两个探针。
//
// 两者的分工是本包最重要的设计点：
//
//	/health  进程还能处理请求就返回 200，**不检查任何依赖**。
//	         失败会导致容器被重启，所以绝不能把依赖故障算进来——
//	         重启解决不了依赖问题，只会让所有实例同时重连、加剧雪崩。
//	/readyz  检查依赖（数据库连通性），不通返回 503。
//	         失败只是从负载均衡摘除，不重启。
//
// 探针刻意不走业务的鉴权中间件与统一响应外壳：它们的消费者是 kubelet、LB
// 这类基础设施，不该因为业务改了鉴权配置或响应格式就全体失联。
// 这也是为什么路由由本包提供、由 cmd/api 单独注册（见 internal/handler/routes.go）。
package health

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/zeromicro/go-zero/rest"
)

// PathHealth 是存活探针路径。
const PathHealth = "/health"

// PathReady 是就绪探针路径。
const PathReady = "/readyz"

// HeaderToken 是探针携带 token 用的头。
const HeaderToken = "X-Health-Token"

// Config 是探针配置。
type Config struct {
	// RequireToken 为 true 时探针必须带 HeaderToken 且值等于 Token。
	//
	// 默认 false 的取舍：k8s 的 httpGet 探针来自 kubelet，来源可判定，
	// 且 /health 只暴露进程存活这一位信息，不泄漏内部状态。
	// 如果探针路径被暴露到不可信网络，再打开这个开关。
	RequireToken bool
	Token        string
}

// Checker 是就绪检查函数。返回 nil 表示依赖可用。
// 传 nil 表示该服务没有外部依赖需要检查。
type Checker func(ctx context.Context) error

// statusBody 是探针的响应体。
//
// 探针刻意不使用业务统一响应外壳（pkg/response.Body）：
// 探针是给 kubelet、LB 这类基础设施消费的，不应该依赖业务的响应约定，
// 否则改一次业务格式就可能让所有探针失联。
type statusBody struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Routes 返回探针路由，由调用方注册到 rest server。
//
// 不复用业务中间件链（trace/auth）：探针不该被鉴权门闩挡住，
// 也不该因为多一层中间件而在依赖出问题时表现得和业务一样。
func Routes(conf Config, check Checker) []rest.Route {
	return []rest.Route{
		{
			Method:  http.MethodGet,
			Path:    PathHealth,
			Handler: guard(conf, handleHealth),
		},
		{
			Method:  http.MethodGet,
			Path:    PathReady,
			Handler: guard(conf, handleReady(check)),
		},
	}
}

// guard 在需要时校验探针 token。
func guard(conf Config, next http.HandlerFunc) http.HandlerFunc {
	if !conf.RequireToken {
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get(HeaderToken)
		// 用 constant-time 比较，避免通过响应耗时逐字节猜 token。
		if subtle.ConstantTimeCompare([]byte(got), []byte(conf.Token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, statusBody{Status: "unauthorized"})
			return
		}
		next(w, r)
	}
}

// handleHealth 是存活探针：进程还在处理请求就返回 200。
//
// 刻意不检查外部依赖——存活探针失败会导致容器被重启，
// 而依赖不可用重启自己是解决不了问题的，只会让雪崩更严重。
// 依赖检查放在就绪探针里。
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusBody{Status: "ok"})
}

// handleReady 是就绪探针：所有已配置的依赖都可用才算就绪。
func handleReady(check Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if check == nil {
			// 没有配置外部依赖，就没有「未就绪」的可能性。
			writeJSON(w, http.StatusOK, statusBody{Status: "ok"})
			return
		}

		if err := check(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, statusBody{
				Status: "unavailable",
				// 探针通常只有运维能访问，把原因带出来能省一次翻日志。
				Error: err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, statusBody{Status: "ok"})
	}
}

func writeJSON(w http.ResponseWriter, code int, body statusBody) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// Code scaffolded by goctl. Safe to edit.
// 本文件是 charon 定制的 logic 模板，详见同目录 README.md。
// goctl {{.version}}

// Package {{.pkgName}} 是 {{.pkgName}} 接口的业务逻辑。
//
// logic 层放业务规则：不解析 HTTP、不拼 SQL。
// 入参是已解析好的请求结构体，出参是响应结构体或 error。
package {{.pkgName}}

import (
	{{.imports}}
)

// {{.logic}} 实现对应接口的业务逻辑。
type {{.logic}} struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

{{if .hasDoc}}{{.doc}}{{end}}
// New{{.logic}} 由 handler 构造。
func New{{.logic}}(ctx context.Context, svcCtx *svc.ServiceContext) *{{.logic}} {
	return &{{.logic}}{
		// logx.WithContext 会把请求的 trace 信息带进日志
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *{{.logic}}) {{.function}}({{.request}}) {{.responseType}} {
	// todo: 在此实现业务逻辑。
	//
	// 出错时统一用 charon/pkg/errors 构造业务错误，不要返回裸 error——
	// 裸 error 会被归为 500 内部错误，拿不到期望的 4xx 语义。示例：
	//
	//	bizerr "charon/pkg/errors"
	//	bizerr.NewFromContext(l.ctx, bizerr.CodeInvalidParam,
	//		bizerr.Newf(bizerr.CodeInvalidParam, "参数不合法: %s", reason))
	//
	// 有返回值的接口记得把上面的错误与 nil 一起返回。

	{{.returnString}}
}

// Package hello 是 hello 接口的业务逻辑。
//
// logic 层放业务规则：不解析 HTTP、不拼 SQL。
// 它的入参是已解析好的请求结构体，出参是响应结构体或 error。
package hello

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	bizerr "charon/pkg/errors"

	"charon/internal/svc"
	"charon/internal/types"
)

// HelloLogic 实现 GET /api/v1/hello。
type HelloLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewHelloLogic 由 handler 构造。
func NewHelloLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HelloLogic {
	return &HelloLogic{
		// logx.WithContext 会把请求的 trace 信息带进日志
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Hello 返回一句问候。
//
// 返回 error 时统一用 bizerr 构造，不要直接返回裸 error：
// 裸 error 会被归为 500 内部错误，而这里的「参数不合法」应该是 400。
func (l *HelloLogic) Hello(req *types.HelloReq) (*types.HelloResp, error) {
	name := req.Name
	if name == "" {
		// 没传就兜底，而不是报错——这是产品决定，放在 logic 层而不是 handler。
		name = "world"
	}

	// 演示参数校验该怎么返回错误码。
	// NewFromContext 会按请求的 Accept-Language 选择错误文案（见 pkg/errors）。
	if len([]rune(name)) > 64 {
		return nil, bizerr.NewFromContext(l.ctx, bizerr.CodeInvalidParam,
			bizerr.Newf(bizerr.CodeInvalidParam, "name 长度 %d 超过上限 64", len([]rune(name))))
	}

	return &types.HelloResp{
		Message: "hello, " + name,
		Time:    time.Now().Format(time.RFC3339),
	}, nil
}

// Package demo 是 demo 接口的业务逻辑，演示完整的 CRUD + 数据库读写。
//
// 这是脚手架自证用的示例，业务开工后连同 handler/repo/model/migration 一起删除。
package demo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	bizerr "charon/pkg/errors"

	"charon/internal/model"
	"charon/internal/repo"
	"charon/internal/svc"
	"charon/internal/types"
)

const (
	// maxNameLen 是 name 的上限，与数据库 varchar(64) 保持一致。
	// 在 logic 层先挡掉，避免把超长数据丢给数据库才报错。
	maxNameLen = 64
)

// DemoLogic 实现 demo 相关接口。
type DemoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// NewDemoLogic 由 handler 构造。
func NewDemoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DemoLogic {
	return &DemoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Create 新增一条记录。
func (l *DemoLogic) Create(req *types.CreateDemoReq) (*types.CreateDemoResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, l.invalidParam("name 不能为空")
	}
	if len([]rune(name)) > maxNameLen {
		return nil, l.invalidParam("name 长度超过上限 %d", maxNameLen)
	}

	repo, err := l.repo()
	if err != nil {
		return nil, err
	}

	id, err := repo.Insert(l.ctx, name)
	if err != nil {
		// 这里不把 err 直接返回给客户端：数据库错误里可能带表名、SQL 片段等
		// 内部信息。用 CodeInternal 包装后，细节只进日志，客户端拿到的是通用文案。
		return nil, bizerr.NewFromContext(l.ctx, bizerr.CodeInternal, err)
	}

	l.Infof("demo_item created: id=%d name=%s", id, name)

	return &types.CreateDemoResp{ID: id}, nil
}

// Get 按 id 查一条。
func (l *DemoLogic) Get(req *types.GetDemoReq) (*types.DemoItem, error) {
	if req.ID <= 0 {
		return nil, l.invalidParam("id 必须为正整数")
	}

	repo, err := l.repo()
	if err != nil {
		return nil, err
	}

	item, err := repo.FindOne(l.ctx, req.ID)
	if err != nil {
		// 把「查不到」翻译成 404。repo 层不知道对外契约，
		// 这个翻译是 logic 的职责——将来对外改成 400 或返回空对象只改这里。
		if errors.Is(err, sql.ErrNoRows) {
			return nil, bizerr.NewFromContext(l.ctx, bizerr.CodeNotFound, err)
		}
		return nil, bizerr.NewFromContext(l.ctx, bizerr.CodeInternal, err)
	}

	return toType(item), nil
}

// List 分页查询。
func (l *DemoLogic) List(req *types.ListDemoReq) (*types.PageResult, error) {
	page, pageSize := normalizePaging(req.Page, req.PageSize)

	repo, err := l.repo()
	if err != nil {
		return nil, err
	}

	items, total, err := repo.List(l.ctx, page, pageSize)
	if err != nil {
		return nil, bizerr.NewFromContext(l.ctx, bizerr.CodeInternal, err)
	}

	// 即使没有数据也要返回空数组而不是 null，前端少一个判空分支。
	list := make([]types.DemoItem, 0, len(items))
	for i := range items {
		list = append(list, *toType(&items[i]))
	}

	return &types.PageResult{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Delete 按 id 删除。
func (l *DemoLogic) Delete(req *types.DeleteDemoReq) error {
	if req.ID <= 0 {
		return l.invalidParam("id 必须为正整数")
	}

	repo, err := l.repo()
	if err != nil {
		return err
	}

	affected, err := repo.Delete(l.ctx, req.ID)
	if err != nil {
		return bizerr.NewFromContext(l.ctx, bizerr.CodeInternal, err)
	}

	// 删一个不存在的 id 视为 404 而不是静默成功：
	// 让调用方知道它删的东西本来就不存在，避免掩盖 id 传错的问题。
	if affected == 0 {
		return bizerr.NewFromContext(l.ctx, bizerr.CodeNotFound,
			bizerr.Newf(bizerr.CodeNotFound, "demo_item id=%d 不存在", req.ID))
	}

	l.Infof("demo_item deleted: id=%d", req.ID)

	return nil
}

// repo 取出 repo，并在数据库未配置时返回明确的 503。
//
// 这个方法是为了把「没配数据库」这件事表达清楚：
// 否则直接访问 l.svcCtx.DemoItemRepo 会 nil pointer panic，
// 排查时要翻代码才能明白原因。
func (l *DemoLogic) repo() (*repo.DemoItemRepo, error) {
	if l.svcCtx.DemoItemRepo == nil {
		return nil, bizerr.NewFromContext(l.ctx, bizerr.CodeCommonUnavailable,
			bizerr.Newf(bizerr.CodeCommonUnavailable,
				"该实例未配置 DB.DataSource，demo 接口不可用"))
	}
	return l.svcCtx.DemoItemRepo, nil
}

// invalidParam 是构造参数错误的辅助方法，省掉每处重复写 ctx。
func (l *DemoLogic) invalidParam(format string, args ...any) error {
	return bizerr.NewFromContext(l.ctx, bizerr.CodeInvalidParam,
		bizerr.Newf(bizerr.CodeInvalidParam, format, args...))
}

// normalizePaging 兜底分页参数。
//
// 已经在 .api 里用 range 声明了约束，但 logic 不应假设调用方一定校验过
// （比如将来被 gRPC 或定时任务复用）。这里再夹一次。
func normalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// toType 把数据库模型转成对外的响应结构。
func toType(item *model.DemoItem) *types.DemoItem {
	return &types.DemoItem{
		ID:        item.ID,
		Name:      item.Name,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
	}
}

// Package repo 是数据访问层，负责拼 SQL 与读写。
//
// 约定：
//   - 只做「取/存」，不含业务判断（比如「名字是否重复」属于 logic）。
//   - 只返回原始错误；是否算 NotFound、要不要包装成业务错误码，由 logic 决定。
//     这样 repo 不知道对外契约，改动对外错误码时不需要动数据层。
package repo

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"charon/internal/model"
)

// DemoItemRepo 是 demo_item 表的读写入口。
type DemoItemRepo struct {
	conn sqlx.SqlConn
}

// NewDemoItemRepo 构造 repo。
func NewDemoItemRepo(conn sqlx.SqlConn) *DemoItemRepo {
	return &DemoItemRepo{conn: conn}
}

// Insert 插入一条记录并返回自增 id。
//
// 用 RETURNING id 而不是 LastInsertId：PostgreSQL 的 database/sql 驱动
// 不保证支持 LastInsertId，RETURNING 是 PG 的原生做法且是一次往返。
func (r *DemoItemRepo) Insert(ctx context.Context, name string) (int64, error) {
	const query = `INSERT INTO demo_item (name) VALUES ($1) RETURNING id`

	var id int64
	if err := r.conn.QueryRowCtx(ctx, &id, query, name); err != nil {
		return 0, err
	}

	return id, nil
}

// FindOne 按 id 查一条。记录不存在时返回 sqlx.ErrNotFound（即 sql.ErrNoRows）。
func (r *DemoItemRepo) FindOne(ctx context.Context, id int64) (*model.DemoItem, error) {
	const query = `SELECT id, name, created_at FROM demo_item WHERE id = $1`

	var item model.DemoItem
	if err := r.conn.QueryRowCtx(ctx, &item, query, id); err != nil {
		return nil, err
	}

	return &item, nil
}

// List 分页查询，返回当前页数据与总条数。
//
// 两次查询没有放在同一个事务里：列表和计数允许有微小的时间差，
// 为这个一致性去开事务不值得。若业务要求强一致，再改用 TransactCtx。
func (r *DemoItemRepo) List(ctx context.Context, page, pageSize int) ([]model.DemoItem, int64, error) {
	const (
		listQuery  = `SELECT id, name, created_at FROM demo_item ORDER BY id DESC LIMIT $1 OFFSET $2`
		countQuery = `SELECT COUNT(*) FROM demo_item`
	)

	var items []model.DemoItem
	if err := r.conn.QueryRowsCtx(ctx, &items, listQuery, pageSize, (page-1)*pageSize); err != nil {
		return nil, 0, err
	}

	var total int64
	if err := r.conn.QueryRowCtx(ctx, &total, countQuery); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Delete 按 id 删除，返回被删除的行数。
// 返回 0 表示该 id 不存在，由调用方决定这算不算错误。
func (r *DemoItemRepo) Delete(ctx context.Context, id int64) (int64, error) {
	const query = `DELETE FROM demo_item WHERE id = $1`

	result, err := r.conn.ExecCtx(ctx, query, id)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

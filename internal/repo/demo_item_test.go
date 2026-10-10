package repo

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// newMockRepo 用 sqlmock 替换真实数据库。
//
// 这样测的是「SQL 语句与扫描映射是否正确」，不依赖真实 PG 实例。
// 真实连库的端到端验证见 README.md 的验证清单——那部分需要能跑 Docker 的环境。
func newMockRepo(t *testing.T) (*DemoItemRepo, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("创建 mock 失败: %v", err)
	}

	// sqlx.NewSqlConnFromDB 让我们能把已有的 *sql.DB 包成 sqlx 连接，
	// 因此 repo 代码完全不用为可测性做改动。
	return NewDemoItemRepo(sqlx.NewSqlConnFromDB(db)), mock
}

func TestInsertUsesReturningID(t *testing.T) {
	repo, mock := newMockRepo(t)

	// 断言 SQL 用的是 PG 的 RETURNING 而不是依赖 LastInsertId。
	mock.ExpectQuery(`INSERT INTO demo_item \(name\) VALUES \(\$1\) RETURNING id`).
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))

	id, err := repo.Insert(context.Background(), "alice")
	if err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestFindOneMapsColumns(t *testing.T) {
	repo, mock := newMockRepo(t)

	rows := sqlmock.NewRows([]string{"id", "name", "created_at"}).
		AddRow(int64(7), "bob", mustParseTime(t, "2026-01-02T03:04:05Z"))

	mock.ExpectQuery(`SELECT id, name, created_at FROM demo_item WHERE id = \$1`).
		WithArgs(int64(7)).
		WillReturnRows(rows)

	item, err := repo.FindOne(context.Background(), 7)
	if err != nil {
		t.Fatalf("FindOne 失败: %v", err)
	}

	if item.ID != 7 || item.Name != "bob" {
		t.Errorf("扫描结果错误: %+v", item)
	}
	if item.CreatedAt.IsZero() {
		t.Error("created_at 未正确映射到 time.Time")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// 查不到时必须返回 sql.ErrNoRows，logic 层依赖它来翻译成 404。
// 这个契约一旦破坏，404 会静默变成 500。
func TestFindOneNotFoundReturnsErrNoRows(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectQuery(`SELECT id, name, created_at FROM demo_item WHERE id = \$1`).
		WithArgs(int64(999)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}))

	_, err := repo.FindOne(context.Background(), 999)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("err = %v, want sql.ErrNoRows", err)
	}
}

// 分页偏移量必须是 (page-1)*pageSize，算错会导致翻页重复或漏数据。
func TestListComputesOffset(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectQuery(`SELECT id, name, created_at FROM demo_item ORDER BY id DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 20). // page=2, pageSize=20 → offset=20
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}).
			AddRow(int64(1), "x", mustParseTime(t, "2026-01-02T03:04:05Z")))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM demo_item`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(99)))

	items, total, err := repo.List(context.Background(), 2, 20)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("返回 %d 条, want 1", len(items))
	}
	if total != 99 {
		t.Errorf("total = %d, want 99", total)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// 空结果必须返回可用的空切片语义，交由 logic 转成 [] 而不是 null。
func TestListEmptyResult(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectQuery(`SELECT id, name, created_at FROM demo_item ORDER BY id DESC LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM demo_item`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	items, total, err := repo.List(context.Background(), 1, 20)
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(items) != 0 || total != 0 {
		t.Errorf("items=%d total=%d, want 0/0", len(items), total)
	}
}

func TestDeleteReturnsAffectedRows(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectExec(`DELETE FROM demo_item WHERE id = \$1`).
		WithArgs(int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	affected, err := repo.Delete(context.Background(), 3)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if affected != 1 {
		t.Errorf("affected = %d, want 1", affected)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// 删不存在的 id 返回 0，由 logic 决定算 404。repo 不应自作主张报错。
func TestDeleteMissingReturnsZero(t *testing.T) {
	repo, mock := newMockRepo(t)

	mock.ExpectExec(`DELETE FROM demo_item WHERE id = \$1`).
		WithArgs(int64(404)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	affected, err := repo.Delete(context.Background(), 404)
	if err != nil {
		t.Fatalf("Delete 不应报错: %v", err)
	}
	if affected != 0 {
		t.Errorf("affected = %d, want 0", affected)
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()

	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("解析时间失败: %v", err)
	}
	return tm
}

// Package svc 负责「装配」：进程启动时把所有依赖（配置、数据库连接、各个 repo）
// 创建好，放进 ServiceContext，再交给每个 logic 使用。
//
// 它相当于依赖注入的容器 + 生命周期管理的集中点。
// logic 本身不要自己 new 数据库连接——那样连接池会失控，也无法在测试里替换。
package svc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	// 注册 pgx 驱动。下划线导入是因为我们只用它的副作用：
	// 它会在 init() 里调用 sql.Register("pgx", ...)，
	// 之后 sqlx.NewSqlConn("pgx", dsn) 才能找到驱动。
	_ "github.com/jackc/pgx/v5/stdlib"

	"charon/internal/config"
	"charon/internal/repo"
)

// ServiceContext 是所有 logic 共享的依赖集合。
//
// 每个 logic 的构造函数都会收到它，需要什么取什么。
type ServiceContext struct {
	Config config.Config

	// DB 是数据库连接池。Config.DB 未配置时为 nil。
	// 注意：只有配置了 DataSource 时它才非 nil，
	// 所以用到 DB 的 logic 必须自己处理「DB 不可用」的情况（见 DemoItemRepo 的用法）。
	DB sqlx.SqlConn

	// DemoItemRepo 是 demo_item 表的读写入口。
	// 新增业务时在这里按同样方式挂上自己的 repo。
	DemoItemRepo *repo.DemoItemRepo
}

// NewServiceContext 装配依赖。任何一步失败都会让进程直接退出。
//
// 关于「连不上就启动失败」的取舍：
// 配置了 DataSource 却连不上时，这里不降级、不回退到内存实现，而是直接失败。
// 理由是配置错误若被兜底掩盖，会在生产上表现成「服务健康但数据全空」，
// 这种故障比启动失败难排查得多。详见 README.md 的「数据库」一节。
func NewServiceContext(c config.Config) *ServiceContext {
	ctx := &ServiceContext{Config: c}

	if c.DB.Enabled() {
		ctx.DB = newDB(c)
		ctx.DemoItemRepo = repo.NewDemoItemRepo(ctx.DB)
		logx.Infof("database connected: driver=%s", c.DB.Driver)
	} else {
		// 不配置 DSN 是合法的（比如只想跑通 HTTP 链路时不装数据库），
		// 但必须留下明确日志：否则接口报 nil pointer 时很难看出是没配数据库。
		logx.Info("未配置 DB.DataSource：数据库相关接口将不可用（返回 503）。")
	}

	return ctx
}

// newDB 建立连接池并在启动期做一次真实连通性验证。
func newDB(c config.Config) sqlx.SqlConn {
	conn := sqlx.NewSqlConn(c.DB.Driver, c.DB.DataSource)

	rawDB, err := conn.RawDB()
	if err != nil {
		logx.Must(err)
	}

	// 这些是 database/sql 层面的池参数，go-zero 的 sqlx 把 DB 句柄透传出来，
	// 需要我们自己设。不设的话 MaxOpenConns 默认 0 = 无上限，
	// 流量突增时会把数据库连接打满。
	rawDB.SetMaxOpenConns(c.DB.MaxOpenConns)
	rawDB.SetMaxIdleConns(c.DB.MaxIdleConns)
	rawDB.SetConnMaxLifetime(c.DB.MaxLifetime)

	// 启动期主动 ping 一次。
	//
	// 关键点：一定要在这里验证，不能等第一个请求进来才发现连不上。
	// 这样编排系统（k8s / compose）能立刻看到启动失败并让部署停下，
	// 而不是把一个「起来了但每个请求都 500」的实例放进负载均衡。
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rawDB.PingContext(pingCtx); err != nil {
		logx.Must(fmt.Errorf("数据库无法连接，启动中止（不做内存回退）: %w", err))
	}

	return conn
}

// DBReady 是给就绪探针用的检查函数。DB 未配置时视为就绪。
func (s *ServiceContext) DBReady(ctx context.Context) error {
	if s.DB == nil {
		return nil
	}

	rawDB, err := s.DB.RawDB()
	if err != nil {
		return err
	}

	return pingWithTimeout(ctx, rawDB)
}

// pingWithTimeout 给探针的 ping 加超时，避免探针请求堆积。
func pingWithTimeout(ctx context.Context, db interface {
	PingContext(context.Context) error
}) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return db.PingContext(ctx)
}

// 编译期确认 *sql.DB 满足上面的接口，避免将来 signature 变化时静默失效。
var _ interface {
	PingContext(context.Context) error
} = (*sql.DB)(nil)

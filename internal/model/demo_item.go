// Package model 放数据结构与数据库行结构。
//
// 约定：
//   - 这里的结构体字段与数据库列一一对应，靠 db tag 显式绑定，不依赖命名推断。
//   - model 只描述数据，不含任何业务规则，也不发起查询。
package model

import "time"

// DemoItem 对应 demo_item 表。
//
// 这是脚手架自证用的表，业务开工后应连同 migrations 一起删除。
type DemoItem struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

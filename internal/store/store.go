// Package store 数据层：SQLite 客户端 + 建表迁移 + 种子数据。
//
// 设计（ADR-003：SQLite 起步可迁移 Postgres）：
//   - 驱动用 modernc.org/sqlite（纯 Go，本机无 gcc 不能用 CGO 版）
//   - ent schema 即迁移真相源，--migrate 时自动建表
//   - 种子数据只在没有数据时执行（幂等，可重复跑）
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动

	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
)

// Open 打开/创建 SQLite 库并按 ent schema 建表。
// dbPath 形如 ./data/homeagent.db；目录不存在时自动创建。
func Open(ctx context.Context, dbPath string) (*ent.Client, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	// modernc 注册的驱动名是 "sqlite"（ent 默认 dialect.SQLite="sqlite3" 是 mattn 驱动名）。
	// 先用 database/sql 打开（驱动名 sqlite），再用 ent 包裹并指定 SQLite 方言。
	db, err := sql.Open("sqlite", "file:"+dbPath+"?cache=shared&_fk=1")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))

	if err := client.Schema.Create(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	return client, nil
}

// MustSeed 幂等种子：仅在库为空时写入家庭/成员/分类。
func MustSeed(ctx context.Context, client *ent.Client) error {
	if n, err := client.Family.Query().Count(ctx); err != nil {
		return fmt.Errorf("query family: %w", err)
	} else if n > 0 {
		return nil // 已有数据，跳过
	}

	var seedErr error
	tx, err := client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer func() {
		if seedErr != nil {
			_ = tx.Rollback()
		}
	}()

	fam, err := tx.Family.Create().
		SetName("我的家").
		SetTimezone("Asia/Shanghai").
		SetLunar(true).
		Save(ctx)
	if err != nil {
		seedErr = fmt.Errorf("seed family: %w", err)
		return seedErr
	}

	// 三成员：家长/老人/小孩，权限模板对齐 ADR-005。
	members := []struct {
		name string
		role member.Role
		tmpl member.PermissionTemplate
		perms map[string]bool
	}{
		{"爸爸", member.RoleParent, member.PermissionTemplateAdmin, map[string]bool{
			"expense.write": true, "expense.read": true,
			"task.write": true, "task.read": true,
			"meal.write": true, "system.admin": true,
		}},
		{"奶奶", member.RoleElder, member.PermissionTemplateLimited, map[string]bool{
			"expense.write": true, "expense.read": true,
			"task.read": true, "meal.write": true,
		}},
		{"孩子", member.RoleChild, member.PermissionTemplateChild, map[string]bool{
			"task.read": true, "meal.write": true,
		}},
	}
	for _, m := range members {
		if err := tx.Member.Create().
			SetName(m.name).
			SetRole(m.role).
			SetPermissionTemplate(m.tmpl).
			SetPermissions(m.perms).
			SetFamily(fam).
			Exec(ctx); err != nil {
			seedErr = fmt.Errorf("seed member %s: %w", m.name, err)
			return seedErr
		}
	}

	// 分类账本（对齐 record_expense 工具的 inputSchema 枚举）。
	categories := []struct {
		name   string
		budget int64 // 月度预算（分），0 表示不设
	}{
		{"食材", 300000}, {"日用", 50000}, {"外卖", 80000},
		{"出行", 30000}, {"餐饮", 60000}, {"其他", 20000},
	}
	for i, c := range categories {
		b := tx.Category.Create().
			SetName(c.name).
			SetSortOrder(i).
			SetIsSystem(true)
		if c.budget > 0 {
			b.SetMonthlyBudgetCents(c.budget)
		}
		if err := b.SetFamily(fam).Exec(ctx); err != nil {
			seedErr = fmt.Errorf("seed category %s: %w", c.name, err)
			return seedErr
		}
	}

	if err := tx.Commit(); err != nil {
		seedErr = fmt.Errorf("commit seed: %w", err)
		return seedErr
	}
	return nil
}

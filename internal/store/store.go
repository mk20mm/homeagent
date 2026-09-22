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
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动

	"github.com/google/uuid"

	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/llmprovider"
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
	// _timezone=UTC：驱动在绑定时间参数前统一转 UTC——SQLite 的时间列是 NUMERIC 亲和性，
	// 混存「Z」与「+08:00」时字符串比较与排序全错（账本倒序错位、游标跳页）。写入侧统一
	// UTC 后字符串比较 == 时刻比较，配合 store.UseUTCTimes hook 双保险。
	db, err := sql.Open("sqlite", "file:"+dbPath+"?cache=shared&_fk=1&_timezone=UTC")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))

	// 历史数据兜底：早期写入未统一时区，库里混存了「Z」和「+08:00」两种字符串。
	// 幂等（已统一的行被 WHERE 拦下），跑过一次后只是几次空扫描。
	if err := normalizeTimes(ctx, db); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("normalize times: %w", err)
	}
	// 写入侧统一 UTC（见 UseUTCTimes 注释）
	client.Use(UseUTCTimes)

	if err := client.Schema.Create(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	return client, nil
}

// UseUTCTimes ent hook：所有时间字段入库前统一转 UTC。
//
// 背景：ent 把 time.Time 按 RFC3339 连同值自身的时区偏移格式化后交给 SQLite，
// 而 SQLite 的时间比较是**字符串比较**。于是同一时刻在库里会有「Z」和「+08:00」
// 两种写法，两者之间的范围查询与排序全部错位——任务到期扫描漏行、账本按日倒序
// 排错、游标分页跳页，都是这一类 bug。写入侧统一 UTC 后，字符串比较 == 时刻比较。
// 查询侧的边界值也要同步转 UTC（见 repo 各 ListDueSoon/ListExpenses/ListActive）。
func UseUTCTimes(next ent.Mutator) ent.Mutator {
	return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
		for _, f := range m.Fields() {
			v, ok := m.Field(f)
			if !ok {
				continue
			}
			if t, ok := v.(time.Time); ok && t.Location() != time.UTC {
				_ = m.SetField(f, t.UTC())
			}
		}
		return next.Mutate(ctx, m)
	})
}

// canonicalUTCSuffix 是 modernc 驱动写入 UTC 时间的 t.String() 格式后缀
// （见 conn.formatTime：默认 writeTimeFormat 为空时用 t.String()）。
// 写入侧、查询参数、本迁移三者的格式必须一致，否则 SQLite 的字符串比较会错位。
const canonicalUTCSuffix = " +0000 UTC"

// parseFormats 覆盖库里可能出现的所有时间字符串写法：
//   - RFC3339Nano / RFC3339（「T」分隔，历史数据与 API 出入口）
//   - t.String()（空格分隔，modernc 驱动绑定的默认格式）
var parseFormats = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05.999999999-07:00",
}

func parseTimeAny(s string) (time.Time, bool) {
	for _, f := range parseFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// dedupeBeforeNormalize 删除时间列归一化会破坏的重复行。
//
// 背景：唯一约束里有时间列时（如 notifications 的 type+ref_id+scheduled_at+member），
// 历史数据里同一逻辑记录可能因时区写法不同存成两行（「Z」与「+08:00」字符串不同，
// 约束没拦住）。归一化把两行改成同一 UTC 字符串后反而触发唯一约束冲突，服务起不来。
//
// 做法：不能按存储字符串分组（两行字符串本就不同）。把唯一键里的时间列替换成
// 「解析后的规范 UTC 字符串」做 GROUP BY——即模拟归一化后的值来判定重复，
// 保留每组 rowid 最小的一行。删完再跑归一化就不会撞约束。
//
// 唯一键取自 sqlite_master 的 CREATE TABLE；解析失败（如表达式索引）就跳过该表。
func dedupeBeforeNormalize(ctx context.Context, db *sql.DB, table string, timeCols []string) (int, error) {
	keys := uniqueKeys(ctx, db, table)
	if len(keys) == 0 {
		return 0, nil
	}
	timeSet := make(map[string]bool, len(timeCols))
	for _, c := range timeCols {
		timeSet[c] = true
	}

	// SQLite 没法在 SQL 里把「Z」与「+08:00」判等，先把每列的规范值读进内存。
	// 表通常很小（通知/报饭这类），整表读取消耗可控。
	normalized := make(map[string]map[int64]string, len(timeCols)) // col -> rowid -> 规范值
	for _, key := range keys {
		for _, col := range key {
			if !timeSet[col] || normalized[col] != nil {
				continue
			}
			vals := make(map[int64]string)
			rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT rowid, %q FROM %q`, col, table))
			if err != nil {
				return 0, fmt.Errorf("read %s.%s: %w", table, col, err)
			}
			for rows.Next() {
				var rowid int64
				var raw []byte
				if err := rows.Scan(&rowid, &raw); err != nil {
					_ = rows.Close()
					return 0, fmt.Errorf("scan %s.%s: %w", table, col, err)
				}
				s := string(raw)
				if s == "" {
					continue
				}
				if t, ok := parseTimeAny(s); ok {
					vals[rowid] = t.UTC().Format(time.RFC3339Nano)
				} else {
					vals[rowid] = s // 无法识别，保留原值
				}
			}
			_ = rows.Close()
			normalized[col] = vals
		}
	}

	deleted := 0
	for _, key := range keys {
		// 用归一化后的值拼分组键；无时间列的键跳过（不归一化就不会产生新重复）。
		hasTime := false
		for _, col := range key {
			if normalized[col] != nil {
				hasTime = true
				break
			}
		}
		if !hasTime {
			continue
		}

		// 取全部行，在 Go 里按归一化值分组，保留每组最小 rowid。
		cols := make([]string, len(key))
		for i, c := range key {
			cols[i] = fmt.Sprintf("%q", c)
		}
		rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT rowid, %s FROM %q`, strings.Join(cols, ", "), table))
		if err != nil {
			return deleted, fmt.Errorf("dedupe read %s: %w", table, err)
		}
		type rowVals struct {
			rowid int64
			vals  []any
		}
		var all []rowVals
		for rows.Next() {
			var rowid int64
			vals := make([][]byte, len(key))
			ptrs := make([]any, len(key))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(append([]any{&rowid}, ptrs...)...); err != nil {
				_ = rows.Close()
				return deleted, fmt.Errorf("dedupe scan %s: %w", table, err)
			}
			cv := make([]any, len(key))
			for i, col := range key {
				if nv, ok := normalized[col]; ok {
					if v, ok := nv[rowid]; ok {
						cv[i] = v
						continue
					}
				}
				cv[i] = string(vals[i])
			}
			all = append(all, rowVals{rowid, cv})
		}
		_ = rows.Close()

		keep := make(map[string]int64, len(all))
		for _, r := range all {
			k := fmt.Sprintf("%v", r.vals)
			if cur, ok := keep[k]; !ok || r.rowid < cur {
				keep[k] = r.rowid
			}
		}
		var drop []int64
		for _, r := range all {
			if keep[fmt.Sprintf("%v", r.vals)] != r.rowid {
				drop = append(drop, r.rowid)
			}
		}
		for _, rowid := range drop {
			res, err := db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %q WHERE rowid = ?`, table), rowid)
			if err != nil {
				return deleted, fmt.Errorf("dedupe delete %s row %d: %w", table, rowid, err)
			}
			n, _ := res.RowsAffected()
			deleted += int(n)
		}
	}
	return deleted, nil
}

// uniqueKeys 从表的唯一约束里解析唯一键：建表语句里的 PRIMARY KEY(...)/UNIQUE(...)，
// 以及 ent 用 .Unique() 生成的独立唯一索引 CREATE UNIQUE INDEX ... ON t (cols)。
// 含表达式的键（如 UNIQUE(lower(x))）返回 nil，让整表跳过。
func uniqueKeys(ctx context.Context, db *sql.DB, table string) [][]string {
	var keys [][]string

	// 1) 建表语句里的表级约束
	var createSQL string
	if err := db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&createSQL); err == nil {
		keys = append(keys, tableLevelKeys(createSQL)...)
	}

	// 2) 独立唯一索引（ent 的 .Unique() 生成这种）
	rows, err := db.QueryContext(ctx, `SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name=? AND sql LIKE 'CREATE UNIQUE%'`, table)
	if err != nil {
		return keys
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var idxSQL string
		if err := rows.Scan(&idxSQL); err != nil {
			continue
		}
		if k := indexKeys(idxSQL); k != nil {
			keys = append(keys, k)
		}
	}
	return keys
}

// tableLevelKeys 解析建表语句里的表级 PRIMARY KEY(...) / UNIQUE(...)。
func tableLevelKeys(createSQL string) [][]string {
	open := strings.Index(createSQL, "(")
	close := strings.LastIndex(createSQL, ")")
	if open < 0 || close <= open {
		return nil
	}
	body := createSQL[open+1 : close]

	// 按深度 0 的逗号切分列/约束定义
	var defs []string
	depth := 0
	start := 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				defs = append(defs, body[start:i])
				start = i + 1
			}
		}
	}
	defs = append(defs, body[start:])

	var keys [][]string
	for _, def := range defs {
		trimmed := strings.TrimSpace(def)
		upper := strings.ToUpper(trimmed)
		if !strings.HasPrefix(upper, "PRIMARY KEY") && !strings.HasPrefix(upper, "UNIQUE") {
			continue
		}
		parenOpen := strings.Index(trimmed, "(")
		parenClose := strings.LastIndex(trimmed, ")")
		if parenOpen < 0 || parenClose <= parenOpen {
			continue // 列级约束（无括号），跳过
		}
		if k := splitKeyList(trimmed[parenOpen+1 : parenClose]); k != nil {
			keys = append(keys, k)
		}
	}
	return keys
}

// indexKeys 解析 CREATE UNIQUE INDEX ... ON t (cols) 的列列表。
func indexKeys(idxSQL string) []string {
	on := strings.Index(strings.ToUpper(idxSQL), " ON ")
	if on < 0 {
		return nil
	}
	rest := idxSQL[on+4:]
	open := strings.Index(rest, "(")
	close := strings.Index(rest, ")")
	if open < 0 || close <= open {
		return nil
	}
	return splitKeyList(rest[open+1 : close])
}

// splitKeyList 把 "a, b" 切成 [a b]；含括号/空格的表达式键返回 nil。
func splitKeyList(inner string) []string {
	parts := strings.Split(inner, ",")
	cols := make([]string, 0, len(parts))
	for _, p := range parts {
		c := strings.Trim(p, "` \"'\t\n\r")
		if c == "" || strings.ContainsAny(c, "()") {
			return nil // 表达式键，整表跳过
		}
		cols = append(cols, c)
	}
	if len(cols) == 0 {
		return nil
	}
	return cols
}

// normalizeTimes 把库里的时间字符串统一成驱动的规范 UTC 格式。
//
// 关键：重写时传 time.Time 参数（而非手写格式化字符串），让 modernc 用与
// ent 写入完全相同的 formatTime 路径格式化——这样「迁移写的」和「业务写的」
// 字符串逐字节一致，SQLite 的字符串比较 == 时刻比较。早先用 RFC3339Nano
// 重写，与驱动的空格格式（' '(0x20) < 'T'(0x54)）混存，新数据永远沉到
// 旧数据下面，账本倒序与游标分页全错。
//
// 幂等：已是规范 UTC 后缀的行被 WHERE 拦下；无法识别的格式不动。
func normalizeTimes(ctx context.Context, db *sql.DB) error {
	tables, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return err
	}
	var names []string
	for tables.Next() {
		var n string
		if err := tables.Scan(&n); err != nil {
			_ = tables.Close()
			return err
		}
		names = append(names, n)
	}
	_ = tables.Close()

	for _, table := range names {
		cols, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%q)`, table))
		if err != nil {
			return err
		}
		var timeCols []string
		for cols.Next() {
			var cid, notnull, pk int
			var name, typ string
			var dflt any
			if err := cols.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				_ = cols.Close()
				return err
			}
			upper := strings.ToUpper(typ)
			if strings.Contains(upper, "DATE") || strings.Contains(upper, "TIME") {
				timeCols = append(timeCols, name)
			}
		}
		_ = cols.Close()
		if len(timeCols) == 0 {
			continue
		}

		// 先去重：唯一键含时间列时，历史重复行会让归一化撞约束（见 dedupeBeforeNormalize）。
		if n, err := dedupeBeforeNormalize(ctx, db, table, timeCols); err != nil {
			return err
		} else if n > 0 {
			slog.Info("deduplicated rows before normalizing times", "table", table, "rows", n)
		}

		// 读出全部行的时间列，解析后只重写非规范的。
		// 必须扫描成 []byte：modernc 把 datetime 列扫描进 string 时会自动
		// 转成「T」格式，拿到的不是库里的原始字节，规范格式判定也就失效。
		selectSQL := fmt.Sprintf(`SELECT rowid, %s FROM %q`, quoteList(timeCols), table)
		rows, err := db.QueryContext(ctx, selectSQL)
		if err != nil {
			return fmt.Errorf("scan %s: %w", table, err)
		}
		type pending struct {
			rowid int64
			vals  map[string]time.Time
		}
		var todo []pending
		for rows.Next() {
			var rowid int64
			vals := make([][]byte, len(timeCols))
			ptrs := make([]any, len(timeCols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(append([]any{&rowid}, ptrs...)...); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan %s row: %w", table, err)
			}
			changes := make(map[string]time.Time)
			for i, col := range timeCols {
				s := string(vals[i]) // NULL → "" 跳过
				if s == "" || strings.HasSuffix(s, canonicalUTCSuffix) {
					continue // 已是规范 UTC 格式
				}
				t, ok := parseTimeAny(s)
				if !ok {
					continue // 无法识别的格式不动它
				}
				changes[col] = t.UTC()
			}
			if len(changes) > 0 {
				todo = append(todo, pending{rowid: rowid, vals: changes})
			}
		}
		_ = rows.Close()

		for _, p := range todo {
			setClauses := make([]string, 0, len(p.vals))
			args := make([]any, 0, len(p.vals)+1)
			for col, v := range p.vals {
				setClauses = append(setClauses, fmt.Sprintf(`%q = ?`, col))
				// 传 time.Time：走驱动的 formatTime，与 ent 写入逐字节同格式。
				args = append(args, v)
			}
			args = append(args, p.rowid)
			q := fmt.Sprintf(`UPDATE %q SET %s WHERE rowid = ?`, table, strings.Join(setClauses, ", "))
			if _, err := db.ExecContext(ctx, q, args...); err != nil {
				return fmt.Errorf("normalize %s row %d: %w", table, p.rowid, err)
			}
		}
		if len(todo) > 0 {
			slog.Info("normalized timestamps to UTC", "table", table, "rows", len(todo))
		}
	}
	return nil
}

// quoteList 把列名拼成 `a, b, c`（已带引号）。
func quoteList(cols []string) string {
	quoted := make([]string, 0, len(cols))
	for _, c := range cols {
		quoted = append(quoted, fmt.Sprintf(`%q`, c))
	}
	return strings.Join(quoted, ", ")
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
	// auth_token 为开发期可预测值（dev-<名字>），--migrate 时打印给本地联调用；
	// 生产应由管理端生成随机串并加密存储（技术债：auth_token 明文）。
	members := []struct {
		name      string
		role      member.Role
		tmpl      member.PermissionTemplate
		perms     map[string]bool
		authToken string
	}{
		{"爸爸", member.RoleParent, member.PermissionTemplateAdmin, map[string]bool{
			"expense.write": true, "expense.read": true,
			"task.write": true, "task.read": true,
			"meal.write": true, "system.admin": true,
		}, "dev-baba"},
		{"奶奶", member.RoleElder, member.PermissionTemplateLimited, map[string]bool{
			"expense.write": true, "expense.read": true,
			"task.read": true, "meal.write": true,
		}, "dev-nainai"},
		{"孩子", member.RoleChild, member.PermissionTemplateChild, map[string]bool{
			"task.read": true, "meal.write": true,
		}, "dev-haizi"},
	}
	for _, m := range members {
		if err := tx.Member.Create().
			SetName(m.name).
			SetRole(m.role).
			SetPermissionTemplate(m.tmpl).
			SetPermissions(m.perms).
			SetAuthToken(m.authToken).
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

	// LLM 供应商与模型清单（P1 会话内切换用；api_key 留空，由环境变量注入）
	providerIDs := map[string]string{}
	for _, p := range []struct {
		name    llmprovider.Name
		baseURL string
	}{
		{llmprovider.NameDeepseek, "https://api.deepseek.com"},
		{llmprovider.NameOpenai, ""},
	} {
		prov, err := tx.LLMProvider.Create().
			SetName(p.name).
			SetBaseURL(p.baseURL).
			SetEnabled(true).
			Save(ctx)
		if err != nil {
			seedErr = fmt.Errorf("seed provider %s: %w", p.name, err)
			return seedErr
		}
		providerIDs[string(p.name)] = prov.ID.String()
	}
	for _, m := range []struct {
		name, display, provider string
		isDefault               bool
	}{
		{"deepseek-chat", "DeepSeek 对话", "deepseek", true},
		{"gpt-4o", "GPT-4o", "openai", false},
	} {
		b := tx.LLMModel.Create().
			SetModelName(m.name).
			SetDisplayName(m.display).
			SetEnabled(true).
			SetProviderID(uuid.MustParse(providerIDs[m.provider]))
		if m.isDefault {
			b.SetIsDefault(true)
		}
		if err := b.Exec(ctx); err != nil {
			seedErr = fmt.Errorf("seed model %s: %w", m.name, err)
			return seedErr
		}
	}

	if err := tx.Commit(); err != nil {
		seedErr = fmt.Errorf("commit seed: %w", err)
		return seedErr
	}
	return nil
}

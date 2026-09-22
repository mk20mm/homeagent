package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestNormalizeTimesUnifiesMixedFormats 回归：库里混存「T」分隔（历史迁移写的）
// 与空格分隔（modernc 驱动 t.String() 写的）两种时间格式时，SQLite 的字符串
// 比较会把较新的空格格式行压到较旧的 T 格式行下面，账本倒序与游标分页全错。
func TestNormalizeTimesUnifiesMixedFormats(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "norm.db") + "?cache=shared&_fk=1&_timezone=UTC"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE expenses (id INTEGER PRIMARY KEY, occurred_at datetime NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// 旧行：T 格式（历史遗留写法），时刻较旧
	const legacy = "2026-09-21T07:14:46.9928016Z"
	if _, err := db.ExecContext(ctx, `INSERT INTO expenses (id, occurred_at) VALUES (1, ?)`, legacy); err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	// 新行：走驱动绑定（业务写入路径），时刻较新
	newer := time.Date(2026, 9, 21, 11, 12, 50, 0, time.UTC)
	if _, err := db.ExecContext(ctx, `INSERT INTO expenses (id, occurred_at) VALUES (2, ?)`, newer); err != nil {
		t.Fatalf("insert newer: %v", err)
	}

	// 修复前：'T'(0x54) > ' '(0x20)，较旧的 T 格式行排到最前，较新行被埋
	var top string
	if err := db.QueryRowContext(ctx, `SELECT occurred_at FROM expenses ORDER BY occurred_at DESC LIMIT 1`).Scan(&top); err != nil {
		t.Fatalf("query before normalize: %v", err)
	}
	if !strings.HasPrefix(top, "2026-09-21T07") {
		t.Fatalf("修复前较旧的 T 格式行应被错排到最前，got %q", top)
	}

	if err := normalizeTimes(ctx, db); err != nil {
		t.Fatalf("normalizeTimes: %v", err)
	}

	// 修复后：两行同为驱动规范格式，较新行回到最前
	if err := db.QueryRowContext(ctx, `SELECT occurred_at FROM expenses ORDER BY occurred_at DESC LIMIT 1`).Scan(&top); err != nil {
		t.Fatalf("query after normalize: %v", err)
	}
	if !strings.HasPrefix(top, "2026-09-21T11:12:50") {
		t.Fatalf("修复后较新行应排在最前，got %q", top)
	}

	// 存储的是原始字节是驱动的规范格式（扫描进 string 会被驱动转成「T」格式，
	// 无法据此判定，用 SQL 侧 hex 看真相）
	var hex1 string
	if err := db.QueryRowContext(ctx, `SELECT hex(occurred_at) FROM expenses ORDER BY occurred_at DESC LIMIT 1`).Scan(&hex1); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(decodeHex(t, hex1), canonicalUTCSuffix) {
		t.Fatalf("存储值应为驱动规范 UTC 格式，raw=%q", decodeHex(t, hex1))
	}

	// 查询参数与存储值同格式：按时刻等值查询应命中
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM expenses WHERE occurred_at = ?`, newer).Scan(&count); err != nil {
		t.Fatalf("query by time: %v", err)
	}
	if count != 1 {
		t.Fatalf("按时刻等值查询应命中 1 行，got %d", count)
	}

	// 幂等：再跑一次不应改写任何行（已在规范格式）
	if err := normalizeTimes(ctx, db); err != nil {
		t.Fatalf("normalizeTimes idempotent: %v", err)
	}
	var hex2 string
	if err := db.QueryRowContext(ctx, `SELECT hex(occurred_at) FROM expenses ORDER BY occurred_at DESC LIMIT 1`).Scan(&hex2); err != nil {
		t.Fatal(err)
	}
	if hex1 != hex2 {
		t.Fatalf("幂等重跑不应改写存储值\nbefore=%q\nafter =%q", decodeHex(t, hex1), decodeHex(t, hex2))
	}
}

// TestDedupeBeforeNormalize 回归：唯一键含时间列时，历史数据里同一逻辑记录因
// 时区写法不同存成两行（「Z」与「+08:00」），归一化后撞唯一约束、服务起不来。
// dedupeBeforeNormalize 保留 rowid 最小的那条，让归一化能过。
func TestDedupeBeforeNormalize(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "dedupe.db") + "?cache=shared&_fk=1&_timezone=UTC"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	const table = "notifications"
	if _, err := db.ExecContext(ctx, `CREATE TABLE `+table+` (
		id INTEGER PRIMARY KEY,
		type TEXT NOT NULL,
		ref_id TEXT NOT NULL,
		scheduled_at datetime NOT NULL,
		member_id TEXT NOT NULL,
		UNIQUE (type, ref_id, scheduled_at, member_id)
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	insert := func(id int, scheduled string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, `INSERT INTO `+table+` (id, type, ref_id, scheduled_at, member_id) VALUES (?, 'task_due', 't1', ?, 'm1')`, id, scheduled); err != nil {
			t.Fatalf("insert %d: %v", id, err)
		}
	}
	insert(1, "2026-09-20T16:00:00Z")      // 最早写入，保留
	insert(2, "2026-09-21T00:00:00+08:00")  // 同一时刻（16:00Z）的 +08:00 写法，重复，应删
	insert(3, "2026-09-21T16:00:00Z")      // 不同记录，保留

	n, err := dedupeBeforeNormalize(ctx, db, table, []string{"scheduled_at"})
	if err != nil {
		t.Fatalf("dedupeBeforeNormalize: %v", err)
	}
	if n != 1 {
		t.Fatalf("应删 1 行重复，got %d", n)
	}

	var ids []int
	rows, err := db.QueryContext(ctx, `SELECT id FROM `+table+` ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 3 {
		t.Fatalf("应保留 id 1 和 3，got %v", ids)
	}

	// 归一化现在能跑过（不再撞约束）
	if err := normalizeTimes(ctx, db); err != nil {
		t.Fatalf("normalizeTimes after dedupe: %v", err)
	}
	// 幂等：再去重一次应无变化
	if n, err := dedupeBeforeNormalize(ctx, db, table, []string{"scheduled_at"}); err != nil || n != 0 {
		t.Fatalf("幂等去重应 0 行，got %d err=%v", n, err)
	}
}

func decodeHex(t *testing.T, h string) string {	t.Helper()
	out := make([]byte, len(h)/2)
	for i := range out {
		var b byte
		for j := 0; j < 2; j++ {
			c := h[i*2+j]
			switch {
			case c >= '0' && c <= '9':
				b = b<<4 | (c - '0')
			case c >= 'a' && c <= 'f':
				b = b<<4 | (c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				b = b<<4 | (c - 'A' + 10)
			default:
				t.Fatalf("bad hex %q", h)
			}
		}
		out[i] = b
	}
	return string(out)
}

package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestUniqueKeysParse(t *testing.T) {
	cases := []struct {
		sql  string
		want string
	}{
		{"CREATE TABLE t (id INTEGER PRIMARY KEY, UNIQUE (a, b))", "[[a b]]"},
		{"CREATE TABLE t (a TEXT, b TEXT, PRIMARY KEY (a, b))", "[[a b]]"},
		{"CREATE TABLE t (id INTEGER PRIMARY KEY, UNIQUE(lower(x)))", "[]"},
		{"CREATE TABLE t (a TEXT)", "[]"},
	}
	for _, c := range cases {
		got := fmt.Sprintf("%v", tableLevelKeys(c.sql))
		if got != c.want {
			t.Errorf("tableLevelKeys(%q) = %s, want %s", c.sql, got, c.want)
		}
	}
}

// TestUniqueKeysFromIndex ent 的 .Unique() 生成独立唯一索引，不在建表语句里，
// uniqueKeys 必须把这种也算上，否则归一化会撞约束（notifications 的真实事故）。
func TestUniqueKeysFromIndex(t *testing.T) {
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "k.db")+"?cache=shared&_fk=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE n (id INTEGER PRIMARY KEY, type TEXT, ref_id TEXT, scheduled_at datetime, member TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX n_key ON n (type, ref_id, scheduled_at, member)`); err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%v", uniqueKeys(ctx, db, "n"))
	want := "[[type ref_id scheduled_at member]]"
	if got != want {
		t.Errorf("uniqueKeys(n) = %s, want %s", got, want)
	}
}

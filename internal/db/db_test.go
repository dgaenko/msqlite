package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	conn, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = conn.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			age INTEGER,
			data BLOB,
			note TEXT DEFAULT 'x'
		);
		CREATE INDEX idx_users_name ON users(name);
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			title TEXT
		);
		CREATE VIEW adult_users AS SELECT id, name FROM users WHERE age >= 18;
		INSERT INTO users (name, age, data, note) VALUES ('Алиса', 30, x'DEADBEEF', NULL);
		INSERT INTO users (name, age) VALUES ('Bob', 17);
		INSERT INTO users (name, age) VALUES ('Carol', 40);
		INSERT INTO posts (user_id, title) VALUES (1, 'hello');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanDir(t *testing.T) {
	path := createTestDB(t)
	files, err := ScanDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "test.db" {
		t.Fatalf("unexpected files: %+v", files)
	}
	if files[0].Size == 0 {
		t.Fatal("expected non-zero size")
	}
	if IsSQLiteFile(filepath.Join(filepath.Dir(path), "notes.txt")) {
		t.Fatal("text file detected as sqlite")
	}
}

func TestObjectsColumnsIndexesAndFKs(t *testing.T) {
	path := createTestDB(t)
	d, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	objs, err := d.Objects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, o := range objs {
		names = append(names, o.Name+":"+o.Type)
	}
	got := strings.Join(names, ",")
	for _, want := range []string{"users:table", "posts:table", "adult_users:view"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}

	cols, err := d.Columns(ctx, "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(cols))
	}
	if cols[0].Name != "id" || cols[0].PK != 1 {
		t.Errorf("unexpected id column: %+v", cols[0])
	}
	if !cols[1].NotNull || cols[1].Name != "name" {
		t.Errorf("unexpected name column: %+v", cols[1])
	}
	if !cols[4].HasDef || cols[4].Default != "'x'" {
		t.Errorf("unexpected default: %+v", cols[4])
	}

	idx, err := d.Indexes(ctx, "users")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, i := range idx {
		if i.Name == "idx_users_name" && len(i.Columns) == 1 && i.Columns[0] == "name" {
			found = true
		}
	}
	if !found {
		t.Errorf("index not found: %+v", idx)
	}

	fks, err := d.ForeignKeys(ctx, "posts")
	if err != nil {
		t.Fatal(err)
	}
	if len(fks) != 1 || fks[0].Table != "users" || fks[0].From != "user_id" || fks[0].OnDelete != "CASCADE" {
		t.Errorf("unexpected fks: %+v", fks)
	}

	schema, err := d.Schema(ctx, "users")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(schema, "CREATE TABLE users") {
		t.Errorf("unexpected schema: %s", schema)
	}
}

func TestPageCountFilterSort(t *testing.T) {
	path := createTestDB(t)
	d, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	cols, err := d.Columns(ctx, "users")
	if err != nil {
		t.Fatal(err)
	}

	n, err := d.Count(ctx, "users", "")
	if err != nil || n != 3 {
		t.Fatalf("count = %d, err = %v", n, err)
	}
	n, err = d.Count(ctx, "users", "age >= 18")
	if err != nil || n != 2 {
		t.Fatalf("filtered count = %d, err = %v", n, err)
	}

	rows, err := d.Page(ctx, "users", cols, PageOptions{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0][1].Text != "Bob" {
		t.Errorf("unexpected row order: %+v", rows[0])
	}
	if !rows[0][3].Null {
		t.Errorf("expected NULL blob column, got %+v", rows[0][3])
	}

	ageIdx := -1
	for i, c := range cols {
		if c.Name == "age" {
			ageIdx = i
		}
	}
	rows, err = d.Page(ctx, "users", cols, PageOptions{OrderColumn: ageIdx, Desc: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0][2].Text != "40" {
		t.Fatalf("unexpected sorted rows: %+v", rows)
	}

	rows, err = d.Page(ctx, "users", cols, PageOptions{Filter: "name = 'Алиса'", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 filtered row, got %d", len(rows))
	}
	if !rows[0][4].Null || rows[0][0].Text != "1" {
		t.Errorf("unexpected filtered row: %+v", rows[0])
	}
	if !rows[0][3].Blob {
		t.Errorf("expected blob marker, got %+v", rows[0][3])
	}
	if !strings.HasPrefix(rows[0][3].Text, "BLOB") {
		t.Errorf("expected blob text, got %q", rows[0][3].Text)
	}
	if _, err := d.Page(ctx, "users", cols, PageOptions{Filter: "1=1; DROP TABLE users"}); err == nil {
		t.Fatal("expected filter validation error")
	}
}

func TestExecAndReadOnly(t *testing.T) {
	path := createTestDB(t)

	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, err := ro.Exec(context.Background(), "DELETE FROM users", 10); err == nil {
		t.Fatal("expected write to fail in read-only mode")
	}
	res, err := ro.Exec(context.Background(), "SELECT name FROM users ORDER BY id", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsSelect || len(res.Rows) != 1 || !res.Truncated {
		t.Fatalf("unexpected result: %+v", res)
	}

	rw, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	if _, err := rw.Exec(context.Background(), "INSERT INTO users (name, age) VALUES ('Dave', 22)", 10); err != nil {
		t.Fatal(err)
	}
	n, err := rw.Count(context.Background(), "users", "")
	if err != nil || n != 4 {
		t.Fatalf("count = %d, err = %v", n, err)
	}
	if _, err := rw.Exec(context.Background(), "CREATE TABLE tmp (x INTEGER)", 10); err != nil {
		t.Fatal(err)
	}
	objs, err := rw.Objects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range objs {
		if o.Name == "tmp" {
			found = true
		}
	}
	if !found {
		t.Error("created table not listed")
	}
}

func TestFormatValueAndQuote(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing.db"), true); err == nil {
		t.Fatal("expected error for missing file")
	}
	c := FormatValue(nil)
	if !c.Null {
		t.Error("nil should be NULL")
	}
	c = FormatValue([]byte{'a', 0, 'b'})
	if !c.Blob {
		t.Error("binary data should be BLOB")
	}
	if got := QuoteIdent(`a"b`); got != `"a""b"` {
		t.Errorf("QuoteIdent = %s", got)
	}
	if got := FormatBytes(1536); got != "1.5 KiB" {
		t.Errorf("FormatBytes = %s", got)
	}
}

func TestQuotedIdentifiers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weird.db")
	conn, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = conn.Exec(`CREATE TABLE "weird table" ("select" TEXT, "a""b" INTEGER); INSERT INTO "weird table" VALUES ('x', 7);`)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()

	d, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	cols, err := d.Columns(ctx, "weird table")
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 2 || cols[0].Name != "select" {
		t.Fatalf("unexpected columns: %+v", cols)
	}
	n, err := d.Count(ctx, "weird table", `"select" = 'x'`)
	if err != nil || n != 1 {
		t.Fatalf("count = %d, err = %v", n, err)
	}
	rows, err := d.Page(ctx, "weird table", cols, PageOptions{OrderColumn: 1, Desc: true, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][1].Text != "7" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestRowMutations(t *testing.T) {
	path := createTestDB(t)
	ctx := context.Background()

	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, err := ro.UpdateCell(ctx, "users", "name", "x", 1); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}
	if _, err := ro.DeleteRow(ctx, "users", 1); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}
	if _, err := ro.InsertRow(ctx, "users", []string{"name"}, []any{"x"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}

	rw, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	cols, err := rw.Columns(ctx, "users")
	if err != nil {
		t.Fatal(err)
	}
	rows, ids, err := rw.PageWithRowIDs(ctx, "users", cols, PageOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || len(ids) != 3 {
		t.Fatalf("rows=%d ids=%d", len(rows), len(ids))
	}
	if rows[0][1].Text != "Алиса" {
		t.Fatalf("unexpected first row: %+v", rows[0])
	}
	if n, err := rw.UpdateCell(ctx, "users", "name", "Алиса 2", ids[0]); err != nil || n != 1 {
		t.Fatalf("update n=%d err=%v", n, err)
	}
	n, err := rw.Count(ctx, "users", "name = 'Алиса 2'")
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	if n, err := rw.DeleteRow(ctx, "users", ids[1]); err != nil || n != 1 {
		t.Fatalf("delete n=%d err=%v", n, err)
	}
	n, err = rw.Count(ctx, "users", "")
	if err != nil || n != 2 {
		t.Fatalf("count after delete=%d err=%v", n, err)
	}
	if n, err := rw.InsertRow(ctx, "users", []string{"name", "age"}, []any{"Dave", 22}); err != nil || n != 1 {
		t.Fatalf("insert n=%d err=%v", n, err)
	}
	n, err = rw.Count(ctx, "users", "name = 'Dave' AND age = 22")
	if err != nil || n != 1 {
		t.Fatalf("count after insert=%d err=%v", n, err)
	}
	if n, err := rw.InsertRow(ctx, "posts", nil, nil); err != nil || n != 1 {
		t.Fatalf("default insert n=%d err=%v", n, err)
	}
}

func TestExportImportDumpAndBlob(t *testing.T) {
	path := createTestDB(t)
	ctx := context.Background()
	d, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	cols, err := d.Columns(ctx, "users")
	if err != nil {
		t.Fatal(err)
	}

	rows, err := d.RawPage(ctx, "users", cols, PageOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if b, ok := rows[0][3].([]byte); !ok || string(b) != "\xde\xad\xbe\xef" {
		t.Fatalf("unexpected raw blob: %#v", rows[0][3])
	}
	raw, err := d.RawRowByID(ctx, "users", cols, 1)
	if err != nil {
		t.Fatal(err)
	}
	if raw[1] != "Алиса" {
		t.Fatalf("unexpected raw row: %#v", raw)
	}
	blob, ok, err := d.FetchBlob(ctx, "users", "data", 1)
	if err != nil || !ok || len(blob) != 4 {
		t.Fatalf("FetchBlob: ok=%v len=%d err=%v", ok, len(blob), err)
	}

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "users.csv")
	n, err := d.ExportTableCSV(ctx, csvPath, "users", cols, PageOptions{})
	if err != nil || n != 3 {
		t.Fatalf("CSV export n=%d err=%v", n, err)
	}
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "id,name") || !strings.Contains(string(data), "Алиса") {
		t.Fatalf("unexpected CSV: %s", data)
	}
	jsonPath := filepath.Join(dir, "users.json")
	n, err = d.ExportTableJSON(ctx, jsonPath, "users", cols, PageOptions{})
	if err != nil || n != 3 {
		t.Fatalf("JSON export n=%d err=%v", n, err)
	}
	var decoded []map[string]any
	data, err = os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 3 || decoded[0]["name"] != "Алиса" {
		t.Fatalf("unexpected JSON: %#v", decoded)
	}

	n, err = d.InsertRows(ctx, "users", []string{"name", "age"}, [][]any{{"Zed", 33}, {"Yan", 44}})
	if err != nil || n != 2 {
		t.Fatalf("InsertRows n=%d err=%v", n, err)
	}
	if n, _ := d.Count(ctx, "users", ""); n != 5 {
		t.Fatalf("expected 5 rows after import, got %d", n)
	}

	dumpPath := filepath.Join(dir, "dump.sql")
	if _, err := d.DumpSQL(ctx, dumpPath); err != nil {
		t.Fatal(err)
	}
	dump, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatal(err)
	}
	restoredPath := filepath.Join(dir, "restored.db")
	restored, err := sql.Open("sqlite", "file:"+restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Exec(string(dump)); err != nil {
		restored.Close()
		t.Fatalf("dump restore failed: %v", err)
	}
	rd, err := Open(restoredPath, true)
	if err != nil {
		restored.Close()
		t.Fatal(err)
	}
	if n, err := rd.Count(ctx, "users", ""); err != nil || n != 5 {
		t.Fatalf("restored count=%d err=%v", n, err)
	}
	rd.Close()
	restored.Close()

	backupPath := filepath.Join(dir, "backup.db")
	if err := d.Backup(ctx, backupPath); err != nil {
		t.Fatal(err)
	}
	bd, err := Open(backupPath, true)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := bd.Count(ctx, "users", ""); err != nil || n != 5 {
		t.Fatalf("backup count=%d err=%v", n, err)
	}
	bd.Close()

	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, err := ro.InsertRows(ctx, "users", []string{"name"}, [][]any{{"x"}}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected ErrReadOnly, got %v", err)
	}
}

func TestCreateDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.db")
	if err := Create(path); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !IsSQLiteFile(path) {
		t.Fatalf("created file is not a SQLite database")
	}
	if err := Create(path); err == nil {
		t.Fatalf("Create over existing file should fail")
	}
	d, err := Open(path, false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	objs, err := d.Objects(context.Background())
	if err != nil {
		t.Fatalf("Objects: %v", err)
	}
	if len(objs) != 0 {
		t.Fatalf("expected empty database, got %d objects", len(objs))
	}
	if err := Create(filepath.Join(dir, "missing", "x.db")); err == nil {
		t.Fatalf("Create in missing directory should fail")
	}
}

func openRaw(t *testing.T, path string, statements string) {
	t.Helper()
	conn, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(statements); err != nil {
		conn.Close()
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRowIDAliasAndShadowing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shadow.db")
	openRaw(t, path, `CREATE TABLE s (rowid TEXT, v INTEGER);
		INSERT INTO s (rowid, v) VALUES ('a', 1), ('b', 2);`)
	d, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	cols, err := d.Columns(ctx, "s")
	if err != nil {
		t.Fatal(err)
	}
	rows, ids, err := d.PageWithRowIDs(ctx, "s", cols, PageOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("unexpected rowids: %v (rows=%+v)", ids, rows)
	}
	if n, err := d.UpdateCell(ctx, "s", "v", 10, ids[0]); err != nil || n != 1 {
		t.Fatalf("update n=%d err=%v", n, err)
	}
	if n, err := d.Count(ctx, "s", "v = 10"); err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	if raw, err := d.RawRowByID(ctx, "s", cols, ids[1]); err != nil || raw[0] != "b" {
		t.Fatalf("raw row=%v err=%v", raw, err)
	}

	path2 := filepath.Join(t.TempDir(), "shadow2.db")
	openRaw(t, path2, `CREATE TABLE s2 (rowid TEXT, _rowid_ TEXT, oid TEXT, v INTEGER);
		INSERT INTO s2 (rowid, _rowid_, oid, v) VALUES ('a', 'b', 'c', 1);`)
	d2, err := Open(path2, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	cols2, err := d2.Columns(ctx, "s2")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := d2.PageWithRowIDs(ctx, "s2", cols2, PageOptions{Limit: 10}); err == nil {
		t.Fatal("expected error when all rowid aliases are shadowed")
	}
	if _, err := d2.UpdateCell(ctx, "s2", "v", 2, 1); err == nil {
		t.Fatal("expected error when all rowid aliases are shadowed")
	}
}

func TestGeneratedColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generated.db")
	openRaw(t, path, `CREATE TABLE g (a INTEGER, b INTEGER GENERATED ALWAYS AS (a * 2) VIRTUAL);
		INSERT INTO g (a) VALUES (5);`)
	d, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	cols, err := d.Columns(ctx, "g")
	if err != nil {
		t.Fatal(err)
	}
	if len(cols) != 2 || cols[1].Name != "b" || !cols[1].Hidden {
		t.Fatalf("unexpected columns: %+v", cols)
	}
	rows, err := d.Page(ctx, "g", cols, PageOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][1].Text != "10" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fk.db")
	openRaw(t, path, `CREATE TABLE p (id INTEGER PRIMARY KEY);
		CREATE TABLE c (pid INTEGER REFERENCES p(id));`)
	d, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Exec(context.Background(), "INSERT INTO c (pid) VALUES (999)", 10); err == nil {
		t.Fatal("expected foreign key violation")
	}
}

func TestPaginationTiebreaker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paging.db")
	openRaw(t, path, `CREATE TABLE t (id INTEGER PRIMARY KEY, k TEXT);`)
	conn, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if _, err := conn.Exec("INSERT INTO t (k) VALUES ('same')"); err != nil {
			t.Fatal(err)
		}
	}
	conn.Close()

	d, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	cols, err := d.Columns(ctx, "t")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for offset := 0; offset < 12; offset += 3 {
		rows, err := d.Page(ctx, "t", cols, PageOptions{OrderColumn: 1, Limit: 3, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			seen[row[0].Text] = true
		}
	}
	if len(seen) != 12 {
		t.Fatalf("pagination returned %d unique rows, want 12", len(seen))
	}
}

func TestSQLClassification(t *testing.T) {
	cases := []struct {
		q    string
		want bool
	}{
		{"SELECT 1", true},
		{"  -- comment\nSELECT 1", true},
		{"/* c */ VALUES (1)", true},
		{"WITH x AS (SELECT 1) SELECT * FROM x", true},
		{"WITH RECURSIVE x(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM x WHERE n < 3) SELECT n FROM x", true},
		{"WITH x AS (SELECT 1) DELETE FROM users", false},
		{"WITH x AS (SELECT 1) UPDATE users SET name = 'x'", false},
		{"WITH x AS (SELECT 1), y AS (SELECT 2) INSERT INTO t SELECT * FROM x", false},
		{"PRAGMA table_info(users)", true},
		{"PRAGMA foreign_keys = ON", false},
		{"INSERT INTO t VALUES (1)", false},
	}
	for _, tc := range cases {
		if got := isReadStatement(tc.q); got != tc.want {
			t.Errorf("isReadStatement(%q) = %v, want %v", tc.q, got, tc.want)
		}
	}
}

func TestFilterHardening(t *testing.T) {
	for _, filter := range []string{"a = 1 --", "a = 1 /* x */", "a = 1; DROP TABLE t"} {
		if err := ValidateFilter(filter); err == nil {
			t.Errorf("expected validation error for %q", filter)
		}
	}
	where, err := filterClause("a = 1")
	if err != nil || where != "(a = 1)" {
		t.Fatalf("filterClause = %q, %v", where, err)
	}
	if where, err := filterClause("  "); err != nil || where != "" {
		t.Fatalf("empty filterClause = %q, %v", where, err)
	}
}

func TestApproxCountsAndTriggers(t *testing.T) {
	path := createTestDB(t)
	openRaw(t, path, `CREATE TRIGGER users_bi AFTER INSERT ON users BEGIN
		UPDATE users SET note = 't' WHERE id = NEW.id;
	END;`)
	d, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.Exec(ctx, "ANALYZE", 1); err != nil {
		t.Fatal(err)
	}
	counts, err := d.ApproxCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts["users"] != 3 {
		t.Fatalf("users estimate = %d, want 3 (%v)", counts["users"], counts)
	}
	triggers, err := d.Triggers(ctx, "users")
	if err != nil {
		t.Fatal(err)
	}
	if len(triggers) != 1 || triggers[0].Name != "users_bi" {
		t.Fatalf("unexpected triggers: %+v", triggers)
	}
}

func TestInsertRowsStream(t *testing.T) {
	path := createTestDB(t)
	d, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	calls := 0
	n, err := d.InsertRowsStream(ctx, "users", []string{"name"}, func() ([][]any, bool, error) {
		calls++
		switch calls {
		case 1:
			return [][]any{{"S1"}, {"S2"}}, false, nil
		case 2:
			return [][]any{{"S3"}}, true, nil
		}
		return nil, true, nil
	})
	if err != nil || n != 3 {
		t.Fatalf("stream insert n=%d err=%v", n, err)
	}
	if got, _ := d.Count(ctx, "users", "name LIKE 'S%'"); got != 3 {
		t.Fatalf("inserted rows = %d, want 3", got)
	}

	before, _ := d.Count(ctx, "users", "")
	calls = 0
	_, err = d.InsertRowsStream(ctx, "users", []string{"name"}, func() ([][]any, bool, error) {
		calls++
		if calls == 1 {
			return [][]any{{"rollback"}}, false, nil
		}
		return nil, true, errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected stream error")
	}
	if got, _ := d.Count(ctx, "users", ""); got != before {
		t.Fatalf("transaction was not rolled back: %d -> %d", before, got)
	}
}

func TestDumpSkipsGeneratedColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gen.db")
	openRaw(t, path, `CREATE TABLE g (a INTEGER, b INTEGER GENERATED ALWAYS AS (a * 2) STORED);
		INSERT INTO g (a) VALUES (7);`)
	d, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	dumpPath := filepath.Join(t.TempDir(), "gen.sql")
	if _, err := d.DumpSQL(context.Background(), dumpPath); err != nil {
		t.Fatal(err)
	}
	dump, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dump), `"b"`) && strings.Contains(string(dump), `INSERT INTO "g" ("a", "b")`) {
		t.Fatalf("dump includes generated column: %s", dump)
	}
	restoredPath := filepath.Join(t.TempDir(), "restored.db")
	conn, err := sql.Open("sqlite", "file:"+restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(string(dump)); err != nil {
		conn.Close()
		t.Fatalf("restore failed: %v", err)
	}
	conn.Close()
	rd, err := Open(restoredPath, true)
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()
	cols, err := rd.Columns(context.Background(), "g")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := rd.Page(context.Background(), "g", cols, PageOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][1].Text != "14" {
		t.Fatalf("unexpected restored row: %+v", rows)
	}
}

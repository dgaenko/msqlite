package ui

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"msqlite/internal/db"
)

func TestParseEditorValue(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"", nil},
		{"NULL", nil},
		{" null ", nil},
		{"''", ""},
		{"x'DEADBEEF'", []byte{0xde, 0xad, 0xbe, 0xef}},
		{"x'de ad'", []byte{0xde, 0xad}},
		{"x'zz'", "x'zz'"},
		{"hello", "hello"},
		{" 42 ", " 42 "},
	}
	for _, tc := range cases {
		got := parseEditorValue(tc.in)
		switch want := tc.want.(type) {
		case []byte:
			b, ok := got.([]byte)
			if !ok || string(b) != string(want) {
				t.Errorf("parseEditorValue(%q) = %#v, want %#v", tc.in, got, want)
			}
		default:
			if got != tc.want {
				t.Errorf("parseEditorValue(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		}
	}
}

func TestWordHelpers(t *testing.T) {
	text := "SELECT имя_1"
	start, end := wordBounds(text, len(text))
	if got := text[start:end]; got != "имя_1" {
		t.Errorf("wordBounds end = %q", got)
	}
	start, end = wordBounds(text, 6)
	if start != 0 || end != 6 {
		t.Errorf("wordBounds midline = %d..%d", start, end)
	}
	if got := previousWord("SELECT * FROM users"); got != "users" {
		t.Errorf("previousWord = %q", got)
	}
	if got := previousWord("SELECT "); got != "SELECT" {
		t.Errorf("previousWord trailing space = %q", got)
	}
	cases := map[string]string{
		"SELECT * FROM ": "tables",
		"SELECT ":        "columns",
		"JOIN ":          "tables",
		"WHERE ":         "columns",
		"garbage ":       "",
	}
	for text, want := range cases {
		if got := completionContext(text, len(text)); got != want {
			t.Errorf("completionContext(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestColumnHelpers(t *testing.T) {
	if got := shortColumnType("INTEGER"); got != "INT" {
		t.Errorf("shortColumnType INTEGER = %q", got)
	}
	if got := shortColumnType("BIGINT"); got != "INT" {
		t.Errorf("shortColumnType BIGINT = %q", got)
	}
	if got := shortColumnType("TEXT"); got != "TEXT" {
		t.Errorf("shortColumnType TEXT = %q", got)
	}
	if got := columnTypeIndex("text"); got != 1 {
		t.Errorf("columnTypeIndex text = %d", got)
	}
	if got := columnTypeIndex("unknown"); got != 1 {
		t.Errorf("columnTypeIndex unknown = %d", got)
	}
}

func TestBuildCreateTableSQL(t *testing.T) {
	if _, err := buildCreateTableSQL("", nil); err == nil {
		t.Fatal("expected empty name error")
	}
	if _, err := buildCreateTableSQL("t", nil); err == nil {
		t.Fatal("expected empty columns error")
	}
	cols := []tableColumnDef{
		{name: "id", typ: "INTEGER", pk: true},
		{name: `na"me`, typ: "TEXT", notNull: true, def: "'x'"},
	}
	sql, err := buildCreateTableSQL("t", cols)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `CREATE TABLE "t"`) {
		t.Errorf("missing table name: %s", sql)
	}
	if !strings.Contains(sql, `"na""me" TEXT NOT NULL DEFAULT 'x'`) {
		t.Errorf("missing quoted column: %s", sql)
	}
	if !strings.Contains(sql, `"id" INTEGER PRIMARY KEY`) {
		t.Errorf("missing primary key: %s", sql)
	}

	pkCols := []tableColumnDef{
		{name: "a", typ: "INTEGER", pk: true},
		{name: "b", typ: "INTEGER", pk: true},
	}
	sql, err = buildCreateTableSQL("t", pkCols)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, `PRIMARY KEY ("a", "b")`) {
		t.Errorf("missing composite primary key: %s", sql)
	}
}

func TestAutoPKIndex(t *testing.T) {
	p := &DataPage{cols: []db.Column{{Name: "id", Type: "INTEGER", PK: 1}}}
	if got := p.autoPKIndex(); got != 0 {
		t.Errorf("autoPKIndex = %d, want 0", got)
	}
	p = &DataPage{cols: []db.Column{
		{Name: "a", Type: "INTEGER", PK: 1},
		{Name: "b", Type: "INTEGER", PK: 2},
	}}
	if got := p.autoPKIndex(); got != -1 {
		t.Errorf("composite autoPKIndex = %d, want -1", got)
	}
	p = &DataPage{cols: []db.Column{{Name: "id", Type: "TEXT", PK: 1}}}
	if got := p.autoPKIndex(); got != -1 {
		t.Errorf("text autoPKIndex = %d, want -1", got)
	}
}

func TestJSONValue(t *testing.T) {
	if got := jsonValue(json.Number("42")); got != int64(42) {
		t.Errorf("jsonValue int = %#v", got)
	}
	if got := jsonValue(json.Number("1.5")); got != 1.5 {
		t.Errorf("jsonValue float = %#v", got)
	}
	if got := jsonValue(json.Number("999999999999999999999")); got != "999999999999999999999" {
		t.Errorf("jsonValue big = %#v", got)
	}
	if got := jsonValue(nil); got != nil {
		t.Errorf("jsonValue nil = %#v", got)
	}
	if got := jsonValue("x"); got != "x" {
		t.Errorf("jsonValue string = %#v", got)
	}
	if got := jsonValue(true); got != true {
		t.Errorf("jsonValue bool = %#v", got)
	}
	if got := jsonValue(map[string]any{"a": 1}); got != `{"a":1}` {
		t.Errorf("jsonValue object = %#v", got)
	}
}

func TestCountObjectsFallsBackToExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("CREATE TABLE t(x); INSERT INTO t(x) VALUES (1), (2), (3)"); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	raw.Close()
	d, err := db.Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	objs, err := d.Objects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{current: d}
	counts := a.countObjects(objs)
	rc, ok := counts["t"]
	if !ok || rc.N != 3 || rc.Est {
		t.Fatalf("counts[t] = %+v, ok=%v; want exact 3", rc, ok)
	}
	if _, err := d.Exec(ctx, "ANALYZE", 1); err != nil {
		t.Fatal(err)
	}
	rc = a.countObjects(objs)["t"]
	if rc.N != 3 || !rc.Est {
		t.Fatalf("after ANALYZE counts[t] = %+v; want estimate 3", rc)
	}
}

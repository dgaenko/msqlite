// Package db provides read access to SQLite databases for the TUI.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

const sqliteMagic = "SQLite format 3\x00"

// File describes a SQLite database file found in a directory.
type File struct {
	Path string
	Name string
	Size int64
}

// Object is a table or view from sqlite_master.
type Object struct {
	Name string
	Type string // "table" or "view"
	SQL  string
}

// Column describes one table column. Hidden marks generated/virtual
// columns that are not writable.
type Column struct {
	CID     int
	Name    string
	Type    string
	NotNull bool
	Default string
	HasDef  bool
	PK      int
	Hidden  bool
}

// Index describes an index on a table.
type Index struct {
	Name    string
	Unique  bool
	Origin  string
	Partial bool
	Columns []string
}

// ForeignKey describes a foreign key constraint.
type ForeignKey struct {
	Table    string
	From     string
	To       string
	OnUpdate string
	OnDelete string
	Match    string
}

// Cell is a single formatted value.
type Cell struct {
	Text string
	Full string
	Null bool
	Blob bool
	Type string
	Raw  any
}

// Row is a row of cells.
type Row []Cell

// Database is an open SQLite database.
type Database struct {
	Path     string
	Name     string
	ReadOnly bool
	conn     *sql.DB
}

// ScanDir returns SQLite database files in dir, sorted by name.
func ScanDir(dir string) ([]File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []File
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if !IsSQLiteFile(path) {
			continue
		}
		files = append(files, File{Path: path, Name: e.Name(), Size: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})
	return files, nil
}

// IsSQLiteFile reports whether path starts with the SQLite header.
func IsSQLiteFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, len(sqliteMagic))
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	return string(buf[:n]) == sqliteMagic
}

// Create creates a new empty SQLite database at path.
func Create(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if info, err := os.Stat(abs); err == nil {
		if info.IsDir() {
			return fmt.Errorf("%s: это каталог", abs)
		}
		return fmt.Errorf("файл уже существует: %s", abs)
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(abs)
	if info, err := os.Stat(parent); err != nil {
		return fmt.Errorf("каталог %s: %w", parent, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%s: не каталог", parent)
	}
	file, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("файл уже существует: %s", abs)
	}
	if err := file.Close(); err != nil {
		os.Remove(abs)
		return err
	}

	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	q := u.Query()
	q.Set("mode", "rw")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	u.RawQuery = q.Encode()

	conn, err := sql.Open("sqlite", u.String())
	if err != nil {
		os.Remove(abs)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(ctx, "PRAGMA user_version = 0"); err != nil {
		conn.Close()
		os.Remove(abs)
		return err
	}
	if err := conn.Close(); err != nil {
		os.Remove(abs)
		return err
	}
	if !IsSQLiteFile(abs) {
		os.Remove(abs)
		return fmt.Errorf("не удалось создать файл SQLite: %s", abs)
	}
	return nil
}

// Open opens a database. When readOnly is true, writes are rejected by SQLite.
func Open(path string, readOnly bool) (*Database, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if !IsSQLiteFile(abs) {
		return nil, fmt.Errorf("%s: не является файлом SQLite", abs)
	}
	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	q := u.Query()
	if readOnly {
		q.Set("mode", "ro")
	} else {
		q.Set("mode", "rw")
		q.Add("_pragma", "foreign_keys(1)")
	}
	q.Add("_pragma", "busy_timeout(5000)")
	u.RawQuery = q.Encode()

	conn, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("не удалось открыть %s: %w", filepath.Base(abs), err)
	}
	return &Database{
		Path:     abs,
		Name:     filepath.Base(abs),
		ReadOnly: readOnly,
		conn:     conn,
	}, nil
}

// Close closes the database.
func (d *Database) Close() error {
	if d == nil || d.conn == nil {
		return nil
	}
	return d.conn.Close()
}

// Objects lists user tables and views.
func (d *Database) Objects(ctx context.Context) ([]Object, error) {
	const q = `SELECT name, type, COALESCE(sql, '') FROM sqlite_master
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY CASE type WHEN 'table' THEN 0 ELSE 1 END, name COLLATE NOCASE`
	rows, err := d.conn.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var objs []Object
	for rows.Next() {
		var o Object
		if err := rows.Scan(&o.Name, &o.Type, &o.SQL); err != nil {
			return nil, err
		}
		objs = append(objs, o)
	}
	return objs, rows.Err()
}

// Schema returns the CREATE statement for a table or view.
func (d *Database) Schema(ctx context.Context, name string) (string, error) {
	var sqlText sql.NullString
	err := d.conn.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE name = ? AND type IN ('table', 'view')`, name).Scan(&sqlText)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("объект %q не найден", name)
	}
	if err != nil {
		return "", err
	}
	return sqlText.String, nil
}

// Columns returns column metadata for a table or view. It uses
// PRAGMA table_xinfo when available so generated columns are included.
func (d *Database) Columns(ctx context.Context, table string) ([]Column, error) {
	cols, err := d.columns(ctx, table, "PRAGMA table_xinfo("+QuoteIdent(table)+")", 7)
	if err == nil {
		return cols, nil
	}
	return d.columns(ctx, table, "PRAGMA table_info("+QuoteIdent(table)+")", 6)
}

func (d *Database) columns(ctx context.Context, table, query string, fields int) ([]Column, error) {
	rows, err := d.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []Column
	for rows.Next() {
		var c Column
		var def sql.NullString
		var notNull, pk, hidden int
		if fields >= 7 {
			if err := rows.Scan(&c.CID, &c.Name, &c.Type, &notNull, &def, &pk, &hidden); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&c.CID, &c.Name, &c.Type, &notNull, &def, &pk); err != nil {
				return nil, err
			}
		}
		c.NotNull = notNull != 0
		c.Default = def.String
		c.HasDef = def.Valid
		c.PK = pk
		c.Hidden = hidden != 0
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// Indexes returns indexes for a table.
func (d *Database) Indexes(ctx context.Context, table string) ([]Index, error) {
	rows, err := d.conn.QueryContext(ctx, "PRAGMA index_list("+QuoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var idxs []Index
	for rows.Next() {
		var seq int
		var idx Index
		var unique, partial int
		if err := rows.Scan(&seq, &idx.Name, &unique, &idx.Origin, &partial); err != nil {
			return nil, err
		}
		idx.Unique = unique != 0
		idx.Partial = partial != 0
		idxs = append(idxs, idx)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range idxs {
		icols, err := d.indexColumns(ctx, idxs[i].Name)
		if err != nil {
			return nil, err
		}
		idxs[i].Columns = icols
	}
	return idxs, nil
}

func (d *Database) indexColumns(ctx context.Context, index string) ([]string, error) {
	rows, err := d.conn.QueryContext(ctx, "PRAGMA index_info("+QuoteIdent(index)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var seqno, cid int
		var name sql.NullString
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		if name.Valid {
			cols = append(cols, name.String)
		} else {
			cols = append(cols, "(выражение)")
		}
	}
	return cols, rows.Err()
}

// ForeignKeys returns foreign key constraints for a table.
func (d *Database) ForeignKeys(ctx context.Context, table string) ([]ForeignKey, error) {
	rows, err := d.conn.QueryContext(ctx, "PRAGMA foreign_key_list("+QuoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var fks []ForeignKey
	for rows.Next() {
		var id, seq int
		var fk ForeignKey
		var to sql.NullString
		if err := rows.Scan(&id, &seq, &fk.Table, &fk.From, &to, &fk.OnUpdate, &fk.OnDelete, &fk.Match); err != nil {
			return nil, err
		}
		fk.To = to.String
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

// Trigger describes a trigger on a table.
type Trigger struct {
	Name  string
	Table string
	SQL   string
}

// Triggers returns triggers attached to a table.
func (d *Database) Triggers(ctx context.Context, table string) ([]Trigger, error) {
	const q = `SELECT name, tbl_name, COALESCE(sql, '') FROM sqlite_master
		WHERE type = 'trigger' AND tbl_name = ? ORDER BY name COLLATE NOCASE`
	rows, err := d.conn.QueryContext(ctx, q, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Trigger
	for rows.Next() {
		var t Trigger
		if err := rows.Scan(&t.Name, &t.Table, &t.SQL); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ApproxCounts returns row-count estimates from sqlite_stat1 (available after
// ANALYZE). Tables without statistics are omitted. It never scans tables.
func (d *Database) ApproxCounts(ctx context.Context) (map[string]int64, error) {
	counts := make(map[string]int64)
	rows, err := d.conn.QueryContext(ctx, `SELECT tbl, stat FROM sqlite_stat1`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return counts, nil
		}
		return counts, err
	}
	defer rows.Close()
	for rows.Next() {
		var tbl string
		var stat sql.NullString
		if err := rows.Scan(&tbl, &stat); err != nil {
			return counts, err
		}
		if !stat.Valid {
			continue
		}
		fields := strings.Fields(stat.String)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		if n > counts[tbl] {
			counts[tbl] = n
		}
	}
	return counts, rows.Err()
}

// PageOptions controls a data page query.
type PageOptions struct {
	OrderColumn int // -1 for natural order
	Desc        bool
	Filter      string
	Limit       int
	Offset      int
}

// ValidateFilter rejects multiple statements and SQL comments in a
// user-supplied WHERE clause.
func ValidateFilter(filter string) error {
	if strings.ContainsRune(filter, ';') {
		return errors.New("фильтр не должен содержать ';'")
	}
	if strings.ContainsRune(filter, 0) {
		return errors.New("недопустимый символ в фильтре")
	}
	if strings.Contains(filter, "--") || strings.Contains(filter, "/*") {
		return errors.New("фильтр не должен содержать комментарии")
	}
	return nil
}

// filterClause validates filter and returns it wrapped in parentheses so it
// cannot affect the rest of the generated statement.
func filterClause(filter string) (string, error) {
	if err := ValidateFilter(filter); err != nil {
		return "", err
	}
	f := strings.TrimSpace(filter)
	if f == "" {
		return "", nil
	}
	return "(" + f + ")", nil
}

// Count returns the number of rows matching filter.
func (d *Database) Count(ctx context.Context, table, filter string) (int64, error) {
	where, err := filterClause(filter)
	if err != nil {
		return 0, err
	}
	q := "SELECT COUNT(*) FROM " + QuoteIdent(table)
	if where != "" {
		q += " WHERE " + where
	}
	var n int64
	err = d.conn.QueryRowContext(ctx, q).Scan(&n)
	return n, err
}

// Page fetches rows from table.
func (d *Database) Page(ctx context.Context, table string, cols []Column, opts PageOptions) ([]Row, error) {
	rows, _, err := d.page(ctx, table, cols, opts, false)
	return rows, err
}

// PageWithRowIDs fetches rows together with their rowids. Only for rowid tables.
func (d *Database) PageWithRowIDs(ctx context.Context, table string, cols []Column, opts PageOptions) ([]Row, []int64, error) {
	return d.page(ctx, table, cols, opts, true)
}

func (d *Database) page(ctx context.Context, table string, cols []Column, opts PageOptions, withRowIDs bool) ([]Row, []int64, error) {
	if len(cols) == 0 {
		return nil, nil, nil
	}
	where, err := filterClause(opts.Filter)
	if err != nil {
		return nil, nil, err
	}
	alias := ""
	if withRowIDs {
		var ok bool
		alias, ok, err = d.rowidAlias(ctx, table)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("таблица %s не поддерживает построчную правку", table)
		}
	}
	selects := make([]string, 0, len(cols)+1)
	if withRowIDs {
		selects = append(selects, QuoteIdent(alias))
	}
	for _, c := range cols {
		q := QuoteIdent(c.Name)
		selects = append(selects, fmt.Sprintf("CASE WHEN typeof(%s) = 'blob' THEN 'BLOB (' || length(%s) || ' B)' ELSE %s END, typeof(%s)", q, q, q, q))
	}
	q := "SELECT " + strings.Join(selects, ", ") + " FROM " + QuoteIdent(table)
	if where != "" {
		q += " WHERE " + where
	}
	order := ""
	if opts.OrderColumn >= 0 && opts.OrderColumn < len(cols) {
		dir := "ASC"
		if opts.Desc {
			dir = "DESC"
		}
		order = " ORDER BY " + QuoteIdent(cols[opts.OrderColumn].Name) + " " + dir
	}
	tiebreaker, err := d.orderTiebreaker(ctx, table, cols, alias)
	if err != nil {
		return nil, nil, err
	}
	if order != "" && tiebreaker != "" {
		order += ", " + tiebreaker
	} else if order == "" && tiebreaker != "" {
		order = " ORDER BY " + tiebreaker
	}
	q += order
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	q += " LIMIT ? OFFSET ?"

	rows, err := d.conn.QueryContext(ctx, q, opts.Limit, opts.Offset)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := make([]Row, 0, opts.Limit)
	var ids []int64
	for rows.Next() {
		offset := 0
		var id sql.NullInt64
		total := len(cols) * 2
		if withRowIDs {
			total++
			offset = 1
		}
		vals := make([]any, total)
		ptrs := make([]any, total)
		if withRowIDs {
			ptrs[0] = &id
		}
		for i := offset; i < total; i++ {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		r := make(Row, len(cols))
		for i := range cols {
			r[i] = FormatValue(vals[offset+i*2])
			r[i].Type = cols[i].Type
			if t, ok := vals[offset+i*2+1].(string); ok && t == "blob" {
				r[i].Blob = true
			}
		}
		out = append(out, r)
		if withRowIDs {
			ids = append(ids, id.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return out, ids, nil
}

// rowidAlias returns an alias for the hidden rowid column that is not
// shadowed by a user column. ok is false for views and WITHOUT ROWID tables.
func (d *Database) rowidAlias(ctx context.Context, table string) (string, bool, error) {
	var typ, sqlText sql.NullString
	err := d.conn.QueryRowContext(ctx,
		`SELECT type, COALESCE(sql, '') FROM sqlite_master WHERE name = ? AND type IN ('table', 'view') LIMIT 1`,
		table).Scan(&typ, &sqlText)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("объект %q не найден", table)
	}
	if err != nil {
		return "", false, err
	}
	if typ.String != "table" || strings.Contains(strings.ToUpper(sqlText.String), "WITHOUT ROWID") {
		return "", false, nil
	}
	cols, err := d.Columns(ctx, table)
	if err != nil {
		return "", false, err
	}
	used := make(map[string]bool, len(cols))
	for _, c := range cols {
		used[strings.ToLower(c.Name)] = true
	}
	for _, alias := range []string{"_rowid_", "rowid", "oid"} {
		if !used[alias] {
			return alias, true, nil
		}
	}
	return "", false, nil
}

// orderTiebreaker returns a deterministic ORDER BY tail so LIMIT/OFFSET
// pagination does not skip or repeat rows when the sort key has duplicates.
func (d *Database) orderTiebreaker(ctx context.Context, table string, cols []Column, alias string) (string, error) {
	var pk []Column
	for _, c := range cols {
		if c.PK > 0 {
			pk = append(pk, c)
		}
	}
	if len(pk) > 0 {
		sort.Slice(pk, func(i, j int) bool { return pk[i].PK < pk[j].PK })
		parts := make([]string, len(pk))
		for i, c := range pk {
			parts[i] = QuoteIdent(c.Name)
		}
		return strings.Join(parts, ", "), nil
	}
	if alias == "" {
		a, ok, err := d.rowidAlias(ctx, table)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", nil
		}
		alias = a
	}
	return QuoteIdent(alias), nil
}

// ErrReadOnly is returned when a write is attempted on a read-only database.
var ErrReadOnly = errors.New("база открыта только для чтения")

func (d *Database) ensureWritable() error {
	if d.ReadOnly {
		return ErrReadOnly
	}
	return nil
}

// UpdateCell updates one column in the row identified by rowID.
func (d *Database) UpdateCell(ctx context.Context, table, column string, value any, rowID int64) (int64, error) {
	if err := d.ensureWritable(); err != nil {
		return 0, err
	}
	alias, ok, err := d.rowidAlias(ctx, table)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("таблица %s не поддерживает построчную правку", table)
	}
	q := "UPDATE " + QuoteIdent(table) + " SET " + QuoteIdent(column) + " = ? WHERE " + QuoteIdent(alias) + " = ?"
	res, err := d.conn.ExecContext(ctx, q, value, rowID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteRow deletes the row identified by rowID.
func (d *Database) DeleteRow(ctx context.Context, table string, rowID int64) (int64, error) {
	if err := d.ensureWritable(); err != nil {
		return 0, err
	}
	alias, ok, err := d.rowidAlias(ctx, table)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("таблица %s не поддерживает построчную правку", table)
	}
	q := "DELETE FROM " + QuoteIdent(table) + " WHERE " + QuoteIdent(alias) + " = ?"
	res, err := d.conn.ExecContext(ctx, q, rowID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// InsertRow inserts a row. Empty columns means INSERT ... DEFAULT VALUES.
func (d *Database) InsertRow(ctx context.Context, table string, columns []string, values []any) (int64, error) {
	if err := d.ensureWritable(); err != nil {
		return 0, err
	}
	if len(columns) != len(values) {
		return 0, errors.New("число колонок и значений не совпадает")
	}
	q := "INSERT INTO " + QuoteIdent(table)
	args := make([]any, len(values))
	if len(columns) == 0 {
		q += " DEFAULT VALUES"
	} else {
		quoted := make([]string, len(columns))
		placeholders := make([]string, len(columns))
		for i, c := range columns {
			quoted[i] = QuoteIdent(c)
			placeholders[i] = "?"
			args[i] = values[i]
		}
		q += " (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	}
	res, err := d.conn.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ResultColumn is a column in an SQL console result.
type ResultColumn struct {
	Name string
	Type string
}

// QueryResult is the result of an SQL console statement.
type QueryResult struct {
	Columns   []ResultColumn
	Rows      []Row
	Affected  int64
	IsSelect  bool
	Truncated bool
	Duration  time.Duration
}

// Exec runs one SQL statement. SELECT-like statements return rows (capped).
func (d *Database) Exec(ctx context.Context, statement string, maxRows int) (*QueryResult, error) {
	start := time.Now()
	trimmed := strings.TrimSpace(statement)
	if trimmed == "" {
		return nil, errors.New("пустой запрос")
	}
	if maxRows <= 0 {
		maxRows = 1000
	}
	if isReadStatement(trimmed) {
		rows, err := d.conn.QueryContext(ctx, trimmed)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		colNames, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		colTypes, err := rows.ColumnTypes()
		if err != nil {
			return nil, err
		}
		res := &QueryResult{IsSelect: true}
		for i, name := range colNames {
			typ := ""
			if i < len(colTypes) {
				typ = colTypes[i].DatabaseTypeName()
			}
			res.Columns = append(res.Columns, ResultColumn{Name: name, Type: typ})
		}
		for rows.Next() {
			if len(res.Rows) >= maxRows {
				res.Truncated = true
				break
			}
			vals := make([]any, len(colNames))
			ptrs := make([]any, len(colNames))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return nil, err
			}
			r := make(Row, len(vals))
			for i, v := range vals {
				r[i] = FormatValue(v)
				r[i].Raw = v
				if i < len(res.Columns) {
					r[i].Type = res.Columns[i].Type
				}
			}
			res.Rows = append(res.Rows, r)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		res.Duration = time.Since(start)
		return res, nil
	}
	execRes, err := d.conn.ExecContext(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	affected, _ := execRes.RowsAffected()
	return &QueryResult{Affected: affected, Duration: time.Since(start)}, nil
}

func isReadStatement(q string) bool {
	switch firstSQLKeyword(q) {
	case "SELECT", "VALUES", "EXPLAIN":
		return true
	case "PRAGMA":
		return !strings.Contains(q, "=")
	case "WITH":
		switch withStatementKeyword(q) {
		case "SELECT", "VALUES", "PRAGMA":
			return true
		}
	}
	return false
}

func firstSQLKeyword(q string) string {
	i := skipSQLSpaceAndComments(q, 0)
	return strings.ToUpper(readSQLWord(q, i))
}

func withStatementKeyword(q string) string {
	i := skipSQLSpaceAndComments(q, 0)
	if !strings.EqualFold(readSQLWord(q, i), "WITH") {
		return ""
	}
	i = skipSQLSpaceAndComments(q, i+len("WITH"))
	if w := readSQLWord(q, i); strings.EqualFold(w, "RECURSIVE") {
		i = skipSQLSpaceAndComments(q, i+len(w))
	}
	for {
		i = skipSQLIdent(q, i)
		i = skipSQLSpaceAndComments(q, i)
		if i < len(q) && q[i] == '(' {
			i = skipSQLParenGroup(q, i)
			i = skipSQLSpaceAndComments(q, i)
		}
		w := readSQLWord(q, i)
		if !strings.EqualFold(w, "AS") {
			return ""
		}
		i = skipSQLSpaceAndComments(q, i+len(w))
		w = readSQLWord(q, i)
		if strings.EqualFold(w, "NOT") {
			i = skipSQLSpaceAndComments(q, i+len(w))
			w = readSQLWord(q, i)
		}
		if strings.EqualFold(w, "MATERIALIZED") {
			i = skipSQLSpaceAndComments(q, i+len(w))
		}
		if i >= len(q) || q[i] != '(' {
			return ""
		}
		i = skipSQLParenGroup(q, i)
		i = skipSQLSpaceAndComments(q, i)
		if i < len(q) && q[i] == ',' {
			i = skipSQLSpaceAndComments(q, i+1)
			continue
		}
		break
	}
	return strings.ToUpper(readSQLWord(q, i))
}

func skipSQLSpaceAndComments(s string, i int) int {
	for i < len(s) {
		switch {
		case s[i] == '-' && i+1 < len(s) && s[i+1] == '-':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case s[i] == '/' && i+1 < len(s) && s[i+1] == '*':
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < len(s) {
				i += 2
			} else {
				i = len(s)
			}
		case s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n':
			i++
		default:
			return i
		}
	}
	return i
}

func readSQLWord(s string, i int) string {
	start := i
	for i < len(s) {
		r := rune(s[i])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			i++
			continue
		}
		break
	}
	return s[start:i]
}

func skipSQLIdent(s string, i int) int {
	i = skipSQLSpaceAndComments(s, i)
	if i >= len(s) {
		return i
	}
	switch s[i] {
	case '"', '`', '\'':
		quote := s[i]
		i++
		for i < len(s) {
			if s[i] == quote {
				if i+1 < len(s) && s[i+1] == quote {
					i += 2
					continue
				}
				return i + 1
			}
			i++
		}
		return i
	case '[':
		i++
		for i < len(s) && s[i] != ']' {
			i++
		}
		if i < len(s) {
			i++
		}
		return i
	}
	return i + len(readSQLWord(s, i))
}

func skipSQLParenGroup(s string, i int) int {
	if i >= len(s) || s[i] != '(' {
		return i
	}
	depth := 0
	for i < len(s) {
		switch s[i] {
		case '"', '`', '\'':
			quote := s[i]
			i++
			for i < len(s) {
				if s[i] == quote {
					if i+1 < len(s) && s[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case '[':
			i++
			for i < len(s) && s[i] != ']' {
				i++
			}
			if i < len(s) {
				i++
			}
		case '-':
			if i+1 < len(s) && s[i+1] == '-' {
				for i < len(s) && s[i] != '\n' {
					i++
				}
			} else {
				i++
			}
		case '/':
			if i+1 < len(s) && s[i+1] == '*' {
				i += 2
				for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
					i++
				}
				if i+1 < len(s) {
					i += 2
				}
			} else {
				i++
			}
		case '(':
			depth++
			i++
		case ')':
			depth--
			i++
			if depth == 0 {
				return i
			}
		default:
			i++
		}
	}
	return i
}

// FormatValue converts a scanned driver value into a display Cell.
func FormatValue(v any) Cell {
	switch x := v.(type) {
	case nil:
		return Cell{Text: "NULL", Full: "NULL", Null: true}
	case []byte:
		if utf8.Valid(x) && !containsNul(x) {
			return textCell(string(x))
		}
		return Cell{
			Text: fmt.Sprintf("BLOB (%d B)", len(x)),
			Full: fmt.Sprintf("BLOB, %d байт", len(x)),
			Blob: true,
		}
	case string:
		return textCell(x)
	case time.Time:
		s := x.Format(time.RFC3339Nano)
		return Cell{Text: s, Full: s}
	case bool:
		if x {
			return Cell{Text: "true", Full: "true"}
		}
		return Cell{Text: "false", Full: "false"}
	case int64:
		s := fmt.Sprintf("%d", x)
		return Cell{Text: s, Full: s}
	case float64:
		s := fmt.Sprintf("%v", x)
		return Cell{Text: s, Full: s}
	default:
		s := fmt.Sprint(x)
		return textCell(s)
	}
}

func containsNul(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

const (
	displayLimit = 500
	fullLimit    = 32 * 1024
)

func textCell(s string) Cell {
	display := sanitize(s, displayLimit)
	full := sanitizeFull(s, fullLimit)
	return Cell{Text: display, Full: full}
}

func sanitize(s string, limit int) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if r < 32 || r == 127 {
			return '·'
		}
		return r
	}, s)
	if limit > 0 {
		s = truncateRunes(s, limit)
	}
	return s
}

func sanitizeFull(s string, limit int) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t':
			return r
		case '\r':
			return -1
		}
		if r < 32 || r == 127 {
			return '·'
		}
		return r
	}, s)
	return truncateRunes(s, limit)
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit]) + "…"
}

// QuoteIdent quotes an SQLite identifier.
func QuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// FormatBytes renders a byte count in a compact human-readable form.
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

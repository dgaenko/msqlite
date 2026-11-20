package db

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const exportBatch = 1000

// RawPage returns raw driver values for a data page.
func (d *Database) RawPage(ctx context.Context, table string, cols []Column, opts PageOptions) ([][]any, error) {
	if len(cols) == 0 {
		return nil, nil
	}
	where, err := filterClause(opts.Filter)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = QuoteIdent(c.Name)
	}
	q := "SELECT " + strings.Join(names, ", ") + " FROM " + QuoteIdent(table)
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
	tiebreaker, err := d.orderTiebreaker(ctx, table, cols, "")
	if err != nil {
		return nil, err
	}
	if order != "" && tiebreaker != "" {
		order += ", " + tiebreaker
	} else if order == "" && tiebreaker != "" {
		order = " ORDER BY " + tiebreaker
	}
	q += order
	if opts.Limit <= 0 {
		opts.Limit = exportBatch
	}
	q += " LIMIT ? OFFSET ?"
	rows, err := d.conn.QueryContext(ctx, q, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		out = append(out, vals)
	}
	return out, rows.Err()
}

// RawRowByID returns raw values of one row identified by rowid.
func (d *Database) RawRowByID(ctx context.Context, table string, cols []Column, rowID int64) ([]any, error) {
	if len(cols) == 0 {
		return nil, nil
	}
	alias, ok, err := d.rowidAlias(ctx, table)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("таблица %s не поддерживает построчную правку", table)
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = QuoteIdent(c.Name)
	}
	q := "SELECT " + strings.Join(names, ", ") + " FROM " + QuoteIdent(table) + " WHERE " + QuoteIdent(alias) + " = ?"
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := d.conn.QueryRowContext(ctx, q, rowID).Scan(ptrs...); err != nil {
		return nil, err
	}
	return vals, nil
}

// FetchBlob returns the raw bytes of a BLOB cell.
func (d *Database) FetchBlob(ctx context.Context, table, column string, rowID int64) ([]byte, bool, error) {
	alias, ok, err := d.rowidAlias(ctx, table)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, fmt.Errorf("таблица %s не поддерживает доступ по rowid", table)
	}
	q := "SELECT " + QuoteIdent(column) + ", typeof(" + QuoteIdent(column) + ") FROM " +
		QuoteIdent(table) + " WHERE " + QuoteIdent(alias) + " = ?"
	var value any
	var typ string
	if err := d.conn.QueryRowContext(ctx, q, rowID).Scan(&value, &typ); err != nil {
		return nil, false, err
	}
	if typ != "blob" {
		return nil, false, nil
	}
	b, _ := value.([]byte)
	return b, true, nil
}

func rawCSVValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		if utf8.Valid(x) && !containsNul(x) {
			return string(x)
		}
		return "x'" + hex.EncodeToString(x) + "'"
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case bool:
		if x {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprint(x)
	}
}

func rawJSONValue(v any) any {
	switch x := v.(type) {
	case []byte:
		if utf8.Valid(x) && !containsNul(x) {
			return string(x)
		}
		return "0x" + hex.EncodeToString(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	default:
		return v
	}
}

// ExportTableCSV writes all rows matching opts to a CSV file.
func (d *Database) ExportTableCSV(ctx context.Context, path, table string, cols []Column, opts PageOptions) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = c.Name
	}
	if err := w.Write(header); err != nil {
		return 0, err
	}
	var total int64
	offset := 0
	for {
		opts.Limit = exportBatch
		opts.Offset = offset
		rows, err := d.RawPage(ctx, table, cols, opts)
		if err != nil {
			return total, err
		}
		for _, row := range rows {
			rec := make([]string, len(row))
			for i, v := range row {
				rec[i] = rawCSVValue(v)
			}
			if err := w.Write(rec); err != nil {
				return total, err
			}
			total++
		}
		if len(rows) < exportBatch {
			break
		}
		offset += exportBatch
		if err := ctx.Err(); err != nil {
			return total, err
		}
	}
	w.Flush()
	return total, w.Error()
}

// ExportTableJSON writes all rows matching opts as a JSON array.
func (d *Database) ExportTableJSON(ctx context.Context, path, table string, cols []Column, opts PageOptions) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := f.WriteString("[\n"); err != nil {
		return 0, err
	}
	var total int64
	offset := 0
	first := true
	for {
		opts.Limit = exportBatch
		opts.Offset = offset
		rows, err := d.RawPage(ctx, table, cols, opts)
		if err != nil {
			return total, err
		}
		for _, row := range rows {
			obj := make(map[string]any, len(cols))
			for i, v := range row {
				obj[cols[i].Name] = rawJSONValue(v)
			}
			b, err := json.Marshal(obj)
			if err != nil {
				return total, err
			}
			if !first {
				if _, err := f.WriteString(",\n"); err != nil {
					return total, err
				}
			}
			first = false
			if _, err := f.Write(b); err != nil {
				return total, err
			}
			total++
		}
		if len(rows) < exportBatch {
			break
		}
		offset += exportBatch
		if err := ctx.Err(); err != nil {
			return total, err
		}
	}
	if _, err := f.WriteString("\n]\n"); err != nil {
		return total, err
	}
	return total, nil
}

// InsertRows inserts rows in a single transaction.
func (d *Database) InsertRows(ctx context.Context, table string, columns []string, rows [][]any) (int64, error) {
	done := false
	return d.InsertRowsStream(ctx, table, columns, func() ([][]any, bool, error) {
		if done {
			return nil, true, nil
		}
		done = true
		return rows, true, nil
	})
}

// InsertRowsStream inserts batches of rows in a single transaction. The next
// callback returns the next batch and whether it is the last one. This keeps
// memory usage bounded for large imports.
func (d *Database) InsertRowsStream(ctx context.Context, table string, columns []string, next func() ([][]any, bool, error)) (int64, error) {
	if err := d.ensureWritable(); err != nil {
		return 0, err
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("нет колонок для вставки")
	}
	quoted := make([]string, len(columns))
	ph := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = QuoteIdent(c)
		ph[i] = "?"
	}
	q := "INSERT INTO " + QuoteIdent(table) + " (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.Join(ph, ", ") + ")"
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	var total int64
	for {
		batch, done, err := next()
		if err != nil {
			return total, err
		}
		for _, row := range batch {
			if len(row) != len(columns) {
				return total, fmt.Errorf("в строке %d значений, ожидалось %d", len(row), len(columns))
			}
			if _, err := stmt.ExecContext(ctx, row...); err != nil {
				return total, err
			}
			total++
		}
		if done {
			break
		}
		if err := ctx.Err(); err != nil {
			return total, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

func sqlLiteral(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return "X'" + hex.EncodeToString(x) + "'"
	case time.Time:
		return quoteSQL(x.Format(time.RFC3339Nano))
	case bool:
		if x {
			return "1"
		}
		return "0"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case string:
		return quoteSQL(x)
	default:
		return quoteSQL(fmt.Sprint(x))
	}
}

func quoteSQL(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// DumpSQL writes a complete SQL dump (schema and data) of the database.
func (d *Database) DumpSQL(ctx context.Context, path string) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	objs, err := d.Objects(ctx)
	if err != nil {
		return 0, err
	}
	var schema []string
	seen := map[string]bool{}
	// Include all schema objects from sqlite_master, not only tables/views.
	rows, err := d.conn.QueryContext(ctx,
		`SELECT type, name, sql FROM sqlite_master
		 WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%'
		 ORDER BY CASE type WHEN 'table' THEN 0 WHEN 'index' THEN 1 WHEN 'trigger' THEN 2 ELSE 3 END, name`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var typ, name, sqlText string
		if err := rows.Scan(&typ, &name, &sqlText); err != nil {
			rows.Close()
			return 0, err
		}
		key := typ + ":" + name
		if seen[key] {
			continue
		}
		seen[key] = true
		schema = append(schema, strings.TrimSpace(sqlText)+";")
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	header := fmt.Sprintf("-- msqlite SQL dump\n-- database: %s\n-- generated: %s\n\n",
		filepath.Base(d.Path), time.Now().Format(time.RFC3339))
	if _, err := f.WriteString(header); err != nil {
		return 0, err
	}
	if _, err := f.WriteString("PRAGMA foreign_keys=OFF;\nBEGIN TRANSACTION;\n\n"); err != nil {
		return 0, err
	}
	for _, s := range schema {
		if _, err := f.WriteString(s + "\n"); err != nil {
			return 0, err
		}
	}
	if _, err := f.WriteString("\n"); err != nil {
		return 0, err
	}

	var total int64
	for _, o := range objs {
		if o.Type != "table" {
			continue
		}
		allCols, err := d.Columns(ctx, o.Name)
		if err != nil {
			return total, err
		}
		cols := make([]Column, 0, len(allCols))
		for _, c := range allCols {
			if !c.Hidden {
				cols = append(cols, c)
			}
		}
		if len(cols) == 0 {
			continue
		}
		quoted := make([]string, len(cols))
		for i, c := range cols {
			quoted[i] = QuoteIdent(c.Name)
		}
		selectCols := make([]string, len(cols))
		for i, c := range cols {
			selectCols[i] = QuoteIdent(c.Name)
		}
		q := "SELECT " + strings.Join(selectCols, ", ") + " FROM " + QuoteIdent(o.Name)
		rowIter, err := d.conn.QueryContext(ctx, q)
		if err != nil {
			return total, err
		}
		prefix := "INSERT INTO " + QuoteIdent(o.Name) + " (" + strings.Join(quoted, ", ") + ") VALUES ("
		for rowIter.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rowIter.Scan(ptrs...); err != nil {
				rowIter.Close()
				return total, err
			}
			parts := make([]string, len(vals))
			for i, v := range vals {
				parts[i] = sqlLiteral(v)
			}
			if _, err := f.WriteString(prefix + strings.Join(parts, ", ") + ");\n"); err != nil {
				rowIter.Close()
				return total, err
			}
			total++
		}
		if err := rowIter.Err(); err != nil {
			rowIter.Close()
			return total, err
		}
		rowIter.Close()
	}
	if _, err := f.WriteString("\nCOMMIT;\n"); err != nil {
		return total, err
	}
	return total, nil
}

// Backup creates a consistent backup using VACUUM INTO. The target must not exist.
func (d *Database) Backup(ctx context.Context, path string) error {
	q := "VACUUM INTO " + quoteSQL(path)
	_, err := d.conn.ExecContext(ctx, q)
	return err
}

// WriteCSV writes already materialized rows to a CSV file.
func WriteCSV(path string, columns []string, rows [][]any) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(columns); err != nil {
		return 0, err
	}
	var total int64
	for _, row := range rows {
		rec := make([]string, len(row))
		for i, v := range row {
			rec[i] = rawCSVValue(v)
		}
		if err := w.Write(rec); err != nil {
			return total, err
		}
		total++
	}
	w.Flush()
	return total, w.Error()
}

// WriteJSON writes already materialized rows to a JSON array.
func WriteJSON(path string, columns []string, rows [][]any) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := f.WriteString("[\n"); err != nil {
		return 0, err
	}
	var total int64
	for i, row := range rows {
		obj := make(map[string]any, len(columns))
		for j, v := range row {
			if j < len(columns) {
				obj[columns[j]] = rawJSONValue(v)
			}
		}
		b, err := json.Marshal(obj)
		if err != nil {
			return total, err
		}
		if i > 0 {
			if _, err := f.WriteString(",\n"); err != nil {
				return total, err
			}
		}
		if _, err := f.Write(b); err != nil {
			return total, err
		}
		total++
	}
	if _, err := f.WriteString("\n]\n"); err != nil {
		return total, err
	}
	return total, nil
}

// RestoreRow inserts a previously deleted row, preserving its rowid when possible.
func (d *Database) RestoreRow(ctx context.Context, table string, cols []Column, vals []any, rowID int64) error {
	if err := d.ensureWritable(); err != nil {
		return err
	}
	if len(cols) != len(vals) {
		return fmt.Errorf("нет колонок для вставки")
	}
	alias, ok, err := d.rowidAlias(ctx, table)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("таблица %s не поддерживает доступ по rowid", table)
	}
	ipk := false
	for _, c := range cols {
		if c.PK == 1 && strings.Contains(strings.ToUpper(c.Type), "INT") {
			ipk = true
		}
	}
	names := make([]string, 0, len(cols)+1)
	args := make([]any, 0, len(cols)+1)
	for i, c := range cols {
		if c.Hidden {
			continue
		}
		names = append(names, QuoteIdent(c.Name))
		args = append(args, vals[i])
	}
	if !ipk {
		names = append([]string{QuoteIdent(alias)}, names...)
		args = append([]any{rowID}, args...)
	}
	q := "INSERT INTO " + QuoteIdent(table) + " (" + strings.Join(names, ", ") + ") VALUES (" +
		strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", ") + ")"
	_, err = d.conn.ExecContext(ctx, q, args...)
	return err
}

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"msqlite/internal/db"
)

func (a *App) exportPage(format string) {
	if a.data == nil {
		return
	}
	def := timestampName(a.data.obj.Name+"-page", "."+format)
	a.pathDialog(trf("Экспорт страницы в %s", strings.ToUpper(format)), def, func(path string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		opts := db.PageOptions{
			OrderColumn: a.data.order,
			Desc:        a.data.desc,
			Filter:      a.data.filter,
			Limit:       a.data.size,
			Offset:      a.data.offset,
		}
		rows, err := a.data.db.RawPage(ctx, a.data.obj.Name, a.data.cols, opts)
		if err != nil {
			a.showError(err)
			return
		}
		names := make([]string, len(a.data.cols))
		for i, c := range a.data.cols {
			names[i] = c.Name
		}
		var n int64
		if format == "csv" {
			n, err = db.WriteCSV(path, names, rows)
		} else {
			n, err = db.WriteJSON(path, names, rows)
		}
		if err != nil {
			a.showError(err)
			return
		}
		a.showModal(trf("Экспортировано строк: %d\n%s", n, path), []string{"OK"}, nil)
	})
}

func (a *App) importJSONDialog() {
	if a.data == nil || a.currentPage != pageData {
		a.showModal(tr("Сначала откройте таблицу."), []string{"OK"}, nil)
		return
	}
	if !a.data.canEdit {
		a.showModal(tr("В эту таблицу нельзя вставлять строки."), []string{"OK"}, nil)
		return
	}
	if a.opts.ReadOnly {
		a.showModal(tr("База открыта в режиме только чтения.\nПереключите режим клавишей F8."), []string{"OK"}, nil)
		return
	}
	a.inputPathDialog(tr("Импорт JSON"), a.data.obj.Name+".json", func(path string) {
		a.importJSON(path)
	})
}

func (a *App) importJSON(path string) {
	if a.data == nil {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		a.showError(err)
		return
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.UseNumber()
	var objects []map[string]any
	if err := dec.Decode(&objects); err != nil {
		a.showError(fmt.Errorf(tr("JSON должен быть массивом объектов: %w"), err))
		return
	}
	if len(objects) == 0 {
		a.showError(fmt.Errorf("%s", tr("JSON не содержит строк")))
		return
	}

	byLower := make(map[string]db.Column, len(a.data.cols))
	for _, c := range a.data.cols {
		byLower[strings.ToLower(c.Name)] = c
	}
	keyFor := make(map[string]string)
	for _, obj := range objects {
		for key := range obj {
			lower := strings.ToLower(key)
			if _, ok := byLower[lower]; !ok {
				continue
			}
			if _, seen := keyFor[lower]; !seen {
				keyFor[lower] = key
			}
		}
	}
	var names []string
	var lowers []string
	for _, c := range a.data.cols {
		lower := strings.ToLower(c.Name)
		if _, ok := keyFor[lower]; ok && !c.Hidden {
			names = append(names, c.Name)
			lowers = append(lowers, lower)
		}
	}
	if len(names) == 0 {
		a.showError(fmt.Errorf(tr("в JSON нет колонок таблицы %s"), a.data.obj.Name))
		return
	}

	rows := make([][]any, 0, len(objects))
	for _, obj := range objects {
		vals := make([]any, len(names))
		for j, lower := range lowers {
			key, ok := keyFor[lower]
			if !ok {
				continue
			}
			v, ok := obj[key]
			if !ok {
				continue
			}
			vals[j] = jsonValue(v)
		}
		rows = append(rows, vals)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	count, err := a.data.db.InsertRows(ctx, a.data.obj.Name, names, rows)
	if err != nil {
		a.showError(err)
		return
	}
	a.data.notice = trf("Импортировано строк: %d", count)
	if err := a.data.Load(); err != nil {
		a.showError(err)
		return
	}
	a.showModal(trf("Импортировано строк: %d", count), []string{"OK"}, nil)
}

func jsonValue(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if !strings.ContainsAny(x.String(), ".eE") {
			return x.String()
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case string, bool, float64:
		return x
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprint(x)
		}
		return string(b)
	}
}

func (a *App) exportSelectedRow(format string) {
	if a.data == nil {
		return
	}
	rowID, ok := a.data.rowIDAtRow()
	if !ok {
		a.showError(fmt.Errorf("%s", tr("для этой таблицы нельзя определить rowid строки")))
		return
	}
	def := timestampName(a.data.obj.Name+"-row", "."+format)
	a.pathDialog(trf("Экспорт строки в %s", strings.ToUpper(format)), def, func(path string) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		raw, err := a.data.db.RawRowByID(ctx, a.data.obj.Name, a.data.cols, rowID)
		if err != nil {
			a.showError(err)
			return
		}
		names := make([]string, len(a.data.cols))
		for i, c := range a.data.cols {
			names[i] = c.Name
		}
		var n int64
		if format == "csv" {
			n, err = db.WriteCSV(path, names, [][]any{raw})
		} else {
			n, err = db.WriteJSON(path, names, [][]any{raw})
		}
		if err != nil {
			a.showError(err)
			return
		}
		a.showModal(trf("Экспортировано строк: %d\n%s", n, path), []string{"OK"}, nil)
	})
}

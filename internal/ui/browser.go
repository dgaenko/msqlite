package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rivo/tview"

	"msqlite/internal/db"
)

type entryKind int

const (
	entryParent entryKind = iota
	entryDir
	entryDB
)

type browserEntry struct {
	kind entryKind
	name string
	path string
	size int64
}

func scanBrowser(dir string) ([]browserEntry, error) {
	var out []browserEntry
	parent := filepath.Dir(dir)
	if parent != dir {
		out = append(out, browserEntry{kind: entryParent, name: "..", path: parent})
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var dirs, files []browserEntry
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		switch {
		case info.IsDir():
			dirs = append(dirs, browserEntry{kind: entryDir, name: e.Name(), path: path})
		case info.Mode().IsRegular() && db.IsSQLiteFile(path):
			files = append(files, browserEntry{kind: entryDB, name: e.Name(), path: path, size: info.Size()})
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].name) < strings.ToLower(dirs[j].name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].name) < strings.ToLower(files[j].name)
	})
	out = append(out, dirs...)
	out = append(out, files...)
	return out, nil
}

func (e browserEntry) label() string {
	switch e.kind {
	case entryParent:
		return "[" + colorName(colText) + "]..[-:-:-]"
	case entryDir:
		return "[" + colorName(colText) + "]" + esc(e.name) + "[-:-:-]"
	default:
		return "[" + colorName(colDim) + "]" + esc(e.name) + "[-:-:-]"
	}
}

func (a *App) entryAt(index int) *browserEntry {
	if index < 0 || index >= len(a.entries) {
		return nil
	}
	return &a.entries[index]
}

func (a *App) selectedEntry() *browserEntry {
	if a.busy {
		return nil
	}
	return a.entryAt(a.dbList.GetCurrentItem())
}

func (a *App) closeDatabase() {
	if a.watcher != nil {
		a.watcher.Unwatch()
	}
	if a.current != nil {
		a.current.Close()
		a.current = nil
	}
	a.objects = nil
	a.visible = nil
	a.objCounts = nil
	a.objFilter = ""
}

func (a *App) clearTables(msg string) {
	saved := a.busy
	a.busy = true
	a.table.Clear()
	a.table.SetCell(0, 0, tview.NewTableCell(msg).
		SetTextColor(colDim).
		SetSelectable(false))
	a.busy = saved
}

func (a *App) selectEntry(index int, showErr bool) {
	a.selectEntryForce(index, showErr, false)
}

func (a *App) selectEntryForce(index int, showErr, force bool) {
	e := a.entryAt(index)
	if e == nil {
		return
	}
	if e.kind == entryDB {
		a.previewDatabase(index, showErr, force)
		return
	}
	a.closeDatabase()
	a.clearTables("(выберите базу данных)")
	a.updateHeader()
	a.updateMainStatus()
}

func (a *App) changeDir(path, selectPath string) {
	a.closeDatabase()
	old := a.dir
	a.dir = path
	if err := a.reloadFiles(selectPath, true); err != nil {
		a.dir = old
		a.showError(err)
		a.reloadFiles("", true)
		return
	}
	a.persistSettings()
	a.updateHeader()
}

func (a *App) goUp() {
	parent := filepath.Dir(a.dir)
	if parent == a.dir {
		a.setMainStatus("Корень файловой системы: " + a.dir)
		return
	}
	a.changeDir(parent, a.dir)
}

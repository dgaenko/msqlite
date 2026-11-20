package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"msqlite/internal/db"
)

type dbTab struct {
	path    string
	db      *db.Database
	objects []db.Object
	visible []db.Object
	counts  map[string]rowCount
	filter  string
}

func (a *App) tabIndexForPath(path string) int {
	for i, t := range a.tabs {
		if t != nil && t.path == path {
			return i
		}
	}
	return -1
}

func (a *App) saveActiveTab() {
	if a.tabIndex < 0 || a.tabIndex >= len(a.tabs) || a.tabs[a.tabIndex] == nil {
		return
	}
	t := a.tabs[a.tabIndex]
	t.objects = a.objects
	t.visible = a.visible
	t.counts = a.objCounts
	t.filter = a.objFilter
}

func (a *App) clearSecondaryPages() {
	for _, name := range []string{pageData, pageStructure, pageSQL, pageInsert, pageHelp} {
		a.pages.RemovePage(name)
	}
	a.data = nil
	a.structure = nil
	a.sql = nil
	a.insert = nil
	a.help = nil
	a.stack = nil
	if a.currentPage != pageMain {
		a.currentPage = pageMain
		a.showPage(pageMain)
	}
}

func (a *App) switchTab(i int) {
	if i < 0 || i >= len(a.tabs) || i == a.tabIndex || a.tabs[i] == nil {
		return
	}
	a.saveActiveTab()
	a.clearSecondaryPages()
	a.tabIndex = i
	t := a.tabs[i]
	a.current = t.db
	a.objects = t.objects
	a.visible = t.visible
	a.objCounts = t.counts
	tabFilter := t.filter
	a.objFilter = ""
	if a.watcher != nil {
		a.watcher.Watch(t.path)
	}
	a.undoStack = nil
	a.redoStack = nil
	a.objFilter = tabFilter
	a.renderTables(a.objects, a.objCounts, "")
	a.syncBrowserSelection(t.path)
	a.updateHeader()
	a.updateMainStatus()
}

func (a *App) nextTab() {
	if len(a.tabs) < 2 {
		return
	}
	a.switchTab((a.tabIndex + 1) % len(a.tabs))
}

func (a *App) closeAllTabs() {
	for _, t := range a.tabs {
		if t != nil && t.db != nil {
			t.db.Close()
		}
	}
	a.tabs = nil
	a.tabIndex = -1
}

func (a *App) closeActiveTab() {
	if a.tabIndex < 0 || a.tabIndex >= len(a.tabs) {
		return
	}
	old := a.tabs[a.tabIndex]
	if old != nil && old.db != nil {
		old.db.Close()
	}
	a.tabs = append(a.tabs[:a.tabIndex], a.tabs[a.tabIndex+1:]...)
	a.current = nil
	a.objects = nil
	a.visible = nil
	a.objCounts = nil
	a.objFilter = ""
	a.undoStack = nil
	a.redoStack = nil
	a.clearSecondaryPages()
	if len(a.tabs) == 0 {
		a.tabIndex = -1
		if a.watcher != nil {
			a.watcher.Unwatch()
		}
		a.clearTables(tr("(нет базы данных)"))
		a.updateHeader()
		a.updateMainStatus()
		return
	}
	if a.tabIndex >= len(a.tabs) {
		a.tabIndex = len(a.tabs) - 1
	}
	t := a.tabs[a.tabIndex]
	a.current = t.db
	a.objects = t.objects
	a.visible = t.visible
	a.objCounts = t.counts
	a.objFilter = t.filter
	if a.watcher != nil {
		a.watcher.Watch(t.path)
	}
	a.renderTables(a.objects, a.objCounts, "")
	a.syncBrowserSelection(t.path)
	a.updateHeader()
	a.updateMainStatus()
}

func (a *App) syncBrowserSelection(path string) {
	for i, e := range a.entries {
		if e.path == path {
			a.busy = true
			a.dbList.SetCurrentItem(i)
			a.busy = false
			return
		}
	}
}

func (a *App) openTab(path string, switchNow bool) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(a.dir, path)
	}
	path = filepath.Clean(path)
	if i := a.tabIndexForPath(path); i >= 0 {
		if switchNow {
			a.switchTab(i)
		}
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		a.showError(fmt.Errorf("%s: %w", path, err))
		return err
	}
	if info.IsDir() {
		a.showError(fmt.Errorf("%s: это каталог", path))
		return fmt.Errorf("directory")
	}
	d, err := db.Open(path, a.opts.ReadOnly)
	if err != nil {
		a.showError(err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	objs, err := d.Objects(ctx)
	if err != nil {
		d.Close()
		a.showError(err)
		return err
	}
	a.saveActiveTab()
	tab := &dbTab{path: path, db: d, objects: objs}
	a.tabs = append(a.tabs, tab)
	a.tabIndex = len(a.tabs) - 1
	a.current = d
	a.objects = objs
	a.visible = nil
	a.objCounts = nil
	a.objFilter = ""
	if a.watcher != nil {
		a.watcher.Watch(path)
	}
	a.fillTables(objs, "")
	a.addRecent(path)
	a.syncBrowserSelection(path)
	a.updateHeader()
	a.updateMainStatus()
	if switchNow {
		a.clearSecondaryPages()
	}
	return nil
}

func (a *App) openTabDialog() {
	initial := a.dir + string(os.PathSeparator)
	a.inputDialog(tr("Открыть в новой вкладке"), tr("Путь: "), initial,
		tr("Enter — открыть · Esc — отмена"), func(value string) {
			if strings.TrimSpace(value) == "" {
				return
			}
			a.openTab(value, true)
		})
}

func (a *App) tabBar() string {
	if len(a.tabs) < 2 {
		return ""
	}
	var b strings.Builder
	for i, t := range a.tabs {
		if i > 0 {
			b.WriteString("  ")
		}
		name := filepath.Base(t.path)
		if i == a.tabIndex {
			b.WriteString("[" + fmt.Sprint(i+1) + "]▶" + name)
		} else {
			b.WriteString("[" + fmt.Sprint(i+1) + "]" + name)
		}
	}
	return b.String()
}

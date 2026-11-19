package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"msqlite/internal/db"
)

// Options configures the application.
type Options struct {
	Dir         string
	DirSet      bool
	OpenPath    string
	ReadOnly    bool
	ReadOnlySet bool
	PageSize    int
	PageSizeSet bool
	NoSplash    bool
	Version     string
}

// rowCount is a row count for a table; Est marks an sqlite_stat1 estimate.
type rowCount struct {
	N   int64
	Est bool
}

const (
	pageMain      = "main"
	pageData      = "data"
	pageStructure = "structure"
	pageSQL       = "sql"
	pageInsert    = "insert"
	pageHelp      = "help"
	pageSplash    = "splash"
	dialogPage    = "__dialog"
)

// App is the msqlite TUI application.
type App struct {
	opts  Options
	app   *tview.Application
	pages *tview.Pages

	dir     string
	entries []browserEntry
	dbList  *tview.List
	table   *tview.Table

	mainHeader *tview.TextView
	mainStatus *tview.TextView
	mainHints  *hintView

	current     *db.Database
	objects     []db.Object
	visible     []db.Object
	objCounts   map[string]rowCount
	objFilter   string
	countsDirty bool

	data      *DataPage
	structure *StructurePage
	sql       *SQLPage
	insert    *InsertPage
	help      *HelpPage

	currentPage  string
	stack        []string
	dialogOpen   bool
	dialogEscape func() bool
	dialogSave   func()
	topMenu      *topMenu
	activeModal  *tview.Modal
	modalButtons []string
	modalDone    func(string)
	modalIndex   int
	focus        int
	busy         bool
	sqlHistory   []string
	colLines     bool
	rememberDir  bool
	noticeTimer  *time.Timer
	noticeGen    int

	splashActive bool
	splashText   *tview.TextView
	splashTimer  *time.Timer
	screen       tcell.Screen
	watcher      *dbWatcher
}

// Run starts the TUI. It blocks until the application exits.
func Run(opts Options) error {
	settings := loadSettings()
	setActiveLang(settings.Language)
	if !setActiveTheme(settings.Theme) {
		setActiveTheme(defaultTheme)
	}
	if !opts.PageSizeSet && settings.PageSize >= 1 && settings.PageSize <= 10000 {
		opts.PageSize = settings.PageSize
	}
	if !opts.ReadOnlySet && settings.ReadOnly != nil {
		opts.ReadOnly = *settings.ReadOnly
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 100
	}
	if settings.NoSplash {
		opts.NoSplash = true
	}
	rememberDir := true
	if settings.RememberDir != nil {
		rememberDir = *settings.RememberDir
	}
	if rememberDir && !opts.DirSet && settings.LastDir != "" {
		if info, err := os.Stat(settings.LastDir); err == nil && info.IsDir() {
			opts.Dir = settings.LastDir
		}
	}
	dir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return err
	}
	if info, err := os.Stat(dir); err != nil {
		return fmt.Errorf(tr("каталог %s: %w"), dir, err)
	} else if !info.IsDir() {
		return fmt.Errorf(tr("%s: не каталог"), dir)
	}

	if opts.OpenPath != "" && !db.IsSQLiteFile(opts.OpenPath) {
		return fmt.Errorf(tr("%s: не является файлом SQLite"), opts.OpenPath)
	}
	a := &App{opts: opts, dir: dir, colLines: true, rememberDir: rememberDir, sqlHistory: settings.SQLHistory}
	if len(a.sqlHistory) > 200 {
		a.sqlHistory = a.sqlHistory[len(a.sqlHistory)-200:]
	}
	if settings.ColumnLines != nil {
		a.colLines = *settings.ColumnLines
	}
	applyTheme()
	a.app = tview.NewApplication()
	a.watcher = newDBWatcher(a, time.Second)
	a.app.EnableMouse(true)
	a.app.SetInputCapture(a.onKey)
	a.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		a.screen = screen
		return false
	})
	a.build()
	if err := a.reloadFiles(opts.OpenPath, true); err != nil {
		return err
	}
	a.persistSettings()
	a.pages.AddPage(pageMain, a.mainRoot(), true, true)
	a.currentPage = pageMain
	a.app.SetRoot(a.pages, true)
	a.updateHeader()
	a.updateMainStatus()
	if !a.opts.NoSplash {
		a.showSplash()
	}

	a.watcher.Start()
	err = a.app.Run()
	a.watcher.Stop()
	if a.splashTimer != nil {
		a.splashTimer.Stop()
	}
	if a.current != nil {
		a.current.Close()
	}
	return err
}

func (a *App) build() {
	a.pages = tview.NewPages()

	a.mainHeader = tview.NewTextView().SetDynamicColors(true)
	a.mainHeader.SetBackgroundColor(colBarBg)
	a.mainHeader.SetTextColor(colText)

	a.mainStatus = tview.NewTextView()
	a.mainStatus.SetBackgroundColor(colStatusBg)
	a.mainStatus.SetTextColor(colStatusFg)

	a.mainHints = newHintView()
	a.mainHints.SetBackgroundColor(colHintBg)
	a.mainHints.SetTextColor(colHintFg)
	a.setMainHints()

	a.dbList = tview.NewList()
	styleList(a.dbList, colPanelBg, colText, colDim)
	a.dbList.SetTitle(tr(" Базы и папки "))
	a.dbList.SetFocusFunc(func() { a.panelFocused(0) })
	a.dbList.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if a.busy {
			return
		}
		a.selectEntry(index, false)
	})
	a.dbList.SetSelectedFunc(func(index int, _, _ string, _ rune) {
		if a.busy {
			return
		}
		e := a.entryAt(index)
		if e == nil {
			return
		}
		if e.kind == entryDB {
			if err := a.previewDatabase(index, true, false); err != nil {
				return
			}
			a.app.SetFocus(a.table)
			return
		}
		if e.kind == entryParent {
			a.changeDir(e.path, a.dir)
		} else {
			a.changeDir(e.path, "")
		}
		a.app.SetFocus(a.dbList)
	})

	a.table = tview.NewTable()
	styleObjectTable(a.table)
	a.table.SetTitle(tr(" Таблицы и представления "))
	a.table.SetFocusFunc(func() { a.panelFocused(1) })
	a.table.SetSelectionChangedFunc(func(_, _ int) {
		if !a.busy {
			a.updateMainStatus()
		}
	})
	a.table.SetSelectedFunc(func(row, _ int) {
		if o := a.objectAtRow(row); o != nil {
			a.openData(*o)
		}
	})
}

func (a *App) mainRoot() tview.Primitive {
	panels := tview.NewFlex().
		AddItem(a.dbList, 0, 2, true).
		AddItem(a.table, 0, 3, false)
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.mainHeader, 1, 0, false).
		AddItem(panels, 0, 1, true).
		AddItem(a.mainStatus, 1, 0, false).
		AddItem(a.mainHints, 1, 0, false)
}

func (a *App) panelFocused(panel int) {
	a.focus = panel
	a.setPanelTitle(a.dbList, "Базы и папки", panel == 0)
	a.setPanelTitle(a.table, "Таблицы и представления", panel == 1)
	if panel == 0 {
		a.dbList.SetBorderColor(colFocus)
		a.table.SetBorderColor(colBorder)
	} else {
		a.dbList.SetBorderColor(colBorder)
		a.table.SetBorderColor(colFocus)
	}
	a.updateMainStatus()
}

func (a *App) setPanelTitle(p interface {
	SetTitle(string) *tview.Box
	SetTitleColor(tcell.Color) *tview.Box
}, title string, active bool) {
	label := tr(" " + title + " ")
	if active {
		p.SetTitle(" ▶" + label)
		p.SetTitleColor(colPanelTitle)
	} else {
		p.SetTitle("  " + label)
		p.SetTitleColor(colDim)
	}
}

func (a *App) reloadFiles(selectPath string, force bool) error {
	prev := selectPath
	if prev == "" {
		if a.current != nil {
			prev = a.current.Path
		} else if e := a.selectedEntry(); e != nil {
			prev = e.path
		}
	}
	entries, err := scanBrowser(a.dir)
	if err != nil {
		return err
	}
	a.entries = entries

	a.busy = true
	a.dbList.Clear()
	for _, e := range entries {
		a.dbList.AddItem(e.label(), "", 0, nil)
	}
	if len(entries) == 0 {
		a.dbList.AddItem(tr("(пустая папка)"), "", 0, nil)
	}
	idx := -1
	for i, e := range entries {
		if e.path == prev {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = 0
		for i, e := range entries {
			if e.kind == entryDB {
				idx = i
				break
			}
		}
	}
	if len(entries) > 0 {
		a.dbList.SetCurrentItem(idx)
	}
	a.busy = false

	if len(entries) == 0 {
		a.closeDatabase()
		a.clearTables(tr("(нет базы данных)"))
		a.updateHeader()
		a.updateMainStatus()
		return nil
	}
	a.selectEntryForce(idx, false, force)
	a.updateHeader()
	return nil
}

func (a *App) previewDatabase(index int, showErr, force bool) error {
	e := a.entryAt(index)
	if e == nil || e.kind != entryDB {
		return nil
	}
	if !force && a.current != nil && a.current.Path == e.path {
		a.updateHeader()
		a.updateMainStatus()
		return nil
	}
	if a.current == nil || a.current.Path != e.path {
		a.objFilter = ""
	}

	var keep string
	if a.current != nil && a.current.Path == e.path && len(a.objects) > 0 {
		if i := a.selectedObjectIndex(); i >= 0 {
			keep = a.objects[i].Name
		}
	}

	if a.current != nil {
		a.current.Close()
		a.current = nil
	}
	a.objects = nil

	d, err := db.Open(e.path, a.opts.ReadOnly)
	if err != nil {
		a.closeDatabase()
		a.clearTables(tr("(ошибка открытия)"))
		a.setMainStatus(tr("Ошибка: ") + translateText(err.Error()))
		a.updateHeader()
		if showErr {
			a.showError(err)
		}
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	objs, err := d.Objects(ctx)
	if err != nil {
		d.Close()
		a.closeDatabase()
		a.clearTables(tr("(ошибка открытия)"))
		a.setMainStatus(tr("Ошибка: ") + translateText(err.Error()))
		a.updateHeader()
		if showErr {
			a.showError(err)
		}
		return err
	}
	a.current = d
	a.objects = objs
	a.watcher.Watch(d.Path)
	a.fillTables(objs, keep)
	a.addRecent(e.path)
	a.updateHeader()
	return nil
}

func (a *App) fillTables(objs []db.Object, keep string) {
	counts := a.countObjects(objs)
	a.objCounts = counts
	a.countsDirty = false
	a.renderTables(objs, counts, keep)
}

func (a *App) renderTables(objs []db.Object, counts map[string]rowCount, keep string) {
	objs = a.filterObjects(objs)
	a.visible = objs
	savedBusy := a.busy
	a.busy = true
	a.table.Clear()
	if len(objs) == 0 {
		msg := tr("(нет таблиц)")
		if a.objFilter != "" {
			msg = tr("(нет совпадений)")
		}
		a.table.SetCell(0, 0, tview.NewTableCell(msg).
			SetTextColor(colDim).
			SetSelectable(false))
	} else {
		a.table.SetCell(0, 0, tview.NewTableCell(tr("Имя")).
			SetTextColor(colTitle).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetExpansion(1))
		a.table.SetCell(0, 1, tview.NewTableCell(tr("Записей")).
			SetTextColor(colTitle).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetAlign(tview.AlignRight))
		for i, o := range objs {
			name := tview.Escape(o.Name)
			if o.Type == "view" {
				name += "  " + tview.Escape("[view]")
			}
			a.table.SetCell(i+1, 0, tview.NewTableCell(name).
				SetTextColor(colText).
				SetExpansion(1))
			countText := "—"
			if rc, ok := counts[o.Name]; ok {
				if rc.Est {
					countText = fmt.Sprintf("~%d", rc.N)
				} else {
					countText = fmt.Sprintf("%d", rc.N)
				}
			}
			a.table.SetCell(i+1, 1, tview.NewTableCell(countText).
				SetTextColor(colDim).
				SetAlign(tview.AlignRight).
				SetSelectable(false))
		}
		idx := 0
		for i, o := range objs {
			if o.Name == keep {
				idx = i
				break
			}
		}
		a.table.Select(idx+1, 0)
	}
	a.busy = savedBusy
	a.updateMainStatus()
}

func (a *App) filterObjects(objs []db.Object) []db.Object {
	filter := strings.ToLower(strings.TrimSpace(a.objFilter))
	if filter == "" {
		return objs
	}
	out := make([]db.Object, 0, len(objs))
	for _, o := range objs {
		if strings.Contains(strings.ToLower(o.Name), filter) {
			out = append(out, o)
		}
	}
	return out
}

func (a *App) openObjectFilter() {
	a.focus = 1
	a.inputDialog("Фильтр объектов", "Имя содержит: ", a.objFilter,
		"Enter — применить · пусто — сбросить · Esc — отмена", func(value string) {
			keep := ""
			if o := a.selectedObject(); o != nil {
				keep = o.Name
			}
			a.objFilter = strings.TrimSpace(value)
			a.renderTables(a.objects, a.objCounts, keep)
			a.app.SetFocus(a.table)
			if a.objFilter == "" {
				a.notify(tr("Фильтр объектов сброшен"))
			} else {
				a.notify(tr("Фильтр объектов: ") + a.objFilter)
			}
		})
}

func (a *App) countObjects(objs []db.Object) map[string]rowCount {
	counts := make(map[string]rowCount, len(objs))
	if a.current == nil {
		return counts
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	estimates, _ := a.current.ApproxCounts(ctx)
	for _, o := range objs {
		if n, ok := estimates[o.Name]; ok {
			counts[o.Name] = rowCount{N: n, Est: true}
			continue
		}
		n, err := a.current.Count(ctx, o.Name, "")
		if err != nil {
			continue
		}
		counts[o.Name] = rowCount{N: n}
	}
	return counts
}

func (a *App) computeExactCounts() {
	if a.current == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	counts := make(map[string]rowCount, len(a.objects))
	for _, o := range a.objects {
		n, err := a.current.Count(ctx, o.Name, "")
		if err != nil {
			continue
		}
		counts[o.Name] = rowCount{N: n}
	}
	keep := ""
	if o := a.selectedObject(); o != nil {
		keep = o.Name
	}
	a.objCounts = counts
	a.renderTables(a.objects, counts, keep)
	a.notify(tr("Счётчики строк пересчитаны"))
}

func (a *App) setMainHints() {
	a.mainHints.SetHints(
		"F1", "Справка",
		"F2", "Файл",
		"F3", "Структ.",
		"F4", "SQL",
		"F5", "Обнов.",
		"F8", "Режим",
		"F9", "Меню",
		"F10", "Выход",
		"/", "Фильтр",
		"Bksp", "Вверх",
	)
}

func (a *App) updateHeader() {
	mode := tr("только чтение")
	if !a.opts.ReadOnly {
		mode = tr("чтение/запись")
	}
	line := fmt.Sprintf("[yellow::b] msqlite [-:-:-][white]SQLite manager   [silver]%s[-:-:-]  [gray::d]· %s", esc(a.dir), mode)
	if a.current != nil {
		line += fmt.Sprintf("  [gray::d]· %s", esc(a.current.Name))
	}
	a.mainHeader.SetText(line)
}

func (a *App) updateMainStatus() {
	if a.currentPage == pageMain {
		a.updateHeader()
	}
	if a.current == nil {
		if e := a.selectedEntry(); e != nil && e.kind != entryDB {
			var dirs, dbs int
			for _, x := range a.entries {
				switch x.kind {
				case entryDir, entryParent:
					dirs++
				case entryDB:
					dbs++
				}
			}
			a.setMainStatus(trf("Каталог: %s · папок: %d · баз: %d · Enter — войти, Bksp — вверх",
				a.dir, dirs, dbs))
			return
		}
		if len(a.entries) == 0 {
			a.setMainStatus(tr("В каталоге нет баз данных и подпапок. F2 — открыть путь, F5 — обновить."))
		} else {
			a.setMainStatus(tr("Выберите базу данных. Enter — открыть, F2 — путь, Bksp — вверх."))
		}
		return
	}
	var tables, views int
	for _, o := range a.objects {
		if o.Type == "view" {
			views++
		} else {
			tables++
		}
	}
	size := int64(0)
	if info, err := os.Stat(a.current.Path); err == nil {
		size = info.Size()
	}
	msg := trf("%s · %s · таблиц: %d · представлений: %d",
		a.current.Name, db.FormatBytes(size), tables, views)
	if a.focus == 1 {
		if i := a.selectedObjectIndex(); i >= 0 {
			msg += trf(" · выбрано: %s (%s)", a.visible[i].Name, typeLabel(a.visible[i].Type))
		}
	}
	if a.objFilter != "" {
		msg += tr(" · фильтр: ") + a.objFilter
	}
	a.setMainStatus(msg)
}

func (a *App) setMainStatus(s string) {
	a.mainStatus.SetText(esc(translateText(s)))
}

func (a *App) onKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() == tcell.KeyCtrlC {
		return ev
	}
	if ev.Key() == tcell.KeyRune && ev.Modifiers()&tcell.ModAlt != 0 {
		switch ev.Rune() {
		case 'x', 'X', 'ч', 'Ч':
			if a.splashActive {
				a.dismissSplash()
			}
			a.confirmQuit()
			return nil
		}
	}
	if ev.Key() == tcell.KeyCtrlP && !a.splashActive && !a.dialogOpen {
		a.openPalette()
		return nil
	}
	if a.splashActive {
		a.dismissSplash()
		return nil
	}
	if a.topMenu != nil {
		a.topMenu.handleKey(ev)
		return nil
	}
	if a.dialogOpen {
		if a.activeModal != nil {
			switch ev.Key() {
			case tcell.KeyEnter:
				a.activateModal()
				return nil
			case tcell.KeyEscape, tcell.KeyF10, tcell.KeyF1:
				a.closeDialog()
				return nil
			case tcell.KeyTab, tcell.KeyRight, tcell.KeyDown:
				a.moveModal(1)
				return nil
			case tcell.KeyBacktab, tcell.KeyLeft, tcell.KeyUp:
				a.moveModal(-1)
				return nil
			case tcell.KeyRune:
				if ev.Rune() == ' ' {
					a.activateModal()
				}
			}
			return nil
		}
		if ev.Key() == tcell.KeyCtrlS && a.dialogSave != nil {
			a.dialogSave()
			return nil
		}
		switch ev.Key() {
		case tcell.KeyEscape:
			if a.dialogEscape != nil && a.dialogEscape() {
				return ev
			}
			a.closeDialog()
			return nil
		case tcell.KeyF10, tcell.KeyF1:
			return nil
		}
		return ev
	}

	if ev.Key() == tcell.KeyEscape && a.currentPage == pageSQL && a.sql != nil && a.sql.running {
		a.sql.cancelQuery()
		return nil
	}
	if ev.Key() == tcell.KeyRune && ev.Rune() == '/' && a.currentPage == pageMain && a.focus == 1 {
		a.openObjectFilter()
		return nil
	}

	switch ev.Key() {
	case tcell.KeyF1:
		a.openHelp()
		return nil
	case tcell.KeyF10:
		a.confirmQuit()
		return nil
	case tcell.KeyEscape:
		a.escape()
		return nil
	case tcell.KeyF2:
		a.f2()
		return nil
	case tcell.KeyF3:
		a.f3()
		return nil
	case tcell.KeyF4:
		a.f4()
		return nil
	case tcell.KeyF5:
		a.f5()
		return nil
	case tcell.KeyF8:
		a.toggleWriteMode()
		return nil
	case tcell.KeyF9:
		a.openTopMenu()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if a.currentPage == pageMain {
			a.goUp()
			return nil
		}
	}

	switch a.currentPage {
	case pageMain:
		if ev.Key() == tcell.KeyTab || ev.Key() == tcell.KeyBacktab {
			a.togglePanel()
			return nil
		}
	case pageData:
		if a.data != nil && a.data.HandleKey(ev) {
			return nil
		}
	case pageSQL:
		if a.sql != nil && a.sql.HandleKey(ev) {
			return nil
		}
	case pageInsert:
		if a.insert != nil && a.insert.HandleKey(ev) {
			return nil
		}
	}
	return ev
}

func (a *App) selectedObjectIndex() int {
	if a.current == nil {
		return -1
	}
	row, _ := a.table.GetSelection()
	i := row - 1
	if i < 0 || i >= len(a.visible) {
		return -1
	}
	return i
}

func (a *App) objectAtRow(row int) *db.Object {
	i := row - 1
	if i < 0 || i >= len(a.visible) {
		return nil
	}
	return &a.visible[i]
}

func (a *App) selectedObject() *db.Object {
	if a.current == nil || len(a.visible) == 0 {
		return nil
	}
	i := a.selectedObjectIndex()
	if i < 0 {
		return nil
	}
	return &a.visible[i]
}

func (a *App) openData(o db.Object) {
	if a.current == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cols, err := a.current.Columns(ctx, o.Name)
	if err != nil {
		a.showError(err)
		return
	}
	dp := newDataPage(a, a.current, o, cols, a.opts.PageSize)
	a.data = dp
	a.openPage(pageData, dp.Root())
	if err := dp.Load(); err != nil {
		a.showError(err)
	}
}

func (a *App) f2() {
	switch a.currentPage {
	case pageMain:
		a.openPathDialog()
	case pageData:
		if a.data != nil {
			a.data.showSelectedCell()
		}
	case pageSQL:
		if a.sql != nil {
			a.sql.showSelectedCell()
		}
	case pageStructure:
		if a.structure != nil {
			a.structure.openEditor()
		}
	}
}

func (a *App) f3() {
	if a.current == nil {
		return
	}
	switch a.currentPage {
	case pageMain:
		if o := a.selectedObject(); o != nil {
			a.openStructure(*o)
		}
	case pageData:
		if a.data != nil {
			a.openStructure(a.data.obj)
		}
	}
}

func (a *App) f4() {
	if a.current == nil {
		a.showModal(tr("Сначала выберите базу данных."), []string{"OK"}, nil)
		return
	}
	a.openSQL()
}

func (a *App) f5() {
	switch a.currentPage {
	case pageMain:
		if err := a.reloadFiles("", true); err != nil {
			a.showError(err)
			return
		}
		a.notify(tr("Список обновлён"))
	case pageData:
		if a.data != nil {
			if err := a.data.Load(); err != nil {
				a.showError(err)
				return
			}
			a.notify(tr("Данные обновлены"))
		}
	case pageStructure:
		if a.structure != nil {
			if err := a.structure.Load(); err != nil {
				a.showError(err)
				return
			}
			a.notify(tr("Структура обновлена"))
		}
	}
}

func (a *App) openStructure(o db.Object) {
	if a.current == nil {
		return
	}
	sp := newStructurePage(a, a.current, o)
	a.structure = sp
	a.openPage(pageStructure, sp.Root())
	if err := sp.Load(); err != nil {
		a.showError(err)
	}
}

func (a *App) toggleWriteMode() {
	newReadOnly := !a.opts.ReadOnly
	if a.current == nil {
		a.opts.ReadOnly = newReadOnly
		a.persistSettings()
		a.refreshModeUI()
		return
	}

	path := a.current.Path
	keep := ""
	if a.data != nil && a.currentPage == pageData {
		keep = a.data.obj.Name
	} else if o := a.selectedObject(); o != nil {
		keep = o.Name
	}

	d, err := db.Open(path, newReadOnly)
	if err != nil {
		a.showError(fmt.Errorf(tr("не удалось переключить режим: %w"), err))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	objs, err := d.Objects(ctx)
	if err != nil {
		d.Close()
		a.showError(fmt.Errorf(tr("не удалось переключить режим: %w"), err))
		return
	}

	old := a.current
	a.current = d
	a.watcher.Watch(d.Path)
	a.opts.ReadOnly = newReadOnly
	a.objects = objs
	a.fillTables(objs, keep)
	old.Close()
	a.persistSettings()

	if a.data != nil {
		a.data.db = d
		a.data.updateHeader()
	}
	if a.structure != nil {
		a.structure.db = d
		a.structure.updateHead()
	}
	if a.sql != nil {
		a.sql.db = d
		a.sql.updateMode()
	}
	if a.insert != nil {
		a.insert.db = d
		a.insert.data.db = d
		a.insert.updateHead()
	}
	a.updateHeader()
	a.updateMainStatus()

	switch a.currentPage {
	case pageData:
		if a.data != nil {
			a.data.Load()
		}
	case pageStructure:
		if a.structure != nil {
			a.structure.Load()
		}
	}
}

func (a *App) refreshModeUI() {
	a.updateHeader()
	a.updateMainStatus()
	if a.data != nil {
		a.data.updateHeader()
	}
	if a.sql != nil {
		a.sql.updateMode()
	}
	if a.structure != nil {
		a.structure.updateHead()
	}
	if a.insert != nil {
		a.insert.updateHead()
	}
}

func (a *App) openInsert(p *DataPage) {
	ip := newInsertPage(a, p)
	a.insert = ip
	a.openPage(pageInsert, ip.Root())
	ip.focus()
}

func (a *App) openSQL() {
	a.openSQLWith("")
}

func (a *App) openSQLWith(initial string) {
	if a.current == nil {
		return
	}
	sp := newSQLPage(a, a.current, a.opts.ReadOnly, initial)
	a.sql = sp
	a.openPage(pageSQL, sp.Root())
	a.app.SetFocus(sp.input)
}

func (a *App) openHelp() {
	if a.currentPage == pageHelp {
		return
	}
	hp := newHelpPage(a)
	a.help = hp
	a.openPage(pageHelp, hp.Root())
}

func (a *App) addSQLHistory(q string) {
	if len(a.sqlHistory) > 0 && a.sqlHistory[len(a.sqlHistory)-1] == q {
		return
	}
	a.sqlHistory = append(a.sqlHistory, q)
	if len(a.sqlHistory) > 200 {
		a.sqlHistory = a.sqlHistory[len(a.sqlHistory)-200:]
	}
	a.persistSettings()
}

func (a *App) refreshDataAfterSchema(oldName, newName string) {
	if a.current == nil || a.data == nil || a.data.obj.Name != oldName {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cols, err := a.current.Columns(ctx, newName)
	if err != nil {
		return
	}
	dp := a.data
	dp.obj.Name = newName
	dp.cols = cols
	dp.order = -1
	dp.filter = ""
	dp.offset = 0
	dp.selRow = 0
	dp.selCol = 1
	if err := dp.Load(); err != nil {
		dp.setErrorStatus(err)
	}
	if a.sql != nil {
		a.sql.completionCols = nil
	}
}

func (a *App) refreshObjects() {
	a.reloadObjects("")
}

func (a *App) refreshObjectsSelect(name string) {
	a.reloadObjects(name)
}

func (a *App) reloadObjects(keep string) {
	if a.current == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	objs, err := a.current.Objects(ctx)
	if err != nil {
		return
	}
	if keep == "" {
		if o := a.selectedObject(); o != nil {
			keep = o.Name
		}
	}
	a.objects = objs
	a.fillTables(objs, keep)
}

// externalDatabaseChange applies a change made to the current database file by
// another process. It is called on the UI goroutine through QueueUpdateDraw.
func (a *App) externalDatabaseChange(path string) {
	if a.current == nil || a.current.Path != path {
		return
	}
	if a.watcher == nil || !a.watcher.IsTarget(path) {
		return
	}
	switch a.currentPage {
	case pageMain, pageData, pageStructure:
	default:
		return
	}
	if a.splashActive || a.dialogOpen || a.topMenu != nil {
		return
	}
	if a.refreshCurrentDatabase() {
		a.watcher.Ack(path)
	}
}

// refreshCurrentDatabase reloads the object list and the active page while
// keeping the current selection (table name and data cursor). It reports
// whether the reload succeeded so a transient error can be retried.
func (a *App) refreshCurrentDatabase() bool {
	if a.current == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	objs, err := a.current.Objects(ctx)
	if err != nil {
		return false
	}
	keep := ""
	if o := a.selectedObject(); o != nil {
		keep = o.Name
	}
	if a.currentPage == pageData && a.data != nil {
		keep = a.data.obj.Name
	}
	a.objects = objs
	switch a.currentPage {
	case pageMain:
		a.fillTables(objs, keep)
	case pageData:
		a.renderTables(objs, a.objCounts, keep)
		a.countsDirty = true
		if a.data != nil {
			if obj := findObject(objs, a.data.obj.Name); obj != nil {
				a.data.setObject(*obj)
			}
			if err := a.data.Reload(); err != nil {
				a.data.setErrorStatus(err)
			}
		}
	case pageStructure:
		a.renderTables(objs, a.objCounts, keep)
		a.countsDirty = true
		if a.structure != nil {
			_ = a.structure.Load()
		}
	}
	a.updateHeader()
	a.updateMainStatus()
	return true
}

// refreshCountsIfDirty recalculates row counts that were skipped by an
// automatic refresh while a data or structure page was open.
func (a *App) refreshCountsIfDirty() {
	if !a.countsDirty || a.current == nil {
		return
	}
	a.countsDirty = false
	keep := ""
	if o := a.selectedObject(); o != nil {
		keep = o.Name
	}
	a.objCounts = a.countObjects(a.objects)
	a.renderTables(a.objects, a.objCounts, keep)
}

func findObject(objs []db.Object, name string) *db.Object {
	for i := range objs {
		if objs[i].Name == name {
			return &objs[i]
		}
	}
	return nil
}

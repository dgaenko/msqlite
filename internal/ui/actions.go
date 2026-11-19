package ui

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"msqlite/internal/db"
)

type paletteAction struct {
	label string
	run   func()
}

func (a *App) openPalette() {
	actions := a.paletteActions()
	list := tview.NewList()
	styleList(list, colDialogBg, colDialogFg, colDialogDim)
	list.SetBorder(false)
	list.SetTitle("")
	list.SetWrapAround(true)
	for _, act := range actions {
		act := act
		list.AddItem(act.label, "", 0, func() {
			a.closeDialog()
			act.run()
		})
	}
	runCurrent := func() {
		index := list.GetCurrentItem()
		if index < 0 || index >= len(actions) {
			return
		}
		action := actions[index]
		a.closeDialog()
		action.run()
	}
	a.showListDialog("Действия (Ctrl+P)", 70, list, runCurrent)
}

type menuCategory struct {
	title   string
	actions []paletteAction
}

// menuFrame draws a single-line frame around a focused child.
type menuFrame struct {
	*tview.Box
	child tview.Primitive
}

func newMenuFrame(child tview.Primitive) *menuFrame {
	m := &menuFrame{Box: tview.NewBox(), child: child}
	m.SetBackgroundColor(colMenuBg)
	return m
}

func (m *menuFrame) Draw(screen tcell.Screen) {
	x, y, width, height := m.GetRect()
	if width < 3 || height < 3 {
		return
	}
	bg := tcell.StyleDefault.Background(colMenuBg)
	for yy := y; yy < y+height; yy++ {
		for xx := x; xx < x+width; xx++ {
			screen.SetContent(xx, yy, ' ', nil, bg)
		}
	}
	border := tcell.StyleDefault.Foreground(colMenuBorder).Background(colMenuBg)
	for xx := x + 1; xx < x+width-1; xx++ {
		screen.SetContent(xx, y, tview.Borders.Horizontal, nil, border)
		screen.SetContent(xx, y+height-1, tview.Borders.Horizontal, nil, border)
	}
	for yy := y + 1; yy < y+height-1; yy++ {
		screen.SetContent(x, yy, tview.Borders.Vertical, nil, border)
		screen.SetContent(x+width-1, yy, tview.Borders.Vertical, nil, border)
	}
	screen.SetContent(x, y, tview.Borders.TopLeft, nil, border)
	screen.SetContent(x+width-1, y, tview.Borders.TopRight, nil, border)
	screen.SetContent(x, y+height-1, tview.Borders.BottomLeft, nil, border)
	screen.SetContent(x+width-1, y+height-1, tview.Borders.BottomRight, nil, border)

	m.child.SetRect(x+1, y+1, width-2, height-2)
	m.child.Draw(screen)
}

func (m *menuFrame) Focus(delegate func(p tview.Primitive)) { m.child.Focus(delegate) }
func (m *menuFrame) HasFocus() bool                         { return m.child.HasFocus() }
func (m *menuFrame) Blur()                                  { m.child.Blur() }
func (m *menuFrame) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return m.child.InputHandler()
}
func (m *menuFrame) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return m.child.MouseHandler()
}
func (m *menuFrame) PasteHandler() func(string, func(tview.Primitive)) {
	return m.child.PasteHandler()
}

type menuBar struct {
	*tview.TextView
	menu *topMenu
}

func (b *menuBar) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return b.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if action != tview.MouseLeftClick || b.menu == nil {
			return false, nil
		}
		x, y := event.Position()
		bx, by, _, _ := b.GetRect()
		if y != by || x < bx {
			return false, nil
		}
		if index := b.menu.categoryAt(x - bx); index >= 0 {
			b.menu.selectCategory(index)
			return true, nil
		}
		return false, nil
	})
}

type topMenu struct {
	app        *App
	categories []menuCategory
	index      int
	list       *tview.List
	bar        *menuBar
	root       *tview.Flex
	submenu    *tview.Flex
	left       *tview.Flex
	frame      *menuFrame
}

func (a *App) openTopMenu() {
	categories := a.menuCategories()
	if len(categories) == 0 {
		return
	}
	tm := &topMenu{app: a, categories: categories}
	tm.build()
	a.topMenu = tm
	a.showDialog(tm.root)
	a.app.SetFocus(tm.list)
}

func (tm *topMenu) build() {
	tm.bar = &menuBar{TextView: tview.NewTextView().SetDynamicColors(true), menu: tm}
	tm.bar.SetBackgroundColor(colMenuBg)
	tm.bar.SetTextColor(colMenuBarFg)

	tm.list = tview.NewList()
	styleList(tm.list, colMenuBg, colMenuFg, colMenuFg)
	tm.list.SetBorder(false)
	tm.list.SetBorderPadding(0, 0, 1, 1)
	tm.list.SetMainTextStyle(tcell.StyleDefault.Foreground(colMenuFg).Background(colMenuBg))
	tm.list.SetSecondaryTextStyle(tcell.StyleDefault.Foreground(colMenuFg).Background(colMenuBg))
	tm.list.SetSelectedStyle(tcell.StyleDefault.Foreground(colMenuSelFg).Background(colMenuSelBg))
	tm.list.SetWrapAround(true)

	tm.frame = newMenuFrame(tm.list)

	tm.left = tview.NewFlex()
	tm.submenu = tview.NewFlex().
		AddItem(tm.left, 0, 0, false).
		AddItem(tm.frame, 0, 0, true).
		AddItem(nil, 0, 1, false)
	tm.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tm.bar, 1, 0, false).
		AddItem(tm.submenu, 4, 0, true).
		AddItem(nil, 0, 1, false)

	tm.index = 0
	tm.load()
}

func (tm *topMenu) screenSize() (int, int) {
	if tm.app.screen != nil {
		w, h := tm.app.screen.Size()
		if w > 0 && h > 0 {
			return w, h
		}
	}
	return 80, 24
}

func (tm *topMenu) categoryOffset(index int) int {
	offset := 0
	for i := 0; i < index && i < len(tm.categories); i++ {
		offset += 1 + tview.TaggedStringWidth(tm.categories[i].title) + 1
	}
	return offset
}

func (tm *topMenu) categoryAt(col int) int {
	offset := 0
	for i, cat := range tm.categories {
		width := 1 + tview.TaggedStringWidth(cat.title) + 1
		if col >= offset && col < offset+width {
			return i
		}
		offset += width
	}
	return -1
}

func (tm *topMenu) selectCategory(index int) {
	if len(tm.categories) == 0 {
		return
	}
	if index < 0 {
		index += len(tm.categories)
	}
	if index < 0 || index >= len(tm.categories) {
		return
	}
	tm.index = index
	tm.load()
	tm.app.app.SetFocus(tm.list)
}

func (tm *topMenu) load() {
	if len(tm.categories) == 0 {
		return
	}
	if tm.index < 0 {
		tm.index = len(tm.categories) - 1
	}
	if tm.index >= len(tm.categories) {
		tm.index = 0
	}
	cat := tm.categories[tm.index]
	tm.list.Clear()
	maxLabel := 0
	for _, act := range cat.actions {
		act := act
		tm.list.AddItem(act.label, "", 0, func() {
			tm.app.closeDialog()
			act.run()
		})
		if w := tview.TaggedStringWidth(act.label); w > maxLabel {
			maxLabel = w
		}
	}
	tm.list.SetTitle("")
	if tm.list.GetItemCount() > 0 {
		tm.list.SetCurrentItem(0)
	}

	screenWidth, screenHeight := tm.screenSize()
	offset := tm.categoryOffset(tm.index)
	width := maxLabel + 4
	if width < 12 {
		width = 12
	}
	if offset+width > screenWidth-1 {
		offset = screenWidth - width - 1
		if offset < 0 {
			offset = 0
			width = screenWidth - 1
		}
	}
	height := len(cat.actions) + 2
	if height < 3 {
		height = 3
	}
	if height > screenHeight-2 {
		height = screenHeight - 2
	}
	if height < 3 {
		height = 3
	}
	tm.submenu.ResizeItem(tm.left, offset, 0)
	tm.submenu.ResizeItem(tm.frame, width, 0)
	tm.root.ResizeItem(tm.submenu, height, 0)
	tm.drawBar()
}

func (tm *topMenu) drawBar() {
	selected := fmt.Sprintf("[%s:%s]", colorName(colMenuBarSelFg), colorName(colMenuBarSelBg))
	normal := fmt.Sprintf("[%s:%s]", colorName(colMenuBarFg), colorName(colMenuBg))
	var b strings.Builder
	for i, cat := range tm.categories {
		if i == tm.index {
			b.WriteString(selected + " " + cat.title + " ")
		} else {
			b.WriteString(normal + " " + cat.title + " ")
		}
	}
	b.WriteString(normal + "  ←/→ раздел · ↑/↓ пункт · Enter · Esc ")
	tm.bar.SetText(b.String())
}

func (tm *topMenu) moveCategory(delta int) {
	n := len(tm.categories)
	if n == 0 {
		return
	}
	tm.selectCategory((tm.index + delta) % n)
}

func (tm *topMenu) moveAction(delta int) {
	n := tm.list.GetItemCount()
	if n == 0 {
		return
	}
	i := tm.list.GetCurrentItem() + delta
	if i < 0 {
		i = n - 1
	}
	if i >= n {
		i = 0
	}
	tm.list.SetCurrentItem(i)
}

func (tm *topMenu) activate() {
	cat := tm.categories[tm.index]
	i := tm.list.GetCurrentItem()
	if i < 0 || i >= len(cat.actions) {
		return
	}
	act := cat.actions[i]
	tm.app.closeDialog()
	act.run()
}

func (tm *topMenu) handleKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyF9:
		tm.app.closeDialog()
	case tcell.KeyLeft, tcell.KeyBacktab:
		tm.moveCategory(-1)
	case tcell.KeyRight, tcell.KeyTab:
		tm.moveCategory(1)
	case tcell.KeyUp:
		tm.moveAction(-1)
	case tcell.KeyDown:
		tm.moveAction(1)
	case tcell.KeyHome:
		if tm.list.GetItemCount() > 0 {
			tm.list.SetCurrentItem(0)
		}
	case tcell.KeyEnd:
		if tm.list.GetItemCount() > 0 {
			tm.list.SetCurrentItem(tm.list.GetItemCount() - 1)
		}
	case tcell.KeyEnter:
		tm.activate()
	case tcell.KeyF8:
		tm.app.closeDialog()
		tm.app.toggleWriteMode()
	case tcell.KeyF10:
		tm.app.closeDialog()
		tm.app.confirmQuit()
	case tcell.KeyF1:
		tm.app.closeDialog()
		tm.app.openHelp()
	case tcell.KeyCtrlP:
		tm.app.closeDialog()
		tm.app.openPalette()
	}
}

func (a *App) menuCategories() []menuCategory {
	file := menuCategory{title: "Файл", actions: []paletteAction{
		{"Открыть файл или каталог…", a.openPathDialog},
		{"Создать базу данных…", a.createDatabaseDialog},
		{"Недавние базы…", a.openRecentDialog},
		{"Выход…", a.confirmQuit},
	}}
	categories := []menuCategory{file}

	if a.current != nil {
		mode := "только чтение"
		if !a.opts.ReadOnly {
			mode = "чтение/запись"
		}
		categories = append(categories, menuCategory{title: "База", actions: []paletteAction{
			{"Режим: переключить (" + mode + ")", a.toggleWriteMode},
			{"Резервная копия базы…", a.backupDialog},
			{"Экспорт SQL-дампа…", a.dumpDialog},
			{"VACUUM", func() { a.runMaintenance("VACUUM", "VACUUM", true) }},
			{"ANALYZE", func() { a.runMaintenance("ANALYZE", "ANALYZE", true) }},
			{"Проверка целостности", func() { a.runMaintenance("PRAGMA integrity_check", "Проверка целостности", false) }},
			{"Посчитать строки точно", a.computeExactCounts},
			{"Создать таблицу…", a.createTableDialog},
			{"Создать представление…", a.createViewDialog},
		}})
	}

	if a.data != nil && a.currentPage == pageData {
		actions := []paletteAction{
			{"Экспорт таблицы в CSV…", func() { a.exportTable("csv") }},
			{"Экспорт таблицы в JSON…", func() { a.exportTable("json") }},
			{"Экспорт страницы в CSV…", func() { a.exportPage("csv") }},
			{"Экспорт страницы в JSON…", func() { a.exportPage("json") }},
			{"Экспорт строки в CSV…", func() { a.exportSelectedRow("csv") }},
			{"Экспорт строки в JSON…", func() { a.exportSelectedRow("json") }},
			{"Копировать ячейку", a.copySelectedCell},
			{"Копировать строку", a.copySelectedRow},
			{"Показать BLOB (hex)", a.showSelectedBlob},
			{"Сохранить BLOB в файл…", a.saveSelectedBlob},
			{"Структура таблицы", func() { a.openStructure(a.data.obj) }},
		}
		if a.data.canEdit {
			actions = append(actions,
				paletteAction{"Импорт CSV в таблицу…", a.importCSVDialog},
				paletteAction{"Импорт JSON в таблицу…", a.importJSONDialog},
				paletteAction{"Дублировать строку", a.data.duplicateSelectedRow},
				paletteAction{"Создать индекс…", a.createIndexDialog},
				paletteAction{"Удалить индекс…", a.dropIndexDialog},
				paletteAction{"Создать индекс (шаблон SQL)", a.createIndexTemplate},
				paletteAction{"Очистить таблицу…", func() { a.clearTable(false) }},
				paletteAction{"Удалить таблицу…", func() { a.clearTable(true) }},
			)
		}
		if a.data.filter != "" {
			actions = append(actions, paletteAction{"Сбросить фильтр", a.data.resetFilter})
		}
		categories = append(categories, menuCategory{title: "Данные", actions: actions})
	}

	if a.currentPage == pageStructure && a.structure != nil {
		categories = append(categories, menuCategory{title: "Структура", actions: []paletteAction{
			{"Редактировать структуру…", a.structure.openEditor},
		}})
	}

	if a.current != nil {
		actions := []paletteAction{
			{"SQL-консоль", a.openSQL},
		}
		if a.currentPage == pageSQL && a.sql != nil {
			actions = append(actions, paletteAction{"Конструктор запроса…", a.sql.openQueryBuilder})
		}
		if a.currentPage == pageSQL && a.sql != nil && a.sql.result != nil && a.sql.result.IsSelect {
			actions = append(actions,
				paletteAction{"Экспорт результата в CSV…", func() { a.exportResult("csv") }},
				paletteAction{"Экспорт результата в JSON…", func() { a.exportResult("json") }},
			)
		}
		categories = append(categories, menuCategory{title: "SQL", actions: actions})
	}

	categories = append(categories, menuCategory{title: "Интерфейс", actions: []paletteAction{
		{"Тема: " + themeLabel(themeMC) + themeMarker(themeMC), func() { a.setTheme(themeMC) }},
		{"Тема: " + themeLabel(themeDOS) + themeMarker(themeDOS), func() { a.setTheme(themeDOS) }},
		{"Тема: " + themeLabel(themeDOSBlue) + themeMarker(themeDOSBlue), func() { a.setTheme(themeDOSBlue) }},
		{"Размер страницы: " + fmt.Sprint(a.opts.PageSize) + "…", a.pageSizeDialog},
		{"Разделители колонок: " + onOff(a.colLines), a.toggleColumnLines},
		{"Запоминать последнюю папку: " + onOff(a.rememberDir), a.toggleRememberDir},
		{"Сплэш при запуске: " + onOff(!a.opts.NoSplash), a.toggleSplash},
	}})

	categories = append(categories, menuCategory{title: "Справка", actions: []paletteAction{
		{"Помощь", a.openHelp},
		{"О программе", a.showSplash},
	}})
	return categories
}

func (a *App) paletteActions() []paletteAction {
	var acts []paletteAction
	for _, cat := range a.menuCategories() {
		acts = append(acts, cat.actions...)
	}
	return acts
}

const noticeDuration = 3 * time.Second

func (a *App) notify(text string) {
	a.noticeGen++
	gen := a.noticeGen
	if a.noticeTimer != nil {
		a.noticeTimer.Stop()
	}

	var restore func()
	switch a.currentPage {
	case pageData:
		if d := a.data; d != nil {
			d.notice = text
			d.updateStatus()
			restore = func() {
				if d.notice != text {
					return
				}
				d.notice = ""
				d.updateStatus()
			}
		}
	case pageSQL:
		if s := a.sql; s != nil {
			prev := s.status.GetText(false)
			notice := "[green] " + esc(text) + "[-:-:-]"
			s.status.SetText(notice)
			restore = func() {
				if s.status.GetText(false) != notice {
					return
				}
				s.status.SetText(prev)
			}
		}
	case pageStructure:
		if s := a.structure; s != nil {
			s.notice = text
			s.updateHead()
			restore = func() {
				if s.notice != text {
					return
				}
				s.notice = ""
				s.updateHead()
			}
		}
	default:
		msg := esc(text)
		a.setMainStatus(text)
		restore = func() {
			if a.mainStatus.GetText(false) != msg {
				return
			}
			a.updateMainStatus()
		}
	}
	if restore == nil {
		return
	}
	a.noticeTimer = time.AfterFunc(noticeDuration, func() {
		a.app.QueueUpdateDraw(func() {
			if a.noticeGen != gen {
				return
			}
			restore()
		})
	})
}

func (a *App) copyText(text string) {
	if a.screen == nil {
		a.showError(fmt.Errorf("терминал не поддерживает буфер обмена"))
		return
	}
	a.screen.SetClipboard([]byte(text))
	a.notify("Скопировано в буфер обмена")
}

func (a *App) copySelectedCell() {
	if a.data == nil {
		return
	}
	text, ok := a.data.selectedCellText()
	if !ok {
		return
	}
	a.copyText(text)
}

func (a *App) copySelectedRow() {
	if a.data == nil {
		return
	}
	text, ok := a.data.selectedRowText()
	if !ok {
		return
	}
	a.copyText(text)
}

func (a *App) pathDialog(title, def string, accept func(string)) {
	a.inputDialog(title, "Файл: ", def, "Enter — продолжить · Esc — отмена", func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if value == "~" || strings.HasPrefix(value, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				value = filepath.Join(home, strings.TrimPrefix(value, "~"))
			}
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(a.dir, value)
		}
		value = filepath.Clean(value)
		if info, err := os.Stat(value); err == nil {
			if info.IsDir() {
				a.showError(fmt.Errorf("%s: это каталог", value))
				return
			}
			a.showModal("Файл уже существует:\n"+value+"\n\nПерезаписать?", []string{"Перезаписать", "Отмена"}, func(label string) {
				if label == "Перезаписать" {
					accept(value)
				}
			})
			return
		}
		accept(value)
	})
}

func timestampName(prefix, ext string) string {
	return fmt.Sprintf("%s-%s%s", prefix, time.Now().Format("20060102-150405"), ext)
}

func (a *App) exportTable(format string) {
	if a.data == nil || a.current == nil {
		return
	}
	def := timestampName(a.data.obj.Name, "."+format)
	title := "Экспорт таблицы в " + strings.ToUpper(format)
	a.pathDialog(title, def, func(path string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		opts := db.PageOptions{
			OrderColumn: a.data.order,
			Desc:        a.data.desc,
			Filter:      a.data.filter,
		}
		var n int64
		var err error
		if format == "csv" {
			n, err = a.data.db.ExportTableCSV(ctx, path, a.data.obj.Name, a.data.cols, opts)
		} else {
			n, err = a.data.db.ExportTableJSON(ctx, path, a.data.obj.Name, a.data.cols, opts)
		}
		if err != nil {
			a.showError(err)
			return
		}
		a.showModal(fmt.Sprintf("Экспортировано строк: %d\n%s", n, path), []string{"OK"}, nil)
	})
}

func (a *App) exportResult(format string) {
	if a.sql == nil || a.sql.result == nil {
		return
	}
	def := timestampName("query", "."+format)
	a.pathDialog("Экспорт результата в "+strings.ToUpper(format), def, func(path string) {
		var n int64
		var err error
		if format == "csv" {
			n, err = a.sql.exportResultCSV(path)
		} else {
			n, err = a.sql.exportResultJSON(path)
		}
		if err != nil {
			a.showError(err)
			return
		}
		a.showModal(fmt.Sprintf("Экспортировано строк: %d\n%s", n, path), []string{"OK"}, nil)
	})
}

func (a *App) dumpDialog() {
	if a.current == nil {
		return
	}
	base := strings.TrimSuffix(a.current.Name, filepath.Ext(a.current.Name))
	def := timestampName(base, ".sql")
	a.pathDialog("Экспорт SQL-дампа", def, func(path string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		n, err := a.current.DumpSQL(ctx, path)
		if err != nil {
			a.showError(err)
			return
		}
		a.showModal(fmt.Sprintf("SQL-дамп создан.\nСтрок данных: %d\n%s", n, path), []string{"OK"}, nil)
	})
}

func (a *App) backupDialog() {
	if a.current == nil {
		return
	}
	base := strings.TrimSuffix(a.current.Name, filepath.Ext(a.current.Name))
	def := timestampName(base+"-backup", ".db")
	a.pathDialog("Резервная копия базы", def, func(path string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if _, statErr := os.Stat(path); statErr == nil {
			if err := os.Remove(path); err != nil {
				a.showError(err)
				return
			}
		}
		if err := a.current.Backup(ctx, path); err != nil {
			a.showError(err)
			return
		}
		a.showModal("Резервная копия создана:\n"+path, []string{"OK"}, nil)
	})
}

func (a *App) runMaintenance(sqlText, title string, requireWrite bool) {
	if a.current == nil {
		return
	}
	if requireWrite && a.opts.ReadOnly {
		a.showModal("База открыта в режиме только чтения.\nПереключите режим клавишей F8.", []string{"OK"}, nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	res, err := a.current.Exec(ctx, sqlText, 1000)
	if err != nil {
		a.showError(err)
		return
	}
	if res.IsSelect {
		var b strings.Builder
		for _, row := range res.Rows {
			parts := make([]string, len(row))
			for i, c := range row {
				parts[i] = c.Text
			}
			b.WriteString(strings.Join(parts, " "))
			b.WriteString("\n")
		}
		text := strings.TrimSpace(b.String())
		if text == "" {
			text = "OK"
		}
		a.showTextDialog(title, text)
		return
	}
	a.showModal(fmt.Sprintf("%s завершено. Затронуто строк: %d", title, res.Affected), []string{"OK"}, nil)
}

func (a *App) showTextDialog(title, text string) {
	tv := tview.NewTextView()
	tv.SetDynamicColors(false)
	tv.SetScrollable(true)
	tv.SetWrap(true)
	tv.SetBackgroundColor(colDialogBg)
	tv.SetTextColor(colDialogFg)
	tv.SetBorder(true)
	tv.SetBorderColor(colDialogBorder)
	tv.SetTitleColor(colDialogTitle)
	tv.SetTitle(" " + esc(title) + "  · Esc — закрыть ")
	tv.SetText(text)
	grid := tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(2, 0, 2).
		AddItem(tv, 1, 1, 1, 1, 0, 0, true)
	grid.SetBackgroundColor(colBg)
	a.showDialog(grid)
	a.app.SetFocus(tv)
}

func (a *App) inputPathDialog(title, def string, accept func(string)) {
	a.inputDialog(title, "Файл: ", def, "Enter — открыть · Esc — отмена", func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if value == "~" || strings.HasPrefix(value, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				value = filepath.Join(home, strings.TrimPrefix(value, "~"))
			}
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(a.dir, value)
		}
		value = filepath.Clean(value)
		info, err := os.Stat(value)
		if err != nil {
			a.showError(fmt.Errorf("%s: %w", value, err))
			return
		}
		if info.IsDir() {
			a.showError(fmt.Errorf("%s: это каталог", value))
			return
		}
		accept(value)
	})
}

func (a *App) importCSVDialog() {
	if a.data == nil || a.currentPage != pageData {
		a.showModal("Сначала откройте таблицу.", []string{"OK"}, nil)
		return
	}
	if !a.data.canEdit {
		a.showModal("В эту таблицу нельзя вставлять строки.", []string{"OK"}, nil)
		return
	}
	def := a.data.obj.Name + ".csv"
	a.inputPathDialog("Импорт CSV", def, func(path string) {
		a.importCSV(path)
	})
}

func (a *App) importCSV(path string) {
	if a.data == nil {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		a.showError(err)
		return
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	cols := make([]db.Column, 0, len(a.data.cols))
	for _, c := range a.data.cols {
		if !c.Hidden {
			cols = append(cols, c)
		}
	}
	first, err := r.Read()
	if err == io.EOF {
		a.showError(fmt.Errorf("CSV пуст"))
		return
	}
	if err != nil {
		a.showError(err)
		return
	}

	matches := 0
	for _, h := range first {
		name := strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))
		for _, c := range cols {
			if strings.EqualFold(c.Name, name) {
				matches++
				break
			}
		}
	}
	header := matches > 0
	var names []string
	var indexes []int
	var pending []string
	if header {
		for j, h := range first {
			name := strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))
			for _, c := range cols {
				if strings.EqualFold(c.Name, name) {
					names = append(names, c.Name)
					indexes = append(indexes, j)
					break
				}
			}
		}
		if len(names) == 0 {
			a.showError(fmt.Errorf("в заголовке CSV нет колонок таблицы %s", a.data.obj.Name))
			return
		}
	} else {
		pending = first
		for _, c := range cols {
			names = append(names, c.Name)
			indexes = append(indexes, len(indexes))
		}
	}

	const batchSize = 1000
	rowNo := 0
	next := func() ([][]any, bool, error) {
		batch := make([][]any, 0, batchSize)
		for len(batch) < batchSize {
			var rec []string
			if pending != nil {
				rec = pending
				pending = nil
			} else {
				rec, err = r.Read()
				if err == io.EOF {
					return batch, true, nil
				}
				if err != nil {
					return batch, true, err
				}
			}
			rowNo++
			vals := make([]any, len(indexes))
			for k, idx := range indexes {
				if idx >= len(rec) {
					return batch, true, fmt.Errorf("строка %d: мало значений", rowNo)
				}
				vals[k] = parseEditorValue(rec[idx])
			}
			batch = append(batch, vals)
		}
		return batch, false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	count, err := a.data.db.InsertRowsStream(ctx, a.data.obj.Name, names, next)
	if err != nil {
		a.showError(err)
		return
	}
	a.data.notice = fmt.Sprintf("Импортировано строк: %d", count)
	if err := a.data.Load(); err != nil {
		a.showError(err)
		return
	}
	a.showModal(fmt.Sprintf("Импортировано строк: %d", count), []string{"OK"}, nil)
}

func (a *App) clearTable(drop bool) {
	if a.data == nil || a.current == nil {
		return
	}
	if !a.data.canEdit {
		a.showModal("Эта таблица не поддерживает изменение строк.", []string{"OK"}, nil)
		return
	}
	action := "Очистить таблицу " + a.data.obj.Name + "?"
	sqlText := "DELETE FROM " + db.QuoteIdent(a.data.obj.Name)
	if drop {
		action = "Удалить таблицу " + a.data.obj.Name + "?"
		sqlText = "DROP TABLE " + db.QuoteIdent(a.data.obj.Name)
	}
	if a.opts.ReadOnly {
		a.showModal("База открыта в режиме только чтения.\nПереключите режим клавишей F8.", []string{"OK"}, nil)
		return
	}
	a.showModal(action, []string{"Да", "Отмена"}, func(label string) {
		if label != "Да" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, err := a.current.Exec(ctx, sqlText, 1); err != nil {
			a.showError(err)
			return
		}
		if drop {
			a.back()
			a.refreshObjects()
		} else {
			a.data.Load()
		}
	})
}

func (a *App) createIndexTemplate() {
	if a.data == nil {
		return
	}
	col := "column_name"
	if len(a.data.cols) > 0 {
		col = a.data.cols[0].Name
	}
	a.openSQLWith(fmt.Sprintf("CREATE INDEX idx_%s_%s ON %s (%s);\n",
		a.data.obj.Name, col, db.QuoteIdent(a.data.obj.Name), db.QuoteIdent(col)))
}

func (a *App) showSelectedBlob() {
	if a.data == nil {
		return
	}
	cell, ok := a.data.selectedCell()
	if !ok || !cell.Blob {
		a.showModal("Выбранная ячейка не является BLOB.", []string{"OK"}, nil)
		return
	}
	rowID, ok := a.data.rowIDAtRow()
	if !ok {
		a.showError(fmt.Errorf("не удалось определить rowid строки"))
		return
	}
	col := a.data.selectedColumn()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, isBlob, err := a.data.db.FetchBlob(ctx, a.data.obj.Name, col.Name, rowID)
	if err != nil {
		a.showError(err)
		return
	}
	if !isBlob {
		a.showModal("Значение не является BLOB.", []string{"OK"}, nil)
		return
	}
	a.showTextDialog(fmt.Sprintf("%s.%s · %d байт", a.data.obj.Name, col.Name, len(data)), hexDump(data))
}

func (a *App) saveSelectedBlob() {
	if a.data == nil {
		return
	}
	cell, ok := a.data.selectedCell()
	if !ok || !cell.Blob {
		a.showModal("Выбранная ячейка не является BLOB.", []string{"OK"}, nil)
		return
	}
	rowID, ok := a.data.rowIDAtRow()
	if !ok {
		a.showError(fmt.Errorf("не удалось определить rowid строки"))
		return
	}
	col := a.data.selectedColumn()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, isBlob, err := a.data.db.FetchBlob(ctx, a.data.obj.Name, col.Name, rowID)
	if err != nil {
		a.showError(err)
		return
	}
	if !isBlob {
		a.showModal("Значение не является BLOB.", []string{"OK"}, nil)
		return
	}
	def := fmt.Sprintf("%s-%s-%d.bin", a.data.obj.Name, col.Name, rowID)
	a.pathDialog("Сохранить BLOB", def, func(path string) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			a.showError(err)
			return
		}
		a.showModal(fmt.Sprintf("Сохранено %d байт:\n%s", len(data), path), []string{"OK"}, nil)
	})
}

func hexDump(data []byte) string {
	const max = 64 * 1024
	truncated := false
	if len(data) > max {
		data = data[:max]
		truncated = true
	}
	var b strings.Builder
	for offset := 0; offset < len(data); offset += 16 {
		end := offset + 16
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]
		fmt.Fprintf(&b, "%08x  ", offset)
		for i := 0; i < 16; i++ {
			if i < len(chunk) {
				fmt.Fprintf(&b, "%02x ", chunk[i])
			} else {
				b.WriteString("   ")
			}
			if i == 7 {
				b.WriteByte(' ')
			}
		}
		b.WriteString(" |")
		for _, c := range chunk {
			if c >= 32 && c < 127 {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("|\n")
	}
	if truncated {
		b.WriteString("\n... показаны первые 64 KiB\n")
	}
	return b.String()
}

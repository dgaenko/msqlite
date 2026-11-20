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
		list.AddItem(" "+act.label+" ", "", 0, func() {
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
		label := " " + act.label + " "
		tm.list.AddItem(label, "", 0, func() {
			tm.app.closeDialog()
			act.run()
		})
		if w := tview.TaggedStringWidth(label); w > maxLabel {
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
	b.WriteString(normal + tr("  ←/→ раздел · ↑/↓ пункт · Enter · Esc "))
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
	file := menuCategory{title: tr("Файл"), actions: []paletteAction{
		{tr("Открыть файл или каталог…"), a.openPathDialog},
		{tr("Открыть в новой вкладке…"), a.openTabDialog},
		{tr("Создать базу данных…"), a.createDatabaseDialog},
		{tr("Недавние базы…"), a.openRecentDialog},
		{tr("Выход…"), a.confirmQuit},
	}}
	if len(a.tabs) > 1 {
		file.actions = append(file.actions,
			paletteAction{tr("Следующая вкладка"), a.nextTab},
			paletteAction{tr("Закрыть вкладку"), a.closeActiveTab},
		)
	}
	categories := []menuCategory{file}

	if a.current != nil {
		mode := tr("только чтение")
		if !a.opts.ReadOnly {
			mode = tr("чтение/запись")
		}
		categories = append(categories, menuCategory{title: tr("База"), actions: []paletteAction{
			{trf("Режим: переключить (%s)", mode), a.toggleWriteMode},
			{tr("Резервная копия базы…"), a.backupDialog},
			{tr("Экспорт SQL-дампа…"), a.dumpDialog},
			{"VACUUM", func() { a.runMaintenance("VACUUM", "VACUUM", true) }},
			{"ANALYZE", func() { a.runMaintenance("ANALYZE", "ANALYZE", true) }},
			{tr("Проверка целостности"), func() {
				a.runMaintenance("PRAGMA integrity_check", tr("Проверка целостности"), false)
			}},
			{tr("Посчитать строки точно"), a.computeExactCounts},
			{tr("Создать таблицу…"), a.createTableDialog},
			{tr("Создать представление…"), a.createViewDialog},
			{tr("Обзор схемы…"), a.openSchemaOverview},
		}})
	}

	if a.data != nil && a.currentPage == pageData {
		actions := []paletteAction{
			{tr("Отменить изменение"), a.undoLast},
			{tr("Повторить изменение"), a.redoLast},
			{tr("История фильтров…"), a.openFilterHistory},
			{tr("История сортировок…"), a.openSortHistory},
			{tr("Перейти по внешнему ключу…"), a.openForeignKey},
			{tr("Экспорт таблицы в CSV…"), func() { a.exportTable("csv") }},
			{tr("Экспорт таблицы в JSON…"), func() { a.exportTable("json") }},
			{tr("Экспорт страницы в CSV…"), func() { a.exportPage("csv") }},
			{tr("Экспорт страницы в JSON…"), func() { a.exportPage("json") }},
			{tr("Экспорт строки в CSV…"), func() { a.exportSelectedRow("csv") }},
			{tr("Экспорт строки в JSON…"), func() { a.exportSelectedRow("json") }},
			{tr("Копировать ячейку"), a.copySelectedCell},
			{tr("Копировать строку"), a.copySelectedRow},
			{tr("Показать BLOB (hex)"), a.showSelectedBlob},
			{tr("Сохранить BLOB в файл…"), a.saveSelectedBlob},
			{tr("Структура таблицы"), func() { a.openStructure(a.data.obj) }},
			{tr("Поиск…"), a.openSearch},
		}
		if a.data.canEdit {
			actions = append(actions,
				paletteAction{tr("Импорт CSV в таблицу…"), a.importCSVDialog},
				paletteAction{tr("Импорт JSON в таблицу…"), a.importJSONDialog},
				paletteAction{tr("Дублировать строку"), a.data.duplicateSelectedRow},
				paletteAction{tr("Создать индекс…"), a.createIndexDialog},
				paletteAction{tr("Удалить индекс…"), a.dropIndexDialog},
				paletteAction{tr("Создать индекс (шаблон SQL)"), a.createIndexTemplate},
				paletteAction{tr("Очистить таблицу…"), func() { a.clearTable(false) }},
				paletteAction{tr("Удалить таблицу…"), func() { a.clearTable(true) }},
			)
		}
		if a.data.filter != "" {
			actions = append(actions, paletteAction{tr("Сбросить фильтр"), a.data.resetFilter})
		}
		categories = append(categories, menuCategory{title: tr("Данные"), actions: actions})
	}

	if a.currentPage == pageStructure && a.structure != nil {
		categories = append(categories, menuCategory{title: tr("Структура"), actions: []paletteAction{
			{tr("Редактировать структуру…"), a.structure.openEditor},
			{tr("Создать триггер…"), a.createTrigger},
			{tr("Удалить триггер…"), a.dropTriggerDialog},
		}})
	}

	if a.current != nil {
		actions := []paletteAction{
			{tr("SQL-консоль"), a.openSQL},
			{tr("Поиск…"), a.openSearch},
		}
		if a.currentPage == pageSQL && a.sql != nil {
			actions = append(actions,
				paletteAction{tr("Конструктор запроса…"), a.sql.openQueryBuilder},
				paletteAction{tr("Сохранить запрос…"), a.saveQueryDialog},
				paletteAction{tr("Сохранённые запросы…"), a.openSavedQueries},
			)
		}
		if a.currentPage == pageSQL && a.sql != nil && a.sql.result != nil && a.sql.result.IsSelect {
			actions = append(actions,
				paletteAction{tr("Экспорт результата в CSV…"), func() { a.exportResult("csv") }},
				paletteAction{tr("Экспорт результата в JSON…"), func() { a.exportResult("json") }},
			)
		}
		categories = append(categories, menuCategory{title: "SQL", actions: actions})
	}

	categories = append(categories, menuCategory{title: tr("Интерфейс"), actions: []paletteAction{
		{trf("Тема: %s…", themeLabel(activeTheme)), a.themeDialog},
		{trf("Размер страницы: %d…", a.opts.PageSize), a.pageSizeDialog},
		{trf("Разделители колонок: %s", onOff(a.colLines)), a.toggleColumnLines},
		{trf("Запоминать последнюю папку: %s", onOff(a.rememberDir)), a.toggleRememberDir},
		{trf("Сплэш при запуске: %s", onOff(!a.opts.NoSplash)), a.toggleSplash},
		{trf("Язык: %s…", languageName(activeLang)), a.languageDialog},
	}})

	categories = append(categories, menuCategory{title: tr("Справка"), actions: []paletteAction{
		{tr("Помощь"), a.openHelp},
		{tr("О программе"), a.showSplash},
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
	text = translateText(text)
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
		a.showError(fmt.Errorf("%s", tr("терминал не поддерживает буфер обмена")))
		return
	}
	a.screen.SetClipboard([]byte(text))
	a.notify(tr("Скопировано в буфер обмена"))
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
			a.showModal(trf("Файл уже существует:\n%s\n\nПерезаписать?", value), []string{"Перезаписать", "Отмена"}, func(label string) {
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
	title := trf("Экспорт таблицы в %s", strings.ToUpper(format))
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
	a.pathDialog(trf("Экспорт результата в %s", strings.ToUpper(format)), def, func(path string) {
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
	a.pathDialog(tr("Экспорт SQL-дампа"), def, func(path string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		n, err := a.current.DumpSQL(ctx, path)
		if err != nil {
			a.showError(err)
			return
		}
		a.showModal(trf("SQL-дамп создан.\nСтрок данных: %d\n%s", n, path), []string{"OK"}, nil)
	})
}

func (a *App) backupDialog() {
	if a.current == nil {
		return
	}
	base := strings.TrimSuffix(a.current.Name, filepath.Ext(a.current.Name))
	def := timestampName(base+"-backup", ".db")
	a.pathDialog(tr("Резервная копия базы"), def, func(path string) {
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
		a.showModal(trf("Резервная копия создана:\n%s", path), []string{"OK"}, nil)
	})
}

func (a *App) runMaintenance(sqlText, title string, requireWrite bool) {
	if a.current == nil {
		return
	}
	if requireWrite && a.opts.ReadOnly {
		a.showModal(tr("База открыта в режиме только чтения.\nПереключите режим клавишей F8."), []string{"OK"}, nil)
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
	a.showModal(trf("%s завершено. Затронуто строк: %d", tr(title), res.Affected), []string{"OK"}, nil)
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
	tv.SetTitle(" " + esc(title) + tr("  · Esc — закрыть "))
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
	a.inputDialog(title, tr("Файл: "), def, tr("Enter — открыть · Esc — отмена"), func(value string) {
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
		a.showModal(tr("Сначала откройте таблицу."), []string{"OK"}, nil)
		return
	}
	if !a.data.canEdit {
		a.showModal(tr("В эту таблицу нельзя вставлять строки."), []string{"OK"}, nil)
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
		a.showError(fmt.Errorf("%s", tr("CSV пуст")))
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
			a.showError(fmt.Errorf(tr("в заголовке CSV нет колонок таблицы %s"), a.data.obj.Name))
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
	a.data.notice = trf("Импортировано строк: %d", count)
	if err := a.data.Load(); err != nil {
		a.showError(err)
		return
	}
	a.showModal(trf("Импортировано строк: %d", count), []string{"OK"}, nil)
}

func (a *App) clearTable(drop bool) {
	if a.data == nil || a.current == nil {
		return
	}
	if !a.data.canEdit {
		a.showModal(tr("Эта таблица не поддерживает изменение строк."), []string{"OK"}, nil)
		return
	}
	action := trf("Очистить таблицу %s?", a.data.obj.Name)
	sqlText := "DELETE FROM " + db.QuoteIdent(a.data.obj.Name)
	if drop {
		action = trf("Удалить таблицу %s?", a.data.obj.Name)
		sqlText = "DROP TABLE " + db.QuoteIdent(a.data.obj.Name)
	}
	if a.opts.ReadOnly {
		a.showModal(tr("База открыта в режиме только чтения.\nПереключите режим клавишей F8."), []string{"OK"}, nil)
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
		a.showModal(tr("Выбранная ячейка не является BLOB."), []string{"OK"}, nil)
		return
	}
	rowID, ok := a.data.rowIDAtRow()
	if !ok {
		a.showError(fmt.Errorf("%s", tr("не удалось определить rowid строки")))
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
		a.showModal(tr("Значение не является BLOB."), []string{"OK"}, nil)
		return
	}
	a.showTextDialog(trf("%s.%s · %d байт", a.data.obj.Name, col.Name, len(data)), hexDump(data))
}

func (a *App) saveSelectedBlob() {
	if a.data == nil {
		return
	}
	cell, ok := a.data.selectedCell()
	if !ok || !cell.Blob {
		a.showModal(tr("Выбранная ячейка не является BLOB."), []string{"OK"}, nil)
		return
	}
	rowID, ok := a.data.rowIDAtRow()
	if !ok {
		a.showError(fmt.Errorf("%s", tr("не удалось определить rowid строки")))
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
		a.showModal(tr("Значение не является BLOB."), []string{"OK"}, nil)
		return
	}
	def := fmt.Sprintf("%s-%s-%d.bin", a.data.obj.Name, col.Name, rowID)
	a.pathDialog(tr("Сохранить BLOB"), def, func(path string) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			a.showError(err)
			return
		}
		a.showModal(trf("Сохранено %d байт:\n%s", len(data), path), []string{"OK"}, nil)
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
		b.WriteString(tr("\n... показаны первые 64 KiB\n"))
	}
	return b.String()
}

func (a *App) rememberFilter(table, filter string) {
	entry := filterEntry{Table: table, Filter: filter}
	out := []filterEntry{entry}
	for _, e := range a.filterHistory {
		if e.Table == table && e.Filter == filter {
			continue
		}
		out = append(out, e)
		if len(out) >= 30 {
			break
		}
	}
	a.filterHistory = out
	a.persistSettings()
}

func (a *App) rememberSort(table, column string, desc bool) {
	entry := sortEntry{Table: table, Column: column, Desc: desc}
	out := []sortEntry{entry}
	for _, e := range a.sortHistory {
		if e.Table == table && e.Column == column && e.Desc == desc {
			continue
		}
		out = append(out, e)
		if len(out) >= 30 {
			break
		}
	}
	a.sortHistory = out
	a.persistSettings()
}

func (a *App) openFilterHistory() {
	if a.data == nil {
		a.showModal(tr("Сначала откройте таблицу."), []string{"OK"}, nil)
		return
	}
	var entries []filterEntry
	for _, e := range a.filterHistory {
		if e.Table == a.data.obj.Name {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		a.showModal(tr("История фильтров пуста."), []string{"OK"}, nil)
		return
	}
	list := tview.NewList()
	styleList(list, colDialogBg, colDialogFg, colDialogDim)
	list.SetBorder(false)
	list.SetWrapAround(true)
	for _, e := range entries {
		list.AddItem(" "+e.Filter+" ", "", 0, nil)
	}
	choose := func() {
		i := list.GetCurrentItem()
		if i < 0 || i >= len(entries) {
			return
		}
		filter := entries[i].Filter
		a.closeDialog()
		a.data.applyFilter(filter)
	}
	list.SetSelectedFunc(func(int, string, string, rune) { choose() })
	a.showListDialog(tr("История фильтров"), 72, list, choose)
}

func (a *App) openSortHistory() {
	if a.data == nil {
		a.showModal(tr("Сначала откройте таблицу."), []string{"OK"}, nil)
		return
	}
	var entries []sortEntry
	for _, e := range a.sortHistory {
		if e.Table == a.data.obj.Name {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		a.showModal(tr("История сортировок пуста."), []string{"OK"}, nil)
		return
	}
	list := tview.NewList()
	styleList(list, colDialogBg, colDialogFg, colDialogDim)
	list.SetBorder(false)
	list.SetWrapAround(true)
	for _, e := range entries {
		dir := "ASC"
		if e.Desc {
			dir = "DESC"
		}
		list.AddItem(" "+e.Column+" "+dir+" ", "", 0, nil)
	}
	choose := func() {
		i := list.GetCurrentItem()
		if i < 0 || i >= len(entries) {
			return
		}
		entry := entries[i]
		a.closeDialog()
		idx := -1
		for j, c := range a.data.cols {
			if c.Name == entry.Column {
				idx = j
				break
			}
		}
		if idx < 0 {
			a.notify(tr("Колонка не найдена"))
			return
		}
		a.data.order = idx
		a.data.desc = entry.Desc
		a.data.offset = 0
		a.data.Load()
	}
	list.SetSelectedFunc(func(int, string, string, rune) { choose() })
	a.showListDialog(tr("История сортировок"), 64, list, choose)
}

func (a *App) saveQueryDialog() {
	if a.sql == nil {
		return
	}
	sqlText := strings.TrimSpace(a.sql.input.GetText())
	if sqlText == "" {
		a.notify(tr("Запрос пуст"))
		return
	}
	a.inputDialog(tr("Сохранить запрос"), tr("Имя: "), "", tr("Enter — сохранить · Esc — отмена"), func(value string) {
		name := strings.TrimSpace(value)
		if name == "" {
			name = fmt.Sprintf("Query %d", len(a.savedQueries)+1)
		}
		out := []savedQuery{{Name: name, SQL: sqlText}}
		for _, q := range a.savedQueries {
			if q.Name != name {
				out = append(out, q)
			}
		}
		if len(out) > 100 {
			out = out[:100]
		}
		a.savedQueries = out
		a.persistSettings()
		a.notify(tr("Запрос сохранён"))
	})
}

func (a *App) openSavedQueries() {
	if a.sql == nil {
		return
	}
	if len(a.savedQueries) == 0 {
		a.showModal(tr("Сохранённых запросов нет."), []string{"OK"}, nil)
		return
	}
	list := tview.NewList()
	styleList(list, colDialogBg, colDialogFg, colDialogDim)
	list.SetBorder(false)
	list.SetWrapAround(true)
	for _, q := range a.savedQueries {
		list.AddItem(" "+q.Name+" ", "", 0, nil)
	}
	choose := func() {
		i := list.GetCurrentItem()
		if i < 0 || i >= len(a.savedQueries) {
			return
		}
		sqlText := a.savedQueries[i].SQL
		a.closeDialog()
		a.sql.input.SetText(sqlText, true)
		a.sql.setFocusInput()
	}
	list.SetSelectedFunc(func(int, string, string, rune) { choose() })
	a.showListDialog(tr("Сохранённые запросы"), 72, list, choose)
}

func (a *App) currentObject() *db.Object {
	if a.data != nil {
		o := a.data.obj
		return &o
	}
	if a.structure != nil {
		o := a.structure.obj
		return &o
	}
	return nil
}

func (a *App) openForeignKey() {
	if a.data == nil || a.current == nil {
		return
	}
	row, _ := a.data.table.GetSelection()
	rowID, ok := a.data.rowIDAt(row)
	if !ok {
		a.notify(tr("Строка не выбрана"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	raw, err := a.data.db.RawRowByID(ctx, a.data.obj.Name, a.data.cols, rowID)
	if err != nil {
		a.showError(err)
		return
	}
	fks, err := a.data.db.ForeignKeys(ctx, a.data.obj.Name)
	if err != nil {
		a.showError(err)
		return
	}
	type target struct {
		table  string
		column string
		value  any
		label  string
	}
	var targets []target
	for _, fk := range fks {
		idx := -1
		for i, c := range a.data.cols {
			if c.Name == fk.From {
				idx = i
				break
			}
		}
		if idx < 0 || idx >= len(raw) || raw[idx] == nil {
			continue
		}
		to := fk.To
		if to == "" {
			ocols, err := a.data.db.Columns(ctx, fk.Table)
			if err != nil {
				continue
			}
			for _, c := range ocols {
				if c.PK > 0 {
					to = c.Name
					break
				}
			}
		}
		if to == "" {
			continue
		}
		targets = append(targets, target{
			table: fk.Table, column: to, value: raw[idx],
			label: fk.From + " → " + fk.Table + "." + to + " = " + fmt.Sprint(raw[idx]),
		})
	}
	if len(targets) == 0 {
		a.showModal(tr("У выбранной строки нет заполненных внешних ключей."), []string{"OK"}, nil)
		return
	}
	list := tview.NewList()
	styleList(list, colDialogBg, colDialogFg, colDialogDim)
	list.SetBorder(false)
	list.SetWrapAround(true)
	for _, t := range targets {
		list.AddItem(" "+t.label+" ", "", 0, nil)
	}
	choose := func() {
		i := list.GetCurrentItem()
		if i < 0 || i >= len(targets) {
			return
		}
		t := targets[i]
		a.closeDialog()
		var obj *db.Object
		for j := range a.objects {
			if a.objects[j].Name == t.table {
				obj = &a.objects[j]
				break
			}
		}
		if obj == nil {
			a.notify(tr("Таблица не найдена"))
			return
		}
		lit := sqlValueLiteral(t.value)
		a.openDataFilter(*obj, db.QuoteIdent(t.column)+" = "+lit)
	}
	list.SetSelectedFunc(func(int, string, string, rune) { choose() })
	a.showListDialog(tr("Внешние ключи"), 72, list, choose)
}

func sqlValueLiteral(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + strings.ReplaceAll(x, "'", "''") + "'"
	case []byte:
		return "X'" + strings.ToUpper(fmt.Sprintf("%x", x)) + "'"
	case bool:
		if x {
			return "1"
		}
		return "0"
	case time.Time:
		return "'" + x.Format(time.RFC3339) + "'"
	default:
		return fmt.Sprint(x)
	}
}

func (a *App) createTrigger() {
	o := a.currentObject()
	if o == nil {
		return
	}
	tpl := "CREATE TRIGGER trigger_name\nAFTER INSERT ON " + db.QuoteIdent(o.Name) + "\nBEGIN\n    -- statements\nEND;"
	a.openSQLWith(tpl)
}

func (a *App) dropTriggerDialog() {
	o := a.currentObject()
	if o == nil || a.current == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	triggers, err := a.current.Triggers(ctx, o.Name)
	if err != nil {
		a.showError(err)
		return
	}
	if len(triggers) == 0 {
		a.showModal(tr("У таблицы нет триггеров."), []string{"OK"}, nil)
		return
	}
	list := tview.NewList()
	styleList(list, colDialogBg, colDialogFg, colDialogDim)
	list.SetBorder(false)
	list.SetWrapAround(true)
	for _, trg := range triggers {
		list.AddItem(" "+trg.Name+" ", "", 0, nil)
	}
	drop := func() {
		i := list.GetCurrentItem()
		if i < 0 || i >= len(triggers) {
			return
		}
		name := triggers[i].Name
		a.closeDialog()
		a.showModal(trf("Удалить триггер %s?", name), []string{"Удалить", "Отмена"}, func(label string) {
			if label != "Удалить" {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if _, err := a.current.Exec(ctx, "DROP TRIGGER "+db.QuoteIdent(name), 1); err != nil {
				a.showError(err)
				return
			}
			a.notify(tr("Триггер удалён: ") + name)
		})
	}
	list.SetSelectedFunc(func(int, string, string, rune) { drop() })
	a.showListDialog(tr("Удалить триггер"), 64, list, drop)
}

func (a *App) openSchemaOverview() {
	if a.current == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	objs, err := a.current.Objects(ctx)
	if err != nil {
		a.showError(err)
		return
	}
	var b strings.Builder
	for _, o := range objs {
		fmt.Fprintf(&b, "%s (%s)\n", o.Name, o.Type)
		cols, err := a.current.Columns(ctx, o.Name)
		if err == nil {
			for _, c := range cols {
				if c.Hidden {
					continue
				}
				line := "  " + c.Name + " " + c.Type
				if c.PK > 0 {
					line += " PK"
				}
				if c.NotNull {
					line += " NOT NULL"
				}
				if c.HasDef {
					line += " DEFAULT " + c.Default
				}
				b.WriteString(line + "\n")
			}
		}
		fks, err := a.current.ForeignKeys(ctx, o.Name)
		if err == nil {
			for _, fk := range fks {
				fmt.Fprintf(&b, "  FK %s -> %s.%s\n", fk.From, fk.Table, fk.To)
			}
		}
		b.WriteString("\n")
	}
	a.showTextDialog(tr("Схема базы: ")+a.current.Name, strings.TrimSpace(b.String()))
}

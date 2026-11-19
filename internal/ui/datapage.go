package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"msqlite/internal/db"
)

// DataPage displays rows of a table or view.
type DataPage struct {
	app     *App
	db      *db.Database
	obj     db.Object
	cols    []db.Column
	size    int
	offset  int
	total   int64
	order   int
	desc    bool
	filter  string
	selCol  int
	selRow  int
	rows    []db.Row
	rowIDs  []int64
	canEdit bool
	err     error
	notice  string

	root    *tview.Flex
	header  *tview.TextView
	table   *tview.Table
	status  *tview.TextView
	preview *tview.TextView
	hints   *tview.TextView
}

func newDataPage(a *App, d *db.Database, obj db.Object, cols []db.Column, size int) *DataPage {
	p := &DataPage{
		app:    a,
		db:     d,
		obj:    obj,
		cols:   cols,
		size:   size,
		order:  -1,
		selCol: 1,
	}
	p.setObject(obj)
	p.build()
	return p
}

func (p *DataPage) setObject(obj db.Object) {
	upper := strings.ToUpper(obj.SQL)
	p.obj = obj
	p.canEdit = obj.Type == "table" &&
		!strings.Contains(upper, "WITHOUT ROWID") &&
		!strings.Contains(upper, "VIRTUAL TABLE")
}

func (p *DataPage) build() {
	p.header = tview.NewTextView().SetDynamicColors(true)
	p.header.SetBackgroundColor(colBg)
	p.header.SetTextColor(colText)
	p.header.SetWrap(false)

	p.table = tview.NewTable()
	styleTable(p.table)
	p.table.SetSeparator(columnLineSeparator(p.app.colLines))
	p.table.SetBorder(true)
	p.table.SetTitle(tr(" Данные "))
	p.table.SetBorderColor(colBorder)
	p.table.SetTitleColor(colTitle)
	p.table.SetSelectedFunc(func(row, col int) { p.editCellAt(row, col) })
	p.table.SetSelectionChangedFunc(func(row, col int) {
		if col > 0 {
			p.selCol = col
		}
		if row >= 2 {
			p.selRow = row - 2
		}
		p.updatePreview(row, col)
	})

	p.status = tview.NewTextView()
	p.status.SetBackgroundColor(colStatusBg)
	p.status.SetTextColor(colStatusFg)
	p.status.SetWrap(false)

	p.preview = tview.NewTextView()
	p.preview.SetBackgroundColor(colBg)
	p.preview.SetTextColor(colDim)
	p.preview.SetWrap(false)

	p.hints = tview.NewTextView().SetDynamicColors(true)
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.hints.SetText(hintBar(
		"F2", "Ячейка",
		"F3", "Структ.",
		"F5", "Обнов.",
		"F6", "Фильтр",
		"^R", "Сброс ф.",
		"Ins", "Встав.",
		"Del", "Удал.",
		"F8", "Режим",
		"Esc", "Назад",
	))

	p.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.header, 1, 0, false).
		AddItem(&columnLineTable{Table: p.table, enabled: func() bool { return p.app.colLines }}, 0, 1, true).
		AddItem(p.status, 1, 0, false).
		AddItem(p.preview, 1, 0, false).
		AddItem(p.hints, 1, 0, false)
}

func (p *DataPage) Root() tview.Primitive { return p.root }

func (p *DataPage) applyColumnLines() {
	p.table.SetSeparator(columnLineSeparator(p.app.colLines))
}

func columnLineSeparator(enabled bool) rune {
	if enabled {
		return tview.Borders.Vertical
	}
	return ' '
}

type columnLineTable struct {
	*tview.Table
	enabled func() bool
}

func (t *columnLineTable) Draw(screen tcell.Screen) {
	t.Table.Draw(screen)
	if t.enabled == nil || !t.enabled() {
		return
	}
	x, y, width, height := t.Table.GetInnerRect()
	if width <= 0 || height <= 1 {
		return
	}
	var all, matched []int
	for xx := x; xx < x+width; xx++ {
		first, _, _, _ := screen.GetContent(xx, y)
		if first != tview.Borders.Vertical {
			continue
		}
		all = append(all, xx)
		second, _, _, _ := screen.GetContent(xx, y+1)
		if second == tview.Borders.Vertical {
			matched = append(matched, xx)
		}
	}
	columns := matched
	if len(columns) == 0 {
		columns = all
	}
	if len(columns) == 0 {
		return
	}
	style := tcell.StyleDefault.Foreground(colBorder).Background(colPanelBg)
	for _, xx := range columns {
		for yy := y; yy < y+height; yy++ {
			screen.SetContent(xx, yy, tview.Borders.Vertical, nil, style)
		}
	}
}

func (p *DataPage) restyle() {
	p.header.SetBackgroundColor(colBg)
	p.header.SetTextColor(colText)
	styleTable(p.table)
	p.applyColumnLines()
	p.table.SetBorderColor(colBorder)
	p.table.SetTitleColor(colTitle)
	p.status.SetBackgroundColor(colStatusBg)
	p.status.SetTextColor(colStatusFg)
	p.preview.SetBackgroundColor(colBg)
	p.preview.SetTextColor(colDim)
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.hints.SetText(hintBar(
		"F2", "Ячейка",
		"F3", "Структ.",
		"F5", "Обнов.",
		"F6", "Фильтр",
		"^R", "Сброс ф.",
		"Ins", "Встав.",
		"Del", "Удал.",
		"F8", "Режим",
		"Esc", "Назад",
	))
	p.populate()
	p.updateStatus()
}

func (p *DataPage) Load() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	total, err := p.db.Count(ctx, p.obj.Name, p.filter)
	if err != nil {
		p.err = err
		p.setErrorStatus(err)
		return err
	}
	p.total = total
	if total == 0 {
		p.offset = 0
	} else if int64(p.offset) >= total {
		p.offset = int((total - 1) / int64(p.size) * int64(p.size))
	}

	opts := db.PageOptions{
		OrderColumn: p.order,
		Desc:        p.desc,
		Filter:      p.filter,
		Limit:       p.size,
		Offset:      p.offset,
	}
	var rows []db.Row
	var ids []int64
	if p.canEdit {
		rows, ids, err = p.db.PageWithRowIDs(ctx, p.obj.Name, p.cols, opts)
	} else {
		rows, err = p.db.Page(ctx, p.obj.Name, p.cols, opts)
	}
	if err != nil {
		p.err = err
		p.setErrorStatus(err)
		return err
	}
	p.err = nil
	p.rows = rows
	p.rowIDs = ids
	p.populate()
	p.updateStatus()
	p.notice = ""
	return nil
}

// Reload re-reads the table definition and rows from disk, keeping the
// selected row, column and page position.
func (p *DataPage) Reload() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cols, err := p.db.Columns(ctx, p.obj.Name)
	if err != nil {
		p.err = err
		p.setErrorStatus(err)
		return err
	}
	if len(cols) == 0 {
		err = fmt.Errorf("таблица %s не найдена", p.obj.Name)
		p.err = err
		p.setErrorStatus(err)
		return err
	}

	selName := ""
	if p.selCol >= 1 && p.selCol <= len(p.cols) {
		selName = p.cols[p.selCol-1].Name
	}
	orderName := ""
	if p.order >= 0 && p.order < len(p.cols) {
		orderName = p.cols[p.order].Name
	}

	p.cols = cols
	p.selCol = 1
	if selName != "" {
		for i, c := range cols {
			if c.Name == selName {
				p.selCol = i + 1
				break
			}
		}
	}
	if orderName != "" {
		p.order = -1
		p.desc = false
		for i, c := range cols {
			if c.Name == orderName {
				p.order = i
				break
			}
		}
	} else if p.order >= len(cols) {
		p.order = -1
		p.desc = false
	}
	return p.Load()
}

func shortColumnType(decl string) string {
	typ := strings.TrimSpace(decl)
	if strings.Contains(strings.ToUpper(typ), "INT") {
		return "INT"
	}
	return typ
}

func (p *DataPage) populate() {
	p.table.Clear()
	p.table.SetFixed(2, 1)

	p.table.SetCell(0, 0, tview.NewTableCell("#").
		SetTextColor(colDim).SetSelectable(false).SetAlign(tview.AlignRight).SetMaxWidth(6))
	p.table.SetCell(1, 0, tview.NewTableCell("").
		SetTextColor(colDim).SetSelectable(false))

	for i, c := range p.cols {
		name := c.Name
		color := colTitle
		if i == p.order {
			if p.desc {
				name += " ▼"
			} else {
				name += " ▲"
			}
			color = colOK
		}
		idx := i
		p.table.SetCell(0, i+1, tview.NewTableCell(name).
			SetTextColor(color).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetExpansion(1).
			SetMaxWidth(40).
			SetClickedFunc(func() bool { p.toggleSort(idx); return true }))

		typ := shortColumnType(c.Type)
		if typ == "" {
			typ = "—"
		}
		if c.PK > 0 {
			typ += " · PK"
		}
		if c.NotNull {
			typ += " · NOT NULL"
		}
		p.table.SetCell(1, i+1, tview.NewTableCell(typ).
			SetTextColor(colDim).
			SetSelectable(false).
			SetExpansion(1).
			SetMaxWidth(40))
	}

	for r, row := range p.rows {
		num := tview.NewTableCell(fmt.Sprintf("%d", int64(p.offset)+int64(r)+1)).
			SetTextColor(colDim).
			SetSelectable(false).
			SetAlign(tview.AlignRight).
			SetMaxWidth(6)
		p.table.SetCell(r+2, 0, num)
		for c, cell := range row {
			tc := tview.NewTableCell(cell.Text).
				SetExpansion(1).
				SetMaxWidth(40)
			switch {
			case cell.Null:
				tc.SetTextColor(colDim).SetAttributes(tcell.AttrItalic)
			case cell.Blob:
				tc.SetTextColor(colBlob)
			default:
				tc.SetTextColor(colText)
			}
			p.table.SetCell(r+2, c+1, tc)
		}
	}

	if len(p.rows) > 0 {
		col := p.selCol
		if col < 1 || col > len(p.cols) {
			col = 1
		}
		row := p.selRow
		if row < 0 {
			row = 0
		}
		if row >= len(p.rows) {
			row = len(p.rows) - 1
		}
		p.table.Select(2+row, col)
	} else {
		p.table.Select(0, 0)
	}
	p.updateHeader()
}

func (p *DataPage) updateHeader() {
	mode := tr("только чтение")
	if p.canEdit && !p.app.opts.ReadOnly {
		mode = tr("правка")
	}
	title := trf("[yellow::b]%s[-:-:-] [white]· %s  [silver]· колонок: %d · %s",
		esc(p.obj.Name), typeLabel(p.obj.Type), len(p.cols), mode)
	p.header.SetText(title)
}

func (p *DataPage) updateStatus() {
	pages := int64(1)
	if p.total > 0 {
		pages = (p.total + int64(p.size) - 1) / int64(p.size)
	}
	page := int64(1)
	if p.total > 0 {
		page = int64(p.offset)/int64(p.size) + 1
	}
	line := trf("Строк: %d · страница %d/%d", p.total, page, pages)
	if p.order >= 0 && p.order < len(p.cols) {
		dir := "ASC"
		if p.desc {
			dir = "DESC"
		}
		line += trf(" · сортировка: %s %s", p.cols[p.order].Name, dir)
	}
	if p.filter != "" {
		line += tr(" · фильтр: ") + truncate(p.filter, 48)
	}
	if p.notice != "" {
		line = p.notice + " · " + line
	}
	if p.err != nil {
		line = tr("Ошибка: ") + translateText(p.err.Error())
	}
	p.status.SetText(esc(line))
}

func (p *DataPage) setErrorStatus(err error) {
	p.status.SetText(esc(tr("Ошибка: ") + translateText(err.Error())))
}

func (p *DataPage) updatePreview(row, col int) {
	if row < 2 || col < 1 || row-2 >= len(p.rows) || col-1 >= len(p.cols) {
		p.preview.SetText(esc(tr("Enter/F2 — правка ячейки · Ins — вставка · Del — удаление · s или клик по заголовку — сортировка")))
		return
	}
	cell := p.rows[row-2][col-1]
	val := truncate(cell.Text, 120)
	if cell.Null {
		val = "NULL"
	}
	p.preview.SetText(esc(fmt.Sprintf("%s.%s = %s", p.obj.Name, p.cols[col-1].Name, val)))
}

func (p *DataPage) showSelectedCell() {
	row, col := p.table.GetSelection()
	p.editCellAt(row, col)
}

func (p *DataPage) editCellAt(row, col int) {
	if row < 2 || col < 1 || row-2 >= len(p.rows) || col-1 >= len(p.cols) {
		return
	}
	cell := p.rows[row-2][col-1]
	if !p.canEdit || cell.Blob || p.cols[col-1].Hidden {
		p.showCellDetailAt(row, col)
		return
	}
	if p.app.opts.ReadOnly {
		p.app.showModal(tr("База открыта в режиме только чтения.\n\nЗапустите msqlite с флагом -rw."), []string{"OK"}, nil)
		return
	}
	p.app.editCellDialog(p, row, col)
}

func (p *DataPage) rowIDAt(row int) (int64, bool) {
	idx := row - 2
	if idx < 0 || idx >= len(p.rowIDs) {
		return 0, false
	}
	return p.rowIDs[idx], true
}

func (p *DataPage) saveCell(row, col int, value any) {
	rowID, ok := p.rowIDAt(row)
	if !ok {
		p.app.showError(fmt.Errorf("%s", tr("не удалось определить rowid строки")))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := p.db.UpdateCell(ctx, p.obj.Name, p.cols[col-1].Name, value, rowID); err != nil {
		p.app.showError(err)
		return
	}
	if err := p.Load(); err != nil {
		p.app.showError(err)
		return
	}
	p.app.notify(tr("Ячейка обновлена"))
}

func (p *DataPage) deleteSelectedRow() {
	row, col := p.table.GetSelection()
	if row < 2 || row-2 >= len(p.rows) {
		return
	}
	if !p.canEdit {
		msg := tr("Эта таблица не поддерживает построчное редактирование.")
		if p.obj.Type == "view" {
			msg = tr("Представления доступны только для чтения.")
		}
		p.app.showModal(msg, []string{"OK"}, nil)
		return
	}
	if p.app.opts.ReadOnly {
		p.app.showModal(tr("База открыта в режиме только чтения.\n\nЗапустите msqlite с флагом -rw."), []string{"OK"}, nil)
		return
	}
	rowID, ok := p.rowIDAt(row)
	if !ok {
		p.app.showError(fmt.Errorf("%s", tr("не удалось определить rowid строки")))
		return
	}
	_ = col
	num := int64(p.offset) + int64(row-2) + 1
	p.app.showModal(trf("Удалить строку №%d из %s?", num, p.obj.Name), []string{"Удалить", "Отмена"}, func(label string) {
		if label != "Удалить" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		n, err := p.db.DeleteRow(ctx, p.obj.Name, rowID)
		if err != nil {
			p.app.showError(err)
			return
		}
		if n == 0 {
			p.app.showError(fmt.Errorf("%s", tr("строка не найдена")))
			return
		}
		if err := p.Load(); err != nil {
			p.app.showError(err)
			return
		}
		p.app.notify(tr("Строка удалена"))
	})
}

func (p *DataPage) openInsert() {
	if !p.canEdit {
		msg := tr("Эта таблица не поддерживает построчное редактирование.")
		if p.obj.Type == "view" {
			msg = tr("Представления доступны только для чтения.")
		}
		p.app.showModal(msg, []string{"OK"}, nil)
		return
	}
	if p.app.opts.ReadOnly {
		p.app.showModal(tr("База открыта в режиме только чтения.\n\nЗапустите msqlite с флагом -rw."), []string{"OK"}, nil)
		return
	}
	p.app.openInsert(p)
}

func (p *DataPage) showCellDetailAt(row, col int) {
	if row < 2 || col < 1 || row-2 >= len(p.rows) || col-1 >= len(p.cols) {
		return
	}
	cell := p.rows[row-2][col-1]
	title := fmt.Sprintf("%s.%s", p.obj.Name, p.cols[col-1].Name)
	p.app.showValue(title, p.cols[col-1].Type, cell)
}

func (p *DataPage) nextPage() {
	if int64(p.offset)+int64(p.size) >= p.total {
		return
	}
	p.offset += p.size
	p.Load()
}

func (p *DataPage) prevPage() {
	if p.offset == 0 {
		return
	}
	p.offset -= p.size
	if p.offset < 0 {
		p.offset = 0
	}
	p.Load()
}

func (p *DataPage) lastPage() {
	if p.total == 0 {
		return
	}
	last := int((p.total - 1) / int64(p.size) * int64(p.size))
	if last == p.offset {
		return
	}
	p.offset = last
	p.Load()
}

func (p *DataPage) sortSelected() {
	_, col := p.table.GetSelection()
	p.toggleSort(col - 1)
}

func (p *DataPage) toggleSort(idx int) {
	if idx < 0 || idx >= len(p.cols) {
		return
	}
	switch {
	case p.order != idx:
		p.order = idx
		p.desc = false
	case !p.desc:
		p.desc = true
	default:
		p.order = -1
		p.desc = false
	}
	p.offset = 0
	p.selRow = 0
	p.Load()
}

// HandleKey processes data-page specific keys and reports whether it consumed the event.
func (p *DataPage) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyF6:
		p.app.filterDialog(p)
		return true
	case tcell.KeyCtrlR:
		p.resetFilter()
		return true
	case tcell.KeyF7:
		p.prevPage()
		return true
	case tcell.KeyPgUp:
		p.prevPage()
		return true
	case tcell.KeyPgDn:
		p.nextPage()
		return true
	case tcell.KeyHome:
		if p.offset != 0 {
			p.offset = 0
			p.Load()
		}
		return true
	case tcell.KeyEnd:
		p.lastPage()
		return true
	case tcell.KeyInsert:
		p.openInsert()
		return true
	case tcell.KeyDelete:
		p.deleteSelectedRow()
		return true
	case tcell.KeyRune:
		switch ev.Rune() {
		case 's', 'S':
			p.sortSelected()
			return true
		}
	}
	return false
}

func (p *DataPage) applyFilter(filter string) {
	filter = strings.TrimSpace(filter)
	if err := db.ValidateFilter(filter); err != nil {
		p.app.showError(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := p.db.Count(ctx, p.obj.Name, filter); err != nil {
		p.app.showError(err)
		return
	}
	p.filter = filter
	p.offset = 0
	p.selRow = 0
	if err := p.Load(); err != nil {
		p.app.showError(err)
	}
}

func (p *DataPage) resetFilter() {
	if p.filter == "" {
		return
	}
	p.applyFilter("")
	if p.filter == "" {
		p.app.notify(tr("Фильтр сброшен"))
	}
}

func (p *DataPage) selectedCell() (db.Cell, bool) {
	row, col := p.table.GetSelection()
	if row < 2 || col < 1 || row-2 >= len(p.rows) || col-1 >= len(p.cols) {
		return db.Cell{}, false
	}
	return p.rows[row-2][col-1], true
}

func (p *DataPage) selectedColumn() db.Column {
	_, col := p.table.GetSelection()
	if col < 1 || col-1 >= len(p.cols) {
		return db.Column{}
	}
	return p.cols[col-1]
}

func (p *DataPage) rowIDAtRow() (int64, bool) {
	row, _ := p.table.GetSelection()
	if row < 2 || row-2 >= len(p.rowIDs) {
		return 0, false
	}
	return p.rowIDs[row-2], true
}

func (p *DataPage) selectedCellText() (string, bool) {
	cell, ok := p.selectedCell()
	if !ok {
		return "", false
	}
	if cell.Null {
		return "NULL", true
	}
	return cell.Full, true
}

func (p *DataPage) selectedRowText() (string, bool) {
	row, _ := p.table.GetSelection()
	if row < 2 || row-2 >= len(p.rows) {
		return "", false
	}
	parts := make([]string, len(p.rows[row-2]))
	for i, cell := range p.rows[row-2] {
		if cell.Null {
			parts[i] = "NULL"
		} else {
			parts[i] = cell.Full
		}
	}
	return strings.Join(parts, "\t"), true
}

func (p *DataPage) autoPKIndex() int {
	idx := -1
	for i, c := range p.cols {
		if c.PK > 0 {
			if idx >= 0 {
				return -1
			}
			idx = i
		}
	}
	if idx >= 0 && strings.Contains(strings.ToUpper(p.cols[idx].Type), "INT") {
		return idx
	}
	return -1
}

func (p *DataPage) duplicateSelectedRow() {
	if !p.canEdit {
		p.app.showModal(tr("Эта таблица не поддерживает построчное редактирование."), []string{"OK"}, nil)
		return
	}
	if p.app.opts.ReadOnly {
		p.app.showModal(tr("База открыта в режиме только чтения.\n\nЗапустите с -rw или нажмите F8."), []string{"OK"}, nil)
		return
	}
	rowID, ok := p.rowIDAtRow()
	if !ok {
		p.app.showError(fmt.Errorf("%s", tr("не удалось определить rowid строки")))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := p.db.RawRowByID(ctx, p.obj.Name, p.cols, rowID)
	if err != nil {
		p.app.showError(err)
		return
	}
	auto := p.autoPKIndex()
	names := make([]string, 0, len(p.cols))
	vals := make([]any, 0, len(p.cols))
	for i, c := range p.cols {
		if i == auto || c.Hidden {
			continue
		}
		names = append(names, c.Name)
		vals = append(vals, raw[i])
	}
	if _, err := p.db.InsertRow(ctx, p.obj.Name, names, vals); err != nil {
		p.app.showError(err)
		return
	}
	if err := p.Load(); err != nil {
		p.app.showError(err)
		return
	}
	p.app.notify(tr("Строка дублирована"))
}

func (p *DataPage) relocalize() {
	p.table.SetTitle(tr(" Данные "))
	p.restyle()
}

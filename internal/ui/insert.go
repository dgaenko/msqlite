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

// InsertPage is a scrollable form for inserting a new row.
type InsertPage struct {
	app    *App
	data   *DataPage
	db     *db.Database
	obj    db.Object
	cols   []db.Column
	inputs []*tview.InputField

	active  int
	offset  int
	visible int

	root   *tview.Flex
	head   *tview.TextView
	fields *tview.Flex
	status *tview.TextView
	hints  *tview.TextView
}

func newInsertPage(a *App, data *DataPage) *InsertPage {
	cols := make([]db.Column, 0, len(data.cols))
	for _, c := range data.cols {
		if !c.Hidden {
			cols = append(cols, c)
		}
	}
	p := &InsertPage{
		app:     a,
		data:    data,
		db:      data.db,
		obj:     data.obj,
		cols:    cols,
		visible: 10,
	}
	if p.visible > len(p.cols) {
		p.visible = len(p.cols)
	}
	if p.visible < 1 {
		p.visible = 1
	}
	p.build()
	return p
}

func (p *InsertPage) build() {
	labelWidth := 6
	for _, c := range p.cols {
		if n := len([]rune(c.Name)); n > labelWidth {
			labelWidth = n
		}
	}
	if labelWidth > 26 {
		labelWidth = 26
	}

	for _, c := range p.cols {
		field := tview.NewInputField()
		field.SetLabel(c.Name)
		field.SetLabelWidth(labelWidth + 1)
		field.SetFieldWidth(0)
		field.SetLabelColor(colTitle)
		field.SetFieldBackgroundColor(colFieldBg)
		field.SetFieldTextColor(colText)
		field.SetPlaceholder(insertPlaceholder(c))
		field.SetPlaceholderTextColor(colDim)
		field.SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				if p.active >= len(p.inputs)-1 {
					p.save()
				} else {
					p.setActive(p.active + 1)
				}
			case tcell.KeyTab:
				p.setActive(p.active + 1)
			case tcell.KeyBacktab:
				p.setActive(p.active - 1)
			}
		})
		p.inputs = append(p.inputs, field)
	}

	p.head = tview.NewTextView().SetDynamicColors(true)
	p.head.SetBackgroundColor(colBg)
	p.head.SetTextColor(colText)
	p.head.SetWrap(false)
	p.updateHead()

	p.fields = tview.NewFlex().SetDirection(tview.FlexRow)
	p.fields.SetBorder(true)
	p.fields.SetBorderColor(colFocus)
	p.fields.SetTitle(" Значения ")
	p.fields.SetTitleColor(colTitle)

	p.status = tview.NewTextView()
	p.status.SetBackgroundColor(colStatusBg)
	p.status.SetTextColor(colStatusFg)
	p.status.SetWrap(false)

	p.hints = tview.NewTextView().SetDynamicColors(true)
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.hints.SetText(hintBar(
		"Tab/↑↓", "Поле",
		"PgUp/PgDn", "Прокрутка",
		"Ctrl+S", "Вставить",
		"Esc", "Отмена",
	))

	p.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.head, 1, 0, false).
		AddItem(p.fields, 0, 1, true).
		AddItem(p.status, 1, 0, false).
		AddItem(p.hints, 1, 0, false)
	p.render()
}

func insertPlaceholder(c db.Column) string {
	typ := c.Type
	if typ == "" {
		typ = "TEXT"
	}
	switch {
	case c.HasDef:
		return typ + " · DEFAULT " + c.Default
	case c.NotNull:
		return typ + " · NOT NULL"
	default:
		return typ + " · NULL"
	}
}

func (p *InsertPage) Root() tview.Primitive { return p.root }

func (p *InsertPage) restyle() {
	for _, field := range p.inputs {
		field.SetLabelColor(colTitle)
		field.SetFieldBackgroundColor(colFieldBg)
		field.SetFieldTextColor(colText)
		field.SetPlaceholderTextColor(colDim)
	}
	p.head.SetBackgroundColor(colBg)
	p.head.SetTextColor(colText)
	p.fields.SetBorderColor(colFocus)
	p.fields.SetTitleColor(colTitle)
	p.status.SetBackgroundColor(colStatusBg)
	p.status.SetTextColor(colStatusFg)
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.hints.SetText(hintBar(
		"Tab/↑↓", "Поле",
		"PgUp/PgDn", "Прокрутка",
		"Ctrl+S", "Вставить",
		"Esc", "Отмена",
	))
}

func (p *InsertPage) updateHead() {
	mode := "только чтение"
	if !p.app.opts.ReadOnly {
		mode = "правка"
	}
	p.head.SetText(fmt.Sprintf("[yellow::b]Добавление строки[-:-:-] [white]· %s  [silver]· колонок: %d · %s",
		esc(p.obj.Name), len(p.cols), mode))
}

func (p *InsertPage) focus() {
	if len(p.inputs) == 0 {
		return
	}
	p.setActive(p.active)
}

func (p *InsertPage) setActive(i int) {
	if len(p.inputs) == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(p.inputs) {
		i = len(p.inputs) - 1
	}
	p.active = i
	if p.active < p.offset {
		p.offset = p.active
	}
	if p.active >= p.offset+p.visible {
		p.offset = p.active - p.visible + 1
	}
	p.render()
	p.updateStatus()
	p.app.app.SetFocus(p.inputs[p.active])
}

func (p *InsertPage) render() {
	if p.fields == nil {
		return
	}
	p.fields.Clear()
	end := p.offset + p.visible
	if end > len(p.inputs) {
		end = len(p.inputs)
	}
	for i := p.offset; i < end; i++ {
		p.fields.AddItem(p.inputs[i], 1, 0, i == p.active)
	}
	p.fields.AddItem(nil, 0, 1, false)
}

func (p *InsertPage) updateStatus() {
	if len(p.inputs) == 0 {
		p.status.SetText("Нет колонок")
		return
	}
	line := fmt.Sprintf("Поле %d/%d · пусто — DEFAULT · NULL — NULL · '' — пустая строка",
		p.active+1, len(p.inputs))
	p.status.SetText(esc(line))
}

func (p *InsertPage) move(delta int) {
	p.setActive(p.active + delta)
}

// HandleKey processes insert-page keys and reports whether it consumed the event.
func (p *InsertPage) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		p.move(-1)
		return true
	case tcell.KeyDown:
		p.move(1)
		return true
	case tcell.KeyPgUp:
		p.move(-p.visible)
		return true
	case tcell.KeyPgDn:
		p.move(p.visible)
		return true
	case tcell.KeyCtrlS:
		p.save()
		return true
	}
	return false
}

func (p *InsertPage) values() ([]string, []any) {
	var names []string
	var vals []any
	for i, field := range p.inputs {
		text := strings.TrimSpace(field.GetText())
		if text == "" {
			continue
		}
		names = append(names, p.cols[i].Name)
		vals = append(vals, parseEditorValue(field.GetText()))
	}
	return names, vals
}

func (p *InsertPage) save() {
	names, vals := p.values()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := p.db.InsertRow(ctx, p.obj.Name, names, vals); err != nil {
		p.app.showError(err)
		return
	}
	p.app.back()
	if p.app.currentPage != pageData {
		return
	}
	if err := p.data.Load(); err != nil {
		p.app.showError(err)
		return
	}
	p.data.status.SetText(esc("Строка добавлена · " + strings.TrimSpace(p.data.status.GetText(true))))
}

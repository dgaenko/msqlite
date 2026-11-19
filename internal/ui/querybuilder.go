package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"msqlite/internal/db"
)

func (p *SQLPage) openQueryBuilder() {
	tables := p.app.objects
	if len(tables) == 0 {
		p.app.showModal(tr("В базе нет таблиц или представлений."), []string{"OK"}, nil)
		return
	}

	var (
		cols     []db.Column
		selected = make(map[int]bool)
	)

	tableDD := tview.NewDropDown().SetLabel(tr("Таблица: ")).SetLabelWidth(12)
	tableNames := make([]string, len(tables))
	for i, obj := range tables {
		tableNames[i] = obj.Name
	}
	tableDD.SetUseStyleTags(false)
	tableDD.SetOptions(tableNames, nil)

	columnsLabel := tview.NewTextView().SetDynamicColors(true)
	columnsLabel.SetBackgroundColor(colDialogBg)
	columnsLabel.SetTextColor(colDialogTitle)
	columnsLabel.SetText(tr("Колонки (Space/Enter — выбрать, пусто — все):"))

	columnList := tview.NewList()
	styleList(columnList, colDialogBg, colDialogFg, colDialogDim)
	columnList.SetBorder(false)
	columnList.SetTitle("")
	columnList.SetWrapAround(true)

	whereField := tview.NewInputField().SetLabel("WHERE: ").SetLabelWidth(12)
	whereField.SetPlaceholder(tr("условие (необязательно)"))

	orderDD := tview.NewDropDown().SetLabel("ORDER BY: ").SetLabelWidth(10)
	orderDD.SetUseStyleTags(false)
	dirDD := tview.NewDropDown().SetLabel(" ").SetLabelWidth(1)
	dirDD.SetUseStyleTags(false)
	dirDD.SetOptions([]string{"ASC", "DESC"}, nil)
	dirDD.SetCurrentOption(0)

	limitField := tview.NewInputField().SetLabel("LIMIT: ").SetLabelWidth(12)
	limitField.SetPlaceholder(tr("без ограничения"))

	for _, dd := range []*tview.DropDown{tableDD, orderDD, dirDD} {
		dd.SetLabelColor(colDialogTitle)
		dd.SetBackgroundColor(colDialogBg)
		dd.SetFieldBackgroundColor(colFieldBg)
		dd.SetFieldTextColor(colText)
		dd.SetListStyles(
			tcell.StyleDefault.Foreground(colDialogFg).Background(colDialogBg),
			tcell.StyleDefault.Foreground(colSelFg).Background(colSelBg).Bold(true),
		)
	}
	for _, f := range []*tview.InputField{whereField, limitField} {
		f.SetLabelColor(colDialogTitle)
		f.SetBackgroundColor(colDialogBg)
		f.SetFieldBackgroundColor(colFieldBg)
		f.SetFieldTextColor(colText)
		f.SetPlaceholderStyle(tcell.StyleDefault.Foreground(colDim).Background(colFieldBg).Italic(true))
	}

	hint := tview.NewTextView().SetDynamicColors(true)
	hint.SetBackgroundColor(colDialogBg)
	hint.SetTextColor(colDialogDim)
	hint.SetWrap(false)
	hint.SetText("[" + colorName(colDialogDim) + "]" + tr("OK — вставить SELECT в SQL-консоль"))

	columnText := func(index int) string {
		name := cols[index].Name
		if selected[index] {
			return "✓ " + esc(name)
		}
		return "  " + esc(name)
	}
	toggleColumn := func(index int) {
		if index < 0 || index >= len(cols) {
			return
		}
		selected[index] = !selected[index]
		columnList.SetItemText(index, columnText(index), "")
	}
	columnList.SetSelectedFunc(func(index int, _, _ string, _ rune) {
		toggleColumn(index)
	})
	columnList.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyTab:
			p.app.app.SetFocus(whereField)
			return nil
		case tcell.KeyBacktab:
			p.app.app.SetFocus(tableDD)
			return nil
		case tcell.KeyRune:
			if ev.Rune() == ' ' {
				toggleColumn(columnList.GetCurrentItem())
				return nil
			}
		}
		return ev
	})

	loadColumns := func() {
		index, _ := tableDD.GetCurrentOption()
		if index < 0 || index >= len(tables) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		loaded, err := p.db.Columns(ctx, tables[index].Name)
		if err != nil {
			hint.SetText("[red]" + tr("Ошибка: ") + esc(translateText(err.Error())) + "[-:-:-]")
			return
		}
		cols = loaded
		selected = make(map[int]bool)
		columnList.Clear()
		for i := range cols {
			columnList.AddItem(columnText(i), "", 0, nil)
		}
		orderOptions := []string{"—"}
		for _, c := range cols {
			orderOptions = append(orderOptions, c.Name)
		}
		orderDD.SetOptions(orderOptions, nil)
		orderDD.SetCurrentOption(0)
	}
	tableDD.SetSelectedFunc(func(string, int) { loadColumns() })

	insertQuery := func() {
		tableIndex, _ := tableDD.GetCurrentOption()
		if tableIndex < 0 || tableIndex >= len(tables) {
			hint.SetText("[red]" + tr("Выберите таблицу") + "[-:-:-]")
			return
		}
		table := tables[tableIndex]
		var picked []string
		for i, c := range cols {
			if selected[i] {
				picked = append(picked, db.QuoteIdent(c.Name))
			}
		}
		selection := "*"
		if len(picked) > 0 {
			selection = strings.Join(picked, ", ")
		}
		where := strings.TrimSpace(whereField.GetText())
		if where != "" {
			if err := db.ValidateFilter(where); err != nil {
				hint.SetText("[red]" + esc(translateText(err.Error())) + "[-:-:-]")
				return
			}
		}
		orderIndex, _ := orderDD.GetCurrentOption()
		limitText := strings.TrimSpace(limitField.GetText())
		limit := 0
		if limitText != "" {
			n, err := strconv.Atoi(limitText)
			if err != nil || n <= 0 {
				hint.SetText("[red]" + tr("LIMIT должен быть положительным числом") + "[-:-:-]")
				return
			}
			limit = n
		}

		var b strings.Builder
		fmt.Fprintf(&b, "SELECT %s FROM %s", selection, db.QuoteIdent(table.Name))
		if where != "" {
			b.WriteString(" WHERE ")
			b.WriteString(where)
		}
		if orderIndex > 0 && orderIndex <= len(cols) {
			dir, _ := dirDD.GetCurrentOption()
			direction := "ASC"
			if dir == 1 {
				direction = "DESC"
			}
			fmt.Fprintf(&b, " ORDER BY %s %s", db.QuoteIdent(cols[orderIndex-1].Name), direction)
		}
		if limit > 0 {
			fmt.Fprintf(&b, " LIMIT %d", limit)
		}
		query := b.String()
		p.app.closeDialog()
		p.input.SetText(query, true)
		p.app.app.SetFocus(p.input)
	}
	okBtn := dialogButton("OK", true, insertQuery)
	cancelBtn := dialogButton("Отмена", false, p.app.closeDialog)

	focus := func(prim tview.Primitive) { p.app.app.SetFocus(prim) }
	nav := func(key tcell.Key, back, next tview.Primitive) bool {
		switch key {
		case tcell.KeyTab:
			focus(next)
			return true
		case tcell.KeyBacktab:
			focus(back)
			return true
		}
		return false
	}
	tableDD.SetFinishedFunc(func(key tcell.Key) { nav(key, cancelBtn, columnList) })
	whereField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			insertQuery()
			return
		}
		nav(key, columnList, orderDD)
	})
	orderDD.SetFinishedFunc(func(key tcell.Key) { nav(key, whereField, dirDD) })
	dirDD.SetFinishedFunc(func(key tcell.Key) { nav(key, orderDD, limitField) })
	limitField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			insertQuery()
			return
		}
		nav(key, dirDD, okBtn)
	})
	okBtn.SetExitFunc(func(key tcell.Key) { nav(key, limitField, cancelBtn) })
	cancelBtn.SetExitFunc(func(key tcell.Key) { nav(key, okBtn, tableDD) })

	orderRow := tview.NewFlex().
		AddItem(orderDD, 0, 1, true).
		AddItem(nil, 1, 0, false).
		AddItem(dirDD, 10, 0, true)

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tableDD, 1, 0, true).
		AddItem(columnsLabel, 1, 0, false).
		AddItem(columnList, 0, 1, true).
		AddItem(whereField, 1, 0, true).
		AddItem(orderRow, 1, 0, true).
		AddItem(limitField, 1, 0, true).
		AddItem(hint, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)

	tableDD.SetCurrentOption(0)
	loadColumns()
	p.app.showFramedDialog(tr("Конструктор запроса"), content, 78, 20, tableDD)
	p.app.dialogEscape = func() bool {
		return tableDD.IsOpen() || orderDD.IsOpen() || dirDD.IsOpen()
	}
}

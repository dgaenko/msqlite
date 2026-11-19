package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"msqlite/internal/db"
)

type filterOperator struct {
	label string
	sql   string
}

var filterOperators = []filterOperator{
	{"=", "="},
	{"<>", "<>"},
	{">", ">"},
	{">=", ">="},
	{"<", "<"},
	{"<=", "<="},
	{"LIKE", "LIKE"},
	{"NOT LIKE", "NOT LIKE"},
	{"IS NULL", "IS NULL"},
	{"IS NOT NULL", "IS NOT NULL"},
}

func (a *App) filterDialog(p *DataPage) {
	if len(p.cols) == 0 {
		return
	}
	names := make([]string, len(p.cols))
	for i, c := range p.cols {
		names[i] = c.Name
	}
	ops := make([]string, len(filterOperators))
	for i, op := range filterOperators {
		ops[i] = op.label
	}

	colDD := tview.NewDropDown().SetLabel("Колонка: ").SetLabelWidth(12)
	colDD.SetOptions(names, nil)
	opDD := tview.NewDropDown().SetLabel("Оператор: ").SetLabelWidth(12)
	opDD.SetOptions(ops, nil)

	valueField := tview.NewInputField().SetLabel("Значение: ").SetLabelWidth(13)
	valueField.SetPlaceholder("значение")
	rawField := tview.NewInputField().SetLabel("WHERE: ").SetLabelWidth(13)
	rawField.SetText(p.filter)
	rawField.SetPlaceholder("выражение WHERE")

	for _, dd := range []*tview.DropDown{colDD, opDD} {
		dd.SetTextOptions(" ", "", " ", "", "")
		dd.SetLabelColor(colDialogTitle)
		dd.SetBackgroundColor(colDialogBg)
		dd.SetFieldBackgroundColor(colFieldBg)
		dd.SetFieldTextColor(colText)
		dd.SetListStyles(
			tcell.StyleDefault.Foreground(colDialogFg).Background(colDialogBg),
			tcell.StyleDefault.Foreground(colSelFg).Background(colSelBg).Bold(true),
		)
	}
	for _, f := range []*tview.InputField{valueField, rawField} {
		f.SetLabelColor(colDialogTitle)
		f.SetBackgroundColor(colDialogBg)
		f.SetFieldBackgroundColor(colFieldBg)
		f.SetFieldTextColor(colText)
		f.SetPlaceholderStyle(tcell.StyleDefault.Foreground(colDim).Background(colFieldBg).Italic(true))
	}

	if idx := p.selCol - 1; idx >= 0 && idx < len(names) {
		colDD.SetCurrentOption(idx)
	} else {
		colDD.SetCurrentOption(0)
	}
	opDD.SetCurrentOption(0)

	buildExpr := func() string {
		colIndex, _ := colDD.GetCurrentOption()
		opIndex, _ := opDD.GetCurrentOption()
		if colIndex < 0 || colIndex >= len(p.cols) || opIndex < 0 || opIndex >= len(filterOperators) {
			return ""
		}
		column := db.QuoteIdent(p.cols[colIndex].Name)
		op := filterOperators[opIndex].sql
		if op == "IS NULL" || op == "IS NOT NULL" {
			return column + " " + op
		}
		value := strings.TrimSpace(valueField.GetText())
		if value == "" {
			return ""
		}
		escaped := strings.ReplaceAll(value, "'", "''")
		literal := "'" + escaped + "'"
		if op == "LIKE" || op == "NOT LIKE" {
			if !strings.ContainsAny(value, "%_") {
				literal = "'%" + escaped + "%'"
			}
		}
		return column + " " + op + " " + literal
	}
	syncExpr := func() {
		rawField.SetText(buildExpr())
	}
	colDD.SetSelectedFunc(func(string, int) { syncExpr() })
	opDD.SetSelectedFunc(func(string, int) { syncExpr() })
	valueField.SetChangedFunc(func(string) { syncExpr() })

	apply := func() {
		expr := strings.TrimSpace(rawField.GetText())
		a.closeDialog()
		p.applyFilter(expr)
	}
	okBtn := dialogButton("OK", true, apply)
	cancelBtn := dialogButton("Отмена", false, a.closeDialog)

	focus := func(p tview.Primitive) {
		a.app.SetFocus(p)
	}
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

	colDD.SetFinishedFunc(func(key tcell.Key) { nav(key, cancelBtn, opDD) })
	opDD.SetFinishedFunc(func(key tcell.Key) { nav(key, colDD, valueField) })
	valueField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			apply()
			return
		}
		nav(key, opDD, rawField)
	})
	rawField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			apply()
			return
		}
		nav(key, valueField, okBtn)
	})
	okBtn.SetExitFunc(func(key tcell.Key) { nav(key, rawField, cancelBtn) })
	cancelBtn.SetExitFunc(func(key tcell.Key) { nav(key, okBtn, colDD) })

	head := tview.NewTextView().SetDynamicColors(true)
	head.SetBackgroundColor(colDialogBg)
	head.SetTextColor(colDialogFg)
	head.SetWrap(false)
	head.SetText(fmt.Sprintf("[%s::b]WHERE[-:-:-]  [%s]· конструктор для %s[-:-:-]",
		colorName(colDialogTitle), colorName(colDialogDim), esc(p.obj.Name)))

	hint := tview.NewTextView().SetDynamicColors(true)
	hint.SetBackgroundColor(colDialogBg)
	hint.SetTextColor(colDialogDim)
	hint.SetWrap(false)
	hint.SetText("[" + colorName(colDialogDim) + "]Выберите колонку, оператор и значение либо введите WHERE вручную")

	inner := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(head, 1, 0, false).
		AddItem(colDD, 1, 0, true).
		AddItem(opDD, 1, 0, true).
		AddItem(valueField, 1, 0, true).
		AddItem(rawField, 1, 0, true).
		AddItem(nil, 1, 0, false).
		AddItem(hint, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)
	content := tview.NewFlex().
		AddItem(nil, 2, 0, false).
		AddItem(inner, 0, 1, true).
		AddItem(nil, 2, 0, false)

	a.showFramedDialog("Фильтр · "+p.obj.Name, content, 76, 13, okBtn)
	a.dialogEscape = func() bool {
		return colDD.IsOpen() || opDD.IsOpen()
	}
}

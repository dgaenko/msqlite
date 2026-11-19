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

var sqlColumnTypes = []string{
	"INTEGER", "TEXT", "REAL", "BLOB", "NUMERIC", "BOOLEAN",
	"DATE", "DATETIME", "TIMESTAMP", "VARCHAR(255)", "DECIMAL(10,2)",
}

type tableColumnDef struct {
	name    string
	typ     string
	def     string
	pk      bool
	notNull bool
}

func (a *App) createTableDialog() {
	if a.current == nil {
		return
	}
	tableName := tview.NewInputField().SetLabel("Таблица: ").SetLabelWidth(10)
	nameField := tableName
	nameField.SetLabelColor(colDialogTitle)
	nameField.SetFieldBackgroundColor(colFieldBg)
	nameField.SetFieldTextColor(colText)

	columnsLabel := tview.NewTextView()
	columnsLabel.SetBackgroundColor(colDialogBg)
	columnsLabel.SetTextColor(colDialogTitle)
	columnsLabel.SetText("Колонки (Enter — загрузить в форму):")

	columnsList := tview.NewList()
	styleList(columnsList, colDialogBg, colDialogFg, colDialogDim)
	columnsList.SetBorder(false)
	columnsList.SetTitle("")
	columnsList.SetWrapAround(true)

	colName := tview.NewInputField().SetLabel("Имя: ").SetLabelWidth(8)
	colType := tview.NewDropDown().SetLabel("Тип: ").SetLabelWidth(6)
	colType.SetUseStyleTags(false)
	colType.SetOptions(sqlColumnTypes, nil)
	colType.SetCurrentOption(1)
	pkCheck := tview.NewCheckbox().SetLabel("PK")
	notNullCheck := tview.NewCheckbox().SetLabel("NOT NULL")
	defaultField := tview.NewInputField().SetLabel("DEFAULT: ").SetLabelWidth(9)
	defaultField.SetPlaceholder("SQL-выражение")

	for _, f := range []*tview.InputField{nameField, colName, defaultField} {
		f.SetLabelColor(colDialogTitle)
		f.SetBackgroundColor(colDialogBg)
		f.SetFieldBackgroundColor(colFieldBg)
		f.SetFieldTextColor(colText)
		f.SetPlaceholderStyle(tcell.StyleDefault.Foreground(colDim).Background(colFieldBg).Italic(true))
	}
	colType.SetLabelColor(colDialogTitle)
	colType.SetBackgroundColor(colDialogBg)
	colType.SetFieldBackgroundColor(colFieldBg)
	colType.SetFieldTextColor(colText)
	colType.SetListStyles(
		tcell.StyleDefault.Foreground(colDialogFg).Background(colDialogBg),
		tcell.StyleDefault.Foreground(colSelFg).Background(colSelBg).Bold(true),
	)
	pkCheck.SetLabelColor(colDialogTitle)
	pkCheck.SetBackgroundColor(colDialogBg)
	pkCheck.SetFieldBackgroundColor(colFieldBg)
	pkCheck.SetFieldTextColor(colText)
	notNullCheck.SetLabelColor(colDialogTitle)
	notNullCheck.SetBackgroundColor(colDialogBg)
	notNullCheck.SetFieldBackgroundColor(colFieldBg)
	notNullCheck.SetFieldTextColor(colText)

	hint := tview.NewTextView().SetDynamicColors(true)
	hint.SetBackgroundColor(colDialogBg)
	hint.SetTextColor(colDialogDim)
	hint.SetWrap(false)
	hint.SetText("[" + colorName(colDialogDim) + "]DEFAULT — SQL-выражение (0, 'x', CURRENT_TIMESTAMP) · Ctrl+S — создать")

	okBtn := dialogButton("OK", true, func() {})
	cancelBtn := dialogButton("Отмена", false, a.closeDialog)
	addBtn := dialogButton("Добавить", false, func() {})
	clearBtn := dialogButton("Очистить", false, func() {})
	delBtn := dialogButton("Удалить", false, func() {})
	addBtn.SetSelectedFunc(func() {})

	var columns []tableColumnDef
	editing := -1

	resetForm := func() {
		editing = -1
		colName.SetText("")
		colType.SetCurrentOption(1)
		pkCheck.SetChecked(false)
		notNullCheck.SetChecked(false)
		defaultField.SetText("")
		addBtn.SetLabel("Добавить")
	}
	renderColumns := func() {
		columnsList.Clear()
		for _, c := range columns {
			label := "  " + esc(c.name) + "  " + esc(c.typ)
			if c.pk {
				label += "  PK"
			}
			if c.notNull {
				label += "  NOT NULL"
			}
			if c.def != "" {
				label += "  DEFAULT " + esc(c.def)
			}
			columnsList.AddItem(label, "", 0, nil)
		}
	}
	loadColumn := func(index int) {
		if index < 0 || index >= len(columns) {
			return
		}
		editing = index
		c := columns[index]
		colName.SetText(c.name)
		colType.SetCurrentOption(columnTypeIndex(c.typ))
		pkCheck.SetChecked(c.pk)
		notNullCheck.SetChecked(c.notNull)
		defaultField.SetText(c.def)
		addBtn.SetLabel("Обновить")
	}
	saveColumn := func() {
		name := strings.TrimSpace(colName.GetText())
		if name == "" {
			hint.SetText("[red]Укажите имя колонки[-:-:-]")
			return
		}
		typeIndex, typ := colType.GetCurrentOption()
		if typeIndex < 0 {
			typ = "TEXT"
		}
		column := tableColumnDef{
			name:    name,
			typ:     typ,
			def:     strings.TrimSpace(defaultField.GetText()),
			pk:      pkCheck.IsChecked(),
			notNull: notNullCheck.IsChecked(),
		}
		if editing >= 0 && editing < len(columns) {
			columns[editing] = column
		} else {
			columns = append(columns, column)
		}
		resetForm()
		renderColumns()
		hint.SetText("[" + colorName(colDialogDim) + "]Колонка добавлена · Ctrl+S — создать таблицу")
		a.app.SetFocus(colName)
	}
	addBtn.SetSelectedFunc(saveColumn)
	clearBtn.SetSelectedFunc(resetForm)
	delBtn.SetSelectedFunc(func() {
		if editing < 0 || editing >= len(columns) {
			hint.SetText("[red]Выберите колонку для удаления[-:-:-]")
			return
		}
		columns = append(columns[:editing], columns[editing+1:]...)
		resetForm()
		renderColumns()
	})

	submit := func() {
		name := strings.TrimSpace(nameField.GetText())
		sqlText, err := buildCreateTableSQL(name, columns)
		if err != nil {
			hint.SetText("[red]" + esc(err.Error()) + "[-:-:-]")
			return
		}
		if a.opts.ReadOnly {
			hint.SetText("[red]База открыта в режиме только чтения. Переключите F8.[-:-:-]")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := a.current.Exec(ctx, sqlText, 0); err != nil {
			hint.SetText("[red]Ошибка: " + esc(err.Error()) + "[-:-:-]")
			return
		}
		a.closeDialog()
		a.refreshObjectsSelect(name)
		a.notify(fmt.Sprintf("Таблица %q создана", name))
	}
	okBtn.SetSelectedFunc(submit)
	a.dialogSave = submit

	columnsList.SetSelectedFunc(func(index int, _, _ string, _ rune) {
		loadColumn(index)
		a.app.SetFocus(colName)
	})
	columnsList.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyTab:
			a.app.SetFocus(colName)
			return nil
		case tcell.KeyBacktab:
			a.app.SetFocus(nameField)
			return nil
		}
		return ev
	})

	nameField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter, tcell.KeyTab:
			a.app.SetFocus(columnsList)
		case tcell.KeyBacktab:
			a.app.SetFocus(cancelBtn)
		}
	})
	colName.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter, tcell.KeyTab:
			a.app.SetFocus(colType)
		case tcell.KeyBacktab:
			a.app.SetFocus(columnsList)
		}
	})
	colType.SetFinishedFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(pkCheck)
		case tcell.KeyBacktab:
			a.app.SetFocus(colName)
		}
	})
	pkCheck.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyTab:
			a.app.SetFocus(notNullCheck)
			return nil
		case tcell.KeyBacktab:
			a.app.SetFocus(colType)
			return nil
		}
		return ev
	})
	notNullCheck.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyTab:
			a.app.SetFocus(defaultField)
			return nil
		case tcell.KeyBacktab:
			a.app.SetFocus(pkCheck)
			return nil
		}
		return ev
	})
	defaultField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			saveColumn()
		case tcell.KeyTab:
			a.app.SetFocus(addBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(notNullCheck)
		}
	})
	addBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(clearBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(defaultField)
		}
	})
	clearBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(delBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(addBtn)
		}
	})
	delBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(okBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(clearBtn)
		}
	})
	okBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(cancelBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(delBtn)
		}
	})
	cancelBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(nameField)
		case tcell.KeyBacktab:
			a.app.SetFocus(okBtn)
		}
	})

	editorRow := tview.NewFlex().
		AddItem(colName, 0, 1, true).
		AddItem(nil, 1, 0, false).
		AddItem(colType, 22, 0, true).
		AddItem(nil, 1, 0, false).
		AddItem(pkCheck, 6, 0, true).
		AddItem(nil, 1, 0, false).
		AddItem(notNullCheck, 13, 0, true)

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nameField, 1, 0, true).
		AddItem(columnsLabel, 1, 0, false).
		AddItem(columnsList, 0, 1, true).
		AddItem(editorRow, 1, 0, true).
		AddItem(defaultField, 1, 0, true).
		AddItem(dialogButtonRow(addBtn, clearBtn, delBtn), 1, 0, false).
		AddItem(hint, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)

	a.showFramedDialog("Создание таблицы", content, 80, 22, nameField)
	a.dialogEscape = func() bool { return colType.IsOpen() }
}

func columnTypeIndex(typ string) int {
	for i, option := range sqlColumnTypes {
		if strings.EqualFold(option, typ) {
			return i
		}
	}
	return 1
}

func buildCreateTableSQL(name string, columns []tableColumnDef) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("укажите имя таблицы")
	}
	if len(columns) == 0 {
		return "", fmt.Errorf("добавьте хотя бы одну колонку")
	}
	var pkNames []string
	for _, c := range columns {
		if c.pk {
			pkNames = append(pkNames, db.QuoteIdent(c.name))
		}
	}
	var defs []string
	for _, c := range columns {
		def := db.QuoteIdent(c.name) + " " + c.typ
		if c.pk && len(pkNames) == 1 {
			def += " PRIMARY KEY"
		}
		if c.notNull {
			def += " NOT NULL"
		}
		if c.def != "" {
			def += " DEFAULT " + c.def
		}
		defs = append(defs, def)
	}
	if len(pkNames) > 1 {
		defs = append(defs, "PRIMARY KEY ("+strings.Join(pkNames, ", ")+")")
	}
	return "CREATE TABLE " + db.QuoteIdent(name) + " (\n  " +
		strings.Join(defs, ",\n  ") + "\n);", nil
}

func (a *App) createViewDialog() {
	if a.current == nil {
		return
	}
	nameField := tview.NewInputField().SetLabel("Имя: ").SetLabelWidth(10)
	nameField.SetLabelColor(colDialogTitle)
	nameField.SetFieldBackgroundColor(colFieldBg)
	nameField.SetFieldTextColor(colText)

	queryArea := tview.NewTextArea().SetMaxLength(1 << 20)
	queryArea.SetWrap(true)
	queryArea.SetBackgroundColor(colFieldBg)
	queryArea.SetTextStyle(tcell.StyleDefault.Foreground(colText).Background(colFieldBg))
	queryArea.SetPlaceholder("SELECT * FROM table_name")
	queryArea.SetPlaceholderStyle(tcell.StyleDefault.Foreground(colDim).Background(colFieldBg).Italic(true))

	hint := tview.NewTextView().SetDynamicColors(true)
	hint.SetBackgroundColor(colDialogBg)
	hint.SetTextColor(colDialogDim)
	hint.SetWrap(false)
	hint.SetText("[" + colorName(colDialogDim) + "]CREATE VIEW имя AS <SELECT> · Ctrl+S — создать")

	var okBtn, cancelBtn *tview.Button
	submit := func() {
		name := strings.TrimSpace(nameField.GetText())
		query := strings.TrimSpace(queryArea.GetText())
		query = strings.TrimRight(query, "; \t\r\n")
		if name == "" {
			hint.SetText("[red]Укажите имя представления[-:-:-]")
			return
		}
		if query == "" {
			hint.SetText("[red]Укажите SELECT-запрос[-:-:-]")
			return
		}
		if a.opts.ReadOnly {
			hint.SetText("[red]База открыта в режиме только чтения. Переключите F8.[-:-:-]")
			return
		}
		sqlText := "CREATE VIEW " + db.QuoteIdent(name) + " AS " + query
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := a.current.Exec(ctx, sqlText, 0); err != nil {
			hint.SetText("[red]Ошибка: " + esc(err.Error()) + "[-:-:-]")
			return
		}
		a.closeDialog()
		a.refreshObjectsSelect(name)
		a.notify(fmt.Sprintf("Представление %q создано", name))
	}
	okBtn = dialogButton("OK", true, submit)
	cancelBtn = dialogButton("Отмена", false, a.closeDialog)
	a.dialogSave = submit

	nameField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter, tcell.KeyTab:
			a.app.SetFocus(queryArea)
		case tcell.KeyBacktab:
			a.app.SetFocus(cancelBtn)
		}
	})
	queryArea.SetFinishedFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(okBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(nameField)
		}
	})
	okBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(cancelBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(queryArea)
		}
	})
	cancelBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(nameField)
		case tcell.KeyBacktab:
			a.app.SetFocus(okBtn)
		}
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nameField, 1, 0, true).
		AddItem(queryArea, 0, 1, true).
		AddItem(hint, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)

	a.showFramedDialog("Создание представления", content, 80, 20, nameField)
}

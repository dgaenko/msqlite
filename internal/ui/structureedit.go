package ui

import (
	"context"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"msqlite/internal/db"
)

func (p *StructurePage) openEditor() {
	if p.app.opts.ReadOnly {
		p.app.showModal(tr("База открыта в режиме только чтения.\nПереключите режим клавишей F8."), []string{"OK"}, nil)
		return
	}
	if p.obj.Type != "table" {
		p.app.showModal(tr("Редактирование структуры доступно только для таблиц."), []string{"OK"}, nil)
		return
	}

	actions := []struct {
		label string
		run   func()
	}{
		{tr("Переименовать таблицу…"), p.renameTableDialog},
		{tr("Добавить колонку…"), p.addColumnDialog},
		{tr("Переименовать колонку…"), p.renameColumnDialog},
		{tr("Удалить колонку…"), p.dropColumnDialog},
	}
	list := tview.NewList()
	styleList(list, colDialogBg, colDialogFg, colDialogDim)
	list.SetBorder(false)
	list.SetTitle("")
	list.SetWrapAround(true)
	for _, act := range actions {
		act := act
		list.AddItem(act.label, "", 0, func() {
			p.app.closeDialog()
			act.run()
		})
	}
	runCurrent := func() {
		index := list.GetCurrentItem()
		if index < 0 || index >= len(actions) {
			return
		}
		p.app.closeDialog()
		actions[index].run()
	}
	p.app.showListDialog(trf("Структура · %s", p.obj.Name), 64, list, runCurrent)
}

func (p *StructurePage) alterSchema(oldName, newName, sqlText, message string) {
	if p.app.opts.ReadOnly {
		p.app.showModal(tr("База открыта в режиме только чтения.\nПереключите режим клавишей F8."), []string{"OK"}, nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := p.db.Exec(ctx, sqlText, 0); err != nil {
		p.app.showError(err)
		return
	}
	if newName == "" {
		newName = oldName
	}
	p.obj.Name = newName
	if err := p.Load(); err != nil {
		p.app.showError(err)
	}
	p.app.refreshObjectsSelect(newName)
	p.app.refreshDataAfterSchema(oldName, newName)
	if message != "" {
		p.app.notify(message)
	}
}

func (p *StructurePage) renameTableDialog() {
	oldName := p.obj.Name
	p.app.inputDialog("Переименовать таблицу", "Новое имя: ", oldName,
		"Enter — переименовать · Esc — отмена", func(value string) {
			newName := strings.TrimSpace(value)
			if newName == "" || newName == oldName {
				return
			}
			sqlText := "ALTER TABLE " + db.QuoteIdent(oldName) + " RENAME TO " + db.QuoteIdent(newName)
			p.alterSchema(oldName, newName, sqlText, trf("Таблица переименована в %q", newName))
		})
}

func (p *StructurePage) addColumnDialog() {
	nameField := tview.NewInputField().SetLabel(tr("Имя: ")).SetLabelWidth(10)
	nameField.SetLabelColor(colDialogTitle)
	nameField.SetFieldBackgroundColor(colFieldBg)
	nameField.SetFieldTextColor(colText)

	typeDD := tview.NewDropDown().SetLabel(tr("Тип: ")).SetLabelWidth(10)
	typeDD.SetUseStyleTags(false)
	typeDD.SetOptions(sqlColumnTypes, nil)
	typeDD.SetCurrentOption(1)
	typeDD.SetLabelColor(colDialogTitle)
	typeDD.SetBackgroundColor(colDialogBg)
	typeDD.SetFieldBackgroundColor(colFieldBg)
	typeDD.SetFieldTextColor(colText)
	typeDD.SetListStyles(
		tcell.StyleDefault.Foreground(colDialogFg).Background(colDialogBg),
		tcell.StyleDefault.Foreground(colSelFg).Background(colSelBg).Bold(true),
	)

	notNullCheck := tview.NewCheckbox().SetLabel("NOT NULL")
	notNullCheck.SetLabelColor(colDialogTitle)
	notNullCheck.SetBackgroundColor(colDialogBg)
	notNullCheck.SetFieldBackgroundColor(colFieldBg)
	notNullCheck.SetFieldTextColor(colText)

	defaultField := tview.NewInputField().SetLabel("DEFAULT: ").SetLabelWidth(10)
	defaultField.SetLabelColor(colDialogTitle)
	defaultField.SetBackgroundColor(colDialogBg)
	defaultField.SetFieldBackgroundColor(colFieldBg)
	defaultField.SetFieldTextColor(colText)
	defaultField.SetPlaceholder(tr("SQL-выражение"))
	defaultField.SetPlaceholderStyle(tcell.StyleDefault.Foreground(colDim).Background(colFieldBg).Italic(true))

	hint := tview.NewTextView().SetDynamicColors(true)
	hint.SetBackgroundColor(colDialogBg)
	hint.SetTextColor(colDialogDim)
	hint.SetWrap(false)
	hint.SetText("[" + colorName(colDialogDim) + "]" + tr("Для NOT NULL у существующей таблицы обычно нужен DEFAULT"))

	var okBtn, cancelBtn *tview.Button
	submit := func() {
		name := strings.TrimSpace(nameField.GetText())
		if name == "" {
			hint.SetText("[red]" + tr("Укажите имя колонки") + "[-:-:-]")
			return
		}
		index, typ := typeDD.GetCurrentOption()
		if index < 0 {
			typ = "TEXT"
		}
		sqlText := "ALTER TABLE " + db.QuoteIdent(p.obj.Name) + " ADD COLUMN " +
			db.QuoteIdent(name) + " " + typ
		if notNullCheck.IsChecked() {
			sqlText += " NOT NULL"
		}
		if def := strings.TrimSpace(defaultField.GetText()); def != "" {
			sqlText += " DEFAULT " + def
		}
		p.alterSchema(p.obj.Name, p.obj.Name, sqlText, trf("Колонка %q добавлена", name))
	}
	okBtn = dialogButton("OK", true, submit)
	cancelBtn = dialogButton("Отмена", false, p.app.closeDialog)
	p.app.dialogSave = submit

	nameField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter, tcell.KeyTab:
			p.app.app.SetFocus(typeDD)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(cancelBtn)
		}
	})
	typeDD.SetFinishedFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			p.app.app.SetFocus(notNullCheck)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(nameField)
		}
	})
	notNullCheck.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyTab:
			p.app.app.SetFocus(defaultField)
			return nil
		case tcell.KeyBacktab:
			p.app.app.SetFocus(typeDD)
			return nil
		}
		return ev
	})
	defaultField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			submit()
		case tcell.KeyTab:
			p.app.app.SetFocus(okBtn)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(notNullCheck)
		}
	})
	okBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			p.app.app.SetFocus(cancelBtn)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(defaultField)
		}
	})
	cancelBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			p.app.app.SetFocus(nameField)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(okBtn)
		}
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nameField, 1, 0, true).
		AddItem(typeDD, 1, 0, true).
		AddItem(notNullCheck, 1, 0, true).
		AddItem(defaultField, 1, 0, true).
		AddItem(hint, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)

	p.app.showFramedDialog(tr("Добавить колонку"), content, 64, 12, nameField)
	p.app.dialogEscape = func() bool { return typeDD.IsOpen() }
}

func (p *StructurePage) renameColumnDialog() {
	if len(p.cols) == 0 {
		p.app.showModal(tr("В таблице нет колонок."), []string{"OK"}, nil)
		return
	}
	names := make([]string, len(p.cols))
	for i, c := range p.cols {
		names[i] = c.Name
	}
	columnDD := tview.NewDropDown().SetLabel(tr("Колонка: ")).SetLabelWidth(10)
	columnDD.SetUseStyleTags(false)
	columnDD.SetOptions(names, nil)
	columnDD.SetCurrentOption(0)
	columnDD.SetLabelColor(colDialogTitle)
	columnDD.SetBackgroundColor(colDialogBg)
	columnDD.SetFieldBackgroundColor(colFieldBg)
	columnDD.SetFieldTextColor(colText)
	columnDD.SetListStyles(
		tcell.StyleDefault.Foreground(colDialogFg).Background(colDialogBg),
		tcell.StyleDefault.Foreground(colSelFg).Background(colSelBg).Bold(true),
	)

	nameField := tview.NewInputField().SetLabel(tr("Новое имя: ")).SetLabelWidth(12)
	nameField.SetLabelColor(colDialogTitle)
	nameField.SetBackgroundColor(colDialogBg)
	nameField.SetFieldBackgroundColor(colFieldBg)
	nameField.SetFieldTextColor(colText)

	hint := tview.NewTextView().SetDynamicColors(true)
	hint.SetBackgroundColor(colDialogBg)
	hint.SetTextColor(colDialogDim)
	hint.SetWrap(false)
	hint.SetText("[" + colorName(colDialogDim) + "]" + tr("Enter — переименовать · Esc — отмена"))

	var okBtn, cancelBtn *tview.Button
	submit := func() {
		index, _ := columnDD.GetCurrentOption()
		newName := strings.TrimSpace(nameField.GetText())
		if index < 0 || index >= len(p.cols) {
			hint.SetText("[red]" + tr("Выберите колонку") + "[-:-:-]")
			return
		}
		oldName := p.cols[index].Name
		if newName == "" || newName == oldName {
			hint.SetText("[red]" + tr("Укажите новое имя колонки") + "[-:-:-]")
			return
		}
		sqlText := "ALTER TABLE " + db.QuoteIdent(p.obj.Name) + " RENAME COLUMN " +
			db.QuoteIdent(oldName) + " TO " + db.QuoteIdent(newName)
		p.alterSchema(p.obj.Name, p.obj.Name, sqlText, trf("Колонка %q переименована", oldName))
	}
	okBtn = dialogButton("OK", true, submit)
	cancelBtn = dialogButton("Отмена", false, p.app.closeDialog)
	p.app.dialogSave = submit

	columnDD.SetFinishedFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			p.app.app.SetFocus(nameField)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(cancelBtn)
		}
	})
	nameField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			submit()
		case tcell.KeyTab:
			p.app.app.SetFocus(okBtn)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(columnDD)
		}
	})
	okBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			p.app.app.SetFocus(cancelBtn)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(nameField)
		}
	})
	cancelBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			p.app.app.SetFocus(columnDD)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(okBtn)
		}
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(columnDD, 1, 0, true).
		AddItem(nameField, 1, 0, true).
		AddItem(hint, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)

	p.app.showFramedDialog(tr("Переименовать колонку"), content, 60, 10, columnDD)
	p.app.dialogEscape = func() bool { return columnDD.IsOpen() }
}

func (p *StructurePage) dropColumnDialog() {
	if len(p.cols) == 0 {
		p.app.showModal(tr("В таблице нет колонок."), []string{"OK"}, nil)
		return
	}
	names := make([]string, len(p.cols))
	for i, c := range p.cols {
		names[i] = c.Name
	}
	columnDD := tview.NewDropDown().SetLabel(tr("Колонка: ")).SetLabelWidth(10)
	columnDD.SetUseStyleTags(false)
	columnDD.SetOptions(names, nil)
	columnDD.SetCurrentOption(0)
	columnDD.SetLabelColor(colDialogTitle)
	columnDD.SetBackgroundColor(colDialogBg)
	columnDD.SetFieldBackgroundColor(colFieldBg)
	columnDD.SetFieldTextColor(colText)
	columnDD.SetListStyles(
		tcell.StyleDefault.Foreground(colDialogFg).Background(colDialogBg),
		tcell.StyleDefault.Foreground(colSelFg).Background(colSelBg).Bold(true),
	)

	hint := tview.NewTextView().SetDynamicColors(true)
	hint.SetBackgroundColor(colDialogBg)
	hint.SetTextColor(colDialogDim)
	hint.SetWrap(false)
	hint.SetText("[" + colorName(colDialogDim) + "]" + tr("Данные выбранной колонки будут удалены"))

	var okBtn, cancelBtn *tview.Button
	submit := func() {
		index, _ := columnDD.GetCurrentOption()
		if index < 0 || index >= len(p.cols) {
			hint.SetText("[red]" + tr("Выберите колонку") + "[-:-:-]")
			return
		}
		name := p.cols[index].Name
		p.app.closeDialog()
		p.app.showModal(trf("Удалить колонку %q?", name), []string{"Удалить"}, func(label string) {
			if label != "Удалить" {
				return
			}
			sqlText := "ALTER TABLE " + db.QuoteIdent(p.obj.Name) + " DROP COLUMN " + db.QuoteIdent(name)
			p.alterSchema(p.obj.Name, p.obj.Name, sqlText, trf("Колонка %q удалена", name))
		})
	}
	okBtn = dialogButton("OK", true, submit)
	cancelBtn = dialogButton("Отмена", false, p.app.closeDialog)
	p.app.dialogSave = submit

	columnDD.SetFinishedFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			p.app.app.SetFocus(okBtn)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(cancelBtn)
		}
	})
	okBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			p.app.app.SetFocus(cancelBtn)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(columnDD)
		}
	})
	cancelBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			p.app.app.SetFocus(columnDD)
		case tcell.KeyBacktab:
			p.app.app.SetFocus(okBtn)
		}
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(columnDD, 1, 0, true).
		AddItem(hint, 1, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)

	p.app.showFramedDialog(tr("Удалить колонку"), content, 60, 9, columnDD)
	p.app.dialogEscape = func() bool { return columnDD.IsOpen() }
}

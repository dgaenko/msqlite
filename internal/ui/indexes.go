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

func (a *App) createIndexDialog() {
	if a.data == nil || a.current == nil {
		return
	}
	if a.opts.ReadOnly {
		a.showModal(tr("База открыта в режиме только чтения.\nПереключите режим клавишей F8."), []string{"OK"}, nil)
		return
	}
	defCol := ""
	if len(a.data.cols) > 0 {
		defCol = a.data.cols[0].Name
	}
	name := "idx_" + a.data.obj.Name
	if defCol != "" {
		name += "_" + defCol
	}
	columns := defCol
	var unique bool

	form := tview.NewForm().
		AddInputField(tr("Имя индекса"), name, 0, nil, func(text string) { name = text }).
		AddInputField(tr("Колонки"), columns, 0, nil, func(text string) { columns = text }).
		AddCheckbox("UNIQUE", false, func(checked bool) { unique = checked })
	styleDialogForm(form)
	form.AddButton(tr("Создать"), func() {
		idxName := strings.TrimSpace(name)
		if idxName == "" {
			a.showError(fmt.Errorf("%s", tr("укажите имя индекса")))
			return
		}
		var parts []string
		for _, raw := range strings.Split(columns, ",") {
			col := strings.TrimSpace(raw)
			if col == "" {
				continue
			}
			parts = append(parts, db.QuoteIdent(col))
		}
		if len(parts) == 0 {
			a.showError(fmt.Errorf("%s", tr("укажите хотя бы одну колонку")))
			return
		}
		sqlText := "CREATE "
		if unique {
			sqlText += "UNIQUE "
		}
		sqlText += "INDEX " + db.QuoteIdent(idxName) + " ON " + db.QuoteIdent(a.data.obj.Name) +
			" (" + strings.Join(parts, ", ") + ")"
		a.closeDialog()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if _, err := a.current.Exec(ctx, sqlText, 1); err != nil {
			a.showError(err)
			return
		}
		a.refreshObjectsSelect(a.data.obj.Name)
		a.data.notice = tr("Индекс создан: ") + idxName
		a.notify(tr("Индекс создан: ") + idxName)
	}).
		AddButton(tr("Отмена"), a.closeDialog)
	a.showFramedDialog(tr("Создать индекс"), form, 64, 13, form)
}

func (a *App) dropIndexDialog() {
	if a.data == nil || a.current == nil {
		return
	}
	if a.opts.ReadOnly {
		a.showModal(tr("База открыта в режиме только чтения.\nПереключите режим клавишей F8."), []string{"OK"}, nil)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	indexes, err := a.current.Indexes(ctx, a.data.obj.Name)
	if err != nil {
		a.showError(err)
		return
	}
	var names []string
	for _, idx := range indexes {
		if idx.Origin == "c" {
			names = append(names, idx.Name)
		}
	}
	if len(names) == 0 {
		a.showModal(tr("В таблице нет индексов, созданных пользователем."), []string{"OK"}, nil)
		return
	}
	list := tview.NewList()
	styleList(list, colDialogBg, colDialogFg, colDialogDim)
	list.SetBorder(false)
	list.SetWrapAround(true)
	for _, name := range names {
		list.AddItem(name, "", 0, nil)
	}
	drop := func() {
		i := list.GetCurrentItem()
		if i < 0 || i >= len(names) {
			return
		}
		idxName := names[i]
		a.closeDialog()
		a.showModal(trf("Удалить индекс %s?", idxName), []string{"Удалить", "Отмена"}, func(label string) {
			if label != "Удалить" {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if _, err := a.current.Exec(ctx, "DROP INDEX "+db.QuoteIdent(idxName), 1); err != nil {
				a.showError(err)
				return
			}
			a.refreshObjectsSelect(a.data.obj.Name)
			a.notify(tr("Индекс удалён: ") + idxName)
		})
	}
	list.SetSelectedFunc(func(int, string, string, rune) { drop() })
	a.showListDialog(tr("Удалить индекс"), 60, list, drop)
}

func styleDialogForm(form *tview.Form) {
	form.SetBackgroundColor(colDialogBg)
	form.SetLabelColor(colDialogTitle)
	form.SetFieldBackgroundColor(colFieldBg)
	form.SetFieldTextColor(colText)
	form.SetButtonBackgroundColor(colOKBg)
	form.SetButtonTextColor(colOKFg)
	form.SetButtonActivatedStyle(tcell.StyleDefault.Foreground(colOKSelFg).Background(colOKBg).Bold(true))
}

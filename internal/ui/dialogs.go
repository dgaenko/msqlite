package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"msqlite/internal/db"
)

func (a *App) showDialog(p tview.Primitive) {
	if a.dialogOpen {
		a.closeDialog()
	}
	a.dialogOpen = true
	a.pages.AddPage(dialogPage, p, true, true)
	a.pages.ShowPage(dialogPage)
	a.app.SetFocus(p)
}

func (a *App) closeDialog() {
	if !a.dialogOpen {
		return
	}
	a.pages.RemovePage(dialogPage)
	a.dialogOpen = false
	a.dialogEscape = nil
	a.dialogSave = nil
	a.topMenu = nil
	a.activeModal = nil
	a.modalButtons = nil
	a.modalDone = nil
	a.modalIndex = 0
	a.pages.SwitchToPage(a.currentPage)
	a.focusPage()
}

func modalTitle(text string, buttons []string) string {
	if strings.HasPrefix(strings.TrimSpace(text), "Ошибка") {
		return "Ошибка"
	}
	for _, label := range buttons {
		switch label {
		case "Да", "Выход", "Удалить", "Перезаписать":
			return "Подтверждение"
		}
	}
	return "Сообщение"
}

func (a *App) showModal(text string, buttons []string, done func(label string)) {
	buttons = ensureCancelButton(buttons)
	rawTitle := modalTitle(text, buttons)
	title := tr(rawTitle)
	text = translateText(text)

	msg := tview.NewTextView().SetDynamicColors(true)
	msg.SetBackgroundColor(colDialogBg)
	msg.SetTextColor(colDialogFg)
	msg.SetWrap(true)
	msg.SetScrollable(true)
	if rawTitle == "Подтверждение" {
		msg.SetTextAlign(tview.AlignCenter)
	}
	msg.SetText(esc(text) + "\n\n[" + colorName(colDialogDim) + "]" + tr("Enter — подтвердить · Esc — отмена") + "[-:-:-]")

	actions := make([]func(), len(buttons))
	for i, label := range buttons {
		label := label
		actions[i] = func() {
			a.closeDialog()
			if done != nil {
				done(label)
			}
		}
	}
	btns := make([]*tview.Button, len(buttons))
	for i, label := range buttons {
		btns[i] = dialogButton(label, i == 0, actions[i])
	}
	for i, b := range btns {
		prev := btns[(i-1+len(btns))%len(btns)]
		next := btns[(i+1)%len(btns)]
		b.SetExitFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyTab:
				a.app.SetFocus(next)
			case tcell.KeyBacktab:
				a.app.SetFocus(prev)
			}
		})
	}
	msg.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			actions[0]()
		case tcell.KeyTab:
			a.app.SetFocus(btns[0])
		}
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(msg, 0, 1, true).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(btns...), 1, 0, false)

	screenWidth, screenHeight := 80, 24
	if a.screen != nil {
		if w, h := a.screen.Size(); w > 0 && h > 0 {
			screenWidth, screenHeight = w, h
		}
	}
	width := screenWidth / 3
	if width < 42 {
		width = 42
	}
	buttonsWidth := 0
	for _, b := range btns {
		buttonsWidth += tview.TaggedStringWidth(b.GetLabel()) + 4 + 1
	}
	if width < buttonsWidth+8 {
		width = buttonsWidth + 8
	}
	if maxWidth := screenWidth - 4; width > maxWidth {
		width = maxWidth
	}
	wrapped := tview.WordWrap(text+"\n\n"+tr("Enter — подтвердить · Esc — отмена"), width-4)
	height := len(wrapped) + 6
	if height < 8 {
		height = 8
	}
	if maxHeight := screenHeight - 2; height > maxHeight {
		height = maxHeight
	}
	a.showFramedDialog(title, content, width, height, msg)
}

func ensureCancelButton(buttons []string) []string {
	for _, label := range buttons {
		if label == "Отмена" {
			return buttons
		}
	}
	return append(append([]string{}, buttons...), "Отмена")
}

func dialogButton(label string, primary bool, action func()) *tview.Button {
	bg, fg, selFg := colCancelBg, colCancelFg, colCancelSelFg
	if primary {
		bg, fg, selFg = colOKBg, colOKFg, colOKSelFg
	}
	b := tview.NewButton(tr(label)).SetSelectedFunc(action)
	b.SetStyle(tcell.StyleDefault.Foreground(fg).Background(bg))
	b.SetActivatedStyle(tcell.StyleDefault.Foreground(selFg).Background(bg).Bold(true))
	b.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyRune && ev.Rune() == ' ' {
			return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
		}
		return ev
	})
	return b
}

func dialogButtonRow(buttons ...*tview.Button) *tview.Flex {
	width := 0
	for _, b := range buttons {
		if w := tview.TaggedStringWidth(b.GetLabel()) + 4; w > width {
			width = w
		}
	}
	row := tview.NewFlex().AddItem(nil, 0, 1, false)
	for i, b := range buttons {
		if i > 0 {
			row.AddItem(nil, 1, 0, false)
		}
		row.AddItem(b, width, 0, true)
	}
	return row.AddItem(nil, 0, 1, false)
}

func (a *App) showListDialog(title string, width int, list *tview.List, ok func()) {
	title = translateText(title)
	okBtn := dialogButton("OK", true, ok)
	cancelBtn := dialogButton("Отмена", false, a.closeDialog)
	list.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyTab:
			a.app.SetFocus(okBtn)
			return nil
		case tcell.KeyBacktab:
			a.app.SetFocus(cancelBtn)
			return nil
		}
		return ev
	})
	okBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(cancelBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(list)
		}
	})
	cancelBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(list)
		case tcell.KeyBacktab:
			a.app.SetFocus(okBtn)
		}
	})
	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)
	rows := list.GetItemCount()
	if rows > 18 {
		rows = 18
	}
	if rows < 3 {
		rows = 3
	}
	a.showFramedDialog(title, content, width, rows+6, list)
}

func (a *App) showFramedDialog(title string, content tview.Primitive, width, height int, focus tview.Primitive) {
	title = translateText(title)
	maxWidth, maxHeight := 80, 24
	if a.screen != nil {
		if w, h := a.screen.Size(); w > 0 && h > 0 {
			maxWidth, maxHeight = w, h
		}
	}
	maxWidth -= 4
	maxHeight -= 2
	if width > maxWidth {
		width = maxWidth
	}
	if height > maxHeight {
		height = maxHeight
	}
	if width < 30 {
		width = 30
	}
	if height < 8 {
		height = 8
	}
	frame := newEditFrame(title, content)
	grid := tview.NewGrid().
		SetRows(0, height, 0).
		SetColumns(0, width, 0).
		AddItem(frame, 1, 1, 1, 1, 0, 0, true)
	a.showDialog(grid)
	if focus != nil {
		a.app.SetFocus(focus)
	}
}

func (a *App) activateModal() {
	if a.activeModal == nil {
		return
	}
	label := ""
	if a.modalIndex >= 0 && a.modalIndex < len(a.modalButtons) {
		label = a.modalButtons[a.modalIndex]
	}
	done := a.modalDone
	a.closeDialog()
	if done != nil {
		done(label)
	}
}

func (a *App) moveModal(delta int) {
	if a.activeModal == nil || len(a.modalButtons) == 0 {
		return
	}
	n := len(a.modalButtons)
	a.modalIndex = (a.modalIndex + delta) % n
	if a.modalIndex < 0 {
		a.modalIndex += n
	}
	a.activeModal.SetFocus(a.modalIndex)
}

func (a *App) showError(err error) {
	if err == nil {
		return
	}
	a.showModal(tr("Ошибка")+":\n\n"+translateText(err.Error()), []string{"OK"}, nil)
}

func (a *App) confirmQuit() {
	a.showModal("Выйти из msqlite?", []string{"Выход", "Отмена"}, func(label string) {
		if label == "Выход" {
			a.app.Stop()
		}
	})
}

func (a *App) inputDialog(title, label, initial, hint string, accept func(string)) {
	title = translateText(title)
	label = translateText(label)
	hint = translateText(hint)
	field := tview.NewInputField().
		SetLabel(label).
		SetText(initial)
	field.SetLabelColor(colDialogTitle)
	field.SetFieldBackgroundColor(colFieldBg)
	field.SetFieldTextColor(colText)
	field.SetFieldWidth(0)

	apply := func() {
		value := field.GetText()
		a.closeDialog()
		if accept != nil {
			accept(value)
		}
	}
	okBtn := dialogButton("OK", true, apply)
	cancelBtn := dialogButton("Отмена", false, a.closeDialog)

	field.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			apply()
		case tcell.KeyTab:
			a.app.SetFocus(okBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(cancelBtn)
		}
	})
	okBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(cancelBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(field)
		}
	})
	cancelBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(field)
		case tcell.KeyBacktab:
			a.app.SetFocus(okBtn)
		}
	})

	hintView := tview.NewTextView()
	hintView.SetDynamicColors(true)
	hintView.SetBackgroundColor(colDialogBg)
	hintView.SetTextColor(colDialogDim)
	hintView.SetWrap(false)
	hintView.SetText("[" + colorName(colDialogDim) + "]" + esc(hint) + "[-:-:-]")

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(hintView, 1, 0, false).
		AddItem(field, 1, 0, true).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)

	width := tview.TaggedStringWidth(title) + 12
	if width < 56 {
		width = 56
	}
	if hintWidth := tview.TaggedStringWidth(hint) + 6; hintWidth > width {
		width = hintWidth
	}
	a.showFramedDialog(title, content, width, 8, field)
}

func (a *App) createDatabaseDialog() {
	initial := filepath.Join(a.dir, "new.db")
	a.inputDialog("Новая база данных", "Файл: ", initial,
		"Enter — создать · Esc — отмена", a.createDatabase)
}

func (a *App) createDatabase(value string) {
	path := strings.TrimSpace(value)
	if path == "" {
		return
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(a.dir, path)
	}
	path = filepath.Clean(path)
	if err := db.Create(path); err != nil {
		a.showError(err)
		return
	}
	a.openPath(path)
	a.notify(tr("База создана: ") + path)
}

func (a *App) openPathDialog() {
	initial := a.dir + string(os.PathSeparator)
	a.inputDialog("Открыть базу данных", "Путь: ", initial,
		"Enter — открыть · Esc — отмена",
		func(value string) { a.openPath(value) })
}

func (a *App) openPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(a.dir, path)
	}
	path = filepath.Clean(path)

	info, err := os.Stat(path)
	if err != nil {
		a.showError(fmt.Errorf("%s: %w", path, err))
		return
	}
	if info.IsDir() {
		a.changeDir(path, "")
		a.app.SetFocus(a.dbList)
		return
	}
	if !db.IsSQLiteFile(path) {
		a.showError(fmt.Errorf("%s: не является файлом SQLite", path))
		return
	}
	a.changeDir(filepath.Dir(path), path)
	a.app.SetFocus(a.table)
}

func (a *App) showValue(title, typ string, cell db.Cell) {
	tv := tview.NewTextView()
	tv.SetDynamicColors(false)
	tv.SetScrollable(true)
	tv.SetWrap(true)
	tv.SetBackgroundColor(colDialogBg)
	tv.SetTextColor(colDialogFg)
	tv.SetBorder(false)
	tv.SetText(cell.Full)

	okBtn := dialogButton("OK", true, a.closeDialog)
	cancelBtn := dialogButton("Отмена", false, a.closeDialog)
	tv.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			a.closeDialog()
		case tcell.KeyTab:
			a.app.SetFocus(okBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(cancelBtn)
		}
	})
	okBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(cancelBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(tv)
		}
	})
	cancelBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(tv)
		case tcell.KeyBacktab:
			a.app.SetFocus(okBtn)
		}
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tv, 0, 1, true).
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(okBtn, cancelBtn), 1, 0, false)

	header := title
	if typ != "" {
		header += "  [" + typ + "]"
	}
	a.showFramedDialog(header, content, 80, 18, tv)
}

package ui

import (
	"github.com/rivo/tview"
)

func (a *App) escape() {
	if len(a.stack) > 0 {
		a.back()
		return
	}
	if a.currentPage != pageMain {
		a.showPage(pageMain)
		return
	}
	a.confirmQuit()
}

func (a *App) back() {
	for len(a.stack) > 0 {
		prev := a.stack[len(a.stack)-1]
		a.stack = a.stack[:len(a.stack)-1]
		if prev != "" && prev != a.currentPage {
			a.showPage(prev)
			return
		}
	}
	a.showPage(pageMain)
}

func (a *App) showPage(name string) {
	focus := a.focus
	a.currentPage = name
	a.pages.SwitchToPage(name)
	a.focus = focus
	if name == pageMain {
		a.refreshCountsIfDirty()
	}
	a.focusPage()
}

func (a *App) openPage(name string, p tview.Primitive) {
	focus := a.focus
	if a.currentPage != "" && a.currentPage != name {
		a.stack = append(a.stack, a.currentPage)
	}
	a.pages.RemovePage(name)
	a.pages.AddPage(name, p, true, false)
	a.currentPage = name
	a.pages.SwitchToPage(name)
	a.focus = focus
	a.focusPage()
}

func (a *App) focusPage() {
	switch a.currentPage {
	case pageMain:
		a.updateHeader()
		if a.focus == 1 {
			a.app.SetFocus(a.table)
		} else {
			a.app.SetFocus(a.dbList)
		}
	case pageData:
		if a.data != nil {
			a.app.SetFocus(a.data.table)
		}
	case pageStructure:
		if a.structure != nil {
			a.app.SetFocus(a.structure.text)
		}
	case pageSQL:
		if a.sql != nil {
			if a.sql.focusGrid {
				a.app.SetFocus(a.sql.table)
			} else {
				a.app.SetFocus(a.sql.input)
			}
		}
	case pageInsert:
		if a.insert != nil {
			a.insert.focus()
		}
	case pageHelp:
		if a.help != nil {
			a.app.SetFocus(a.help.text)
		}
	}
}

package ui

import (
	"fmt"
	"strconv"
	"strings"
)

func (a *App) setTheme(name string) {
	if name == activeTheme {
		return
	}
	if !setActiveTheme(name) {
		return
	}
	applyTheme()
	a.restyleTheme()
	a.persistSettings()
	a.notify("Тема интерфейса: " + themeLabel(name))
}

func (a *App) toggleSplash() {
	a.opts.NoSplash = !a.opts.NoSplash
	a.persistSettings()
	if a.opts.NoSplash {
		a.notify("Сплэш при запуске выключен")
	} else {
		a.notify("Сплэш при запуске включён")
	}
}

func (a *App) toggleRememberDir() {
	a.rememberDir = !a.rememberDir
	a.persistSettings()
	if a.rememberDir {
		a.notify("Запоминание последней папки включено")
	} else {
		a.notify("Запоминание последней папки выключено")
	}
}

func (a *App) toggleColumnLines() {
	a.colLines = !a.colLines
	a.applyColumnLines()
	a.persistSettings()
	if a.colLines {
		a.notify("Разделители колонок включены")
	} else {
		a.notify("Разделители колонок выключены")
	}
}

func (a *App) applyColumnLines() {
	if a.data != nil {
		a.data.applyColumnLines()
	}
	if a.sql != nil {
		a.sql.applyColumnLines()
	}
}

func (a *App) pageSizeDialog() {
	a.inputDialog("Настройки", "Строк на страницу: ", strconv.Itoa(a.opts.PageSize),
		"1–10000 · Enter — сохранить · Esc — отмена", a.setPageSize)
}

func (a *App) setPageSize(value string) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 || n > 10000 {
		a.showError(fmt.Errorf("размер страницы должен быть от 1 до 10000"))
		return
	}
	a.opts.PageSize = n
	if a.data != nil {
		a.data.size = n
		a.data.offset = 0
		if a.currentPage == pageData {
			if err := a.data.Load(); err != nil {
				a.showError(err)
				return
			}
		}
	}
	a.persistSettings()
	a.notify(fmt.Sprintf("Размер страницы: %d", n))
}

func (a *App) restyleTheme() {
	a.mainHeader.SetBackgroundColor(colBarBg)
	a.mainHeader.SetTextColor(colText)
	a.mainStatus.SetBackgroundColor(colStatusBg)
	a.mainStatus.SetTextColor(colStatusFg)
	a.mainHints.SetBackgroundColor(colHintBg)
	a.mainHints.SetTextColor(colHintFg)
	a.setMainHints()

	styleList(a.dbList, colPanelBg, colText, colDim)
	styleObjectTable(a.table)
	a.panelFocused(a.focus)
	a.restyleObjectTableCells()

	switch a.currentPage {
	case pageData:
		if a.data != nil {
			a.data.restyle()
		}
	case pageStructure:
		if a.structure != nil {
			a.structure.restyle()
		}
	case pageSQL:
		if a.sql != nil {
			a.sql.restyle()
		}
	case pageInsert:
		if a.insert != nil {
			a.insert.restyle()
		}
	case pageHelp:
		if a.help != nil {
			a.help.restyle()
		}
	}

	a.updateHeader()
	a.updateMainStatus()
}

func (a *App) restyleObjectTableCells() {
	rows := a.table.GetRowCount()
	cols := a.table.GetColumnCount()
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			cell := a.table.GetCell(r, c)
			if cell == nil {
				continue
			}
			color := colDim
			if len(a.objects) > 0 {
				switch {
				case r == 0:
					color = colTitle
				case c == 0:
					color = colText
				}
			}
			cell.SetTextColor(color)
		}
	}
}

func (a *App) togglePanel() {
	if a.focus == 0 {
		a.app.SetFocus(a.table)
	} else {
		a.app.SetFocus(a.dbList)
	}
}

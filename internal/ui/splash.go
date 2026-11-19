package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const splashArt = `███╗   ███╗███████╗ ██████╗ ██╗     ██╗████████╗███████╗
████╗ ████║██╔════╝██╔═══██╗██║     ██║╚══██╔══╝██╔════╝
██╔████╔██║███████╗██║   ██║██║     ██║   ██║   █████╗
██║╚██╔╝██║╚════██║██║▄▄ ██║██║     ██║   ██║   ██╔══╝
██║ ╚═╝ ██║███████║╚██████╔╝███████╗██║   ██║   ███████╗
╚═╝     ╚═╝╚══════╝ ╚═════╝ ╚══════╝╚═╝   ╚═╝   ╚══════╝`

func splashColors() (bg, fg, border, title, dim, accent, art tcell.Color) {
	if isDOSTheme(activeTheme) {
		return colDialogBg, colDialogFg, colDialogBorder, colDialogTitle, colDialogDim, colTitle, colDialogFg
	}
	return colHeaderBg, colText, colDialogBorder, colDialogTitle, colDim, colTitle, colTitle
}

// buildSplash returns the startup modal.
func (a *App) buildSplash() tview.Primitive {
	bg, fg, border, title, dim, accent, art := splashColors()
	credit := func(label, name string) string {
		return "[" + colorName(accent) + "]" + padRight(label+":", 14) + "[-:-:-]" +
			"[" + colorName(fg) + "]" + padRight(name, 15)
	}
	text := fmt.Sprintf(
		"\n\n[%s::b]%s[-:-:-]\n\n"+
			"[%s::b]SQLite manager для терминала[-:-:-]\n"+
			"[%s]в стиле Midnight Commander / Far Manager / DOS Navigator[-:-:-]\n\n"+
			"%s\n%s\n%s\n\n"+
			"[%s]Нажмите любую клавишу…   ·   v%s[-:-:-]",
		colorName(art), splashArt,
		colorName(fg),
		colorName(dim),
		credit("Автор идеи", "Павел Гусаров"),
		credit("Создатель", "Дмитрий Гаенко"),
		credit("Разработчик", "DeepSeek"),
		colorName(dim), esc(a.opts.Version))

	v := tview.NewTextView().SetDynamicColors(true)
	v.SetTextAlign(tview.AlignCenter)
	v.SetBackgroundColor(bg)
	v.SetTextColor(fg)
	v.SetBorder(true)
	v.SetBorderColor(border)
	v.SetTitle(" msqlite ")
	v.SetTitleColor(title)
	v.SetTitleAlign(tview.AlignCenter)
	v.SetText(text)
	a.splashText = v

	// Center a fixed-height window over the main screen.
	horizontal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(v, 0, 8, true).
		AddItem(nil, 0, 1, false)
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(horizontal, 19, 0, true).
		AddItem(nil, 0, 1, false)
}

func (a *App) showSplash() {
	a.pages.AddPage(pageSplash, a.buildSplash(), true, true)
	a.pages.ShowPage(pageSplash)
	a.splashActive = true
	a.app.SetFocus(a.splashText)
	a.splashTimer = time.AfterFunc(3*time.Second, func() {
		a.app.QueueUpdateDraw(func() { a.dismissSplash() })
	})
}

func (a *App) dismissSplash() {
	if !a.splashActive {
		return
	}
	a.splashActive = false
	if a.splashTimer != nil {
		a.splashTimer.Stop()
		a.splashTimer = nil
	}
	a.pages.RemovePage(pageSplash)
	a.focusPage()
}

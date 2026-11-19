package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	themeMC      = "mc"
	themeDOS     = "dos"
	themeDOSBlue = "dos-blue"

	defaultTheme = themeMC
)

func isDOSTheme(name string) bool {
	return name == themeDOS || name == themeDOSBlue
}

type themePalette struct {
	title        string
	bg           tcell.Color
	panelBg      tcell.Color
	text         tcell.Color
	dim          tcell.Color
	accent       tcell.Color
	panelTitle   tcell.Color
	border       tcell.Color
	focus        tcell.Color
	selBg        tcell.Color
	selFg        tcell.Color
	header       tcell.Color
	bar          tcell.Color
	status       tcell.Color
	statusF      tcell.Color
	hintBg       tcell.Color
	hintFg       tcell.Color
	keyBg        tcell.Color
	keyFg        tcell.Color
	error        tcell.Color
	ok           tcell.Color
	blob         tcell.Color
	dialog       tcell.Color
	dialogFg     tcell.Color
	dialogDm     tcell.Color
	dialogBorder tcell.Color
	dialogTitle  tcell.Color
	buttonBg     tcell.Color
	buttonFg     tcell.Color
	buttonSelFg  tcell.Color
	okBg         tcell.Color
	okFg         tcell.Color
	okSelFg      tcell.Color
	cancelBg     tcell.Color
	cancelFg     tcell.Color
	cancelSelFg  tcell.Color
	menuBg       tcell.Color
	menuFg       tcell.Color
	menuBorder   tcell.Color
	menuBarFg    tcell.Color
	menuBarSelBg tcell.Color
	menuBarSelFg tcell.Color
	menuSelBg    tcell.Color
	menuSelFg    tcell.Color
	fieldBg      tcell.Color
	cursor       tcell.Color
	cursorF      tcell.Color
}

var themes = map[string]themePalette{
	themeMC: {
		title:        "Midnight Commander",
		bg:           tcell.NewHexColor(0x000087),
		panelBg:      tcell.NewHexColor(0x000087),
		text:         tcell.ColorWhite,
		dim:          tcell.NewHexColor(0xAAAAAA),
		accent:       tcell.ColorYellow,
		panelTitle:   tcell.ColorYellow,
		border:       tcell.ColorLightGray,
		focus:        tcell.ColorYellow,
		selBg:        tcell.NewHexColor(0x00AAAA),
		selFg:        tcell.ColorBlack,
		header:       tcell.NewHexColor(0x00005F),
		bar:          tcell.NewHexColor(0x000087),
		status:       tcell.NewHexColor(0x00AAAA),
		statusF:      tcell.ColorBlack,
		hintBg:       tcell.NewHexColor(0x000087),
		hintFg:       tcell.ColorWhite,
		keyBg:        tcell.ColorYellow,
		keyFg:        tcell.ColorBlack,
		error:        tcell.NewHexColor(0xFF6B6B),
		ok:           tcell.ColorLightGreen,
		blob:         tcell.NewHexColor(0xFF87FF),
		dialog:       tcell.NewHexColor(0x005F5F),
		dialogFg:     tcell.ColorWhite,
		dialogDm:     tcell.NewHexColor(0xAAAAAA),
		dialogBorder: tcell.ColorYellow,
		dialogTitle:  tcell.ColorYellow,
		buttonBg:     tcell.NewHexColor(0x005F5F),
		buttonFg:     tcell.ColorWhite,
		buttonSelFg:  tcell.ColorYellow,
		okBg:         tcell.NewHexColor(0x00AAAA),
		okFg:         tcell.ColorBlack,
		okSelFg:      tcell.ColorYellow,
		cancelBg:     tcell.NewHexColor(0x005F5F),
		cancelFg:     tcell.ColorWhite,
		cancelSelFg:  tcell.ColorYellow,
		menuBg:       tcell.NewHexColor(0x00AAAA),
		menuFg:       tcell.NewHexColor(0xFCFCFC),
		menuBorder:   tcell.NewHexColor(0xFCFCFC),
		menuBarFg:    tcell.NewHexColor(0xFCFC54),
		menuBarSelBg: tcell.ColorBlack,
		menuBarSelFg: tcell.NewHexColor(0xFCFC54),
		menuSelBg:    tcell.ColorBlack,
		menuSelFg:    tcell.NewHexColor(0xFCFCFC),
		fieldBg:      tcell.ColorBlack,
		cursor:       tcell.ColorYellow,
		cursorF:      tcell.ColorBlack,
	},
	themeDOS: {
		title:        "DOS Navigator",
		bg:           tcell.NewHexColor(0x555555),
		panelBg:      tcell.NewHexColor(0x555555),
		text:         tcell.NewHexColor(0xFFFFFF),
		dim:          tcell.NewHexColor(0xAAAAAA),
		accent:       tcell.NewHexColor(0xFFFF55),
		panelTitle:   tcell.NewHexColor(0xFFFF55),
		border:       tcell.NewHexColor(0xFFFFFF),
		focus:        tcell.NewHexColor(0x55FFFF),
		selBg:        tcell.NewHexColor(0x00AAAA),
		selFg:        tcell.ColorBlack,
		header:       tcell.NewHexColor(0x0000AA),
		bar:          tcell.NewHexColor(0x0000AA),
		status:       tcell.NewHexColor(0x555555),
		statusF:      tcell.NewHexColor(0xFFFF55),
		hintBg:       tcell.NewHexColor(0xAAAAAA),
		hintFg:       tcell.ColorBlack,
		keyBg:        tcell.NewHexColor(0xAAAAAA),
		keyFg:        tcell.NewHexColor(0xAA0000),
		error:        tcell.NewHexColor(0xFF5555),
		ok:           tcell.NewHexColor(0x55FF55),
		blob:         tcell.NewHexColor(0xFF55FF),
		dialog:       tcell.NewHexColor(0xA5A5A5),
		dialogFg:     tcell.ColorBlack,
		dialogDm:     tcell.NewHexColor(0x555555),
		dialogBorder: tcell.ColorWhite,
		dialogTitle:  tcell.ColorWhite,
		buttonBg:     tcell.NewHexColor(0x545454),
		buttonFg:     tcell.ColorWhite,
		buttonSelFg:  tcell.NewHexColor(0xFFFF55),
		okBg:         tcell.NewHexColor(0x00AAAA),
		okFg:         tcell.ColorBlack,
		okSelFg:      tcell.NewHexColor(0xFFFF55),
		cancelBg:     tcell.NewHexColor(0x545454),
		cancelFg:     tcell.NewHexColor(0xFFFFFF),
		cancelSelFg:  tcell.NewHexColor(0xFFFF55),
		menuBg:       tcell.NewHexColor(0xAAAAAA),
		menuFg:       tcell.ColorBlack,
		menuBorder:   tcell.ColorBlack,
		menuBarFg:    tcell.ColorBlack,
		menuBarSelBg: tcell.NewHexColor(0x00AAAA),
		menuBarSelFg: tcell.ColorBlack,
		menuSelBg:    tcell.NewHexColor(0x00AAAA),
		menuSelFg:    tcell.ColorBlack,
		fieldBg:      tcell.ColorBlack,
		cursor:       tcell.NewHexColor(0x00AAAA),
		cursorF:      tcell.ColorBlack,
	},
	themeDOSBlue: {
		title:        "DOS Navigator Blue",
		bg:           tcell.NewHexColor(0x0000A8),
		panelBg:      tcell.NewHexColor(0x0000A8),
		text:         tcell.NewHexColor(0xFCFCFC),
		dim:          tcell.NewHexColor(0xA8A8A8),
		accent:       tcell.NewHexColor(0xFCFC54),
		panelTitle:   tcell.NewHexColor(0xFCFCFC),
		border:       tcell.NewHexColor(0x54FCFC),
		focus:        tcell.NewHexColor(0x54FCFC),
		selBg:        tcell.NewHexColor(0x00A8A8),
		selFg:        tcell.ColorBlack,
		header:       tcell.NewHexColor(0x00A8A8),
		bar:          tcell.ColorBlack,
		status:       tcell.NewHexColor(0x0000A8),
		statusF:      tcell.NewHexColor(0xFCFC54),
		hintBg:       tcell.ColorBlack,
		hintFg:       tcell.NewHexColor(0xFCFCFC),
		keyBg:        tcell.ColorBlack,
		keyFg:        tcell.NewHexColor(0xA80000),
		error:        tcell.NewHexColor(0xA80000),
		ok:           tcell.NewHexColor(0x00A800),
		blob:         tcell.NewHexColor(0xFC54FC),
		dialog:       tcell.NewHexColor(0xA8A8A8),
		dialogFg:     tcell.ColorBlack,
		dialogDm:     tcell.NewHexColor(0x545454),
		dialogBorder: tcell.NewHexColor(0xFCFCFC),
		dialogTitle:  tcell.NewHexColor(0xFCFCFC),
		buttonBg:     tcell.NewHexColor(0x00A800),
		buttonFg:     tcell.ColorBlack,
		buttonSelFg:  tcell.ColorBlack,
		okBg:         tcell.NewHexColor(0x5252FF),
		okFg:         tcell.NewHexColor(0xFFFFFF),
		okSelFg:      tcell.NewHexColor(0xFCFC54),
		cancelBg:     tcell.NewHexColor(0x545454),
		cancelFg:     tcell.NewHexColor(0xFCFCFC),
		cancelSelFg:  tcell.NewHexColor(0xFCFC54),
		menuBg:       tcell.NewHexColor(0x000000),
		menuFg:       tcell.NewHexColor(0xFCFCFC),
		menuBorder:   tcell.NewHexColor(0x54FCFC),
		menuBarFg:    tcell.NewHexColor(0xFCFCFC),
		menuBarSelBg: tcell.NewHexColor(0x00A8A8),
		menuBarSelFg: tcell.ColorBlack,
		menuSelBg:    tcell.NewHexColor(0x00A8A8),
		menuSelFg:    tcell.ColorBlack,
		fieldBg:      tcell.ColorBlack,
		cursor:       tcell.NewHexColor(0x00A8A8),
		cursorF:      tcell.ColorBlack,
	},
}

var (
	activeTheme = defaultTheme

	colBg           tcell.Color
	colPanelBg      tcell.Color
	colText         tcell.Color
	colDim          tcell.Color
	colTitle        tcell.Color
	colPanelTitle   tcell.Color
	colBorder       tcell.Color
	colFocus        tcell.Color
	colSelBg        tcell.Color
	colSelFg        tcell.Color
	colHeaderBg     tcell.Color
	colBarBg        tcell.Color
	colStatusBg     tcell.Color
	colStatusFg     tcell.Color
	colHintBg       tcell.Color
	colHintFg       tcell.Color
	colKeyBg        tcell.Color
	colKeyFg        tcell.Color
	colError        tcell.Color
	colOK           tcell.Color
	colBlob         tcell.Color
	colDialogBg     tcell.Color
	colDialogFg     tcell.Color
	colDialogDim    tcell.Color
	colDialogBorder tcell.Color
	colDialogTitle  tcell.Color
	colButtonBg     tcell.Color
	colButtonFg     tcell.Color
	colButtonSelFg  tcell.Color
	colOKBg         tcell.Color
	colOKFg         tcell.Color
	colOKSelFg      tcell.Color
	colCancelBg     tcell.Color
	colCancelFg     tcell.Color
	colCancelSelFg  tcell.Color
	colMenuBg       tcell.Color
	colMenuFg       tcell.Color
	colMenuBorder   tcell.Color
	colMenuBarFg    tcell.Color
	colMenuBarSelBg tcell.Color
	colMenuBarSelFg tcell.Color
	colMenuSelBg    tcell.Color
	colMenuSelFg    tcell.Color
	colFieldBg      tcell.Color
	colCursorBg     tcell.Color
	colCursorFg     tcell.Color
)

func setActiveTheme(name string) bool {
	if _, ok := themes[name]; !ok {
		return false
	}
	activeTheme = name
	return true
}

func themeLabel(name string) string {
	if p, ok := themes[name]; ok {
		return p.title
	}
	return name
}

func themeMarker(name string) string {
	if name == activeTheme {
		return " ✓"
	}
	return ""
}

func applyTheme() {
	p, ok := themes[activeTheme]
	if !ok {
		p = themes[defaultTheme]
	}
	colBg = p.bg
	colPanelBg = p.panelBg
	colText = p.text
	colDim = p.dim
	colTitle = p.accent
	colPanelTitle = p.panelTitle
	colBorder = p.border
	colFocus = p.focus
	colSelBg = p.selBg
	colSelFg = p.selFg
	colHeaderBg = p.header
	colBarBg = p.bar
	colStatusBg = p.status
	colStatusFg = p.statusF
	colHintBg = p.hintBg
	colHintFg = p.hintFg
	colKeyBg = p.keyBg
	colKeyFg = p.keyFg
	colError = p.error
	colOK = p.ok
	colBlob = p.blob
	colDialogBg = p.dialog
	colDialogFg = p.dialogFg
	colDialogDim = p.dialogDm
	colDialogBorder = p.dialogBorder
	colDialogTitle = p.dialogTitle
	colButtonBg = p.buttonBg
	colButtonFg = p.buttonFg
	colButtonSelFg = p.buttonSelFg
	colOKBg = p.okBg
	colOKFg = p.okFg
	colOKSelFg = p.okSelFg
	colCancelBg = p.cancelBg
	colCancelFg = p.cancelFg
	colCancelSelFg = p.cancelSelFg
	colMenuBg = p.menuBg
	colMenuFg = p.menuFg
	colMenuBorder = p.menuBorder
	colMenuBarFg = p.menuBarFg
	colMenuBarSelBg = p.menuBarSelBg
	colMenuBarSelFg = p.menuBarSelFg
	colMenuSelBg = p.menuSelBg
	colMenuSelFg = p.menuSelFg
	colFieldBg = p.fieldBg
	colCursorBg = p.cursor
	colCursorFg = p.cursorF

	tview.Styles.PrimitiveBackgroundColor = colBg
	tview.Styles.ContrastBackgroundColor = colPanelBg
	tview.Styles.MoreContrastBackgroundColor = colDialogBg
	tview.Styles.PrimaryTextColor = colText
	tview.Styles.SecondaryTextColor = colDim
	tview.Styles.TertiaryTextColor = tcell.ColorGray
	tview.Styles.InverseTextColor = tcell.ColorBlack
	tview.Styles.ContrastSecondaryTextColor = colText
	tview.Styles.BorderColor = colBorder
	tview.Styles.TitleColor = colTitle
	tview.Styles.GraphicsColor = colBorder
}

func styleList(l *tview.List, bg, fg, dim tcell.Color) {
	l.SetBackgroundColor(bg)
	l.SetMainTextStyle(tcell.StyleDefault.Foreground(fg).Background(bg))
	l.SetSecondaryTextStyle(tcell.StyleDefault.Foreground(dim).Background(bg))
	l.SetShortcutStyle(tcell.StyleDefault.Foreground(dim).Background(bg))
	l.SetSelectedStyle(tcell.StyleDefault.Foreground(colSelFg).Background(colSelBg).Bold(true))
	l.SetSelectedFocusOnly(false)
	l.SetHighlightFullLine(true)
	l.ShowSecondaryText(false)
	l.SetWrapAround(false)
	l.SetBorder(true)
	l.SetBorderColor(colBorder)
	l.SetTitleColor(colTitle)
	l.SetTitleAlign(tview.AlignCenter)
}

func styleTable(t *tview.Table) {
	t.SetBackgroundColor(colPanelBg)
	t.SetBordersColor(colBorder)
	t.SetBorders(false)
	t.SetSeparator(' ')
	t.SetSelectedStyle(tcell.StyleDefault.Foreground(colSelFg).Background(colSelBg))
	t.SetFixed(2, 1)
	t.SetSelectable(true, true)
	t.SetWrapSelection(false, false)
}

func styleObjectTable(t *tview.Table) {
	t.SetBackgroundColor(colPanelBg)
	t.SetBordersColor(colBorder)
	t.SetBorders(false)
	t.SetSeparator(' ')
	t.SetSelectedStyle(tcell.StyleDefault.Foreground(colSelFg).Background(colSelBg).Bold(true))
	t.SetFixed(1, 0)
	t.SetSelectable(true, false)
	t.SetWrapSelection(false, false)
	t.SetBorder(true)
	t.SetBorderColor(colBorder)
	t.SetTitleColor(colTitle)
	t.SetTitleAlign(tview.AlignCenter)
}

func styleTextView(v *tview.TextView) {
	v.SetDynamicColors(true)
	v.SetBackgroundColor(colPanelBg)
	v.SetTextColor(colText)
}

func colorName(c tcell.Color) string {
	switch c {
	case tcell.ColorYellow:
		return "yellow"
	case tcell.ColorBlack:
		return "black"
	case tcell.ColorWhite:
		return "white"
	case tcell.ColorSilver:
		return "silver"
	}
	if name := c.Name(); name != "" {
		return name
	}
	if css := c.CSS(); css != "" {
		return css
	}
	return "default"
}

func esc(s string) string { return tview.Escape(s) }

func padRight(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func padLeft(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return strings.Repeat(" ", width-n) + s
}

func truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	r := []rune(s)
	return string(r[:limit]) + "…"
}

func onOff(v bool) string {
	if v {
		return tr("вкл")
	}
	return tr("выкл")
}

func typeLabel(t string) string {
	if t == "view" {
		return tr("представление")
	}
	return tr("таблица")
}

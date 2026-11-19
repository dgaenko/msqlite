package ui

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/crypto/bcrypt"

	"msqlite/internal/db"
)

type cellEditorKind int

const (
	cellEditorNumber cellEditorKind = iota
	cellEditorText
	cellEditorOther
)

// editCellDialog opens an editor sized for the column type.
func (a *App) editCellDialog(p *DataPage, row, col int) {
	cell := p.rows[row-2][col-1]
	column := p.cols[col-1]
	kind := editorKind(column, cell)

	text := cell.Full
	if cell.Null {
		text = ""
	}

	var (
		editor   tview.Primitive
		readText func() string
		area     *tview.TextArea
		field    *tview.InputField
	)
	if kind == cellEditorNumber {
		field = tview.NewInputField().
			SetText(text).
			SetFieldWidth(0)
		field.SetFieldBackgroundColor(colFieldBg)
		field.SetFieldTextColor(colText)
		field.SetPlaceholder("NULL")
		field.SetPlaceholderStyle(tcell.StyleDefault.Foreground(colDim).Background(colFieldBg).Italic(true))
		readText = field.GetText
		editor = field
	} else {
		area = tview.NewTextArea().
			SetText(text, false).
			SetMaxLength(1 << 20)
		area.SetWrap(true)
		area.SetWordWrap(true)
		area.SetBackgroundColor(colFieldBg)
		area.SetTextStyle(tcell.StyleDefault.Foreground(colText).Background(colFieldBg))
		area.SetPlaceholder("NULL")
		area.SetPlaceholderStyle(tcell.StyleDefault.Foreground(colDim).Background(colFieldBg).Italic(true))
		readText = area.GetText
		editor = area
	}
	if cell.Null {
		if field != nil {
			field.SetText("")
		} else {
			area.SetText("", false)
		}
	}

	save := func() {
		value := readText()
		a.closeDialog()
		p.saveCell(row, col, parseEditorValue(value))
	}

	title := fmt.Sprintf("Правка · %s.%s", p.obj.Name, column.Name)
	headText := fmt.Sprintf("[%s::b]%s.%s[-:-:-]", colorName(colDialogTitle), esc(p.obj.Name), esc(column.Name))
	if column.Type != "" {
		headText += fmt.Sprintf("  [%s][%s][-:-:-]", colorName(colDialogDim), esc(column.Type))
	}
	headText += fmt.Sprintf("  [%s]· строка %d[-:-:-]", colorName(colDialogDim), int64(p.offset)+int64(row-2)+1)
	head := tview.NewTextView().SetDynamicColors(true)
	head.SetBackgroundColor(colDialogBg)
	head.SetTextColor(colDialogFg)
	head.SetWrap(false)
	head.SetText(headText)

	hint := tview.NewTextView().SetDynamicColors(true)
	hint.SetBackgroundColor(colDialogBg)
	hint.SetTextColor(colDialogDim)
	hint.SetWrap(false)
	if kind == cellEditorNumber {
		hint.SetText("[" + colorName(colDialogDim) + "]Enter — сохранить · Esc — отмена · пусто — NULL")
	} else if kind == cellEditorText {
		hint.SetText("[" + colorName(colDialogDim) + "]Ctrl+S — сохранить · Tab — кнопки · Esc — отмена · пусто — NULL")
	} else {
		hint.SetText("[" + colorName(colDialogDim) + "]Ctrl+S — сохранить · Tab — кнопки · Esc — отмена · пусто — NULL · x'ABCD' — BLOB")
	}

	var hashBtns []*tview.Button
	var hashRow tview.Primitive
	if kind == cellEditorText {
		hashLabel := tview.NewTextView().SetDynamicColors(true)
		hashLabel.SetBackgroundColor(colDialogBg)
		hashLabel.SetTextColor(colDialogTitle)
		hashLabel.SetText("Хеш: ")
		hashRow = tview.NewFlex().
			AddItem(hashLabel, 6, 0, false)
		for index, opt := range cellHashOptions {
			opt := opt
			if index > 0 {
				hashRow.(*tview.Flex).AddItem(nil, 1, 0, false)
			}
			btn := dialogButton(opt.label, false, func() {
				value, err := opt.hash(area.GetText())
				if err != nil {
					hint.SetText("[red]Ошибка: " + esc(err.Error()) + "[-:-:-]")
					return
				}
				area.SetText(value, true)
			})
			hashBtns = append(hashBtns, btn)
			hashRow.(*tview.Flex).AddItem(btn, tview.TaggedStringWidth(opt.label)+4, 0, true)
		}
	}

	saveBtn := dialogButton("OK", true, save)
	cancelBtn := dialogButton("Отмена", false, a.closeDialog)

	focusEditor := func() { a.app.SetFocus(editor) }
	prevFromSave := editor
	if len(hashBtns) > 0 {
		prevFromSave = hashBtns[len(hashBtns)-1]
	}
	for i, btn := range hashBtns {
		prev := tview.Primitive(editor)
		if i > 0 {
			prev = hashBtns[i-1]
		}
		next := tview.Primitive(saveBtn)
		if i+1 < len(hashBtns) {
			next = hashBtns[i+1]
		}
		previous, following := prev, next
		btn.SetExitFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyTab:
				a.app.SetFocus(following)
			case tcell.KeyBacktab:
				a.app.SetFocus(previous)
			}
		})
	}
	saveBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			a.app.SetFocus(cancelBtn)
		case tcell.KeyBacktab:
			a.app.SetFocus(prevFromSave)
		}
	})
	cancelBtn.SetExitFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyTab:
			focusEditor()
		case tcell.KeyBacktab:
			a.app.SetFocus(saveBtn)
		}
	})

	if field != nil {
		field.SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				save()
			case tcell.KeyTab:
				a.app.SetFocus(saveBtn)
			case tcell.KeyBacktab:
				a.app.SetFocus(cancelBtn)
			}
		})
	} else {
		area.SetFinishedFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyTab:
				if len(hashBtns) > 0 {
					a.app.SetFocus(hashBtns[0])
				} else {
					a.app.SetFocus(saveBtn)
				}
			case tcell.KeyBacktab:
				a.app.SetFocus(cancelBtn)
			}
		})
	}

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(head, 1, 0, false).
		AddItem(editor, 0, 1, true).
		AddItem(hint, 1, 0, false)
	if hashRow != nil {
		content.AddItem(nil, 1, 0, false)
		content.AddItem(hashRow, 1, 0, false)
	}
	content.
		AddItem(nil, 1, 0, false).
		AddItem(dialogButtonRow(saveBtn, cancelBtn), 1, 0, false)

	width, height := a.cellEditorSize(kind, text)
	frame := newEditFrame(title, content)
	grid := tview.NewGrid().
		SetRows(0, height, 0).
		SetColumns(0, width, 0).
		AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	a.dialogSave = save
	a.showDialog(grid)
	a.app.SetFocus(editor)
}

type cellHashOption struct {
	label string
	hash  func(string) (string, error)
}

var cellHashOptions = []cellHashOption{
	{"MD5", func(s string) (string, error) {
		sum := md5.Sum([]byte(s))
		return hex.EncodeToString(sum[:]), nil
	}},
	{"SHA-1", func(s string) (string, error) {
		sum := sha1.Sum([]byte(s))
		return hex.EncodeToString(sum[:]), nil
	}},
	{"SHA-256", func(s string) (string, error) {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:]), nil
	}},
	{"SHA-512", func(s string) (string, error) {
		sum := sha512.Sum512([]byte(s))
		return hex.EncodeToString(sum[:]), nil
	}},
	{"bcrypt", func(s string) (string, error) {
		data, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
		return string(data), err
	}},
}

func editorKind(column db.Column, cell db.Cell) cellEditorKind {
	if cell.Blob {
		return cellEditorOther
	}
	typ := strings.ToUpper(strings.TrimSpace(column.Type))
	switch {
	case strings.Contains(typ, "INT"), strings.Contains(typ, "REAL"),
		strings.Contains(typ, "FLOA"), strings.Contains(typ, "DOUB"),
		strings.Contains(typ, "NUM"), strings.Contains(typ, "DEC"):
		return cellEditorNumber
	case strings.Contains(typ, "CHAR"), strings.Contains(typ, "CLOB"),
		strings.Contains(typ, "TEXT"):
		return cellEditorText
	default:
		return cellEditorOther
	}
}

func (a *App) cellEditorSize(kind cellEditorKind, text string) (int, int) {
	width, height := 56, 9
	switch kind {
	case cellEditorText:
		width, height = 76, 17
	case cellEditorOther:
		width, height = 88, 20
	}
	if kind != cellEditorNumber {
		longest := 0
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			if n := utf8.RuneCountInString(line); n > longest {
				longest = n
			}
		}
		if longest+8 > width {
			width = longest + 8
		}
		if len(lines)+10 > height {
			height = len(lines) + 10
		}
	}
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
	return width, height
}

type editFrame struct {
	*tview.Box
	title string
	child tview.Primitive
}

func newEditFrame(title string, child tview.Primitive) *editFrame {
	f := &editFrame{Box: tview.NewBox(), title: title, child: child}
	f.SetBackgroundColor(colDialogBg)
	return f
}

func (f *editFrame) Draw(screen tcell.Screen) {
	if activeTheme == themeDOSBlue {
		f.drawDN(screen)
		return
	}
	x, y, width, height := f.GetRect()
	if width < 6 || height < 4 {
		return
	}
	screenWidth, screenHeight := screen.Size()
	shadow := tcell.StyleDefault.Background(tcell.ColorBlack)
	for yy := y + 1; yy < y+height && yy < screenHeight; yy++ {
		xx := x + width
		if xx >= 0 && xx < screenWidth {
			screen.SetContent(xx, yy, ' ', nil, shadow)
		}
	}
	for xx := x + 2; xx <= x+width && xx < screenWidth; xx++ {
		yy := y + height
		if yy >= 0 && yy < screenHeight {
			screen.SetContent(xx, yy, ' ', nil, shadow)
		}
	}

	background := tcell.StyleDefault.Background(colDialogBg)
	for yy := y; yy < y+height && yy < screenHeight; yy++ {
		for xx := x; xx < x+width && xx < screenWidth; xx++ {
			screen.SetContent(xx, yy, ' ', nil, background)
		}
	}

	border := tcell.StyleDefault.Foreground(colDialogBorder).Background(colDialogBg)
	for xx := x + 1; xx < x+width-1 && xx < screenWidth; xx++ {
		screen.SetContent(xx, y, '═', nil, border)
		screen.SetContent(xx, y+height-1, '═', nil, border)
	}
	for yy := y + 1; yy < y+height-1 && yy < screenHeight; yy++ {
		screen.SetContent(x, yy, '║', nil, border)
		screen.SetContent(x+width-1, yy, '║', nil, border)
	}
	corner := func(xx, yy int, r rune) {
		if xx >= 0 && xx < screenWidth && yy >= 0 && yy < screenHeight {
			screen.SetContent(xx, yy, r, nil, border)
		}
	}
	corner(x, y, '╔')
	corner(x+width-1, y, '╗')
	corner(x, y+height-1, '╚')
	corner(x+width-1, y+height-1, '╝')

	label := " " + f.title + " "
	if utf8.RuneCountInString(label) > width-4 {
		label = " " + truncate(f.title, width-6) + " "
	}
	titleStyle := tcell.StyleDefault.Foreground(colDialogTitle).Background(colDialogBg)
	tx := x + (width-utf8.RuneCountInString(label))/2
	for i, r := range []rune(label) {
		xx := tx + i
		if xx > x && xx < x+width-1 && xx < screenWidth && y < screenHeight {
			screen.SetContent(xx, y, r, nil, titleStyle)
		}
	}

	f.child.SetRect(x+2, y+2, width-4, height-4)
	f.child.Draw(screen)
}

func (f *editFrame) drawDN(screen tcell.Screen) {
	x, y, width, height := f.GetRect()
	if width < 4 || height < 4 {
		return
	}
	screenWidth, screenHeight := screen.Size()
	shadow := tcell.StyleDefault.Background(tcell.ColorBlack)
	for yy := y + 1; yy <= y+height && yy < screenHeight; yy++ {
		if xx := x + width; xx >= 0 && xx < screenWidth {
			screen.SetContent(xx, yy, ' ', nil, shadow)
		}
	}
	for xx := x + 2; xx <= x+width && xx < screenWidth; xx++ {
		if yy := y + height; yy >= 0 && yy < screenHeight {
			screen.SetContent(xx, yy, ' ', nil, shadow)
		}
	}

	background := tcell.StyleDefault.Background(colDialogBg)
	for yy := y; yy < y+height && yy < screenHeight; yy++ {
		for xx := x; xx < x+width && xx < screenWidth; xx++ {
			screen.SetContent(xx, yy, ' ', nil, background)
		}
	}

	border := tcell.StyleDefault.Foreground(colDialogBorder).Background(colDialogBg)
	put := func(xx, yy int, r rune) {
		if xx >= 0 && xx < screenWidth && yy >= 0 && yy < screenHeight {
			screen.SetContent(xx, yy, r, nil, border)
		}
	}
	for xx := x + 1; xx < x+width-1 && xx < screenWidth; xx++ {
		put(xx, y, tview.Borders.Horizontal)
		put(xx, y+height-1, tview.Borders.Horizontal)
	}
	for yy := y + 1; yy < y+height-1 && yy < screenHeight; yy++ {
		put(x, yy, tview.Borders.Vertical)
		put(x+width-1, yy, tview.Borders.Vertical)
	}
	put(x, y, tview.Borders.TopLeft)
	put(x+width-1, y, tview.Borders.TopRight)
	put(x, y+height-1, tview.Borders.BottomLeft)
	put(x+width-1, y+height-1, tview.Borders.BottomRight)

	label := " " + f.title + " "
	if utf8.RuneCountInString(label) > width-4 {
		label = " " + truncate(f.title, width-6) + " "
	}
	titleStyle := tcell.StyleDefault.Foreground(colDialogTitle).Background(colDialogBg)
	tx := x + (width-utf8.RuneCountInString(label))/2
	for i, r := range []rune(label) {
		xx := tx + i
		if xx > x && xx < x+width-1 && xx < screenWidth && y < screenHeight {
			screen.SetContent(xx, y, r, nil, titleStyle)
		}
	}

	f.child.SetRect(x+2, y+2, width-4, height-4)
	f.child.Draw(screen)
}

func (f *editFrame) Focus(delegate func(p tview.Primitive)) { f.child.Focus(delegate) }
func (f *editFrame) HasFocus() bool                         { return f.child.HasFocus() }
func (f *editFrame) Blur()                                  { f.child.Blur() }
func (f *editFrame) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return f.child.InputHandler()
}
func (f *editFrame) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return f.child.MouseHandler()
}
func (f *editFrame) PasteHandler() func(string, func(tview.Primitive)) {
	return f.child.PasteHandler()
}

func parseEditorValue(text string) any {
	t := strings.TrimSpace(text)
	if t == "" || strings.EqualFold(t, "NULL") {
		return nil
	}
	if t == "''" {
		return ""
	}
	if len(t) >= 3 && (t[0] == 'x' || t[0] == 'X') && t[1] == '\'' && t[len(t)-1] == '\'' {
		h := strings.ReplaceAll(t[2:len(t)-1], " ", "")
		if b, err := hex.DecodeString(h); err == nil {
			return b
		}
	}
	return text
}

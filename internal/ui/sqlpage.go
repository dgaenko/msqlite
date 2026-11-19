package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"msqlite/internal/db"
)

// SQLPage is a simple SQL console with a result grid.
type SQLPage struct {
	app            *App
	db             *db.Database
	readOnly       bool
	root           *tview.Flex
	header         *tview.TextView
	table          *tview.Table
	status         *tview.TextView
	input          *tview.TextArea
	hints          *tview.TextView
	historyIdx     int
	focusGrid      bool
	result         *db.QueryResult
	running        bool
	cancel         context.CancelFunc
	canceled       bool
	runGen         int
	completionCols map[string][]string
	compActive     bool
	compStart      int
	compEnd        int
	compText       string
	compCands      []string
	compIndex      int
}

func newSQLPage(a *App, d *db.Database, readOnly bool, initial string) *SQLPage {
	p := &SQLPage{app: a, db: d, readOnly: readOnly}
	p.historyIdx = len(a.sqlHistory)
	p.build()
	if initial != "" {
		p.input.SetText(initial, true)
	}
	return p
}

func (p *SQLPage) build() {
	p.header = tview.NewTextView().SetDynamicColors(true)
	p.header.SetBackgroundColor(colBg)
	p.header.SetTextColor(colText)
	p.header.SetWrap(false)
	p.updateMode()

	p.table = tview.NewTable()
	styleTable(p.table)
	p.table.SetSeparator(columnLineSeparator(p.app.colLines))
	p.table.SetFixed(1, 1)
	p.table.SetBorder(true)
	p.table.SetTitle(" Результат ")
	p.table.SetTitleColor(colTitle)
	p.table.SetBorderColor(colBorder)
	p.table.SetSelectable(true, true)
	p.table.SetFocusFunc(func() { p.focusGrid = true })

	p.status = tview.NewTextView().SetDynamicColors(true)
	p.status.SetBackgroundColor(colBg)
	p.status.SetWrap(false)
	p.status.SetTextColor(colDim)
	p.status.SetText("[silver]Введите SQL и нажмите Enter.[-:-:-]")

	p.input = tview.NewTextArea()
	p.input.SetLabel("SQL> ")
	p.input.SetLabelStyle(tcell.StyleDefault.Foreground(colTitle).Background(colFieldBg))
	p.input.SetBackgroundColor(colFieldBg)
	p.input.SetTextStyle(tcell.StyleDefault.Foreground(colText).Background(colFieldBg))
	p.input.SetPlaceholder("SELECT * FROM ...")
	p.input.SetPlaceholderStyle(tcell.StyleDefault.Foreground(colDim).Background(colFieldBg).Italic(true))
	p.input.SetWrap(true)
	p.input.SetMaxLength(1 << 20)
	p.input.SetBorder(true)
	p.input.SetBorderColor(colBorder)
	p.input.SetTitle(" Запрос ")
	p.input.SetTitleColor(colTitle)
	p.input.SetFocusFunc(func() { p.focusGrid = false })
	p.input.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEnter {
			if ev.Modifiers()&tcell.ModAlt != 0 {
				return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
			}
			p.execute()
			return nil
		}
		return ev
	})

	p.hints = tview.NewTextView().SetDynamicColors(true)
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.hints.SetText(hintBar(
		"Enter", "Выполнить",
		"Alt+Enter", "Строка",
		"Tab", "Дополнить",
		"F2", "Знач.",
		"F6", "Констр.",
		"Ctrl+Space", "Дополнить",
		"Alt+↑↓", "История",
		"Esc", "Стоп/Назад",
	))

	p.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.header, 1, 0, false).
		AddItem(&columnLineTable{Table: p.table, enabled: func() bool { return p.app.colLines }}, 0, 1, true).
		AddItem(p.status, 1, 0, false).
		AddItem(p.input, 5, 0, true).
		AddItem(p.hints, 1, 0, false)
}

func (p *SQLPage) Root() tview.Primitive { return p.root }

func (p *SQLPage) applyColumnLines() {
	p.table.SetSeparator(columnLineSeparator(p.app.colLines))
}

func (p *SQLPage) restyle() {
	p.header.SetBackgroundColor(colBg)
	p.header.SetTextColor(colText)
	styleTable(p.table)
	p.table.SetSeparator(columnLineSeparator(p.app.colLines))
	p.table.SetBorderColor(colBorder)
	p.table.SetTitleColor(colTitle)
	p.status.SetBackgroundColor(colBg)
	p.status.SetTextColor(colDim)
	p.input.SetLabelStyle(tcell.StyleDefault.Foreground(colTitle).Background(colFieldBg))
	p.input.SetBackgroundColor(colFieldBg)
	p.input.SetTextStyle(tcell.StyleDefault.Foreground(colText).Background(colFieldBg))
	p.input.SetPlaceholderStyle(tcell.StyleDefault.Foreground(colDim).Background(colFieldBg).Italic(true))
	p.input.SetBorderColor(colBorder)
	p.input.SetTitleColor(colTitle)
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.hints.SetText(hintBar(
		"Enter", "Выполнить",
		"Alt+Enter", "Строка",
		"Tab", "Дополнить",
		"F2", "Знач.",
		"F6", "Констр.",
		"Ctrl+Space", "Дополнить",
		"Alt+↑↓", "История",
		"Esc", "Стоп/Назад",
	))
	p.updateMode()
	p.table.Clear()
	if p.result != nil && p.result.IsSelect {
		p.populate(p.result)
	}
}

func (p *SQLPage) updateMode() {
	p.readOnly = p.app.opts.ReadOnly
	mode := "чтение/запись"
	if p.readOnly {
		mode = "только чтение"
	}
	p.header.SetText(fmt.Sprintf("[yellow::b]SQL[-:-:-] [white]· %s  [gray::d]· %s",
		esc(p.db.Name), mode))
}

func (p *SQLPage) execute() {
	if p.running {
		return
	}
	q := strings.TrimSpace(p.input.GetText())
	if q == "" {
		return
	}
	p.app.addSQLHistory(q)
	p.historyIdx = len(p.app.sqlHistory)
	p.completionCols = nil
	p.status.SetText("[yellow]Выполняется… Esc — отмена[-:-:-]")

	p.running = true
	p.canceled = false
	p.runGen++
	gen := p.runGen
	d := p.db
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	p.cancel = cancel

	go func() {
		res, err := d.Exec(ctx, q, 1000)
		cancel()
		p.app.app.QueueUpdateDraw(func() {
			if gen != p.runGen {
				return
			}
			p.running = false
			p.cancel = nil
			if err != nil {
				if p.canceled {
					p.status.SetText("[yellow]Запрос отменён[-:-:-]")
				} else {
					p.status.SetText("[red::b]Ошибка: [-:-:-]" + esc(err.Error()))
				}
				p.canceled = false
				p.app.app.SetFocus(p.input)
				return
			}
			p.canceled = false
			p.result = res
			p.input.SetText("", false)
			if res.IsSelect {
				p.populate(res)
				msg := fmt.Sprintf("[green::b]OK[-:-:-] · строк: %d · %d мс",
					len(res.Rows), res.Duration.Milliseconds())
				if res.Truncated {
					msg += " · [yellow]показаны первые 1000[-:-:-]"
				}
				p.status.SetText(msg)
			} else {
				p.table.Clear()
				p.status.SetText(fmt.Sprintf("[green::b]OK[-:-:-] · затронуто строк: %d · %d мс",
					res.Affected, res.Duration.Milliseconds()))
				p.app.refreshObjects()
			}
			p.app.app.SetFocus(p.input)
		})
	}()
}

// cancelQuery aborts the currently running statement.
func (p *SQLPage) cancelQuery() {
	if !p.running || p.cancel == nil {
		return
	}
	p.status.SetText("[yellow]Отмена…[-:-:-]")
	p.canceled = true
	p.cancel()
}

func (p *SQLPage) populate(res *db.QueryResult) {
	p.table.Clear()
	p.table.SetFixed(1, 1)
	p.table.SetCell(0, 0, tview.NewTableCell("#").
		SetTextColor(colDim).SetSelectable(false).SetAlign(tview.AlignRight).SetMaxWidth(6))
	for i, c := range res.Columns {
		name := c.Name
		if c.Type != "" {
			name += " [" + c.Type + "]"
		}
		p.table.SetCell(0, i+1, tview.NewTableCell(name).
			SetTextColor(colTitle).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetExpansion(1).
			SetMaxWidth(40))
	}
	for r, row := range res.Rows {
		p.table.SetCell(r+1, 0, tview.NewTableCell(fmt.Sprintf("%d", r+1)).
			SetTextColor(colDim).
			SetSelectable(false).
			SetAlign(tview.AlignRight).
			SetMaxWidth(6))
		for c, cell := range row {
			tc := tview.NewTableCell(cell.Text).SetExpansion(1).SetMaxWidth(40)
			switch {
			case cell.Null:
				tc.SetTextColor(colDim).SetAttributes(tcell.AttrItalic)
			case cell.Blob:
				tc.SetTextColor(colBlob)
			default:
				tc.SetTextColor(colText)
			}
			p.table.SetCell(r+1, c+1, tc)
		}
	}
	if len(res.Rows) > 0 {
		p.table.Select(1, 1)
	}
}

func (p *SQLPage) showSelectedCell() {
	if p.result == nil || !p.focusGrid {
		return
	}
	row, col := p.table.GetSelection()
	if row < 1 || col < 1 || row-1 >= len(p.result.Rows) || col-1 >= len(p.result.Columns) {
		return
	}
	cell := p.result.Rows[row-1][col-1]
	title := p.result.Columns[col-1].Name
	p.app.showValue(title, p.result.Columns[col-1].Type, cell)
}

func (p *SQLPage) historyUp() {
	h := p.app.sqlHistory
	if len(h) == 0 {
		return
	}
	if p.historyIdx > 0 {
		p.historyIdx--
	}
	if p.historyIdx >= 0 && p.historyIdx < len(h) {
		p.input.SetText(h[p.historyIdx], true)
	}
}

func (p *SQLPage) historyDown() {
	h := p.app.sqlHistory
	if len(h) == 0 {
		return
	}
	if p.historyIdx < len(h)-1 {
		p.historyIdx++
		p.input.SetText(h[p.historyIdx], true)
	} else {
		p.historyIdx = len(h)
		p.input.SetText("", false)
	}
}

// HandleKey processes SQL-page specific keys and reports whether it consumed the event.
func (p *SQLPage) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyF6:
		p.openQueryBuilder()
		return true
	case tcell.KeyTab:
		if p.focusGrid {
			p.focusGrid = false
			p.app.app.SetFocus(p.input)
		} else if p.completionAvailable() {
			p.complete()
		} else if p.table.GetRowCount() > 0 {
			p.focusGrid = true
			p.app.app.SetFocus(p.table)
		}
		return true
	case tcell.KeyBacktab:
		if p.focusGrid {
			p.focusGrid = false
			p.app.app.SetFocus(p.input)
		} else if p.table.GetRowCount() > 0 {
			p.focusGrid = true
			p.app.app.SetFocus(p.table)
		}
		return true
	case tcell.KeyCtrlSpace:
		if !p.focusGrid {
			p.complete()
			return true
		}
	case tcell.KeyUp:
		if !p.focusGrid && ev.Modifiers()&tcell.ModAlt != 0 {
			p.historyUp()
			return true
		}
	case tcell.KeyDown:
		if !p.focusGrid && ev.Modifiers()&tcell.ModAlt != 0 {
			p.historyDown()
			return true
		}
	}
	return false
}

var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "INSERT", "INTO", "VALUES", "UPDATE", "SET",
	"DELETE", "CREATE", "TABLE", "INDEX", "VIEW", "DROP", "ALTER", "ADD",
	"COLUMN", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "FULL", "CROSS",
	"ON", "USING", "GROUP", "BY", "ORDER", "ASC", "DESC", "HAVING", "LIMIT",
	"OFFSET", "AS", "AND", "OR", "NOT", "NULL", "IS", "IN", "LIKE", "BETWEEN",
	"DISTINCT", "UNION", "ALL", "CASE", "WHEN", "THEN", "ELSE", "END",
	"PRAGMA", "BEGIN", "COMMIT", "ROLLBACK", "TRANSACTION", "PRIMARY", "KEY",
	"FOREIGN", "REFERENCES", "DEFAULT", "UNIQUE", "CHECK", "AUTOINCREMENT",
	"COUNT", "SUM", "AVG", "MIN", "MAX", "CAST", "COALESCE", "IFNULL",
	"WITH", "RECURSIVE", "EXISTS", "REPLACE", "TRIGGER", "BEFORE", "AFTER",
	"INSTEAD", "OF", "FOR", "EACH", "ROW", "CONSTRAINT", "WITHOUT", "ROWID",
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$'
}

func wordBounds(text string, pos int) (int, int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(text) {
		pos = len(text)
	}
	start := pos
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if !isWordRune(r) {
			break
		}
		start -= size
	}
	end := pos
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		if !isWordRune(r) {
			break
		}
		end += size
	}
	return start, end
}

func textAreaCursorIndex(t *tview.TextArea) int {
	_, _, row, column := t.GetCursor()
	text := t.GetText()
	lines := strings.Split(text, "\n")
	if row < 0 {
		return 0
	}
	if row >= len(lines) {
		return len(text)
	}
	index := 0
	for i := 0; i < row; i++ {
		index += len(lines[i]) + 1
	}
	line := lines[row]
	width := 0
	for i, r := range line {
		runes := string(r)
		cellWidth := 1
		if r == '\t' {
			cellWidth = 8
		} else if w := tview.TaggedStringWidth(runes); w > 0 {
			cellWidth = w
		}
		if width+cellWidth > column {
			return index + i
		}
		width += cellWidth
		if width >= column {
			return index + i + len(runes)
		}
	}
	return index + len(line)
}

func (p *SQLPage) loadCompletionColumns() {
	if p.completionCols != nil {
		return
	}
	p.completionCols = make(map[string][]string)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, obj := range p.app.objects {
		cols, err := p.db.Columns(ctx, obj.Name)
		if err != nil {
			continue
		}
		for _, c := range cols {
			p.completionCols[obj.Name] = append(p.completionCols[obj.Name], c.Name)
		}
	}
}

func (p *SQLPage) completionCandidates(prefix, prefer string) []string {
	lower := strings.ToLower(prefix)
	seen := make(map[string]bool)
	var out []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		if lower != "" && !strings.HasPrefix(strings.ToLower(name), lower) {
			return
		}
		seen[name] = true
		out = append(out, name)
	}

	var tables, columns []string
	for _, obj := range p.app.objects {
		tables = append(tables, obj.Name)
	}
	p.loadCompletionColumns()
	for _, cols := range p.completionCols {
		columns = append(columns, cols...)
	}
	sort.Strings(tables)
	sort.Strings(columns)

	addAll := func(names []string) {
		for _, name := range names {
			add(name)
		}
	}
	addKeywords := func() {
		for _, keyword := range sqlKeywords {
			add(keyword)
		}
	}

	if prefix == "" && prefer == "tables" && len(tables) > 0 {
		addAll(tables)
		return out
	}
	if prefix == "" && prefer == "columns" && len(columns) > 0 {
		addAll(columns)
		return out
	}
	switch prefer {
	case "tables":
		addAll(tables)
		addAll(columns)
		addKeywords()
	case "columns":
		addAll(columns)
		addAll(tables)
		addKeywords()
	default:
		addKeywords()
		addAll(tables)
		addAll(columns)
	}
	if len(out) == 0 {
		addKeywords()
	}
	return out
}

func previousWord(text string) string {
	trimmed := strings.TrimRight(text, " \t\n\r")
	start, end := wordBounds(trimmed, len(trimmed))
	return trimmed[start:end]
}

func completionContext(text string, wordStart int) string {
	switch strings.ToUpper(previousWord(text[:wordStart])) {
	case "FROM", "JOIN", "INTO", "UPDATE", "TABLE", "DELETE", "ALTER", "DROP":
		return "tables"
	case "SELECT", "WHERE", "ON", "BY", "SET", "HAVING", "GROUP", "ORDER", "AND", "OR":
		return "columns"
	}
	return ""
}

func (p *SQLPage) completionAvailable() bool {
	text := p.input.GetText()
	cursor := textAreaCursorIndex(p.input)
	if p.compActive && p.compText == text && cursor == p.compEnd && len(p.compCands) > 0 {
		return true
	}
	start, _ := wordBounds(text, cursor)
	if start < cursor {
		return true
	}
	return completionContext(text, start) != ""
}

func (p *SQLPage) complete() {
	text := p.input.GetText()
	cursor := textAreaCursorIndex(p.input)
	if p.compActive && p.compText == text && cursor == p.compEnd && len(p.compCands) > 0 {
		next := (p.compIndex + 1) % len(p.compCands)
		candidate := p.compCands[next]
		p.input.Replace(p.compStart, p.compEnd, candidate)
		p.compIndex = next
		p.compEnd = p.compStart + len(candidate)
		p.compText = p.input.GetText()
		p.showCompletionStatus()
		return
	}
	start, end := wordBounds(text, cursor)
	prefix := text[start:end]
	candidates := p.completionCandidates(prefix, completionContext(text, start))
	if len(candidates) == 0 {
		p.status.SetText("[silver]Нет вариантов автодополнения[-:-:-]")
		return
	}
	candidate := candidates[0]
	p.compActive = true
	p.compStart = start
	p.compEnd = start + len(candidate)
	p.compCands = candidates
	p.compIndex = 0
	p.input.Replace(start, end, candidate)
	p.compText = p.input.GetText()
	p.showCompletionStatus()
}

func (p *SQLPage) showCompletionStatus() {
	if len(p.compCands) == 0 {
		return
	}
	preview := strings.Join(p.compCands[:min(len(p.compCands), 6)], " · ")
	if len(p.compCands) > 6 {
		preview += " …"
	}
	p.status.SetText(fmt.Sprintf("[silver]Автодополнение (%d): %s[-:-:-]", len(p.compCands), esc(preview)))
}

func (p *SQLPage) resultRawRows() [][]any {
	if p.result == nil {
		return nil
	}
	rows := make([][]any, len(p.result.Rows))
	for i, row := range p.result.Rows {
		vals := make([]any, len(row))
		for j, cell := range row {
			switch {
			case cell.Null:
				vals[j] = nil
			case cell.Raw != nil:
				vals[j] = cell.Raw
			default:
				vals[j] = cell.Full
			}
		}
		rows[i] = vals
	}
	return rows
}

func (p *SQLPage) resultColumnNames() []string {
	if p.result == nil {
		return nil
	}
	names := make([]string, len(p.result.Columns))
	for i, c := range p.result.Columns {
		names[i] = c.Name
	}
	return names
}

func (p *SQLPage) exportResultCSV(path string) (int64, error) {
	return db.WriteCSV(path, p.resultColumnNames(), p.resultRawRows())
}

func (p *SQLPage) exportResultJSON(path string) (int64, error) {
	return db.WriteJSON(path, p.resultColumnNames(), p.resultRawRows())
}

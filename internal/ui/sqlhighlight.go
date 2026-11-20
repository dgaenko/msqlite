package ui

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// sqlInput is a TextArea with lightweight SQL syntax highlighting.
type sqlInput struct {
	*tview.TextArea
}

func newSQLInput() *sqlInput {
	return &sqlInput{TextArea: tview.NewTextArea()}
}

func (in *sqlInput) Draw(screen tcell.Screen) {
	in.TextArea.Draw(screen)
	if strings.TrimSpace(in.GetText()) == "" {
		return
	}
	highlightSQL(screen, in.TextArea)
}

type sqlClass int

const (
	sqlPlain sqlClass = iota
	sqlKeyword
	sqlString
	sqlNumber
	sqlComment
)

type sqlLexState struct {
	blockComment bool
	quote        rune
}

var sqlKeywordSet = map[string]bool{
	"ABORT": true, "ADD": true, "ALL": true, "ALTER": true, "ANALYZE": true,
	"AND": true, "AS": true, "ASC": true, "ATTACH": true, "AUTOINCREMENT": true,
	"BEFORE": true, "BEGIN": true, "BETWEEN": true, "BY": true, "CASCADE": true,
	"CASE": true, "CAST": true, "CHECK": true, "COLLATE": true, "COLUMN": true,
	"COMMIT": true, "CONFLICT": true, "CONSTRAINT": true, "CREATE": true,
	"CROSS": true, "CURRENT": true, "DATABASE": true, "DEFAULT": true,
	"DEFERRABLE": true, "DELETE": true, "DESC": true, "DETACH": true,
	"DISTINCT": true, "DO": true, "DROP": true, "EACH": true, "ELSE": true,
	"END": true, "ESCAPE": true, "EXCEPT": true, "EXCLUSIVE": true, "EXISTS": true,
	"EXPLAIN": true, "FAIL": true, "FILTER": true, "FIRST": true, "FOLLOWING": true,
	"FOR": true, "FOREIGN": true, "FROM": true, "FULL": true, "GLOB": true,
	"GROUP": true, "HAVING": true, "IF": true, "IGNORE": true, "IMMEDIATE": true,
	"IN": true, "INDEX": true, "INDEXED": true, "INITIALLY": true, "INNER": true,
	"INSERT": true, "INSTEAD": true, "INTERSECT": true, "INTO": true, "IS": true,
	"ISNULL": true, "JOIN": true, "KEY": true, "LAST": true, "LEFT": true,
	"LIKE": true, "LIMIT": true, "MATCH": true, "NATURAL": true, "NO": true,
	"NOT": true, "NOTHING": true, "NOTNULL": true, "NULL": true, "NULLS": true,
	"OF": true, "OFFSET": true, "ON": true, "OR": true, "ORDER": true,
	"OUTER": true, "OVER": true, "PARTITION": true, "PLAN": true,
	"PRAGMA": true, "PRECEDING": true, "PRIMARY": true, "QUERY": true,
	"RAISE": true, "RANGE": true, "RECURSIVE": true, "REFERENCES": true,
	"REGEXP": true, "REINDEX": true, "RELEASE": true, "RENAME": true,
	"REPLACE": true, "RESTRICT": true, "RETURNING": true, "RIGHT": true,
	"ROLLBACK": true, "ROW": true, "ROWS": true, "SAVEPOINT": true,
	"SELECT": true, "SET": true, "TABLE": true, "TEMP": true, "TEMPORARY": true,
	"THEN": true, "TO": true, "TRANSACTION": true, "TRIGGER": true, "UNBOUNDED": true,
	"UNION": true, "UNIQUE": true, "UPDATE": true, "USING": true, "VACUUM": true,
	"VALUES": true, "VIEW": true, "VIRTUAL": true, "WHEN": true, "WHERE": true,
	"WINDOW": true, "WITH": true, "WITHOUT": true,
}

func highlightSQL(screen tcell.Screen, ta *tview.TextArea) {
	x, y, width, height := ta.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	labelWidth := ta.GetLabelWidth()
	if labelWidth == 0 && ta.GetLabel() != "" {
		labelWidth = tview.TaggedStringWidth(ta.GetLabel())
	}
	if labelWidth >= width {
		return
	}
	x += labelWidth
	width -= labelWidth

	state := sqlLexState{}
	for row := 0; row < height; row++ {
		runes := make([]rune, width)
		for col := 0; col < width; col++ {
			ch, _, _, _ := screen.GetContent(x+col, y+row)
			runes[col] = ch
		}
		classes := classifySQLLine(runes, &state)
		for col, class := range classes {
			if class == sqlPlain || runes[col] == 0 || runes[col] == ' ' {
				continue
			}
			ch, comb, style, _ := screen.GetContent(x+col, y+row)
			_, bg, _ := style.Decompose()
			if bg != colFieldBg {
				continue
			}
			color := sqlClassColor(class)
			style = style.Foreground(color)
			if class == sqlKeyword {
				style = style.Bold(true)
			}
			screen.SetContent(x+col, y+row, ch, comb, style)
		}
	}
}

func sqlClassColor(class sqlClass) tcell.Color {
	switch class {
	case sqlKeyword:
		return colTitle
	case sqlString:
		return colOK
	case sqlNumber:
		return colFocus
	case sqlComment:
		return colDim
	}
	return colText
}

func classifySQLLine(runes []rune, state *sqlLexState) []sqlClass {
	out := make([]sqlClass, len(runes))
	i := 0
	for i < len(runes) {
		if state.blockComment {
			start := i
			for i < len(runes) {
				if runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '/' {
					i += 2
					state.blockComment = false
					break
				}
				i++
			}
			markClass(out, start, i, sqlComment)
			continue
		}
		if state.quote != 0 {
			start := i
			closed := false
			for i < len(runes) {
				if runes[i] == state.quote {
					if state.quote != '[' && i+1 < len(runes) && runes[i+1] == state.quote {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			markClass(out, start, i, sqlString)
			if closed {
				state.quote = 0
			}
			continue
		}

		ch := runes[i]
		switch {
		case unicode.IsSpace(ch):
			i++
		case ch == '-' && i+1 < len(runes) && runes[i+1] == '-':
			markClass(out, i, len(runes), sqlComment)
			i = len(runes)
		case ch == '/' && i+1 < len(runes) && runes[i+1] == '*':
			start := i
			i += 2
			closed := false
			for i < len(runes) {
				if runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '/' {
					i += 2
					closed = true
					break
				}
				i++
			}
			markClass(out, start, i, sqlComment)
			state.blockComment = !closed
		case ch == '\'' || ch == '"' || ch == '`':
			start := i
			quote := ch
			i++
			closed := false
			for i < len(runes) {
				if runes[i] == quote {
					if i+1 < len(runes) && runes[i+1] == quote {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			markClass(out, start, i, sqlString)
			if !closed {
				state.quote = quote
			}
		case ch == '[':
			start := i
			i++
			closed := false
			for i < len(runes) {
				if runes[i] == ']' {
					i++
					closed = true
					break
				}
				i++
			}
			markClass(out, start, i, sqlString)
			if !closed {
				state.quote = '['
			}
		case isDigit(ch) || (ch == '.' && i+1 < len(runes) && isDigit(runes[i+1])):
			start := i
			if ch == '0' && i+1 < len(runes) && (runes[i+1] == 'x' || runes[i+1] == 'X') {
				i += 2
				for i < len(runes) && isHexDigit(runes[i]) {
					i++
				}
			} else {
				for i < len(runes) && (isDigit(runes[i]) || runes[i] == '.') {
					i++
				}
				if i < len(runes) && (runes[i] == 'e' || runes[i] == 'E') {
					i++
					if i < len(runes) && (runes[i] == '+' || runes[i] == '-') {
						i++
					}
					for i < len(runes) && isDigit(runes[i]) {
						i++
					}
				}
			}
			markClass(out, start, i, sqlNumber)
		case isIdentStart(ch):
			start := i
			for i < len(runes) && isIdentPart(runes[i]) {
				i++
			}
			if sqlKeywordSet[strings.ToUpper(string(runes[start:i]))] {
				markClass(out, start, i, sqlKeyword)
			}
		default:
			i++
		}
	}
	return out
}

func markClass(out []sqlClass, start, end int, class sqlClass) {
	if start < 0 {
		start = 0
	}
	if end > len(out) {
		end = len(out)
	}
	for i := start; i < end; i++ {
		out[i] = class
	}
}

func isDigit(r rune) bool      { return r >= '0' && r <= '9' }
func isHexDigit(r rune) bool   { return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') }
func isIdentStart(r rune) bool { return unicode.IsLetter(r) || r == '_' || r == '$' }
func isIdentPart(r rune) bool  { return isIdentStart(r) || isDigit(r) }

package ui

import (
	"errors"
	"regexp"
	"strings"
)

var (
	reNearToken = regexp.MustCompile(`near "([^"]*)"`)
	reMissing   = regexp.MustCompile(`no such (?:table|column|function): (.+)`)
)

type sqliteCoder interface {
	Code() int
}

// formatDBError returns the full SQLite error text with code and query position.
func formatDBError(err error, query string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	var b strings.Builder
	code := 0
	var coder sqliteCoder
	if errors.As(err, &coder) {
		code = coder.Code()
	}
	if code != 0 {
		b.WriteString(trf("Ошибка SQLite (код %d)", code))
		b.WriteString("\n")
	}
	b.WriteString(msg)

	if query == "" {
		return b.String()
	}
	line, col, snippet, ok := queryPosition(query, msg)
	if !ok {
		return b.String()
	}
	b.WriteString("\n\n")
	b.WriteString(trf("Строка %d, колонка %d", line, col))
	if snippet != "" {
		b.WriteString("\n\n")
		b.WriteString(snippet)
	}
	return b.String()
}

func findToken(query, token string) int {
	if token == "" {
		return -1
	}
	lower := strings.ToLower(query)
	needle := strings.ToLower(token)
	for from := 0; from <= len(lower); {
		i := strings.Index(lower[from:], needle)
		if i < 0 {
			return -1
		}
		i += from
		beforeOK := i == 0 || !isIdentRune(rune(query[i-1]))
		after := i + len(needle)
		afterOK := after >= len(query) || !isIdentRune(rune(query[after]))
		if beforeOK && afterOK {
			return i
		}
		from = i + len(needle)
	}
	return -1
}

func isIdentRune(r rune) bool {
	return r == '_' || r == '$' || (r >= '0' && r <= '9') ||
		(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127
}

func queryPosition(query, msg string) (line, col int, snippet string, ok bool) {
	token := ""
	if m := reNearToken.FindStringSubmatch(msg); len(m) == 2 {
		token = m[1]
	} else if m := reMissing.FindStringSubmatch(msg); len(m) == 2 {
		token = strings.TrimSpace(m[1])
	}
	if token == "" {
		return 0, 0, "", false
	}
	idx := findToken(query, token)
	if idx < 0 {
		return 0, 0, "", false
	}
	before := query[:idx]
	line = strings.Count(before, "\n") + 1
	lastNL := strings.LastIndex(before, "\n")
	col = len([]rune(before[lastNL+1:])) + 1

	start := lastNL + 1
	end := strings.Index(query[start:], "\n")
	if end >= 0 {
		end += start
	} else {
		end = len(query)
	}
	sourceLine := query[start:end]
	caretCol := col - 1
	if caretCol < 0 {
		caretCol = 0
	}
	snippet = sourceLine + "\n" + strings.Repeat(" ", caretCol) + "^"
	return line, col, snippet, true
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

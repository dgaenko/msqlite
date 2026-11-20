package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestQueryPosition(t *testing.T) {
	line, col, snippet, ok := queryPosition("SELECT 1;\nSELEC 2;", `near "SELEC": syntax error`)
	if !ok || line != 2 || col != 1 {
		t.Fatalf("position = %d:%d ok=%v", line, col, ok)
	}
	if !strings.Contains(snippet, "SELEC 2;") || !strings.Contains(snippet, "^") {
		t.Fatalf("snippet = %q", snippet)
	}
}

func TestFormatDBError(t *testing.T) {
	prev := activeLang
	setActiveLang(langEN)
	defer setActiveLang(prev)
	err := errors.New(`near "FROM": syntax error`)
	got := formatDBError(err, "SELECT FROM users")
	if !strings.Contains(got, "FROM") || !strings.Contains(got, "1, column 8") {
		t.Fatalf("formatDBError = %q", got)
	}
}

func TestSQLValueLiteral(t *testing.T) {
	if got := sqlValueLiteral("O'Reilly"); got != "'O''Reilly'" {
		t.Fatalf("string literal = %q", got)
	}
	if got := sqlValueLiteral([]byte{0xAB}); got != "X'AB'" {
		t.Fatalf("blob literal = %q", got)
	}
}

package ui

import "testing"

func TestClassifySQLLine(t *testing.T) {
	state := sqlLexState{}
	line := []rune("SELECT * FROM t WHERE x = 'a' -- note")
	classes := classifySQLLine(line, &state)

	check := func(from, to int, want sqlClass) {
		t.Helper()
		for i := from; i < to; i++ {
			if classes[i] != want {
				t.Fatalf("position %d (%q): class %d, want %d", i, line[i], classes[i], want)
			}
		}
	}
	check(0, 6, sqlKeyword)
	check(9, 13, sqlKeyword)
	check(14, 15, sqlPlain)
	check(16, 21, sqlKeyword)
	check(26, 29, sqlString)
	check(30, 37, sqlComment)

	state = sqlLexState{}
	classes = classifySQLLine([]rune("/* open comment"), &state)
	if !state.blockComment {
		t.Fatal("block comment state not set")
	}
	if classes[0] != sqlComment || classes[len(classes)-1] != sqlComment {
		t.Fatal("block comment not classified")
	}
	classes = classifySQLLine([]rune("still comment */ SELECT"), &state)
	if state.blockComment {
		t.Fatal("block comment state not cleared")
	}
	if classes[0] != sqlComment || classes[len(classes)-1] != sqlKeyword {
		t.Fatal("block comment continuation/close not classified")
	}
}

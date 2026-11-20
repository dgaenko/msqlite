package ui

import "testing"

func TestTranslations(t *testing.T) {
	prev := activeLang
	defer setActiveLang(prev)

	setActiveLang(langEN)
	if got := tr("Файл"); got != "File" {
		t.Fatalf("tr(Файл) = %q, want File", got)
	}
	if got := trf("Строк: %d · страница %d/%d", 12, 1, 2); got != "Rows: 12 · page 1/2" {
		t.Fatalf("trf status = %q", got)
	}
	if got := translateText("таблица users не найдена"); got != "table users not found" {
		t.Fatalf("translateText pattern = %q", got)
	}
	if got := translateText("some.db: это каталог"); got != "some.db: is a directory" {
		t.Fatalf("translateText error = %q", got)
	}

	setActiveLang(langRU)
	if got := tr("Файл"); got != "Файл" {
		t.Fatalf("tr(Файл) in RU = %q", got)
	}
}

func TestEnglishDictionaryNotEmpty(t *testing.T) {
	for ru, en := range enText {
		if en == "" {
			t.Errorf("empty translation for %q", ru)
		}
	}
}

func TestDetectSystemLanguage(t *testing.T) {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		t.Setenv(key, "")
	}

	t.Setenv("LANG", "ru_RU.UTF-8")
	if got := detectSystemLanguage(); got != langRU {
		t.Fatalf("ru locale -> %q", got)
	}

	t.Setenv("LANG", "en_US.UTF-8")
	if got := detectSystemLanguage(); got != langEN {
		t.Fatalf("en locale -> %q", got)
	}

	t.Setenv("LC_ALL", "ru-RU")
	if got := detectSystemLanguage(); got != langRU {
		t.Fatalf("LC_ALL ru -> %q", got)
	}

	t.Setenv("LC_ALL", "C")
	t.Setenv("LANG", "ru_RU.UTF-8")
	if got := detectSystemLanguage(); got != langEN {
		t.Fatalf("LC_ALL=C overrides LANG -> %q", got)
	}
}

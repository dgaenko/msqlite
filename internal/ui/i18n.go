package ui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

const (
	langRU = "ru"
	langEN = "en"
)

var activeLang = langRU

// langExplicit is true when the language was chosen by the user or loaded
// from settings; otherwise it is detected from the system locale.
var langExplicit bool

func setActiveLang(code string) bool {
	switch code {
	case "", langRU:
		activeLang = langRU
		return true
	case langEN:
		activeLang = langEN
		return true
	}
	return false
}

func isEN() bool { return activeLang == langEN }

func languageName(code string) string {
	if code == langEN {
		return "English"
	}
	return "Русский"
}

// tr translates a canonical Russian UI string to English when needed.
func tr(s string) string {
	if !isEN() || s == "" {
		return s
	}
	if v, ok := enText[s]; ok {
		return v
	}
	return s
}

// trf translates a format string and formats it with args.
func trf(format string, args ...any) string {
	return fmt.Sprintf(tr(format), args...)
}

type enPattern struct {
	re   *regexp.Regexp
	repl string
}

var enPatterns = []enPattern{
	{regexp.MustCompile(`^таблица (.+) не найдена$`), "table $1 not found"},
	{regexp.MustCompile(`^таблица (.+) не поддерживает построчную правку$`), "table $1 does not support row editing"},
	{regexp.MustCompile(`^объект (.+) не найден$`), "object $1 not found"},
	{regexp.MustCompile(`^каталог (.+): (.+)$`), "directory $1: $2"},
	{regexp.MustCompile(`^(.+): не каталог$`), "$1: not a directory"},
	{regexp.MustCompile(`^(.+): это каталог$`), "$1: is a directory"},
	{regexp.MustCompile(`^(.+): не является файлом SQLite$`), "$1: not a SQLite file"},
	{regexp.MustCompile(`^не удалось открыть (.+): (.+)$`), "failed to open $1: $2"},
	{regexp.MustCompile(`^не удалось создать файл SQLite: (.+)$`), "failed to create SQLite file: $1"},
	{regexp.MustCompile(`^в строке (\d+) значений, ожидалось (\d+)$`), "row $1 has $2 values, expected $3"},
	{regexp.MustCompile(`^строка (\d+): мало значений$`), "row $1: too few values"},
	{regexp.MustCompile(`^в заголовке CSV нет колонок таблицы (.+)$`), "CSV header has no columns of table $1"},
	{regexp.MustCompile(`^в JSON нет колонок таблицы (.+)$`), "JSON has no columns of table $1"},
	{regexp.MustCompile(`^Импортировано строк: (\d+)$`), "Rows imported: $1"},
	{regexp.MustCompile(`(?s)^Экспортировано строк: (\d+)\n(.+)$`), "Rows exported: $1\n$2"},
	{regexp.MustCompile(`(?s)^Сохранено (\d+) байт:\n(.+)$`), "Saved $1 bytes:\n$2"},
	{regexp.MustCompile(`(?s)^(.+) завершено\. Затронуто строк: (\d+)$`), "$1 completed. Rows affected: $2"},
	{regexp.MustCompile(`^BLOB, (\d+) байт$`), "BLOB, $1 bytes"},
	{regexp.MustCompile(`^Таблица "(.+)" создана$`), `Table "$1" created`},
	{regexp.MustCompile(`^Представление "(.+)" создана$`), `View "$1" created`},
	{regexp.MustCompile(`^Таблица переименована в "(.+)"$`), `Table renamed to "$1"`},
	{regexp.MustCompile(`^Колонка "(.+)" добавлена$`), `Column "$1" added`},
	{regexp.MustCompile(`^Колонка "(.+)" переименована$`), `Column "$1" renamed`},
	{regexp.MustCompile(`^Колонка "(.+)" удалена$`), `Column "$1" dropped`},
}

var enFragments = [][2]string{
	{"только чтение", "read-only"},
	{"чтение/запись", "read/write"},
	{"не удалось переключить режим", "could not switch mode"},
	{"не удалось определить rowid строки", "could not determine the row rowid"},
	{"для этой таблицы нельзя определить rowid строки", "cannot determine the row rowid for this table"},
	{"не удалось определить rowid", "could not determine rowid"},
	{"база открыта только для чтения", "database is open read-only"},
	{"таблица", "table"},
	{"представление", "view"},
	{"колонок: ", "columns: "},
	{"строк: ", "rows: "},
	{"таблиц: ", "tables: "},
	{"представлений: ", "views: "},
	{"папок: ", "folders: "},
	{"баз: ", "databases: "},
	{"фильтр: ", "filter: "},
	{"выбрано: ", "selected: "},
	{"структура · ", "schema · "},
}

// translateText translates exact strings and common dynamic error/status text.
func translateText(s string) string {
	if !isEN() || s == "" {
		return s
	}
	if v, ok := enText[s]; ok {
		return v
	}
	for _, p := range enPatterns {
		if p.re.MatchString(s) {
			return p.re.ReplaceAllString(s, p.repl)
		}
	}
	out := s
	for _, f := range enFragments {
		out = strings.ReplaceAll(out, f[0], f[1])
	}
	return out
}

var enText = map[string]string{
	// Application chrome.
	" Базы и папки ":                             " Databases and folders ",
	" Таблицы и представления ":                  " Tables and views ",
	"(пустая папка)":                             "(empty folder)",
	"(нет базы данных)":                          "(no database)",
	"(ошибка открытия)":                          "(open error)",
	"(нет таблиц)":                               "(no tables)",
	"(нет совпадений)":                           "(no matches)",
	"(выберите базу данных)":                     "(select a database)",
	"Ошибка: ":                                   "Error: ",
	"Имя":                                        "Name",
	"Записей":                                    "Rows",
	"Имя содержит: ":                             "Name contains: ",
	"Фильтр объектов":                            "Object filter",
	"Фильтр объектов сброшен":                    "Object filter reset",
	"Фильтр объектов: ":                          "Object filter: ",
	"Счётчики строк пересчитаны":                 "Row counts recalculated",
	"Сначала выберите базу данных.":              "Select a database first.",
	"Список обновлён":                            "List refreshed",
	"Данные обновлены":                           "Data refreshed",
	"Структура обновлена":                        "Schema refreshed",
	"Корень файловой системы: ":                  "Filesystem root: ",
	"Настройки":                                  "Settings",
	"Строк на страницу: ":                        "Rows per page: ",
	"1–10000 · Enter — сохранить · Esc — отмена": "1–10000 · Enter — save · Esc — cancel",
	"Enter — применить · пусто — сбросить · Esc — отмена": "Enter — apply · empty — reset · Esc — cancel",
	"Изменить":             "Edit",
	"· конструктор для %s": "· builder for %s",
	"Каталог: %s · папок: %d · баз: %d · Enter — войти, Bksp — вверх":         "Directory: %s · folders: %d · databases: %d · Enter — open, Bksp — up",
	"В каталоге нет баз данных и подпапок. F2 — открыть путь, F5 — обновить.": "No databases or subfolders in this directory. F2 — open path, F5 — refresh.",
	"Выберите базу данных. Enter — открыть, F2 — путь, Bksp — вверх.":         "Select a database. Enter — open, F2 — path, Bksp — up.",
	"%s · %s · таблиц: %d · представлений: %d":                                "%s · %s · tables: %d · views: %d",
	" · выбрано: %s (%s)":              " · selected: %s (%s)",
	" · фильтр: ":                      " · filter: ",
	"каталог %s: %w":                   "directory %s: %w",
	"%s: не каталог":                   "%s: not a directory",
	"%s: не является файлом SQLite":    "%s: not a SQLite file",
	"не удалось переключить режим: %w": "could not switch mode: %w",

	// Keys and modes.
	"Справка":       "Help",
	"Файл":          "File",
	"Структ.":       "Schema",
	"Обнов.":        "Refresh",
	"Режим":         "Mode",
	"Меню":          "Menu",
	"Выход":         "Exit",
	"Фильтр":        "Filter",
	"Вверх":         "Up",
	"только чтение": "read-only",
	"чтение/запись": "read/write",
	"правка":        "edit",
	"вкл":           "on",
	"выкл":          "off",
	"да":            "yes",
	"таблица":       "table",
	"представление": "view",

	// Menu categories and actions.
	"База":      "Database",
	"Данные":    "Data",
	"Структура": "Schema",
	"Интерфейс": "Interface",
	"Открыть файл или каталог…":               "Open file or directory…",
	"Открыть в новой вкладке…":                "Open in new tab…",
	"Открыть в новой вкладке":                 "Open in new tab",
	"Следующая вкладка":                       "Next tab",
	"Закрыть вкладку":                         "Close tab",
	"Создать базу данных…":                    "Create database…",
	"Недавние базы…":                          "Recent databases…",
	"Выход…":                                  "Exit…",
	"Режим: переключить (%s)":                 "Mode: switch (%s)",
	"Резервная копия базы…":                   "Back up database…",
	"Экспорт SQL-дампа…":                      "Export SQL dump…",
	"Проверка целостности":                    "Integrity check",
	"Посчитать строки точно":                  "Recount rows exactly",
	"Создать таблицу…":                        "Create table…",
	"Создать представление…":                  "Create view…",
	"Экспорт таблицы в CSV…":                  "Export table to CSV…",
	"Экспорт таблицы в JSON…":                 "Export table to JSON…",
	"Экспорт страницы в CSV…":                 "Export page to CSV…",
	"Экспорт страницы в JSON…":                "Export page to JSON…",
	"Экспорт строки в CSV…":                   "Export row to CSV…",
	"Экспорт строки в JSON…":                  "Export row to JSON…",
	"Копировать ячейку":                       "Copy cell",
	"Копировать строку":                       "Copy row",
	"Показать BLOB (hex)":                     "Show BLOB (hex)",
	"Сохранить BLOB в файл…":                  "Save BLOB to file…",
	"Структура таблицы":                       "Table schema",
	"Импорт CSV в таблицу…":                   "Import CSV into table…",
	"Импорт JSON в таблицу…":                  "Import JSON into table…",
	"Дублировать строку":                      "Duplicate row",
	"Создать индекс…":                         "Create index…",
	"Удалить индекс…":                         "Drop index…",
	"Создать индекс (шаблон SQL)":             "Create index (SQL template)",
	"Очистить таблицу…":                       "Clear table…",
	"Удалить таблицу…":                        "Drop table…",
	"Сбросить фильтр":                         "Reset filter",
	"Редактировать структуру…":                "Edit schema…",
	"SQL-консоль":                             "SQL console",
	"Конструктор запроса…":                    "Query builder…",
	"Экспорт результата в CSV…":               "Export result to CSV…",
	"Экспорт результата в JSON…":              "Export result to JSON…",
	"Тема: %s…":                               "Theme: %s…",
	"Тема: ":                                  "Theme: ",
	"Тема интерфейса":                         "Interface theme",
	"Размер страницы: %d…":                    "Page size: %d…",
	"Разделители колонок: %s":                 "Column separators: %s",
	"Запоминать последнюю папку: %s":          "Remember last folder: %s",
	"Сплэш при запуске: %s":                   "Splash on startup: %s",
	"Язык: %s…":                               "Language: %s…",
	"Язык: ":                                  "Language: ",
	"Язык интерфейса":                         "Interface language",
	"Язык интерфейса: %s":                     "Interface language: %s",
	"Помощь":                                  "Help",
	"О программе":                             "About",
	"Действия (Ctrl+P)":                       "Actions (Ctrl+P)",
	"  ←/→ раздел · ↑/↓ пункт · Enter · Esc ": "  ←/→ section · ↑/↓ item · Enter · Esc ",

	// Data page.
	" Данные ": " Data ",
	"Ячейка":   "Cell",
	"Назад":    "Back",
	"Сброс ф.": "Reset flt.",
	"Встав.":   "Insert",
	"Удал.":    "Delete",
	"Строк: %d · страница %d/%d": "Rows: %d · page %d/%d",
	" · сортировка: %s %s":       " · sort: %s %s",
	"таблица %s не найдена":      "table %s not found",
	"Ячейка обновлена":           "Cell updated",
	"Эта таблица не поддерживает построчное редактирование.":                  "This table does not support row editing.",
	"Представления доступны только для чтения.":                               "Views are read-only.",
	"База открыта в режиме только чтения.\n\nЗапустите msqlite с флагом -rw.": "The database is open read-only.\n\nStart msqlite with the -rw flag.",
	"База открыта в режиме только чтения.\n\nЗапустите с -rw или нажмите F8.": "The database is open read-only.\n\nStart with -rw or press F8.",
	"Удалить строку №%d из %s?":                                               "Delete row #%d from %s?",
	"Удалить":                            "Delete",
	"Отмена":                             "Cancel",
	"строка не найдена":                  "row not found",
	"Строка удалена":                     "Row deleted",
	"Фильтр сброшен":                     "Filter reset",
	"Строка дублирована":                 "Row duplicated",
	"не удалось определить rowid строки": "could not determine the row rowid",
	"Enter/F2 — правка ячейки · Ins — вставка · Del — удаление · s или клик по заголовку — сортировка": "Enter/F2 — edit cell · Ins — insert · Del — delete · s or header click — sort",

	// Dialogs and common messages.
	"Ошибка":        "Error",
	"Подтверждение": "Confirmation",
	"Сообщение":     "Message",
	"Enter — подтвердить · Esc — отмена": "Enter — confirm · Esc — cancel",
	"Да":                "Yes",
	"Перезаписать":      "Overwrite",
	"Выйти из msqlite?": "Exit msqlite?",
	"Новая база данных": "New database",
	"Файл: ":            "File: ",
	"Путь: ":            "Path: ",
	"Enter — создать · Esc — отмена":            "Enter — create · Esc — cancel",
	"Enter — открыть · Esc — отмена":            "Enter — open · Esc — cancel",
	"Enter — продолжить · Esc — отмена":         "Enter — continue · Esc — cancel",
	"База создана: ":                            "Database created: ",
	"Открыть базу данных":                       "Open database",
	"Файл уже существует:\n%s\n\nПерезаписать?": "File already exists:\n%s\n\nOverwrite?",
	"терминал не поддерживает буфер обмена":     "the terminal does not support the clipboard",
	"Скопировано в буфер обмена":                "Copied to clipboard",
	"не является файлом SQLite":                 "is not a SQLite file",
	"это каталог":                               "is a directory",

	// Export, import, maintenance.
	"Экспорт страницы в %s":                        "Export page to %s",
	"Экспорт строки в %s":                          "Export row to %s",
	"Экспорт таблицы в %s":                         "Export table to %s",
	"Экспорт результата в %s":                      "Export result to %s",
	"Экспорт SQL-дампа":                            "Export SQL dump",
	"Резервная копия базы":                         "Database backup",
	"Экспортировано строк: %d\n%s":                 "Rows exported: %d\n%s",
	"SQL-дамп создан.\nСтрок данных: %d\n%s":       "SQL dump created.\nData rows: %d\n%s",
	"Резервная копия создана:\n%s":                 "Backup created:\n%s",
	"Сначала откройте таблицу.":                    "Open a table first.",
	"В эту таблицу нельзя вставлять строки.":       "Rows cannot be inserted into this table.",
	"Эта таблица не поддерживает изменение строк.": "This table does not support row changes.",
	"База открыта в режиме только чтения.\nПереключите режим клавишей F8.": "The database is open read-only.\nToggle the mode with F8.",
	"Импорт CSV":  "Import CSV",
	"Импорт JSON": "Import JSON",
	"CSV пуст":    "CSV is empty",
	"JSON должен быть массивом объектов: %w": "JSON must be an array of objects: %w",
	"JSON не содержит строк":                 "JSON contains no rows",
	"в заголовке CSV нет колонок таблицы %s": "CSV header has no columns of table %s",
	"в JSON нет колонок таблицы %s":          "JSON has no columns of table %s",
	"строка %d: мало значений":               "row %d: too few values",
	"Импортировано строк: %d":                "Rows imported: %d",
	"Очистить таблицу %s?":                   "Clear table %s?",
	"Удалить таблицу %s?":                    "Drop table %s?",
	"Сохранено %d байт:\n%s":                 "Saved %d bytes:\n%s",
	"Сохранить BLOB":                         "Save BLOB",
	"Выбранная ячейка не является BLOB.":     "The selected cell is not a BLOB.",
	"Значение не является BLOB.":             "The value is not a BLOB.",
	"%s завершено. Затронуто строк: %d":      "%s completed. Rows affected: %d",
	"\n... показаны первые 64 KiB\n":         "\n... first 64 KiB shown\n",

	// Cell editor.
	"Правка · %s.%s": "Edit · %s.%s",
	"· строка %d":    "· row %d",
	"Enter — сохранить · Esc — отмена · пусто — NULL":                                  "Enter — save · Esc — cancel · empty — NULL",
	"Ctrl+S — сохранить · Tab — кнопки · Esc — отмена · пусто — NULL":                  "Ctrl+S — save · Tab — buttons · Esc — cancel · empty — NULL",
	"Ctrl+S — сохранить · Tab — кнопки · Esc — отмена · пусто — NULL · x'ABCD' — BLOB": "Ctrl+S — save · Tab — buttons · Esc — cancel · empty — NULL · x'ABCD' — BLOB",
	"Хеш: ": "Hash: ",

	// Filter builder.
	"Колонка: ":       "Column: ",
	"Оператор: ":      "Operator: ",
	"Значение: ":      "Value: ",
	"значение":        "value",
	"выражение WHERE": "WHERE expression",
	"Фильтр · %s":     "Filter · %s",
	"Выберите колонку, оператор и значение либо введите WHERE вручную": "Choose a column, operator and value, or enter WHERE manually",

	// Indexes.
	"Создать индекс":               "Create index",
	"Удалить индекс":               "Drop index",
	"Имя индекса":                  "Index name",
	"Колонки":                      "Columns",
	"Создать":                      "Create",
	"укажите имя индекса":          "enter an index name",
	"укажите хотя бы одну колонку": "enter at least one column",
	"Индекс создан: ":              "Index created: ",
	"Индекс удалён: ":              "Index dropped: ",
	"В таблице нет индексов, созданных пользователем.": "The table has no user-created indexes.",
	"Удалить индекс %s?": "Drop index %s?",

	// Insert form.
	" Значения ":        " Values ",
	"Поле":              "Field",
	"Прокрутка":         "Scroll",
	"Вставить":          "Insert",
	"Добавление строки": "Insert row",
	"Нет колонок":       "No columns",
	"Поле %d/%d · пусто — DEFAULT · NULL — NULL · '' — пустая строка": "Field %d/%d · empty — DEFAULT · NULL — NULL · '' — empty string",
	"Строка добавлена · ": "Row inserted · ",

	// Query builder.
	"В базе нет таблиц или представлений.": "The database has no tables or views.",
	"Таблица: ": "Table: ",
	"Колонки (Space/Enter — выбрать, пусто — все):": "Columns (Space/Enter — select, empty — all):",
	"условие (необязательно)":                       "condition (optional)",
	"без ограничения":                               "no limit",
	"OK — вставить SELECT в SQL-консоль":            "OK — insert SELECT into the SQL console",
	"Выберите таблицу":                              "Select a table",
	"LIMIT должен быть положительным числом":        "LIMIT must be a positive number",
	"Конструктор запроса":                           "Query builder",

	// SQL console.
	" Результат ":                  " Result ",
	" Запрос ":                     " Query ",
	"Введите SQL и нажмите Enter.": "Enter SQL and press Enter.",
	"Выполнить":                    "Execute",
	"Строка":                       "New line",
	"Дополнить":                    "Complete",
	"Знач.":                        "Value",
	"Констр.":                      "Builder",
	"История":                      "History",
	"Стоп/Назад":                   "Stop/Back",
	"Выполняется… Esc — отмена":        "Running… Esc — cancel",
	"Запрос отменён":                   "Query cancelled",
	"Отмена…":                          "Cancelling…",
	"Нет вариантов автодополнения":     "No completions",
	"Автодополнение (%d): %s":          "Completions (%d): %s",
	"OK · строк: %d · %d мс":           "OK · rows: %d · %d ms",
	"OK · затронуто строк: %d · %d мс": "OK · affected rows: %d · %d ms",
	" · показаны первые 1000":          " · first 1000 shown",

	// SQL schema builder.
	"Колонки (Enter — загрузить в форму):": "Columns (Enter — load into form):",
	"Имя: ":         "Name: ",
	"Тип: ":         "Type: ",
	"SQL-выражение": "SQL expression",
	"DEFAULT — SQL-выражение (0, 'x', CURRENT_TIMESTAMP) · Ctrl+S — создать": "DEFAULT — SQL expression (0, 'x', CURRENT_TIMESTAMP) · Ctrl+S — create",
	"Добавить":            "Add",
	"Очистить":            "Clear",
	"Обновить":            "Refresh",
	"Укажите имя колонки": "Enter a column name",
	"Колонка добавлена · Ctrl+S — создать таблицу":         "Column added · Ctrl+S — create table",
	"Выберите колонку для удаления":                        "Select a column to drop",
	"База открыта в режиме только чтения. Переключите F8.": "The database is open read-only. Toggle with F8.",
	"Таблица %q создана":                                   "Table %q created",
	"Создание таблицы":                                     "Create table",
	"укажите имя таблицы":                                  "enter a table name",
	"добавьте хотя бы одну колонку":                        "add at least one column",
	"Имя представления: ":                                  "View name: ",
	"Укажите имя представления":                            "Enter a view name",
	"Укажите SELECT-запрос":                                "Enter a SELECT query",
	"Представление %q создана":                             "View %q created",
	"CREATE VIEW имя AS <SELECT> · Ctrl+S — создать":       "CREATE VIEW name AS <SELECT> · Ctrl+S — create",
	"Создание представления":                               "Create view",

	// Schema editor.
	"Редактирование структуры доступно только для таблиц.":     "Schema editing is available for tables only.",
	"Переименовать таблицу…":                                   "Rename table…",
	"Добавить колонку…":                                        "Add column…",
	"Переименовать колонку…":                                   "Rename column…",
	"Удалить колонку…":                                         "Drop column…",
	"Структура · %s":                                           "Schema · %s",
	"Переименовать таблицу":                                    "Rename table",
	"Новое имя: ":                                              "New name: ",
	"Enter — переименовать · Esc — отмена":                     "Enter — rename · Esc — cancel",
	"Таблица переименована в %q":                               "Table renamed to %q",
	"Для NOT NULL у существующей таблицы обычно нужен DEFAULT": "For NOT NULL on an existing table a DEFAULT is usually required",
	"Колонка %q добавлена":                                     "Column %q added",
	"Добавить колонку":                                         "Add column",
	"В таблице нет колонок.":                                   "The table has no columns.",
	"Выберите колонку":                                         "Select a column",
	"Укажите новое имя колонки":                                "Enter a new column name",
	"Колонка %q переименована":                                 "Column %q renamed",
	"Переименовать колонку":                                    "Rename column",
	"Данные выбранной колонки будут удалены":                   "Data in the selected column will be deleted",
	"Удалить колонку %q?":                                      "Drop column %q?",
	"Колонка %q удалена":                                       "Column %q dropped",
	"Удалить колонку":                                          "Drop column",

	// Structure page.
	" Структура ":   " Schema ",
	"структура":     "schema",
	"строк: ":       "rows: ",
	"(нет колонок)": "(no columns)",
	"Тип":           "Type",
	"Индексы":       "Indexes",
	"(нет)":         "(none)",
	"Внешние ключи": "Foreign keys",
	"Триггеры":      "Triggers",

	// Recent databases.
	"Список недавних баз пуст.": "The recent databases list is empty.",
	"Недавние базы":             "Recent databases",

	// Splash.
	"SQLite manager для терминала":                             "SQLite manager for the terminal",
	"в стиле Midnight Commander / Far Manager / DOS Navigator": "in the style of Midnight Commander / Far Manager / DOS Navigator",
	"Нажмите любую клавишу…   ·   v%s":                         "Press any key…   ·   v%s",
	"Автор идеи":         "Idea",
	"Создатель":          "Creator",
	"  · Esc — закрыть ": "  · Esc — close ",
	"%s.%s · %d байт":    "%s.%s · %d bytes",
	"Запоминание последней папки включено":                                        "Remembering the last folder enabled",
	"Запоминание последней папки выключено":                                       "Remembering the last folder disabled",
	"Разделители колонок включены":                                                "Column separators enabled",
	"Разделители колонок выключены":                                               "Column separators disabled",
	"Размер страницы: %d":                                                         "Page size: %d",
	"Сплэш при запуске включён":                                                   "Startup splash enabled",
	"Сплэш при запуске выключен":                                                  "Startup splash disabled",
	"Тема интерфейса: %s":                                                         "UI theme: %s",
	"размер страницы должен быть от 1 до 10000":                                   "page size must be between 1 and 10000",
	"[yellow::b]%s[-:-:-] [white]· %s  [silver]· колонок: %d · %s":                "[yellow::b]%s[-:-:-] [white]· %s  [silver]· columns: %d · %s",
	"[yellow::b]Добавление строки[-:-:-] [white]· %s  [silver]· колонок: %d · %s": "[yellow::b]Insert row[-:-:-] [white]· %s  [silver]· columns: %d · %s",
	" · затронуто строк: %d · %d мс":                                              " · affected rows: %d · %d ms",
	" · строк: %d · %d мс":                                                        " · rows: %d · %d ms",
	"Представление %q создано":                                                    "View %q created",
	"[yellow::b]%s[-:-:-]  [silver]%s[-:-:-]   строк: [white]%s\n\n":              "[yellow::b]%s[-:-:-]  [silver]%s[-:-:-]   rows: [white]%s\n\n",
	"[yellow::b]%s[-:-:-] [white]· %s  [silver]· структура · %s":                  "[yellow::b]%s[-:-:-] [white]· %s  [silver]· schema · %s",
	"для этой таблицы нельзя определить rowid строки":                             "cannot determine the row rowid for this table",
	"помощь": "help",
	" Клавиши и возможности ":          " Keys and features ",
	"Разработчик":                      "Developer",
	"Enter — сохранить · Esc — отмена": "Enter — save · Esc — cancel",
	"Запрос пуст":                      "Query is empty",
	"Запрос сохранён":                  "Query saved",
	"История сортировок…":              "Sort history…",
	"История сортировок":               "Sort history",
	"История сортировок пуста.":        "Sort history is empty.",
	"История фильтров…":                "Filter history…",
	"История фильтров":                 "Filter history",
	"История фильтров пуста.":          "Filter history is empty.",
	"Колонка не найдена":               "Column not found",
	"Отменить изменение":               "Undo change",
	"Перейти по внешнему ключу…":       "Follow foreign key…",
	"Повторить изменение":              "Redo change",
	"Поиск…":                           "Search…",
	"Создать триггер…":                 "Create trigger…",
	"Сохранить запрос":                 "Save query",
	"Сохранить запрос…":                "Save query…",
	"Сохранённые запросы":              "Saved queries",
	"Сохранённые запросы…":             "Saved queries…",
	"Сохранённых запросов нет.":        "No saved queries.",
	"Строка не выбрана":                "No row selected",
	"Таблица не найдена":               "Table not found",
	"Триггер удалён: ":                 "Trigger dropped: ",
	"У выбранной строки нет заполненных внешних ключей.": "The selected row has no populated foreign keys.",
	"У таблицы нет триггеров.":                           "The table has no triggers.",
	"Удалить триггер":                                    "Drop trigger",
	"Удалить триггер %s?":                                "Drop trigger %s?",
	"Удалить триггер…":                                   "Drop trigger…",
	"Enter — найти · Esc — отмена":                       "Enter — find · Esc — cancel",
	"Найти: ":                "Find: ",
	"Не найдено: %s":         "Not found: %s",
	"Поиск":                  "Search",
	"Ошибка SQLite (код %d)": "SQLite error (code %d)",
	"Строка %d, колонка %d":  "Line %d, column %d",
	"Изменение отменено":     "Change undone",
	"Изменение повторено":    "Change redone",
	"Нечего отменять":        "Nothing to undo",
	"Нечего повторять":       "Nothing to redo",
	"Отмена доступна только для открытой таблицы": "Undo is only available for the open table",
	"Обзор схемы…": "Schema overview…",
	"Схема базы: ": "Database schema: ",
}

// detectSystemLanguage returns ru for a Russian locale and en otherwise.
func detectSystemLanguage() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			continue
		}
		if localeIsRussian(v) {
			return langRU
		}
		return langEN
	}
	if localeIsRussian(os.Getenv("LANGUAGE")) {
		return langRU
	}
	return langEN
}

func localeIsRussian(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return false
	}
	if i := strings.IndexByte(v, ':'); i >= 0 {
		for _, part := range strings.Split(v, ":") {
			if localeIsRussian(part) {
				return true
			}
		}
		return false
	}
	if i := strings.IndexByte(v, '.'); i >= 0 {
		v = v[:i]
	}
	if i := strings.IndexByte(v, '@'); i >= 0 {
		v = v[:i]
	}
	return v == "ru" || strings.HasPrefix(v, "ru_") || strings.HasPrefix(v, "ru-")
}

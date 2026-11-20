package ui

import (
	"fmt"

	"github.com/rivo/tview"
)

const helpTextRU = `[yellow::b]Запуск[-:-:-]
  msqlite [ФАЙЛ.db | КАТАЛОГ] [-dir КАТАЛОГ] [-page N] [-rw] [-no-splash]
  При старте показываются папки и SQLite-файлы текущего каталога.
  Текущий режим: [white]%s[-:-:-]

[yellow::b]Главный экран[-:-:-]
  [white]Tab[-:-:-]           переключить панель (базы / таблицы)
  [white]Enter[-:-:-]         открыть базу/таблицу, войти в папку или «..»
  [white]Bksp[-:-:-]          подняться на каталог выше (до корня /)
  [white]F2[-:-:-]            открыть файл или каталог по пути
  [white]F3[-:-:-]            структура выбранной таблицы
  [white]F4[-:-:-]            SQL-консоль
  [white]/[-:-:-]             фильтр списка таблиц по имени
  [white]F5[-:-:-]            обновить список и счётчики строк
  [white]F8[-:-:-]            режим: только чтение / редактирование
  [white]F9[-:-:-]            верхнее меню: Файл, База, Данные, SQL, Интерфейс, Справка
  [white]F1[-:-:-]            эта справка
  [white]F10 / Alt+X[-:-:-]   выход (с подтверждением)
  [white]Ctrl+P[-:-:-]        действия: экспорт, импорт, обслуживание
  [white]Alt+1…9 / Ctrl+Tab[-:-:-] переключение вкладок баз
  [white]Esc[-:-:-]           назад / подтверждение выхода

[yellow::b]Данные[-:-:-]
  [white]Стрелки[-:-:-]       перемещение по ячейкам
  [white]Enter / F2[-:-:-]    редактировать ячейку / показать значение
  [white]Ctrl+S[-:-:-]        сохранить ячейку, Ins — вставить строку, Del — удалить
  [white]Ctrl+Z / Ctrl+Y[-:-:-] отменить / повторить правку или удаление строки
  Редактор ячейки: число — компактное окно (Enter сохраняет), текст —
  большое, BLOB и прочие типы — максимальное (Ctrl+S сохраняет, Tab — кнопки).
  Для текста доступны хеши: MD5, SHA-1, SHA-256, SHA-512 и bcrypt.
  [white]F3[-:-:-]            структура таблицы
  [white]F6[-:-:-]            фильтр: конструктор WHERE (колонка, оператор, значение) или ручной ввод
  [white]Ctrl+R[-:-:-]        сбросить фильтр (или F6 → пусто → OK)
  [white]F7 / PgUp[-:-:-]     предыдущая страница
  [white]PgDn[-:-:-]          следующая страница
  [white]PgUp / PgDn[-:-:-]   то же; Home/End — первая/последняя страница
  [white]Ctrl+F[-:-:-]         поиск по текущей странице данных
  [white]s / клик по шапке[-:-:-] сортировка колонки: ASC → DESC → без сортировки
  [white]F5[-:-:-]            перечитать данные
  [white]Esc[-:-:-]           назад

[yellow::b]Структура таблицы[-:-:-]
  [white]F2[-:-:-]            переименовать таблицу/колонку, добавить/удалить колонку
  [white]F5[-:-:-]            обновить
  [white]F4[-:-:-]            SQL-консоль
  [white]Esc[-:-:-]           назад

[yellow::b]SQL-консоль[-:-:-]
  [white]Enter[-:-:-]         выполнить запрос
  [white]Alt+Enter[-:-:-]     новая строка в запросе
  [white]Tab[-:-:-]           автодополнение (как в bash); повтор — следующий вариант.
                           Если дополнять нечего — фокус на результат
  [white]Ctrl+Space[-:-:-]    то же автодополнение: SQL-команды, таблицы и поля
  [white]F6[-:-:-]            конструктор SELECT: таблица, колонки, WHERE, ORDER BY, LIMIT
  [white]Alt+↑ / Alt+↓[-:-:-] история запросов (сохраняется между запусками)
  [white]Ctrl+F[-:-:-]         поиск в результате запроса
  [white]F2[-:-:-]            полное значение выбранной ячейки
  [white]Esc[-:-:-]            отменить выполняющийся запрос
  Выполняется одна инструкция за раз; SELECT показывает не более 1000 строк.
  Ключевые слова, строки, числа и комментарии подсвечиваются.
  Результат показывает вертикальные линии колонок (отключаются в «Интерфейсе»).

[yellow::b]Верхнее меню (F9) и палитра (Ctrl+P)[-:-:-]
  F9: разделы ←/→ или Tab, пункты ↑/↓, Enter — выполнить, Esc — закрыть.
  • О программе — повторный показ стартового экрана (раздел «Справка»).
  • Интерфейс — тема (Midnight Commander / DOS Navigator / DOS Navigator Blue),
    язык (русский / English), размер страницы, вертикальные разделители колонок
    таблицы и запоминание последней папки. DN Blue:
    синие панели, бирюзовые рамки, чёрные верхняя и нижняя полосы, а также
    диалоги в стиле DN II: светло-серый фон, белая рамка, синяя кнопка OK
    и серая кнопка Отмена.
  • Настройки (тема, язык, размер страницы, разделители колонок, режим
    чтения/записи, запоминание последней папки, сплэш, история SQL)
    запоминаются в ~/.config/msqlite/config.json.
  • Открытие недавних баз и создание новой базы данных.
  • Экспорт всей таблицы, текущей страницы, выбранной строки или результата SQL в CSV/JSON.
  • Импорт CSV и JSON в текущую таблицу.
  • Точный пересчёт счётчиков строк (по умолчанию — оценки ~N из sqlite_stat1, для таблиц без статистики — точный COUNT(*)).
  • Сброс активного фильтра таблицы (Ctrl+R или F6 → пусто → OK).
  • Во всех диалогах есть кнопки OK и Отмена (Tab/Shift+Tab — переключение).
  • Экспорт SQL-дампа базы и резервная копия (VACUUM INTO).
  • VACUUM, ANALYZE, PRAGMA integrity_check.
  • Дублирование строки, копирование ячейки/строки в буфер обмена.
  • Просмотр BLOB в hex и сохранение BLOB в файл.
  • Создание таблиц (визуальный конструктор колонок: тип, PK, NOT NULL,
    DEFAULT) и представлений; создание и удаление индексов, очистка и
    удаление таблицы. В структуре видны триггеры и generated-колонки.

[yellow::b]Примечания[-:-:-]
  • По умолчанию база открыта только для чтения. F8 переключает режим
    на лету (текущая база переоткрывается с правами записи); выбранный
    режим запоминается для следующих запусков.
  • В режиме только чтения изменения запрещены; SQL-консоль вернёт
    ошибку SQLite на INSERT/UPDATE/DELETE/DDL.
  • CSV-экспорт выгружает все строки с учётом фильтра и сортировки;
    экспорт страницы — только текущую страницу, результата SQL — первые
    1000 строк.
  • Импорт CSV: первая строка считается заголовком, если её поля
    совпадают с именами колонок таблицы. Импорт JSON ожидает массив
    объектов с ключами-именами колонок.
  • Generated-колонки доступны только для чтения и не попадают в форму
    вставки. В режиме записи включена проверка внешних ключей SQLite.
  • Если язык ещё не выбран в меню, он определяется по системной локали:
    ru_* — русский, иначе английский.
  • При изменении файла открытой базы другим процессом список таблиц,
    данные и структура обновляются автоматически; позиция курсора,
    выбранная колонка и страница сохраняются.
  • Файлы определяются по заголовку SQLite, а не по расширению.
  • Работает мышь: клик по панели/строке, двойной клик — открыть;
    в меню F9 кликабельны разделы верхней строки и пункты подменю.
  • Для Raspberry Pi 4 соберите arm64-бинарник: make pi
`

const helpTextEN = `[yellow::b]Startup[-:-:-]
  msqlite [FILE.db | DIRECTORY] [-dir DIRECTORY] [-page N] [-rw] [-no-splash]
  On startup, folders and SQLite files in the current directory are shown.
  Current mode: [white]%s[-:-:-]

[yellow::b]Main screen[-:-:-]
  [white]Tab[-:-:-]           switch panel (databases / tables)
  [white]Enter[-:-:-]         open a database/table, enter a folder or ".."
  [white]Bksp[-:-:-]          go up one directory (up to /)
  [white]F2[-:-:-]            open a file or directory by path
  [white]F3[-:-:-]            schema of the selected table
  [white]F4[-:-:-]            SQL console
  [white]/[-:-:-]             filter the table list by name
  [white]F5[-:-:-]            refresh the list and row counts
  [white]F8[-:-:-]            mode: read-only / editing
  [white]F9[-:-:-]            top menu: File, Database, Data, SQL, Interface, Help
  [white]F1[-:-:-]            this help
  [white]F10 / Alt+X[-:-:-]   exit (with confirmation)
  [white]Ctrl+P[-:-:-]        actions: export, import, maintenance
  [white]Alt+1…9 / Ctrl+Tab[-:-:-] switch database tabs
  [white]Esc[-:-:-]           back / confirm exit

[yellow::b]Data[-:-:-]
  [white]Arrows[-:-:-]        move between cells
  [white]Enter / F2[-:-:-]    edit a cell / show its value
  [white]Ctrl+S[-:-:-]        save the cell, Ins — insert a row, Del — delete
  [white]Ctrl+Z / Ctrl+Y[-:-:-] undo / redo a cell edit or row deletion
  Cell editor: numbers — compact window (Enter saves), text —
  large, BLOB and other types — largest (Ctrl+S saves, Tab — buttons).
  Text values support hashes: MD5, SHA-1, SHA-256, SHA-512 and bcrypt.
  [white]F3[-:-:-]            table schema
  [white]F6[-:-:-]            filter: WHERE builder (column, operator, value) or manual input
  [white]Ctrl+R[-:-:-]        reset filter (or F6 → empty → OK)
  [white]F7 / PgUp[-:-:-]     previous page
  [white]PgDn[-:-:-]          next page
  [white]PgUp / PgDn[-:-:-]   same; Home/End — first/last page
  [white]Ctrl+F[-:-:-]         search the current data page
  [white]s / header click[-:-:-] sort column: ASC → DESC → unsorted
  [white]F5[-:-:-]            reload data
  [white]Esc[-:-:-]           back

[yellow::b]Table schema[-:-:-]
  [white]F2[-:-:-]            rename table/column, add/drop column
  [white]F5[-:-:-]            refresh
  [white]F4[-:-:-]            SQL console
  [white]Esc[-:-:-]           back

[yellow::b]SQL console[-:-:-]
  [white]Enter[-:-:-]         execute query
  [white]Alt+Enter[-:-:-]     new line in the query
  [white]Tab[-:-:-]           autocomplete (like bash); repeat — next variant.
                           If there is nothing to complete — focus the result
  [white]Ctrl+Space[-:-:-]    same autocomplete: SQL commands, tables and columns
  [white]F6[-:-:-]            SELECT builder: table, columns, WHERE, ORDER BY, LIMIT
  [white]Alt+↑ / Alt+↓[-:-:-] query history (persists between runs)
  [white]Ctrl+F[-:-:-]         search the query result
  [white]F2[-:-:-]            full value of the selected cell
  [white]Esc[-:-:-]           cancel the running query
  One statement is executed at a time; SELECT shows at most 1000 rows.
  Keywords, strings, numbers and comments are highlighted.
  The result shows vertical column lines (disabled in "Interface").

[yellow::b]Top menu (F9) and palette (Ctrl+P)[-:-:-]
  F9: sections ←/→ or Tab, items ↑/↓, Enter — execute, Esc — close.
  • About — show the startup screen again (Help section).
  • Interface — theme (Midnight Commander / DOS Navigator / DOS Navigator Blue),
    language (Russian / English), page size, vertical table column separators and
    last-folder memory. DN Blue:
    blue panels, cyan borders, black top and bottom bars, and DN II-style
    dialogs: light gray background, white border, blue OK button
    and gray Cancel button.
  • Settings (theme, language, page size, column separators, mode
    read/write, last-folder memory, splash, SQL history)
    are saved to ~/.config/msqlite/config.json.
  • Open recent databases and create a new database.
  • Export the whole table, current page, selected row or SQL result to CSV/JSON.
  • Import CSV and JSON into the current table.
  • Exact row-count recount (by default — ~N estimates from sqlite_stat1, for tables without statistics — exact COUNT(*)).
  • Reset the active table filter (Ctrl+R or F6 → empty → OK).
  • All dialogs have OK and Cancel buttons (Tab/Shift+Tab — switch).
  • Export the database SQL dump and backup (VACUUM INTO).
  • VACUUM, ANALYZE, PRAGMA integrity_check.
  • Duplicate a row, copy a cell/row to the clipboard.
  • View BLOBs in hex and save BLOBs to a file.
  • Create tables (visual column builder: type, PK, NOT NULL,
    DEFAULT) and views; create and drop indexes, clear and
    drop a table. Triggers and generated columns are shown in the schema.

[yellow::b]Notes[-:-:-]
  • By default the database is opened read-only. F8 switches the mode
    on the fly (the current database is reopened with write access); the selected
    mode is remembered for subsequent runs.
  • In read-only mode changes are forbidden; the SQL console returns
    an SQLite error for INSERT/UPDATE/DELETE/DDL.
  • CSV export dumps all rows with the current filter and sorting;
    page export — only the current page, SQL result — the first
    1000 rows.
  • CSV import: the first row is treated as a header if its fields
    match the table column names. JSON import expects an array of
    objects with column-name keys.
  • Generated columns are read-only and are not included in the insert
    form. SQLite foreign key checks are enabled in write mode.
  • If no language has been chosen in the menu yet, it is detected from the
    system locale: ru_* — Russian, otherwise English.
  • When the open database file is changed by another process, the table
    list, data and schema are refreshed automatically; the cursor position,
    selected column and page are preserved.
  • Files are detected by the SQLite header, not by extension.
  • Mouse is supported: click a panel/row, double-click — open;
    in the F9 menu the top-row sections and submenu items are clickable.
  • For Raspberry Pi 4 build the arm64 binary: make pi
`

// HelpPage shows key bindings and notes.
type HelpPage struct {
	app   *App
	root  *tview.Flex
	head  *tview.TextView
	text  *tview.TextView
	hints *hintView
}

func newHelpPage(a *App) *HelpPage {
	p := &HelpPage{app: a}
	p.head = tview.NewTextView().SetDynamicColors(true)
	p.head.SetBackgroundColor(colBg)
	p.head.SetTextColor(colText)
	p.head.SetText(fmt.Sprintf("[yellow::b]msqlite[-:-:-][white] · %s  [gray::d]v%s", tr("помощь"), esc(a.opts.Version)))

	p.text = tview.NewTextView()
	styleTextView(p.text)
	p.text.SetScrollable(true)
	p.text.SetWrap(true)
	p.text.SetBorder(true)
	p.text.SetTitle(tr(" Клавиши и возможности "))
	p.text.SetTitleColor(colTitle)
	p.text.SetBorderColor(colBorder)

	p.setText()

	p.hints = newHintView()
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.hints.SetHints("Esc", "Назад", "↑/↓", "Прокрутка", "F10", "Выход")

	p.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.head, 1, 0, false).
		AddItem(p.text, 0, 1, true).
		AddItem(p.hints, 1, 0, false)
	return p
}

func (p *HelpPage) setText() {
	mode := "только чтение (по умолчанию)"
	if !p.app.opts.ReadOnly {
		mode = "чтение/запись (-rw)"
	}
	text := helpTextRU
	if isEN() {
		text = helpTextEN
		mode = "read-only (default)"
		if !p.app.opts.ReadOnly {
			mode = "read/write (-rw)"
		}
	}
	p.text.SetText(fmt.Sprintf(text, mode))
}

func (p *HelpPage) relocalize() {
	p.head.SetText(fmt.Sprintf("[yellow::b]msqlite[-:-:-][white] · %s  [gray::d]v%s", tr("помощь"), esc(p.app.opts.Version)))
	p.text.SetTitle(tr(" Клавиши и возможности "))
	p.setText()
	p.hints.SetHints("Esc", "Назад", "↑/↓", "Прокрутка", "F10", "Выход")
}

func (p *HelpPage) Root() tview.Primitive { return p.root }

func (p *HelpPage) restyle() {
	p.head.SetBackgroundColor(colBg)
	p.head.SetTextColor(colText)
	styleTextView(p.text)
	p.text.SetBorderColor(colBorder)
	p.text.SetTitleColor(colTitle)
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.hints.SetHints("Esc", "Назад", "↑/↓", "Прокрутка", "F10", "Выход")
}

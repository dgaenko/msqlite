package ui

import (
	"fmt"

	"github.com/rivo/tview"
)

// HelpPage shows key bindings and notes.
type HelpPage struct {
	root  *tview.Flex
	head  *tview.TextView
	text  *tview.TextView
	hints *tview.TextView
}

func newHelpPage(a *App) *HelpPage {
	p := &HelpPage{}
	p.head = tview.NewTextView().SetDynamicColors(true)
	p.head.SetBackgroundColor(colBg)
	p.head.SetTextColor(colText)
	p.head.SetText(fmt.Sprintf("[yellow::b]msqlite[-:-:-][white] · помощь  [gray::d]v%s", esc(a.opts.Version)))

	p.text = tview.NewTextView()
	styleTextView(p.text)
	p.text.SetScrollable(true)
	p.text.SetWrap(true)
	p.text.SetBorder(true)
	p.text.SetTitle(" Клавиши и возможности ")
	p.text.SetTitleColor(colTitle)
	p.text.SetBorderColor(colBorder)

	mode := "только чтение (по умолчанию)"
	if !a.opts.ReadOnly {
		mode = "чтение/запись (-rw)"
	}
	p.text.SetText(fmt.Sprintf(`[yellow::b]Запуск[-:-:-]
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
  [white]Esc[-:-:-]           назад / подтверждение выхода

[yellow::b]Данные[-:-:-]
  [white]Стрелки[-:-:-]       перемещение по ячейкам
  [white]Enter / F2[-:-:-]    редактировать ячейку / показать значение
  [white]Ctrl+S[-:-:-]        сохранить ячейку, Ins — вставить строку, Del — удалить
  Редактор ячейки: число — компактное окно (Enter сохраняет), текст —
  большое, BLOB и прочие типы — максимальное (Ctrl+S сохраняет, Tab — кнопки).
  Для текста доступны хеши: MD5, SHA-1, SHA-256, SHA-512 и bcrypt.
  [white]F3[-:-:-]            структура таблицы
  [white]F6[-:-:-]            фильтр: конструктор WHERE (колонка, оператор, значение) или ручной ввод
  [white]Ctrl+R[-:-:-]        сбросить фильтр (или F6 → пусто → OK)
  [white]F7 / PgUp[-:-:-]     предыдущая страница
  [white]PgDn[-:-:-]           следующая страница
  [white]PgUp / PgDn[-:-:-]   то же; Home/End — первая/последняя страница
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
  [white]F2[-:-:-]            полное значение выбранной ячейки
  [white]Esc[-:-:-]            отменить выполняющийся запрос
  Выполняется одна инструкция за раз; SELECT показывает не более 1000 строк.
  Результат показывает вертикальные линии колонок (отключаются в «Интерфейсе»).

[yellow::b]Верхнее меню (F9) и палитра (Ctrl+P)[-:-:-]
  F9: разделы ←/→ или Tab, пункты ↑/↓, Enter — выполнить, Esc — закрыть.
  • О программе — повторный показ стартового экрана (раздел «Справка»).
  • Интерфейс — тема (Midnight Commander / DOS Navigator / DOS Navigator Blue),
    размер страницы, вертикальные разделители колонок таблицы и запоминание
    последней папки. DN Blue:
    синие панели, бирюзовые рамки, чёрные верхняя и нижняя полосы, а также
    диалоги в стиле DN II: светло-серый фон, белая рамка, синяя кнопка OK
    и серая кнопка Отмена.
  • Настройки (тема, размер страницы, разделители колонок, режим
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
  • При изменении файла открытой базы другим процессом список таблиц,
    данные и структура обновляются автоматически; позиция курсора,
    выбранная колонка и страница сохраняются.
  • Файлы определяются по заголовку SQLite, а не по расширению.
  • Работает мышь: клик по панели/строке, двойной клик — открыть;
    в меню F9 кликабельны разделы верхней строки и пункты подменю.
  • Для Raspberry Pi 4 соберите arm64-бинарник: make pi
`, mode))

	p.hints = tview.NewTextView().SetDynamicColors(true)
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.hints.SetText(hintBar("Esc", "Назад", "↑/↓", "Прокрутка", "F10", "Выход"))

	p.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.head, 1, 0, false).
		AddItem(p.text, 0, 1, true).
		AddItem(p.hints, 1, 0, false)
	return p
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
	p.hints.SetText(hintBar("Esc", "Назад", "↑/↓", "Прокрутка", "F10", "Выход"))
}

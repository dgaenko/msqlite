package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rivo/tview"

	"msqlite/internal/db"
)

// StructurePage shows columns, indexes, foreign keys and DDL of an object.
type StructurePage struct {
	app    *App
	db     *db.Database
	obj    db.Object
	cols   []db.Column
	notice string
	root   *tview.Flex
	head   *tview.TextView
	text   *tview.TextView
	hints  *hintView
}

func newStructurePage(a *App, d *db.Database, obj db.Object) *StructurePage {
	p := &StructurePage{app: a, db: d, obj: obj}
	p.build()
	return p
}

func (p *StructurePage) build() {
	p.head = tview.NewTextView().SetDynamicColors(true)
	p.head.SetBackgroundColor(colBg)
	p.head.SetTextColor(colText)
	p.head.SetWrap(false)
	p.updateHead()

	p.text = tview.NewTextView()
	styleTextView(p.text)
	p.text.SetScrollable(true)
	p.text.SetWrap(false)
	p.text.SetBorder(true)
	p.text.SetTitle(tr(" Структура "))
	p.text.SetTitleColor(colTitle)
	p.text.SetBorderColor(colBorder)

	p.hints = newHintView()
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.setHints()

	p.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.head, 1, 0, false).
		AddItem(p.text, 0, 1, true).
		AddItem(p.hints, 1, 0, false)
}

func (p *StructurePage) Root() tview.Primitive { return p.root }

func (p *StructurePage) setHints() {
	p.hints.SetHints(
		"F2", "Изменить",
		"Esc", "Назад",
		"F5", "Обнов.",
		"F4", "SQL",
		"F8", "Режим",
		"F9", "Меню",
	)
}

func (p *StructurePage) restyle() {
	p.head.SetBackgroundColor(colBg)
	p.head.SetTextColor(colText)
	styleTextView(p.text)
	p.text.SetBorderColor(colBorder)
	p.text.SetTitleColor(colTitle)
	p.hints.SetBackgroundColor(colHintBg)
	p.hints.SetTextColor(colHintFg)
	p.setHints()
}

func (p *StructurePage) updateHead() {
	mode := tr("только чтение")
	if !p.app.opts.ReadOnly {
		mode = tr("правка")
	}
	head := trf("[yellow::b]%s[-:-:-] [white]· %s  [silver]· структура · %s",
		esc(p.obj.Name), typeLabel(p.obj.Type), mode)
	if p.notice != "" {
		head += "  [green::b]" + esc(p.notice) + "[-:-:-]"
	}
	p.head.SetText(head)
}

func (p *StructurePage) Load() error {
	p.notice = ""
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cols, err := p.db.Columns(ctx, p.obj.Name)
	if err != nil {
		return err
	}
	p.cols = cols
	indexes, err := p.db.Indexes(ctx, p.obj.Name)
	if err != nil {
		return err
	}
	fks, err := p.db.ForeignKeys(ctx, p.obj.Name)
	if err != nil {
		return err
	}
	triggers, err := p.db.Triggers(ctx, p.obj.Name)
	if err != nil {
		return err
	}
	schema, err := p.db.Schema(ctx, p.obj.Name)
	if err != nil {
		return err
	}
	rowCount := "—"
	if n, err := p.db.Count(ctx, p.obj.Name, ""); err == nil {
		rowCount = strconv.FormatInt(n, 10)
	}

	var b strings.Builder
	fmt.Fprintf(&b, tr("[yellow::b]%s[-:-:-]  [silver]%s[-:-:-]   строк: [white]%s\n\n"),
		esc(p.obj.Name), typeLabel(p.obj.Type), rowCount)

	b.WriteString("[yellow::b]" + tr("Колонки") + "[-:-:-]\n")
	if len(cols) == 0 {
		b.WriteString("[silver]  " + tr("(нет колонок)") + "[-:-:-]\n")
	} else {
		fmt.Fprintf(&b, "[silver] %-4s %-30s %-20s %-9s %-20s %s[-:-:-]\n",
			"#", tr("Имя"), tr("Тип"), "NOT NULL", "DEFAULT", "PK")
		for _, c := range cols {
			typ := c.Type
			if typ == "" {
				typ = "—"
			}
			if c.Hidden {
				typ += " · generated"
			}
			nn := "—"
			if c.NotNull {
				nn = tr("да")
			}
			def := "—"
			if c.HasDef {
				def = c.Default
			}
			pk := ""
			if c.PK > 0 {
				pk = strconv.Itoa(c.PK)
			}
			fmt.Fprintf(&b, " %s %s %s %s %s %s\n",
				padLeft(strconv.Itoa(c.CID), 4),
				esc(padRight(c.Name, 30)),
				esc(padRight(typ, 20)),
				esc(padRight(nn, 9)),
				esc(padRight(def, 20)),
				pk)
		}
	}

	b.WriteString("\n[yellow::b]" + tr("Индексы") + "[-:-:-]\n")
	if len(indexes) == 0 {
		b.WriteString("[silver]  " + tr("(нет)") + "[-:-:-]\n")
	} else {
		for _, idx := range indexes {
			flags := []string{}
			if idx.Unique {
				flags = append(flags, "UNIQUE")
			}
			if idx.Partial {
				flags = append(flags, "PARTIAL")
			}
			switch idx.Origin {
			case "pk":
				flags = append(flags, "PRIMARY KEY")
			case "u":
				flags = append(flags, "CONSTRAINT")
			}
			colsText := make([]string, len(idx.Columns))
			for i, c := range idx.Columns {
				colsText[i] = esc(c)
			}
			flagText := ""
			if len(flags) > 0 {
				flagText = " [" + strings.Join(flags, ", ") + "]"
			}
			fmt.Fprintf(&b, "  [white]%s[-:-:-][silver]%s  →  %s[-:-:-]\n",
				esc(idx.Name), flagText, strings.Join(colsText, ", "))
		}
	}

	b.WriteString("\n[yellow::b]" + tr("Внешние ключи") + "[-:-:-]\n")
	if len(fks) == 0 {
		b.WriteString("[silver]  " + tr("(нет)") + "[-:-:-]\n")
	} else {
		for _, fk := range fks {
			to := fk.Table
			if fk.To != "" {
				to += "." + fk.To
			}
			fmt.Fprintf(&b, "  [white]%s[-:-:-][silver] → %s   ON UPDATE %s  ON DELETE %s[-:-:-]\n",
				esc(fk.From), esc(to), esc(fk.OnUpdate), esc(fk.OnDelete))
		}
	}

	b.WriteString("\n[yellow::b]" + tr("Триггеры") + "[-:-:-]\n")
	if len(triggers) == 0 {
		b.WriteString("[silver]  " + tr("(нет)") + "[-:-:-]\n")
	} else {
		for _, tr := range triggers {
			fmt.Fprintf(&b, "  [white]%s[-:-:-]\n", esc(tr.Name))
		}
	}

	b.WriteString("\n[yellow::b]DDL[-:-:-]\n")
	fmt.Fprintf(&b, "[silver]%s[-:-:-]\n", esc(strings.TrimSpace(schema)))

	p.text.SetText(b.String())
	p.text.ScrollToBeginning()
	return nil
}

func (p *StructurePage) relocalize() {
	p.text.SetTitle(tr(" Структура "))
	p.setHints()
	p.updateHead()
	_ = p.Load()
}

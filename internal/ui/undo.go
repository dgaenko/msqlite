package ui

import (
	"context"
	"fmt"
	"time"

	"msqlite/internal/db"
)

type undoKind int

const (
	undoCell undoKind = iota
	undoDelete
)

type undoOp struct {
	kind   undoKind
	table  string
	column string
	rowID  int64
	oldVal any
	newVal any
	cols   []db.Column
	vals   []any
}

func (a *App) recordUndo(op undoOp) {
	a.undoStack = append(a.undoStack, op)
	if len(a.undoStack) > 50 {
		a.undoStack = a.undoStack[1:]
	}
	a.redoStack = nil
}

func (a *App) canUndo() bool {
	return a.currentPage == pageData && a.data != nil && len(a.undoStack) > 0
}

func (a *App) canRedo() bool {
	return a.currentPage == pageData && a.data != nil && len(a.redoStack) > 0
}

func (a *App) undoLast() {
	if !a.canUndo() {
		a.notify(tr("Нечего отменять"))
		return
	}
	op := a.undoStack[len(a.undoStack)-1]
	a.undoStack = a.undoStack[:len(a.undoStack)-1]
	if err := a.applyUndo(op, true); err != nil {
		a.showError(err)
		return
	}
	a.redoStack = append(a.redoStack, op)
	a.notify(tr("Изменение отменено"))
}

func (a *App) redoLast() {
	if !a.canRedo() {
		a.notify(tr("Нечего повторять"))
		return
	}
	op := a.redoStack[len(a.redoStack)-1]
	a.redoStack = a.redoStack[:len(a.redoStack)-1]
	if err := a.applyUndo(op, false); err != nil {
		a.showError(err)
		return
	}
	a.undoStack = append(a.undoStack, op)
	a.notify(tr("Изменение повторено"))
}

func (a *App) applyUndo(op undoOp, reverse bool) error {
	if a.current == nil || a.data == nil || a.data.obj.Name != op.table {
		return fmt.Errorf("%s", tr("Отмена доступна только для открытой таблицы"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	switch op.kind {
	case undoCell:
		value := op.newVal
		if reverse {
			value = op.oldVal
		}
		if _, err := a.current.UpdateCell(ctx, op.table, op.column, value, op.rowID); err != nil {
			return err
		}
	case undoDelete:
		if reverse {
			if err := a.current.RestoreRow(ctx, op.table, op.cols, op.vals, op.rowID); err != nil {
				return err
			}
		} else {
			if _, err := a.current.DeleteRow(ctx, op.table, op.rowID); err != nil {
				return err
			}
		}
	}
	return a.data.Load()
}

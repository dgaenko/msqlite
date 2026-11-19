package ui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/rivo/tview"
)

type recentState struct {
	Paths []string `json:"paths"`
}

func recentFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "msqlite", "recent.json"), nil
}

func loadRecent() []string {
	path, err := recentFilePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var st recentState
	if json.Unmarshal(data, &st) != nil {
		return nil
	}
	return st.Paths
}

func (a *App) addRecent(path string) {
	paths := loadRecent()
	out := []string{path}
	for _, p := range paths {
		if p != path {
			out = append(out, p)
		}
	}
	if len(out) > 15 {
		out = out[:15]
	}
	file, err := recentFilePath()
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(recentState{Paths: out}, "", "  ")
	if err != nil {
		return
	}
	_ = writeFileAtomic(file, append(data, '\n'), 0o600)
}

func (a *App) openRecentDialog() {
	paths := loadRecent()
	if len(paths) == 0 {
		a.showModal(tr("Список недавних баз пуст."), []string{"OK"}, nil)
		return
	}
	list := tview.NewList()
	styleList(list, colDialogBg, colDialogFg, colDialogDim)
	list.SetBorder(false)
	list.SetTitle("")
	list.SetWrapAround(true)
	for _, path := range paths {
		path := path
		list.AddItem(path, "", 0, func() {
			a.closeDialog()
			a.openPath(path)
		})
	}
	openCurrent := func() {
		index := list.GetCurrentItem()
		if index < 0 || index >= len(paths) {
			return
		}
		path := paths[index]
		a.closeDialog()
		a.openPath(path)
	}
	a.showListDialog(tr("Недавние базы"), 70, list, openCurrent)
}

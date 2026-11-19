package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type appSettings struct {
	Theme       string   `json:"theme,omitempty"`
	Language    string   `json:"language,omitempty"`
	PageSize    int      `json:"page_size,omitempty"`
	ReadOnly    *bool    `json:"read_only,omitempty"`
	ColumnLines *bool    `json:"column_lines,omitempty"`
	RememberDir *bool    `json:"remember_dir,omitempty"`
	LastDir     string   `json:"last_dir,omitempty"`
	NoSplash    bool     `json:"no_splash,omitempty"`
	SQLHistory  []string `json:"sql_history,omitempty"`
}

func settingsFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "msqlite", "config.json"), nil
}

func loadSettings() appSettings {
	var s appSettings
	path, err := settingsFilePath()
	if err != nil {
		return s
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	if s.Theme == "" {
		// Migrate the theme file used by earlier versions.
		legacy := filepath.Join(filepath.Dir(path), "theme")
		if data, err := os.ReadFile(legacy); err == nil {
			s.Theme = strings.TrimSpace(string(data))
		}
	}
	return s
}

func saveSettings(s appSettings) {
	path, err := settingsFilePath()
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = writeFileAtomic(path, append(data, '\n'), 0o600)
}

func (a *App) persistSettings() {
	readOnly := a.opts.ReadOnly
	columnLines := a.colLines
	history := a.sqlHistory
	if len(history) > 200 {
		history = history[len(history)-200:]
	}
	rememberDir := a.rememberDir
	var lastDir string
	if rememberDir {
		lastDir = a.dir
	}
	saveSettings(appSettings{
		Theme:       activeTheme,
		Language:    activeLang,
		PageSize:    a.opts.PageSize,
		ReadOnly:    &readOnly,
		ColumnLines: &columnLines,
		RememberDir: &rememberDir,
		LastDir:     lastDir,
		NoSplash:    a.opts.NoSplash,
		SQLHistory:  history,
	})
}

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"msqlite/internal/ui"
)

var (
	version   = "1.0.0-dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Использование: msqlite [опции] [ФАЙЛ.db | КАТАЛОГ]")
		flag.PrintDefaults()
	}
	var (
		dir      string
		rw       bool
		pageSize int
		noSplash bool
		showVer  bool
	)
	flag.StringVar(&dir, "dir", ".", "каталог с базами данных")
	flag.BoolVar(&rw, "rw", false, "открыть базы в режиме чтения и записи")
	flag.IntVar(&pageSize, "page", 100, "размер страницы данных")
	flag.BoolVar(&noSplash, "no-splash", false, "не показывать сплэш при запуске")
	flag.BoolVar(&showVer, "version", false, "показать версию и выйти")
	flag.Parse()

	pageSizeSet, rwSet, dirSet := false, false, false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "page":
			pageSizeSet = true
		case "rw":
			rwSet = true
		case "dir":
			dirSet = true
		}
	})

	if showVer {
		fmt.Printf("msqlite %s\n", version)
		fmt.Printf("commit:  %s\n", commit)
		fmt.Printf("built:   %s\n", buildDate)
		fmt.Printf("go:      %s\n", runtime.Version())
		return
	}
	if pageSize < 1 || pageSize > 10000 {
		fmt.Fprintln(os.Stderr, "msqlite: -page должен быть от 1 до 10000")
		os.Exit(2)
	}

	openPath := ""
	if args := flag.Args(); len(args) > 0 {
		dirSet = true
		path, err := filepath.Abs(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "msqlite:", err)
			os.Exit(1)
		}
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "msqlite:", err)
			os.Exit(1)
		}
		if info.IsDir() {
			dir = path
		} else {
			dir = filepath.Dir(path)
			openPath = path
		}
	}

	err := ui.Run(ui.Options{
		Dir:         dir,
		OpenPath:    openPath,
		ReadOnly:    !rw,
		ReadOnlySet: rwSet,
		PageSize:    pageSize,
		PageSizeSet: pageSizeSet,
		NoSplash:    noSplash,
		DirSet:      dirSet,
		Version:     version,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "msqlite:", err)
		os.Exit(1)
	}
}

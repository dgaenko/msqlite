package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"msqlite/internal/db"
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
		execSQL  string
		dumpSQL  bool
		csvOut   bool
	)
	flag.StringVar(&dir, "dir", ".", "каталог с базами данных")
	flag.BoolVar(&rw, "rw", false, "открыть базы в режиме чтения и записи")
	flag.IntVar(&pageSize, "page", 100, "размер страницы данных")
	flag.BoolVar(&noSplash, "no-splash", false, "не показывать сплэш при запуске")
	flag.BoolVar(&showVer, "version", false, "показать версию и выйти")
	flag.StringVar(&execSQL, "exec", "", "выполнить SQL и выйти")
	flag.BoolVar(&dumpSQL, "dump", false, "вывести SQL-дамп базы и выйти")
	flag.BoolVar(&csvOut, "csv", false, "выводить результат -exec в CSV")
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

	if execSQL != "" || dumpSQL {
		if openPath == "" {
			fmt.Fprintln(os.Stderr, "msqlite: для -exec/-dump укажите файл базы данных")
			os.Exit(2)
		}
		if err := runCLI(openPath, execSQL, dumpSQL, csvOut, !rw); err != nil {
			fmt.Fprintln(os.Stderr, "msqlite:", err)
			os.Exit(1)
		}
		return
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

func runCLI(path, sqlText string, dump, csvOut, readOnly bool) error {
	d, err := db.Open(path, readOnly)
	if err != nil {
		return err
	}
	defer d.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if dump {
		f, err := os.CreateTemp("", "msqlite-dump-*.sql")
		if err != nil {
			return err
		}
		name := f.Name()
		f.Close()
		defer os.Remove(name)
		if _, err := d.DumpSQL(ctx, name); err != nil {
			return err
		}
		f, err = os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(os.Stdout, f)
		return err
	}

	res, err := d.Exec(ctx, sqlText, 1000)
	if err != nil {
		return err
	}
	if !res.IsSelect {
		fmt.Printf("rows affected: %d\n", res.Affected)
		return nil
	}
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()
	if csvOut {
		header := make([]string, len(res.Columns))
		for i, c := range res.Columns {
			header[i] = c.Name
		}
		if err := w.Write(header); err != nil {
			return err
		}
		for _, row := range res.Rows {
			rec := make([]string, len(row))
			for i, cell := range row {
				rec[i] = rawCLIValue(cell.Raw)
			}
			if err := w.Write(rec); err != nil {
				return err
			}
		}
		return nil
	}
	header := make([]string, len(res.Columns))
	for i, c := range res.Columns {
		header[i] = c.Name
	}
	fmt.Println(joinTSV(header))
	for _, row := range res.Rows {
		rec := make([]string, len(row))
		for i, cell := range row {
			rec[i] = rawCLIValue(cell.Raw)
		}
		fmt.Println(joinTSV(rec))
	}
	return nil
}

func rawCLIValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}

func joinTSV(values []string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += "\t"
		}
		out += v
	}
	return out
}

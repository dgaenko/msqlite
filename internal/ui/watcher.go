package ui

import (
	"os"
	"sync"
	"time"
)

// fileStamp is a lightweight snapshot of a file's state.
type fileStamp struct {
	exists  bool
	size    int64
	modTime int64
}

func statFile(path string) fileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{
		exists:  true,
		size:    info.Size(),
		modTime: info.ModTime().UnixNano(),
	}
}

// dbWatcher polls the currently open SQLite file and reports external changes.
// It watches the main database file and its WAL sidecar; the WAL is where
// committed changes appear when the database uses write-ahead logging.
type dbWatcher struct {
	app      *App
	interval time.Duration
	stop     chan struct{}

	mu      sync.Mutex
	started bool
	stopped bool
	target  string
	stamps  map[string]fileStamp
	pending bool
}

func newDBWatcher(a *App, interval time.Duration) *dbWatcher {
	if interval <= 0 {
		interval = time.Second
	}
	return &dbWatcher{
		app:      a,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

// Start begins polling in a background goroutine.
func (w *dbWatcher) Start() {
	w.mu.Lock()
	if w.started || w.stopped {
		w.mu.Unlock()
		return
	}
	w.started = true
	w.mu.Unlock()
	go w.run()
}

// Stop terminates the watcher goroutine.
func (w *dbWatcher) Stop() {
	w.mu.Lock()
	if !w.started || w.stopped {
		w.mu.Unlock()
		return
	}
	w.stopped = true
	close(w.stop)
	w.mu.Unlock()
}

// Watch switches monitoring to path. Switching to a different file resets the
// baseline so the first poll only records the current state.
func (w *dbWatcher) Watch(path string) {
	w.mu.Lock()
	if w.target != path {
		w.target = path
		w.stamps = stampsFor(path)
		w.pending = false
	}
	w.mu.Unlock()
}

// Unwatch stops monitoring the current file.
func (w *dbWatcher) Unwatch() {
	w.mu.Lock()
	w.target = ""
	w.stamps = nil
	w.pending = false
	w.mu.Unlock()
}

// IsTarget reports whether path is the file currently being monitored.
func (w *dbWatcher) IsTarget(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.target == path
}

// Ack marks the pending change for path as handled.
func (w *dbWatcher) Ack(path string) {
	w.mu.Lock()
	if w.target == path {
		w.pending = false
	}
	w.mu.Unlock()
}

func (w *dbWatcher) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			if path, ok := w.poll(); ok {
				w.app.app.QueueUpdateDraw(func() {
					w.app.externalDatabaseChange(path)
				})
			}
		}
	}
}

func (w *dbWatcher) poll() (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.target == "" {
		w.stamps = nil
		w.pending = false
		return "", false
	}
	current := stampsFor(w.target)
	if w.stamps == nil {
		w.stamps = current
		return "", false
	}
	if !sameStamps(w.stamps, current) {
		w.stamps = current
		w.pending = true
	}
	if !w.pending {
		return "", false
	}
	return w.target, true
}

func stampsFor(path string) map[string]fileStamp {
	return map[string]fileStamp{
		path:          statFile(path),
		path + "-wal": statFile(path + "-wal"),
	}
}

func sameStamps(a, b map[string]fileStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for path, stamp := range a {
		if other, ok := b[path]; !ok || other != stamp {
			return false
		}
	}
	return true
}

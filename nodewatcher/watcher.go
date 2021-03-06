package nodewatcher

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/matarc/filewatcher/log"
	"github.com/matarc/filewatcher/shared"
)

type Watcher struct {
	dir     string
	watcher *fsnotify.Watcher
	quitCh  chan struct{}
}

// NewWatcher returns a `Watcher` that lists and keeps track of all files present in
// `dir` and its subdirectories.
// It returns nil if the `fsnotify.Watcher` can't be created.
func NewWatcher(dir string) *Watcher {
	w := new(Watcher)
	w.dir = dir
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error(err)
		return nil
	}
	w.watcher = watcher
	w.quitCh = make(chan struct{})
	return w
}

// CheckDir returns nil if `dir` is a watchable directory, an error otherwise.
func (w *Watcher) CheckDir() error {
	info, err := os.Lstat(w.dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("'%s' is not a directory", w.dir)
	}
	err = w.watcher.Add(w.dir)
	if err != nil {
		return fmt.Errorf("'%s' : %s", w.dir, err)
	}
	return nil
}

// WatchDir recursively watches all files in `dir` directory and its subdirectories.
// It sends the list of all files in `dir` and its subdirectories to the `PathManager`.
func (w *Watcher) WatchDir(pathCh chan<- []shared.Operation) error {
	operations := []shared.Operation{}
	err := filepath.Walk(w.dir, func(path string, info os.FileInfo, err error) error {
		select {
		case <-w.quitCh:
			return shared.ErrQuit
		default:
		}
		if err != nil {
			log.Error(err)
			return filepath.SkipDir
		}
		newPath, err := Chroot(path, w.dir)
		if err != nil {
			log.Error(err)
			return filepath.SkipDir
		}
		operations = append(operations, shared.Operation{Path: newPath, Event: shared.Create})
		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				log.Error(err)
				return filepath.SkipDir
			}
		}
		return nil
	})
	pathCh <- operations
	return err
}

// HandleFileEvents notifies our pathmanager whenever there are new files or deleted files in the directory watched
// by the `Watcher` as well as its subdirectories.
func (w *Watcher) HandleFileEvents(pathCh chan<- []shared.Operation) {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				newPath, err := Chroot(event.Name, w.dir)
				if err != nil {
					log.Error(err)
					continue
				}
				if isDir(event.Name) {
					err = w.watcher.Add(event.Name)
					if err != nil {
						log.Error(err)
					}
				}
				pathCh <- []shared.Operation{shared.Operation{Path: newPath, Event: shared.Create}}
			}
			if event.Op&fsnotify.Remove == fsnotify.Remove ||
				event.Op&fsnotify.Rename == fsnotify.Rename {
				newPath, err := Chroot(event.Name, w.dir)
				if err != nil {
					log.Error(err)
					continue
				}
				if isDir(event.Name) {
					w.watcher.Remove(event.Name)
				}
				pathCh <- []shared.Operation{shared.Operation{Path: newPath, Event: shared.Remove}}
			}
		case <-w.quitCh:
			return
		}
	}
}

// isDir is a utility function that returns true if `path` is a directory, false otherwise.
// It follows symlinks.
func isDir(path string) bool {
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	if w.quitCh != nil {
		close(w.quitCh)
		w.quitCh = nil
	}
	w.watcher.Close()
}

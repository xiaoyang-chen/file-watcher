package watcher

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	// ErrDurationTooShort occurs when calling the watcher's Start
	// method with a duration that's less than 1 nanosecond.
	ErrDurationTooShort = errors.New("error: duration is less than 1ns")
	// ErrWatcherRunning occurs when trying to call the watcher's
	// Start method and the polling cycle is still already running
	// from previously calling Start and not yet calling Close.
	ErrWatcherRunning = errors.New("error: watcher is already running")
	// ErrWatchedFileDeleted is an error that occurs when a file or folder that was
	// being watched has been deleted.
	ErrWatchedFileDeleted = errors.New("error: watched file or folder deleted")
	// ErrSkip is less of an error, but more of a way for path hooks to skip a file or
	// directory.
	ErrSkip = errors.New("error: skipping file")
)

// An Op is a type that is used to describe what type
// of event has occurred during the watching process.
type Op uint32

// Ops
const (
	Create Op = iota
	Write
	Remove
	Rename
	Chmod
	Move
)

// String prints the string version of the Op consts
func (e Op) String() (str string) {

	switch e {
	case Create:
		str = "CREATE"
	case Write:
		str = "WRITE"
	case Remove:
		str = "REMOVE"
	case Rename:
		str = "RENAME"
	case Chmod:
		str = "CHMOD"
	case Move:
		str = "MOVE"
	default:
		str = "???"
	}
	return
}

// An Event describes an event that is received when files or directory
// changes occur. It includes the os.FileInfo of the changed file or
// directory and the type of event that's occurred and the full path of the file.
type Event struct {
	Op
	Path    string
	OldPath string
	os.FileInfo
}

// String returns a string depending on what type of event occurred and the
// file name associated with the event.
func (e Event) String() string {

	if e.FileInfo == nil {
		return "???"
	}
	var pathType = "FILE"
	if e.IsDir() {
		pathType = "DIRECTORY"
	}
	return fmt.Sprintf("%s %q %s [%s]", pathType, e.Name(), e.Op, e.Path)
}

// FilterFileHookFunc is a function that is called to filter files during listings.
// If a file is ok to be listed, nil is returned otherwise ErrSkip is returned.
type FilterFileHookFunc func(info os.FileInfo, fullPath string) error

// RegexFilterHook is a function that accepts or rejects a file
// for listing based on whether it's filename or full path matches
// a regular expression.
func RegexFilterHook(r *regexp.Regexp, useFullPath bool) FilterFileHookFunc {

	return func(info os.FileInfo, fullPath string) error {
		var str = info.Name()
		if useFullPath {
			str = fullPath
		}
		// Match
		if r.MatchString(str) {
			return nil
		}
		// No match.
		return ErrSkip
	}
}

// Watcher describes a process that watches files for changes.
type Watcher struct {
	Event  chan Event
	Error  chan error
	Closed chan struct{}
	close  chan struct{}
	wg     *sync.WaitGroup
	// mu protects the following.
	mu           *sync.Mutex
	ffh          []FilterFileHookFunc
	names        map[string]bool        // bool for recursive or not.
	files        map[string]os.FileInfo // map of files.
	ignored      map[string]struct{}    // ignored files or directories.
	ops          map[Op]bool            // Op filtering, the ops you will only get. if empty, you can get all ops, if not empty, you will only receive the ops those in this map ops
	maxEvents    int                    // max sent events per cycle, maxEvents controls the maximum amount of events that are sent on, the Event channel per watching cycle, If max events is less than 1, there is no limit, which is the default.
	ignoreHidden bool                   // ignore hidden files or not.
	running      bool
}

// New creates a new Watcher.
func New() *Watcher {

	// Set up the WaitGroup for w.Wait().
	var wg sync.WaitGroup
	wg.Add(1)
	return &Watcher{
		Event:   make(chan Event),
		Error:   make(chan error),
		Closed:  make(chan struct{}),
		close:   make(chan struct{}),
		wg:      &wg,
		ffh:     make([]FilterFileHookFunc, 0, 2),
		mu:      new(sync.Mutex),
		names:   make(map[string]bool, 4),
		files:   make(map[string]os.FileInfo, 8),
		ignored: make(map[string]struct{}, 2),
	}
}

// SetMaxEvents controls the maximum amount of events that are sent on
// the Event channel per watching cycle, If max events is less than 1, there is
// no limit, which is the default.
func (w *Watcher) SetMaxEvents(delta int) {
	w.mu.Lock()
	w.maxEvents = delta
	w.mu.Unlock()
}

func (w *Watcher) IsGreaterThanMaxEvents(numEvents int) (isGt bool) {
	w.mu.Lock()
	isGt = w.maxEvents > 0 && numEvents > w.maxEvents
	w.mu.Unlock()
	return
}

// AddFilterHook
func (w *Watcher) AddFilterHook(f FilterFileHookFunc) {
	w.mu.Lock()
	w.ffh = append(w.ffh, f)
	w.mu.Unlock()
}

// IgnoreHiddenFiles sets the watcher to ignore any file or directory
// that starts with a dot.
func (w *Watcher) IgnoreHiddenFiles(ignore bool) {
	w.mu.Lock()
	w.ignoreHidden = ignore
	w.mu.Unlock()
}

// FilterOps filters which event op types should be returned
// when an event occurs.
func (w *Watcher) FilterOps(ops ...Op) {
	w.mu.Lock()
	w.ops = make(map[Op]bool, len(ops))
	for _, op := range ops {
		w.ops[op] = true
	}
	w.mu.Unlock()
}

func (w *Watcher) IsOpSkipByFilterOps(inOp Op) (skip bool) {
	w.mu.Lock()
	skip = len(w.ops) > 0 && !w.ops[inOp]
	w.mu.Unlock()
	return
}

// Add adds either a single file or directory to the file list.
func (w *Watcher) Add(name string) (err error) {

	w.mu.Lock()
	defer w.mu.Unlock()

	if name, err = filepath.Abs(name); err != nil {
		return
	}
	// If name is on the ignored list or if hidden files are
	// ignored and name is a hidden file or directory, simply return.
	var isHidden bool
	if isHidden, err = isHiddenFile(name); err != nil {
		return
	}
	if _, ignored := w.ignored[name]; ignored || (isHidden && w.ignoreHidden) {
		return
	}
	// Add the directory's contents to the files list.
	var fileList map[string]os.FileInfo
	if fileList, err = w.list(name); err != nil {
		return
	}
	for k, v := range fileList {
		w.files[k] = v
	}
	// Add the name to the names list.
	w.names[name] = false
	return
}

// list return file or a dir with dirs and files below it but no recursive
func (w *Watcher) list(name string) (fileList map[string]os.FileInfo, err error) {

	// Make sure name exists.
	var stat os.FileInfo
	if stat, err = os.Stat(name); err != nil {
		return
	}
	fileList = make(map[string]os.FileInfo, 4)
	fileList[name] = stat
	// If it's not a directory, just return.
	if !stat.IsDir() {
		return
	}
	// It's a directory.
	var entries []fs.DirEntry
	if entries, err = os.ReadDir(name); err != nil {
		return
	}
	var fInfo fs.FileInfo
	var fInfoList = make([]fs.FileInfo, 0, len(entries))
	for _, entry := range entries {
		if fInfo, err = entry.Info(); err != nil {
			return
		}
		fInfoList = append(fInfoList, fInfo)
	}
	// Add all of the files in the directory to the file list as long
	// as they aren't on the ignored list or are hidden files if ignoreHidden
	// is set to true.
	var path string
	var isHidden bool
outer:
	for _, fInfo = range fInfoList {
		path = filepath.Join(name, fInfo.Name())
		if isHidden, err = isHiddenFile(path); err != nil {
			return
		}
		if _, ignored := w.ignored[path]; ignored || (isHidden && w.ignoreHidden) {
			continue
		}
		for _, f := range w.ffh {
			switch err = f(fInfo, path); err {
			case nil:
			case ErrSkip:
				err = nil
				continue outer
			default:
				return
			}
		}
		fileList[path] = fInfo
	}
	return
}

// AddRecursive adds either a single file or directory recursively to the file list.
func (w *Watcher) AddRecursive(name string) (err error) {

	w.mu.Lock()
	defer w.mu.Unlock()

	if name, err = filepath.Abs(name); err != nil {
		return
	}
	var fileList map[string]os.FileInfo
	if fileList, err = w.listRecursive(name); err != nil {
		return
	}
	for k, v := range fileList {
		w.files[k] = v
	}
	// Add the name to the names list.
	w.names[name] = true
	return
}

func (w *Watcher) listRecursive(name string) (fileList map[string]os.FileInfo, err error) {

	fileList = make(map[string]fs.FileInfo, 4)
	return fileList, filepath.Walk(name, func(path string, info os.FileInfo, inErr error) (err error) {
		if inErr != nil {
			err = inErr
			return
		}
		// If path is ignored and it's a directory, skip the directory. If it's
		// ignored and it's a single file, skip the file.
		var isHidden bool
		if isHidden, err = isHiddenFile(path); err != nil {
			return
		}
		if _, ignored := w.ignored[path]; ignored || (isHidden && w.ignoreHidden) {
			if info.IsDir() {
				err = filepath.SkipDir
			}
			return
		}
		// callbacks after skip ignored files
		for _, f := range w.ffh {
			switch err = f(info, path); err {
			case nil:
			case ErrSkip:
				err = nil
				return
			default:
				return
			}
		}
		// Add the path and it's info to the file list.
		// notice: if a dir skipped by w.ffh but the files below it do not, the files will add into this fileLists
		fileList[path] = info
		return
	})
}

// Remove removes either a single file or directory from the file's list.
func (w *Watcher) Remove(name string) (err error) {

	w.mu.Lock()
	err = w.remove(name)
	w.mu.Unlock()
	return
}

// remove removes either a single file or directory from the file's list but without lock
func (w *Watcher) remove(name string) (err error) {

	if name, err = filepath.Abs(name); err != nil {
		return
	}
	// Remove the name from w's names list.
	delete(w.names, name)
	// If name is a single file, remove it and return.
	var info, found = w.files[name]
	if !found {
		return // Doesn't exist, just return.
	}
	delete(w.files, name)
	if !info.IsDir() {
		return
	}
	// If it's a directory, delete all of it's contents from w.files.
	for path := range w.files {
		if filepath.Dir(path) == name {
			delete(w.files, path)
		}
	}
	return
}

// RemoveRecursive removes either a single file or a directory recursively from the file's list.
func (w *Watcher) RemoveRecursive(name string) (err error) {

	w.mu.Lock()
	err = w.removeRecursive(name)
	w.mu.Unlock()
	return
}

// removeRecursive removes either a single file or a directory recursively from the file's list but without lock
func (w *Watcher) removeRecursive(name string) (err error) {

	if name, err = filepath.Abs(name); err != nil {
		return
	}
	// Remove the name from w's names list.
	delete(w.names, name)
	// If name is a single file, remove it and return.
	var info, found = w.files[name]
	if !found {
		return // Doesn't exist, just return.
	}
	delete(w.files, name)
	if !info.IsDir() {
		return
	}
	// If it's a directory, delete all of it's contents recursively from w.files.
	for path := range w.files {
		if strings.HasPrefix(path, name) {
			delete(w.files, path)
		}
	}
	return
}

// Ignore adds paths that should be ignored.
// For files that are already added, Ignore removes them.
func (w *Watcher) Ignore(paths ...string) (err error) {

	for _, path := range paths {
		if path, err = filepath.Abs(path); err != nil {
			return
		}
		// Remove any of the paths that were already added.
		if err = w.RemoveRecursive(path); err != nil {
			return
		}
		w.mu.Lock()
		w.ignored[path] = struct{}{}
		w.mu.Unlock()
	}
	return nil
}

// WatchedFiles returns a map of files added to a Watcher.
func (w *Watcher) WatchedFiles() (files map[string]os.FileInfo) {

	w.mu.Lock()
	files = make(map[string]fs.FileInfo, len(w.files))
	for k, v := range w.files {
		files[k] = v
	}
	w.mu.Unlock()
	return
}

func (w *Watcher) GetWatchedFileInfoByPath(path string) (fileInfo os.FileInfo) {

	w.mu.Lock()
	fileInfo = w.files[path]
	w.mu.Unlock()
	return
}

// fileInfo is an implementation of os.FileInfo that can be used
// as a mocked os.FileInfo when triggering an event when the specified
// os.FileInfo is nil.
type fileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	sys     interface{}
	dir     bool
}

func (fs *fileInfo) IsDir() bool        { return fs.dir }
func (fs *fileInfo) ModTime() time.Time { return fs.modTime }
func (fs *fileInfo) Mode() os.FileMode  { return fs.mode }
func (fs *fileInfo) Name() string       { return fs.name }
func (fs *fileInfo) Size() int64        { return fs.size }
func (fs *fileInfo) Sys() interface{}   { return fs.sys }

// TriggerEvent is a method that can be used to trigger an event, separate to
// the file watching process.
func (w *Watcher) TriggerEvent(eventType Op, file os.FileInfo) {

	w.Wait()
	if file == nil {
		file = &fileInfo{name: "triggered event", modTime: time.Now()}
	}
	w.Event <- Event{Op: eventType, Path: "-", FileInfo: file}
}

func (w *Watcher) retrieveFileList() (fileList map[string]os.FileInfo) {

	w.mu.Lock()
	defer w.mu.Unlock()

	fileList = make(map[string]os.FileInfo, 4)
	var list map[string]os.FileInfo
	var err error
	for name, recursive := range w.names {
		if recursive {
			if list, err = w.listRecursive(name); err != nil {
				if os.IsNotExist(err) {
					// panic: interface conversion: error is syscall.Errno, not *fs.PathError
					if pathError, ok := err.(*os.PathError); ok && pathError.Path == name {
						w.Error <- ErrWatchedFileDeleted
						w.removeRecursive(name)
					}
				} else {
					w.Error <- err
				}
			}
		} else {
			if list, err = w.list(name); err != nil {
				if os.IsNotExist(err) {
					// panic: interface conversion: error is syscall.Errno, not *fs.PathError
					if pathError, ok := err.(*os.PathError); ok && pathError.Path == name {
						w.Error <- ErrWatchedFileDeleted
						w.remove(name)
					}
				} else {
					w.Error <- err
				}
			}
		}
		// Add the file's to the file list.
		for k, v := range list {
			fileList[k] = v
		}
	}
	return
}

// Start begins the polling cycle which repeats every specified
// duration until Close is called.
func (w *Watcher) Start(d time.Duration) (err error) {

	// Return an error if d is less than 1 nanosecond.
	if d < time.Nanosecond {
		err = ErrDurationTooShort
		return
	}
	// Make sure the Watcher is not already running.
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		err = ErrWatcherRunning
		return
	}
	w.running = true
	w.mu.Unlock()
	// Unblock w.Wait().
	w.wg.Done()
	for {
		// done lets the inner polling cycle loop know when the
		// current cycle's method has finished executing.
		var done = make(chan struct{})
		// Any events that are found are first piped to evt before
		// being sent to the main Event channel.
		var evt = make(chan Event)
		// Retrieve the file list for all watched file's and dirs.
		var fileList = w.retrieveFileList()
		// cancel can be used to cancel the current event polling function.
		var cancel = make(chan struct{})
		// Look for events.
		go func() {
			w.pollEvents(fileList, evt, cancel)
			done <- struct{}{}
		}()
		// numEvents holds the number of events for the current cycle.
		var numEvents = 0

	inner:
		for {
			select {
			case <-w.close:
				close(cancel)
				close(w.Closed)
				return
			case event := <-evt:
				if w.IsOpSkipByFilterOps(event.Op) {
					continue
				}
				numEvents++
				if w.IsGreaterThanMaxEvents(numEvents) {
					close(cancel)
					break inner
				}
				w.Event <- event
			case <-done: // Current cycle is finished.
				break inner
			}
		}

		// Update the file's list.
		w.mu.Lock()
		w.files = fileList
		w.mu.Unlock()
		// Sleep and then continue to the next loop iteration.
		time.Sleep(d)
	}
}

func (w *Watcher) pollEvents(files map[string]os.FileInfo, evt chan Event, cancel chan struct{}) {

	// Store create and remove events for use to check for rename events.
	var (
		creates = make(map[string]os.FileInfo, len(files))
		removes = make(map[string]os.FileInfo, len(files))
	)
	// Check for removed files.
	for path, info := range w.WatchedFiles() {
		if files[path] == nil {
			removes[path] = info
		}
	}
	// Check for created files, writes and chmods.
	var oldInfo os.FileInfo
	for path, info := range files {
		if oldInfo = w.GetWatchedFileInfoByPath(path); oldInfo == nil { // A file was created.
			// first scan, if file renames, will send removed event, second scan, file wrote by created, will only send created event, if time of rename and write is more than time of one scan. so it will not send write event. should we send write event when create, or just setting a bigger sleep gap between two scan?
			// now we send write event when create if file size > 0. see the code below when create events are sended
			creates[path] = info
			continue
		}
		if oldInfo.ModTime() != info.ModTime() || oldInfo.Size() != info.Size() {
			select {
			case <-cancel:
				return
			case evt <- Event{Write, path, path, info}:
			}
		}
		if oldInfo.Mode() != info.Mode() {
			select {
			case <-cancel:
				return
			case evt <- Event{Chmod, path, path, info}:
			}
		}
	}
	// Check for renames and moves.
	for path1, info1 := range removes {
		for path2, info2 := range creates {
			if sameFile(info1, info2) {
				var e = Event{Move, path2, path1, info1}
				// If they are from the same directory, it's a rename
				// instead of a move event.
				if filepath.Dir(path1) == filepath.Dir(path2) {
					e.Op = Rename
				}
				delete(removes, path1)
				delete(creates, path2)
				select {
				case <-cancel:
					return
				case evt <- e:
				}
			}
		}
	}
	// Send all the remaining create and remove events.
	for path, info := range creates {
		select {
		case <-cancel:
			return
		case evt <- Event{Create, path, "", info}:
		}
		// if file size > 0, we also send write event
		if info.Size() > 0 {
			select {
			case <-cancel:
				return
			case evt <- Event{Write, path, "", info}:
			}
		}
	}
	for path, info := range removes {
		select {
		case <-cancel:
			return
		case evt <- Event{Remove, path, path, info}:
		}
	}
}

// Wait blocks until the watcher is started.
func (w *Watcher) Wait() { w.wg.Wait() }

// Close stops a Watcher and unlocks its mutex, then sends a close signal.
func (w *Watcher) Close() {

	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.files = make(map[string]os.FileInfo)
	w.names = make(map[string]bool)
	w.mu.Unlock()
	// Send a close signal to the Start method.
	w.close <- struct{}{}
}

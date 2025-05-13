package watcher

import (
	radovskybwatcher "github.com/xiaoyang-chen/file-watcher/radovskyb-watcher"

	"github.com/fsnotify/fsnotify"
)

// wrap for adapting to [github.com/fsnotify/fsnotify](https://github.com/fsnotify/fsnotify)

type Op = fsnotify.Op

// check them with [Event.Has].
const (
	// A new pathname was created.
	Create Op = fsnotify.Create
	// The pathname was written to; this does *not* mean the write has finished,
	// and a write can be followed by more writes.
	Write Op = fsnotify.Write
	// The path was removed; any watches on it will be removed. Some "remove"
	// operations may trigger a Rename if the file is actually moved (for
	// example "remove to trash" is often a rename).
	Remove Op = fsnotify.Remove
	// The path was renamed to something else; any watches on it will be
	// removed.
	Rename Op = fsnotify.Rename
	// File attributes were changed.
	//
	// It's generally not recommended to take action on this event, as it may
	// get triggered very frequently by some software. For example, Spotlight
	// indexing on macOS, anti-virus software, backup software, etc.
	Chmod Op = fsnotify.Chmod
)

var _mapRadovskybwatcherOp = map[radovskybwatcher.Op]Op{
	radovskybwatcher.Create: Create,
	radovskybwatcher.Write:  Write,
	radovskybwatcher.Remove: Remove,
	radovskybwatcher.Rename: Rename,
	radovskybwatcher.Chmod:  Chmod,
}

type Event interface {
	// Name return the path to the file or directory.
	Name() string
	Has(op Op) bool
	String() string
	SetOp(op Op) Event
}

var _ Event = fsnotifyEventWrapper{}
var _ Event = radovskybwatcherEventWrapper{}

type fsnotifyEventWrapper struct {
	e fsnotify.Event
}

func (w fsnotifyEventWrapper) Name() string      { return w.e.Name }
func (w fsnotifyEventWrapper) String() string    { return w.e.String() }
func (w fsnotifyEventWrapper) Has(op Op) bool    { return w.e.Has(op) }
func (w fsnotifyEventWrapper) SetOp(op Op) Event { w.e.Op = op; return w }

type radovskybwatcherEventWrapper struct {
	e      radovskybwatcher.Event
	wrapOp Op // see _mapRadovskybwatcherOp
}

func (w radovskybwatcherEventWrapper) Name() string      { return w.e.Path }
func (w radovskybwatcherEventWrapper) String() string    { return w.e.String() }
func (w radovskybwatcherEventWrapper) Has(op Op) bool    { return w.wrapOp.Has(op) }
func (w radovskybwatcherEventWrapper) SetOp(op Op) Event { w.wrapOp = op; return w }

func newFsnotifyEventWrapper(e fsnotify.Event) Event { return fsnotifyEventWrapper{e: e} }

func newRadovskybwatcherEventWrapper(e radovskybwatcher.Event) (ifsEvent Event) {
	return radovskybwatcherEventWrapper{e: e, wrapOp: _mapRadovskybwatcherOp[e.Op]}
}

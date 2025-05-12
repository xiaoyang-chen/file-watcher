package watcher

import (
	"time"

	logger "github.com/xiaoyang-chen/file-watcher/logger"
	radovskybwatcher "github.com/xiaoyang-chen/file-watcher/radovskyb-watcher"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
)

type FSEventHandler interface {
	FSHandle(event Event)
}

type EventHookFunc func(etIn Event) (etOut Event, isSkip bool)

type Watcher interface {
	AddPaths(paths ...string) (err error)
	Close() (err error)
}

var _ Watcher = fsnotifyWatcherWrapper{}         // github.com/fsnotify/fsnotify
var _ Watcher = radovskybwatcherWatcherWrapper{} // https://github.com/radovskyb/watcher

type fsnotifyWatcherWrapper struct {
	logHandler logger.Logger
	eventHook  EventHookFunc
	handlers   []FSEventHandler
	watcher    *fsnotify.Watcher
}

func (w fsnotifyWatcherWrapper) AddPaths(paths ...string) (err error) {

	for _, path := range paths {
		if err = w.watcher.Add(path); err != nil {
			err = errors.WithStack(err)
			break
		}
	}
	return
}
func (w fsnotifyWatcherWrapper) Close() (err error) {

	if w.watcher != nil {
		err = errors.WithStack(w.watcher.Close())
	}
	return
}

type radovskybwatcherWatcherWrapper struct {
	logHandler   logger.Logger
	eventHook    EventHookFunc
	handlers     []FSEventHandler
	watcher      *radovskybwatcher.Watcher
	watchGap     time.Duration
	errChanStart chan error
}

func (w radovskybwatcherWatcherWrapper) AddPaths(paths ...string) (err error) {

	for _, path := range paths {
		if err = w.watcher.Add(path); err != nil {
			err = errors.WithStack(err)
			break
		}
	}
	return
}
func (w radovskybwatcherWatcherWrapper) Close() (err error) {

	if w.watcher != nil {
		w.watcher.Close()
	}
	return
}

func NewFsnotifyWatcher(logHandler logger.Logger, eventHook EventHookFunc, fsEventHandlers ...FSEventHandler) (watcher Watcher, err error) {

	if logHandler == nil {
		logHandler = logger.NewNoop()
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	var wrapper = fsnotifyWatcherWrapper{
		logHandler: logHandler,
		eventHook:  eventHook,
		handlers:   fsEventHandlers,
		watcher:    fw,
	}
	go func(wrapper fsnotifyWatcherWrapper) {
		for {
			select {
			case et, ok := <-wrapper.watcher.Events:
				if !ok {
					wrapper.logHandler.Warn("watcher event chan was closed")
					return
				}
				var etWarpper = newFsnotifyEventWrapper(et)
				wrapper.logHandler.Info("event happen ", etWarpper.String())
				if wrapper.eventHook != nil {
					var isSkip = false
					if etWarpper, isSkip = wrapper.eventHook(etWarpper); isSkip {
						continue
					}
				}
				eventTwoPartHandles(etWarpper, wrapper.handlers)
			case err, ok := <-wrapper.watcher.Errors:
				if !ok {
					wrapper.logHandler.Warn("watcher error chan was closed")
					return
				}
				wrapper.logHandler.Error(err)
			}
		}
	}(wrapper)
	watcher = wrapper
	return
}

// NewRadovskybwatcherWatcher watchGap 循环
func NewRadovskybwatcherWatcher(logHandler logger.Logger, eventHook EventHookFunc, watchGap time.Duration, fsEventHandlers ...FSEventHandler) (watcher Watcher, err error) {

	if logHandler == nil {
		logHandler = logger.NewNoop()
	}
	var wrapper = radovskybwatcherWatcherWrapper{
		logHandler:   logHandler,
		eventHook:    eventHook,
		handlers:     fsEventHandlers,
		watcher:      radovskybwatcher.New(),
		watchGap:     watchGap,
		errChanStart: make(chan error, 1),
	}
	go func(wrapper radovskybwatcherWatcherWrapper) {
		for {
			select {
			case et, ok := <-wrapper.watcher.Event:
				if !ok {
					wrapper.logHandler.Warn("watcher event chan was closed")
					return
				}
				var etWarpper = newRadovskybwatcherEventWrapper(et)
				wrapper.logHandler.Info("event happen ", etWarpper.String())
				if wrapper.eventHook != nil {
					var isSkip = false
					if etWarpper, isSkip = wrapper.eventHook(etWarpper); isSkip {
						continue
					}
				}
				eventTwoPartHandles(etWarpper, wrapper.handlers)
			case err, ok := <-wrapper.watcher.Error:
				if !ok {
					wrapper.logHandler.Warn("watcher error chan was closed")
					return
				}
				wrapper.logHandler.Error(err)
			case <-wrapper.watcher.Closed:
				wrapper.logHandler.Warn("watcher was closed")
				return
			}
		}
	}(wrapper)
	var funcStart = func(wrapper radovskybwatcherWatcherWrapper) {
		// we should not use wrapper.watcher.Wait(), because it will cause this go routine leaks if wrapper.watcher.Start returns first before wrapper.watcher.Wait()
		// wrapper.watcher.Wait(); wrapper.errChanStart <- nil
		wrapper.errChanStart <- wrapper.watcher.Start(wrapper.watchGap)
	}
	// we run twice for checking whether watcher starts success
	go funcStart(wrapper)
	go funcStart(wrapper)
	if err = <-wrapper.errChanStart; err != radovskybwatcher.ErrWatcherRunning {
		wrapper.Close() // clear resource and started golang routine
		err = errors.WithStack(err)
		return
	}
	watcher, err = wrapper, nil
	return
}

func eventTwoPartHandles(et Event, handles []FSEventHandler) {

	var l, h = 0, len(handles) - 1
	for ; l < h; l, h = l+1, h-1 {
		go handles[l].FSHandle(et)
		go handles[h].FSHandle(et)
	}
	if l == h {
		go handles[l].FSHandle(et)
	}
}

func emptyFuncForTest() {}

/**
 * Created by 2020/10/20.
 */

package filewatcher

import (
	"os"
	"sync"
	"time"
)

type FileEventType int

const (
	EventTypeModify FileEventType = iota
)

type FileEvent struct {
	FileName  string
	EventType FileEventType
}

type FileWatcher struct {
	wg          sync.WaitGroup
	mutex       sync.RWMutex
	checkPeriod int
	files       map[string]int64
	exitChan    chan struct{}
	Event       chan *FileEvent
}

func NewFileWatcher() *FileWatcher {
	fw := &FileWatcher{}
	fw.checkPeriod = 1
	fw.files = make(map[string]int64)
	fw.exitChan = make(chan struct{})
	fw.Event = make(chan *FileEvent)
	return fw
}

func (f *FileWatcher) AddFile(fileName string) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if fileName == "" {
		return
	}
	f.files[fileName] = getFileModTime(fileName)
}

func (f *FileWatcher) SetCheckPeriod(sec int) {
	f.checkPeriod = sec
}

func (f *FileWatcher) Start() {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	fileLen := len(f.files)
	f.Event = make(chan *FileEvent, fileLen)

	f.wg.Add(1)
	go f.run()
}

func (f *FileWatcher) Close() {
	f.exitChan <- struct{}{}
	f.wg.Wait()
}

func (f *FileWatcher) run() {
	tick := time.NewTicker(time.Duration(f.checkPeriod) * time.Second)

	defer func() {
		f.wg.Done()
		tick.Stop()
	}()

	for {
		select {
		case <-f.exitChan:
			return
		case <-tick.C:
			f.checkFileTime()
		}
	}
}

func (f *FileWatcher) checkFileTime() {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	for fileName, modTime := range f.files {
		newModTime := getFileModTime(fileName)
		if modTime < newModTime {
			f.files[fileName] = newModTime
			f.Event <- &FileEvent{
				FileName:  fileName,
				EventType: EventTypeModify,
			}
		}
	}
}

func getFileModTime(fileName string) int64 {
	fi, err := os.Stat(fileName)
	if err != nil {
		return 0
	}
	return fi.ModTime().Unix()
}

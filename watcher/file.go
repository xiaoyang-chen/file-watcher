package watcher

import (
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/xiaoyang-chen/zapx/log"
)

func WatchFileChange(filePath string, run func(in fsnotify.Event)) {

	go func() {

		var watcher, err = fsnotify.NewWatcher()
		if err != nil {
			log.Error("fsnotify.NewWatcher err", log.Error2Field(err))
			return
		}
		defer watcher.Close()

		var file = filepath.Clean(filePath)
		var fileDir, _ = filepath.Split(file)
		if err = watcher.Add(fileDir); err != nil {
			log.Error("watcher.Add(fileDir) err", log.String("fileDir", fileDir), log.Error2Field(err))
			return
		}

		var realConfigFile, _ = filepath.EvalSymlinks(filePath)
		var currentConfigFile string
		var event fsnotify.Event
		var isOpen bool
		const writeOrCreateMask = fsnotify.Write | fsnotify.Create
		for {
			select {
			case event, isOpen = <-watcher.Events:
				if !isOpen { // 'Events' channel is closed
					log.Error("watch file change end, event, isOpen = <-watcher.Events, this channel is closed")
					return
				}
				currentConfigFile, _ = filepath.EvalSymlinks(filePath)
				// we only care about the file with the following cases:
				// 1 - if the file was modified or created
				// 2 - if the real path to the file changed (eg: k8s ConfigMap replacement)
				if (filepath.Clean(event.Name) == file &&
					event.Op&writeOrCreateMask != 0) ||
					(currentConfigFile != "" &&
						currentConfigFile != realConfigFile) {
					realConfigFile = currentConfigFile
					run(event)
				} else if filepath.Clean(event.Name) == file &&
					event.Op&fsnotify.Remove != 0 {
					log.Error("watch file change end, file is removed!!!")
					return
				} /*else {
					log.Error("file change but unknown change type")
				}*/
			case err, isOpen = <-watcher.Errors:
				log.Error("watch file change end, err, isOpen = <-watcher.Errors", log.NamedError("err", err), log.Bool("isOpen", isOpen))
				return
			}
		}
	}()
}

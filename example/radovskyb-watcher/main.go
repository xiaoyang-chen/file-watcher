// export GOOS=linux GOARCH=amd64 && go build -trimpath -ldflags="-s -w" -o ./check-file-watch github.com/xiaoyang-chen/file-watcher/example/radovskyb-watcher
// export watch_path="." && ./check-file-watch
package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xiaoyang-chen/file-watcher/logger"
	"github.com/xiaoyang-chen/file-watcher/watcher"
)

func main() {

	fmt.Println("main start")
	defer fmt.Println("main end")
	// add pprof
	var pprofAddr = fmt.Sprintf(":%d", 7780)
	go func() { fmt.Println(http.ListenAndServe(pprofAddr, nil)) }()
	fmt.Printf("pprof listen %s\n", pprofAddr)
	// set env
	const _envKeyWatchPath = "watch_path"
	var _watchPath = "."
	// set _configFilePath
	if path, exist := os.LookupEnv(_envKeyWatchPath); exist {
		_watchPath = path
	}
	// set watcher
	// fsnotifywatcher
	var fsnotifywatcher, err = watcher.NewFsnotifyWatcher(logger.NewStdLog(), func(etIn watcher.Event) (etOut watcher.Event, isSkip bool) {
		fmt.Println("fsnotifywatcher", etIn.String())
		return etIn, true
	})
	if err != nil {
		panic(err)
	}
	defer fsnotifywatcher.Close()
	fsnotifywatcher.AddPaths(_watchPath)
	// radovskybwatcher
	radovskybwatcher, err := watcher.NewRadovskybwatcherWatcher(logger.NewStdLog(), func(etIn watcher.Event) (etOut watcher.Event, isSkip bool) {
		fmt.Println("radovskybwatcher", etIn.String())
		return etIn, true
	}, time.Second)
	if err != nil {
		panic(err)
	}
	defer radovskybwatcher.Close()
	radovskybwatcher.AddPaths(_watchPath)
	// notify os signal
	var exitSign = make(chan os.Signal, 1)
	// go func() {
	// 	time.Sleep(5 * time.Second)
	// 	exitSign <- syscall.SIGINT
	// }()
	signal.Notify(exitSign,
		syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT,
	)
	<-exitSign
}

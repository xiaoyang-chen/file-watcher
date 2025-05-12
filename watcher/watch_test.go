package watcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/xiaoyang-chen/file-watcher/logger"
)

func Test_emptyFuncForTest(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "fsnotifywatcher"},
		{name: "radovskybwatcher"},
	}
	// set watcher
	fmt.Println("Test_emptyFuncForTest start")
	// fsnotifywatcher
	var fsnotifywatcher, err = NewFsnotifyWatcher(logger.NewStdLog(), func(etIn Event) (etOut Event, isSkip bool) {
		fmt.Println("fsnotifywatcher", etIn.String())
		return etIn, true
	})
	if err != nil {
		panic(err)
	}
	defer fsnotifywatcher.Close()
	fsnotifywatcher.AddPaths(".")
	// radovskybwatcher
	radovskybwatcher, err := NewRadovskybwatcherWatcher(logger.NewStdLog(), func(etIn Event) (etOut Event, isSkip bool) {
		fmt.Println("radovskybwatcher", etIn.String())
		return etIn, true
	}, time.Second)
	if err != nil {
		panic(err)
	}
	defer radovskybwatcher.Close()
	radovskybwatcher.AddPaths(".")
	time.Sleep(20 * time.Second)
	fmt.Println("Test_emptyFuncForTest end")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emptyFuncForTest()
		})
	}
}

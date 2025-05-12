package logger

import (
	"fmt"
	"log"
	"os"
	"unsafe"
)

const _stdLogCalldepth = 2

type Logger interface {
	Errorf(format string, args ...interface{})
	Error(args ...interface{})
	Warnf(format string, args ...interface{})
	Warn(args ...interface{})
	Infof(format string, args ...interface{})
	Info(args ...interface{})
	Debugf(format string, args ...interface{})
	Debug(args ...interface{})
	WithFields(fields map[string]any) Logger
}

var _ Logger = stNoop{}
var _ Logger = stStdLog{}

type stNoop struct{}

func (null stNoop) Errorf(format string, args ...interface{}) {}
func (null stNoop) Error(args ...interface{})                 {}
func (null stNoop) Warnf(format string, args ...interface{})  {}
func (null stNoop) Warn(args ...interface{})                  {}
func (null stNoop) Infof(format string, args ...interface{})  {}
func (null stNoop) Info(args ...interface{})                  {}
func (null stNoop) Debugf(format string, args ...interface{}) {}
func (null stNoop) Debug(args ...interface{})                 {}
func (null stNoop) WithFields(fields map[string]any) Logger   { return null }

type stStdLog struct{ log *log.Logger }

func (stdLog stStdLog) Errorf(format string, args ...interface{}) {
	stdLog.log.Output(_stdLogCalldepth, fmt.Sprintf(concatStrings(`"level":"ERROR","service_name":"github.com/xiaoyang-chen/file-watcher",`, format), args...))
}
func (stdLog stStdLog) Error(args ...interface{}) {
	stdLog.log.Output(_stdLogCalldepth, concatStrings(`"level":"ERROR","service_name":"github.com/xiaoyang-chen/file-watcher",`, fmt.Sprint(args...)))
}
func (stdLog stStdLog) Warnf(format string, args ...interface{}) {
	stdLog.log.Output(_stdLogCalldepth, fmt.Sprintf(concatStrings(`"level":"WARN","service_name":"github.com/xiaoyang-chen/file-watcher",`, format), args...))
}
func (stdLog stStdLog) Warn(args ...interface{}) {
	stdLog.log.Output(_stdLogCalldepth, concatStrings(`"level":"WARN","service_name":"github.com/xiaoyang-chen/file-watcher",`, fmt.Sprint(args...)))
}
func (stdLog stStdLog) Infof(format string, args ...interface{}) {
	stdLog.log.Output(_stdLogCalldepth, fmt.Sprintf(concatStrings(`"level":"INFO","service_name":"github.com/xiaoyang-chen/file-watcher",`, format), args...))
}
func (stdLog stStdLog) Info(args ...interface{}) {
	stdLog.log.Output(_stdLogCalldepth, concatStrings(`"level":"INFO","service_name":"github.com/xiaoyang-chen/file-watcher",`, fmt.Sprint(args...)))
}
func (stdLog stStdLog) Debugf(format string, args ...interface{}) {
	stdLog.log.Output(_stdLogCalldepth, fmt.Sprintf(concatStrings(`"level":"DEBUG","service_name":"github.com/xiaoyang-chen/file-watcher",`, format), args...))
}
func (stdLog stStdLog) Debug(args ...interface{}) {
	stdLog.log.Output(_stdLogCalldepth, concatStrings(`"level":"DEBUG","service_name":"github.com/xiaoyang-chen/file-watcher",`, fmt.Sprint(args...)))
}
func (stdLog stStdLog) WithFields(fields map[string]any) Logger { return stdLog }

func NewNoop() Logger { return stNoop{} }

// NewStdLog this log does not have WithFields method and it will output all logs regardless your log level
func NewStdLog() Logger {
	return stStdLog{log: log.New(os.Stderr, "", log.LstdFlags)}
}

func concatStrings(ss ...string) string {

	var length = len(ss)
	if length == 0 {
		return ""
	}
	var i, n = 0, 0
	for i = 0; i < length; i++ {
		n += len(ss[i])
	}
	var b = make([]byte, 0, n)
	for i = 0; i < length; i++ {
		b = append(b, ss[i]...)
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

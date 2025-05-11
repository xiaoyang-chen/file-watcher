package logger

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

func NewNoop() Logger { return stNoop{} }

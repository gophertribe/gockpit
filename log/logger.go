package log

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/fatih/color"
)

var (
	red   = color.New(color.FgRed).SprintFunc()
	green = color.New(color.FgGreen).SprintFunc()
	white = color.New(color.FgHiWhite).SprintFunc()
)

func init() {
	color.NoColor = false
}

type LeveledLogger struct {
	out    map[Level]*log.Logger
	writer io.Writer
}

func (l *LeveledLogger) SetError(_ context.Context, ns, code string, err error) {
	l.Errorf("%s|%s: %s", ns, code, err.Error())
}

func (l *LeveledLogger) ClearError(_ context.Context, ns, code string, _ error) {
	l.Infof("%s|%s: clear error", ns, code)
}

func NewLeveledLogger(writer io.Writer) *LeveledLogger {
	return &LeveledLogger{
		writer: writer,
		out: map[Level]*log.Logger{
			// debug is disabled by default
			LevelDebug: log.New(io.Discard, white("DBG "), log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile),
			LevelInfo:  log.New(writer, green("INF "), log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile),
			LevelError: log.New(writer, red("ERR "), log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile),
		},
	}
}

func (l *LeveledLogger) SetDebug(enable bool) {
	if enable {
		l.out[LevelDebug].SetOutput(l.writer)
		return
	}
	l.out[LevelDebug].SetOutput(io.Discard)
}

func (l *LeveledLogger) Error(msg string) {
	l.log(LevelError, msg)
}

func (l *LeveledLogger) Errorf(msg string, args ...interface{}) {
	l.logf(LevelError, msg, args...)
}

func (l *LeveledLogger) Info(msg string) {
	l.log(LevelInfo, msg)
}

func (l *LeveledLogger) Infof(msg string, args ...interface{}) {
	l.logf(LevelInfo, msg, args...)
}

func (l *LeveledLogger) Debug(msg string) {
	l.log(LevelDebug, msg)
}

func (l *LeveledLogger) Debugf(msg string, args ...interface{}) {
	l.logf(LevelDebug, msg, args...)
}

func (l *LeveledLogger) log(lvl Level, msg string) {
	err := l.out[lvl].Output(3, msg)
	if err != nil {
		fmt.Printf("fatal: could not output logs: %v\n", err)
	}
}

func (l *LeveledLogger) logf(lvl Level, msg string, args ...interface{}) {
	err := l.out[lvl].Output(3, fmt.Sprintf(msg, args...))
	if err != nil {
		fmt.Printf("fatal: could not output logs: %v\n", err)
	}
}

type NamespaceLogger struct {
	*LeveledLogger
	ns string
}

func NewNamespaceLogger(writer io.Writer, namespace string) *NamespaceLogger {
	return &NamespaceLogger{
		LeveledLogger: NewLeveledLogger(writer),
		ns:            namespace,
	}
}

func (l *NamespaceLogger) log(lvl Level, msg string) {
	l.LeveledLogger.log(lvl, fmt.Sprintf("|%s| %s", l.ns, msg))
}

func (l *NamespaceLogger) logf(lvl Level, msg string, args ...interface{}) {
	l.LeveledLogger.log(lvl, fmt.Sprintf("|%s| %s", l.ns, fmt.Sprintf(msg, args...)))
}

func (l *NamespaceLogger) Error(msg string) {
	l.log(LevelError, msg)
}

func (l *NamespaceLogger) Errorf(msg string, args ...interface{}) {
	l.logf(LevelError, msg, args...)
}

func (l *NamespaceLogger) Info(msg string) {
	l.log(LevelInfo, msg)
}

func (l *NamespaceLogger) Infof(msg string, args ...interface{}) {
	l.logf(LevelInfo, msg, args...)
}

func (l *NamespaceLogger) Debug(msg string) {
	l.log(LevelDebug, msg)
}

func (l *NamespaceLogger) Debugf(msg string, args ...interface{}) {
	l.logf(LevelDebug, msg, args...)
}

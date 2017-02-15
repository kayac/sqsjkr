package sqsjkr

import (
	"fmt"
	"log"
	"os"
)

// LogLevel type
type LogLevel int

// Log level const
const (
	ErrorLevel LogLevel = iota
	WarnLevel
	InfoLevel
	DebugLevel
)

// Logger is sqsjkr logger struct
type Logger struct {
	Logger *log.Logger
	Level  LogLevel
}

// Errorf output error log
func (l Logger) Errorf(format string, args ...interface{}) {
	l.output(fmt.Sprint("[error] ", format), args...)
}

// Warnf output warning log
func (l Logger) Warnf(format string, args ...interface{}) {
	if l.Level > ErrorLevel {
		l.output(fmt.Sprint("[warn] ", format), args...)
	}
}

// Infof output information log
func (l Logger) Infof(format string, args ...interface{}) {
	if l.Level > WarnLevel {
		l.output(fmt.Sprint("[info] ", format), args...)
	}
}

// Debugf output for debug
func (l Logger) Debugf(format string, args ...interface{}) {
	if l.Level > InfoLevel {
		l.output(fmt.Sprint("[debug] ", format), args...)
	}
}

func (l Logger) output(str string, args ...interface{}) {
	if args != nil {
		l.Logger.Output(3, fmt.Sprintf(str, args...))
	} else {
		l.Logger.Output(3, str)
	}
}

// SetLevel set a logger level
func (l *Logger) SetLevel(level string) {
	switch level {
	case "error":
		l.Level = ErrorLevel
	case "warn":
		l.Level = WarnLevel
	case "info":
		l.Level = InfoLevel
	case "debug":
		l.Level = DebugLevel
	default:
		l.Level = InfoLevel
	}
}

// NewLogger returns Logger struct
func NewLogger() Logger {
	lgg := log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)
	return Logger{
		Logger: lgg,
	}
}

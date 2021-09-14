package clog

import (
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/lineprefix"
	"log"
	"time"
)

type Logger struct {
	*log.Logger
}

func defaultOpt() lineprefix.Option {
	return lineprefix.PrefixFunc(func() string {
		const timeFormat = "15:04:05"
		return fmt.Sprintf("[%s] %s", config.AppName(), time.Now().Format(timeFormat))
	})
}

// New creates a new logger.
func New() *Logger {
	writer := lineprefix.New(defaultOpt())
	return &Logger{Logger: log.New(writer, "", 0)}
}

// For creates a new logger for s.
func For(s string) *Logger {
	writer := lineprefix.New(defaultOpt(), lineprefix.Prefix(s+":"))
	return &Logger{Logger: log.New(writer, "", 0)}
}

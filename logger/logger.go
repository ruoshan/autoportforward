package logger

import (
	"io"
	"log"
	"os"
)

var logger *log.Logger
var nullLogger *log.Logger

func GetLogger() *log.Logger {
	if logger == nil {
		f, _ := os.OpenFile("/tmp/autoportforward.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger = log.New(f, "", log.LstdFlags|log.Lshortfile)
	}
	return logger
}

func GetNullLogger() *log.Logger {
	if nullLogger == nil {
		nullLogger = log.New(io.Discard, "", 0)
	}
	return nullLogger
}

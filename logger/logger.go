// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package logger

import (
	log "code.google.com/p/log4go"
	"os"
	"path/filepath"
)

type Logger interface {
	Info(m interface{}, args ...interface{})
	Error(m interface{}, args ...interface{}) error
	Debug(m interface{}, args ...interface{})
}

var Global Logger

func init() {
	Global = log.Global
}

func SetupLogging(loggingLevel, logFile string) {
	level := log.DEBUG
	switch loggingLevel {
	case "info":
		level = log.INFO
	case "warn":
		level = log.WARNING
	case "error":
		level = log.ERROR
	}

	log.Global = make(map[string]*log.Filter)
	Global = log.Global
	if logFile == "stdout" || logFile == "" {
		flw := log.NewConsoleLogWriter()
		log.AddFilter("stdout", level, flw)
	} else if logFile == "stderr" || logFile == "" {
		flw := log.NewConsoleLogWriter()
		log.AddFilter("stderr", level, flw)
	} else {
		logFileDir := filepath.Dir(logFile)
		os.MkdirAll(logFileDir, 0744)

		flw := log.NewFileLogWriter(logFile, false)
		log.AddFilter("file", level, flw)

		flw.SetFormat("[%D %T] [%L] (%S) %M")
		flw.SetRotate(true)
		flw.SetRotateSize(0)
		flw.SetRotateLines(0)
		flw.SetRotateDaily(true)
	}

	Global.Info("Redirectoring logging to %s %s", logFile, level)
}

func Info(message string, args ...interface{}) {
	Global.Info(message, args...)
}

func Error(message string, args ...interface{}) {
	Global.Error(message, args...)
}

func Debug(message string, args ...interface{}) {
	Global.Debug(message, args...)
}

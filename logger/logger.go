// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package logger

import (
	log "code.google.com/p/log4go"
	"os"
	"path/filepath"
)

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
	if logFile == "stdout" || logFile == "" {
		flw := log.NewConsoleLogWriter()
		log.AddFilter("stdout", level, flw)
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

	log.Info("Redirectoring logging to %s", logFile)
}

func Info(message string, args ...interface{}) {
	log.Info(message, args...)
}

func Error(message string, args ...interface{}) {
	log.Error(message, args...)
}

func Debug(message string, args ...interface{}) {
	log.Debug(message, args...)
}

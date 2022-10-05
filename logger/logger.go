/*
Copyright © 2022 Dyego Alexandre Eugenio dyegoe@gmail.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package logger

import (
	"log"
	"os"
)

// awssLog is a logger for awss
type awssLog struct {
	Warning *log.Logger
	Info    *log.Logger
	Debug   *log.Logger
	Error   *log.Logger
	Fatal   *log.Logger
}

// Warningf logs a warning message
func (l *awssLog) Warningf(format string, v ...interface{}) {
	l.Warning.Printf(format, v...)
}

// Infof logs an info message
func (l *awssLog) Infof(format string, v ...interface{}) {
	l.Info.Printf(format, v...)
}

// Debugf logs a debug message
func (l *awssLog) Debugf(format string, v ...interface{}) {
	l.Info.Printf(format, v...)
}

// Errorf logs an error message
func (l *awssLog) Errorf(format string, v ...interface{}) {
	l.Error.Printf(format, v...)
}

// Fatalf logs a fatal message
func (l *awssLog) Fatalf(format string, v ...interface{}) {
	l.Error.Printf(format, v...)
	os.Exit(1)
}

// NewLog creates a new logger
func NewLog() *awssLog {
	return &awssLog{
		Warning: log.New(os.Stdout, "[WARNING] ", log.Ldate|log.Ltime),
		Info:    log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime),
		Debug:   log.New(os.Stdout, "[DEBUG] ", log.Ldate|log.Ltime),
		Error:   log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime),
		Fatal:   log.New(os.Stderr, "[FATAL] ", log.Ldate|log.Ltime),
	}
}

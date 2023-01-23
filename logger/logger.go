/*
Copyright © 2022 Dyego Alexandre Eugenio github@dyego.com.br

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

// Package logger contains the logger wrapper. It uses zerolog.
package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
)

// DefaultOutput is the default output for the logger.
var DefaultOutput = os.Stdout

// Logger is a wrapper for zerolog.Logger.
type Logger struct {
	zerolog.Logger
}

// NewLogger returns a new Logger.
//
// The output might be any io.Writer, like os.Stdout or os.Stderr.
// The fields are pairs of key and value to be added to the logger as string field.
func NewLogger(output io.Writer, fields map[string]string) *Logger {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: output}).With().Timestamp().Logger()
	for k, v := range fields {
		logger = logger.With().Str(k, v).Logger()
	}
	return &Logger{logger}
}

// SetLogLevel sets the log level.
//
// The level is the log level. It can be one of the following:
// debug, info, warn, error, fatal, panic, disabled.
func SetLogLevel(level string) error {
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "disabled":
		zerolog.SetGlobalLevel(zerolog.Disabled)
	default:
		return InvalidLogLevelError{level}
	}
	return nil
}

// InvalidLogLevelError represents an error if the log level is invalid
type InvalidLogLevelError struct {
	level string
}

func (e InvalidLogLevelError) Error() string {
	return fmt.Sprintf("invalid log level: %s", e.level)
}

// AddFields adds a pair of fields to the logger.
//
// The fields are pairs of key and value to be added to the logger as string field.
func (l *Logger) AddFields(fields map[string]string) {
	for k, v := range fields {
		l.Logger = l.Logger.With().Str(k, v).Logger()
	}
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string) {
	l.Logger.Debug().Msg(msg)
}

// Debugf logs a debug message.
func (l *Logger) Debugf(msg string, args ...interface{}) {
	l.Logger.Debug().Msgf(msg, args...)
}

// Info logs an info message.
func (l *Logger) Info(msg string) {
	l.Logger.Info().Msg(msg)
}

// Infof logs an info message.
func (l *Logger) Infof(msg string, args ...interface{}) {
	l.Logger.Info().Msgf(msg, args...)
}

// Warn logs a warn message.
func (l *Logger) Warn(msg string, err error) {
	l.Logger.Warn().Err(err).Msg(msg)
}

// Warnf logs a warn message.
func (l *Logger) Warnf(msg string, err error, args ...interface{}) {
	l.Logger.Warn().Err(err).Msgf(msg, args...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, err error) {
	l.Logger.Error().Err(err).Msg(msg)
}

// Errorf logs an error message.
func (l *Logger) Errorf(msg string, err error, args ...interface{}) {
	l.Logger.Error().Err(err).Msgf(msg, args...)
}

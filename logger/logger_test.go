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
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr bool
	}{
		{"debug", "debug", false},
		{"info", "info", false},
		{"warn", "warn", false},
		{"error", "error", false},
		{"disabled", "disabled", false},
		{"invalid", "invalid", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SetLogLevel(tt.level); (err != nil) != tt.wantErr {
				t.Errorf("SetLogLevel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLogger_Debug(t *testing.T) {
	if err := SetLogLevel("debug"); err != nil {
		t.Errorf("error during SetLogLevel() = %v", err)
	}
	output := bytes.Buffer{}
	log := NewLogger(&output, map[string]string{"key": "value"})
	log.Debug("test")
	//nolint:lll
	want := fmt.Sprintf("\x1b[90m%s\x1b[0m \x1b[33mDBG\x1b[0m test \x1b[36mkey=\x1b[0mvalue\n", time.Now().Format(time.Kitchen))
	if got := output.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLogger_Debugf(t *testing.T) {
	if err := SetLogLevel("debug"); err != nil {
		t.Errorf("error during SetLogLevel() = %v", err)
	}
	output := bytes.Buffer{}
	log := NewLogger(&output, map[string]string{"key": "value"})
	log.Debugf("test %s", "format")
	//nolint:lll
	want := fmt.Sprintf("\x1b[90m%s\x1b[0m \x1b[33mDBG\x1b[0m test format \x1b[36mkey=\x1b[0mvalue\n", time.Now().Format(time.Kitchen))
	if got := output.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLogger_Info(t *testing.T) {
	if err := SetLogLevel("info"); err != nil {
		t.Errorf("error during SetLogLevel() = %v", err)
	}
	output := bytes.Buffer{}
	log := NewLogger(&output, map[string]string{"key": "value"})
	log.Info("test")
	//nolint:lll
	want := fmt.Sprintf("\x1b[90m%s\x1b[0m \x1b[32mINF\x1b[0m test \x1b[36mkey=\x1b[0mvalue\n", time.Now().Format(time.Kitchen))
	if got := output.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLogger_Infof(t *testing.T) {
	if err := SetLogLevel("info"); err != nil {
		t.Errorf("error during SetLogLevel() = %v", err)
	}
	output := bytes.Buffer{}
	log := NewLogger(&output, map[string]string{"key": "value"})
	log.Infof("test %s", "format")
	//nolint:lll
	want := fmt.Sprintf("\x1b[90m%s\x1b[0m \x1b[32mINF\x1b[0m test format \x1b[36mkey=\x1b[0mvalue\n", time.Now().Format(time.Kitchen))
	if got := output.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLogger_Warn(t *testing.T) {
	if err := SetLogLevel("warn"); err != nil {
		t.Errorf("error during SetLogLevel() = %v", err)
	}
	output := bytes.Buffer{}
	log := NewLogger(&output, map[string]string{"key": "value"})
	log.Warn("test", fmt.Errorf("error"))
	//nolint:lll
	want := fmt.Sprintf("\x1b[90m%s\x1b[0m \x1b[31mWRN\x1b[0m test \x1b[36merror=\x1b[0m\x1b[31merror\x1b[0m \x1b[36mkey=\x1b[0mvalue\n", time.Now().Format(time.Kitchen))
	if got := output.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLogger_Warnf(t *testing.T) {
	if err := SetLogLevel("warn"); err != nil {
		t.Errorf("error during SetLogLevel() = %v", err)
	}
	output := bytes.Buffer{}
	log := NewLogger(&output, map[string]string{"key": "value"})
	log.Warnf("test %s", fmt.Errorf("error"), "format")
	//nolint:lll
	want := fmt.Sprintf("\x1b[90m%s\x1b[0m \x1b[31mWRN\x1b[0m test format \x1b[36merror=\x1b[0m\x1b[31merror\x1b[0m \x1b[36mkey=\x1b[0mvalue\n", time.Now().Format(time.Kitchen))
	if got := output.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLogger_Error(t *testing.T) {
	if err := SetLogLevel("error"); err != nil {
		t.Errorf("error during SetLogLevel() = %v", err)
	}
	output := bytes.Buffer{}
	log := NewLogger(&output, map[string]string{"key": "value"})
	log.Error("test", fmt.Errorf("error"))
	//nolint:lll
	want := fmt.Sprintf("\x1b[90m%s\x1b[0m \x1b[1m\x1b[31mERR\x1b[0m\x1b[0m test \x1b[36merror=\x1b[0m\x1b[31merror\x1b[0m \x1b[36mkey=\x1b[0mvalue\n", time.Now().Format(time.Kitchen))
	if got := output.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLogger_Errorf(t *testing.T) {
	if err := SetLogLevel("error"); err != nil {
		t.Errorf("error during SetLogLevel() = %v", err)
	}
	output := bytes.Buffer{}
	log := NewLogger(&output, map[string]string{"key": "value"})
	log.Errorf("test %s", fmt.Errorf("error"), "format")
	//nolint:lll
	want := fmt.Sprintf("\x1b[90m%s\x1b[0m \x1b[1m\x1b[31mERR\x1b[0m\x1b[0m test format \x1b[36merror=\x1b[0m\x1b[31merror\x1b[0m \x1b[36mkey=\x1b[0mvalue\n", time.Now().Format(time.Kitchen))
	if got := output.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

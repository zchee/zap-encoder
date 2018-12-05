// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stackdriver

import (
	"runtime"
	"strconv"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	sourceKey = "logging.googleapis.com/sourceLocation"
)

// SourceLocation additional information about the source code location that produced the log entry.
//
//  https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#logentrysourcelocation
type SourceLocation struct {
	// Optional. Source file name. Depending on the runtime environment, this might
	// be a simple name or a fully-qualified name.
	File string `json:"file"`

	// Optional. Line within the source file. 1-based; 0 indicates no line number
	// available.
	Line string `json:"line"`

	// Optional. Human-readable name of the function or method being invoked, with
	// optional context such as the class or package name. This information may be
	// used in contexts such as the logs viewer, where a file and line number are less
	// meaningful.
	//
	// The format should be dir/package.func.
	Function string `json:"function"`
}

// Clone implements zapcore.Encoder.
func (sl *SourceLocation) Clone() *SourceLocation {
	return &SourceLocation{
		File:     sl.File,
		Line:     sl.Line,
		Function: sl.Function,
	}
}

// MarshalLogObject implements zapcore.ObjectMarshaler.
func (sl SourceLocation) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("file", sl.File)
	enc.AddString("line", sl.Line)
	enc.AddString("function", sl.Function)

	return nil
}

// LogSourceLocation adds the correct Stackdriver "SourceLocation" field.
func LogSourceLocation(pc uintptr, file string, line int, ok bool) zap.Field {
	return zap.Object(sourceKey, NewSourceLocation(pc, file, line, ok))
}

// NewSourceLocation returns a new SourceLocation struct, based on
// the pc, file, line and ok arguments.
func NewSourceLocation(pc uintptr, file string, line int, ok bool) *SourceLocation {
	if !ok {
		return nil
	}

	sl := &SourceLocation{
		File: file,
		Line: strconv.Itoa(line),
	}
	if fn := runtime.FuncForPC(pc); fn != nil {
		sl.Function = fn.Name()
	}

	return sl
}

// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stackdriver

import (
	"context"
	"errors"
	"fmt"
	"time"

	sdlogging "cloud.google.com/go/logging"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

type Encoder struct {
	lg *sdlogging.Logger
	zapcore.Encoder
}

func NewStackdriverEncoder(ctx context.Context, projectID, logID string) zapcore.Encoder {
	client, err := sdlogging.NewClient(ctx, projectID)
	if err != nil {
		panic(fmt.Errorf("failed to create logging client: %+v", err))
	}
	client.OnError = func(error) {}

	ctxFn := func() (context.Context, func()) {
		ctx, span := trace.StartSpan(ctx, "this span will not be exported", trace.WithSampler(trace.NeverSample()))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		afterCallFn := func() {
			span.End()
			cancel()
		}
		return ctx, afterCallFn
	}

	return &Encoder{
		lg:      client.Logger(logID, sdlogging.ContextFunc(ctxFn)),
		Encoder: zapcore.NewJSONEncoder(NewStackdriverEncoderConfig()),
	}
}

// NewStackdriverConfig ...
func NewStackdriverConfig() zap.Config {
	return zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "stackdriver",
		EncoderConfig:    NewStackdriverEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stdout"},
	}
}

func NewStackdriverEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:       "eventTime",
		LevelKey:      "severity",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "message",
		StacktraceKey: "trace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel: func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
			switch l {
			case zapcore.DebugLevel:
				enc.AppendString("DEBUG")
			case zapcore.InfoLevel:
				enc.AppendString("INFO")
			case zapcore.WarnLevel:
				enc.AppendString("WARNING")
			case zapcore.ErrorLevel:
				enc.AppendString("ERROR")
			case zapcore.DPanicLevel:
				enc.AppendString("CRITICAL")
			case zapcore.PanicLevel:
				enc.AppendString("ALERT")
			case zapcore.FatalLevel:
				enc.AppendString("EMERGENCY")
			}
		},
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

func (e *Encoder) Clone() zapcore.Encoder {
	return &Encoder{
		lg:      e.lg,
		Encoder: e.Encoder.Clone(),
	}
}

func (Encoder) parseLevel(l zapcore.Level) (sev sdlogging.Severity) {
	switch l {
	case zapcore.DebugLevel:
		sev = sdlogging.Debug
	case zapcore.InfoLevel:
		sev = sdlogging.Info
	case zapcore.WarnLevel:
		sev = sdlogging.Warning
	case zapcore.ErrorLevel:
		sev = sdlogging.Error
	case zapcore.DPanicLevel:
		sev = sdlogging.Critical
	case zapcore.PanicLevel:
		sev = sdlogging.Alert
	case zapcore.FatalLevel:
		sev = sdlogging.Emergency
	default:
		sev = sdlogging.Default
	}

	return sev
}

func (e *Encoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	if ent.Caller.Defined {
		for _, f := range fields {
			if f.Key == "error" && f.Type == zapcore.ErrorType {
				ent.Message = ent.Message + "\ndue to error: " + f.Interface.(error).Error()
			}
		}
	}

	if ent.Caller.Defined {
		fields = append(fields, zap.Object("sourceLocation", reportLocation{
			File: ent.Caller.File,
			Line: ent.Caller.Line,
		}))
		ent.Caller.Defined = false
	}

	if ent.Stack != "" {
		ent.Message = ent.Message + "\n" + ent.Stack
		ent.Stack = ""
	}

	buf, err := e.Encoder.EncodeEntry(ent, fields)

	entry := sdlogging.Entry{
		Timestamp: ent.Time,
		Payload:   buf.String(),
		Severity:  e.parseLevel(ent.Level),
	}
	e.lg.Log(entry)

	return buf, err
}

type ServiceContext struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

// MarshalLogObject implements zapcore ObjectMarshaler.
func (sc ServiceContext) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	if sc.Service == "" {
		return errors.New("service name is mandatory")
	}
	enc.AddString("service", sc.Service)
	enc.AddString("version", sc.Version)

	return nil
}

// LINK: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest
type HTTPRequest struct {
	Method    string
	URL       string
	UserAgent string
	Referrer  string
	Status    int
	RemoteIP  string
	Latency   time.Duration
}

// MarshalLogObject implements zapcore ObjectMarshaler.
func (req HTTPRequest) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("requestMethod", req.Method)
	enc.AddString("requestUrl", req.URL)
	enc.AddString("userAgent", req.UserAgent)
	enc.AddString("referrer", req.Referrer)
	enc.AddString("remoteIp", req.RemoteIP)
	enc.AddInt("status", req.Status)
	enc.AddString("latency", fmt.Sprintf("%gs", req.Latency.Seconds()))

	return nil
}

type reportLocation struct {
	File     string
	Line     int
	Function string
}

// MarshalLogObject implements zapcore ObjectMarshaler.
func (rl reportLocation) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("file", rl.File)
	enc.AddInt("line", rl.Line)
	if rl.Function != "" {
		enc.AddString("function", rl.Function)
	}

	return nil
}

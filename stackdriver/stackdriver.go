// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stackdriver

import (
	"context"
	"fmt"
	"runtime"
	"time"

	sdlogging "cloud.google.com/go/logging"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

const (
	keyServiceContext        = "serviceContext"
	keyContext               = "context"
	keyContextHTTPRequest    = "context.httpRequest"
	keyContextUser           = "context.user"
	keyContextReportLocation = "context.reportLocation"
)

type Encoder struct {
	lg                *sdlogging.Logger
	SetReportLocation bool
	ctx               *Context

	zapcore.Encoder
	*zapcore.EncoderConfig
}

// NewStackdriverEncoder returns the stackdriver zapcore.Encoder.
func NewStackdriverEncoder(ctx context.Context, encoderConfig zapcore.EncoderConfig, projectID, logID string) zapcore.Encoder {
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
		lg:            client.Logger(logID, sdlogging.ContextFunc(ctxFn)),
		Encoder:       zapcore.NewJSONEncoder(encoderConfig),
		EncoderConfig: &encoderConfig,
	}
}

var logLevelSeverity = map[zapcore.Level]string{
	zapcore.DebugLevel:  "DEBUG",
	zapcore.InfoLevel:   "INFO",
	zapcore.WarnLevel:   "WARNING",
	zapcore.ErrorLevel:  "ERROR",
	zapcore.DPanicLevel: "CRITICAL",
	zapcore.PanicLevel:  "ALERT",
	zapcore.FatalLevel:  "EMERGENCY",
}

func LevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(logLevelSeverity[l])
}

// NewStackdriverEncoderConfig returns the new zapcore.EncoderConfig with stackdriver encoder config.
func NewStackdriverEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "eventTime",
		LevelKey:       "severity",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "trace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    LevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// NewStackdriverConfig returns the stackdriver encoder zap.Config.
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

func RegisterStackdriverEncoder(ctx context.Context, projectID, logID string) (string, func(zapcore.EncoderConfig) (zapcore.Encoder, error)) {
	return "stackdriver", func(zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return NewStackdriverEncoder(ctx, NewStackdriverEncoderConfig(), projectID, logID), nil
	}
}

func (e *Encoder) encoder() zapcore.Encoder {
	return e.Encoder.(zapcore.Encoder)
}

func (e *Encoder) primitiveArrayEncoder() zapcore.PrimitiveArrayEncoder {
	return e.Encoder.(zapcore.PrimitiveArrayEncoder)
}

func (e *Encoder) Clone() zapcore.Encoder {
	return &Encoder{
		lg:                e.lg,
		SetReportLocation: e.SetReportLocation,
		ctx:               e.ctx,
		Encoder:           e.Encoder.Clone(),
		EncoderConfig:     e.EncoderConfig,
	}
}

func (e *Encoder) cloneCtx() *Context {
	if e.ctx == nil {
		return &Context{}
	}

	return e.ctx.Clone()
}

func parseLevel(l zapcore.Level) (sev sdlogging.Severity) {
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

func (e *Encoder) parseEntry(enc zapcore.Encoder, ent zapcore.Entry, cfg *zapcore.EncoderConfig) {
	if cfg != nil {
		if !ent.Time.IsZero() && cfg.TimeKey != "" {
			enc.AddTime(cfg.TimeKey, ent.Time)
		}
		if ent.LoggerName != "" && cfg.NameKey != "" {
			enc.AddString(cfg.NameKey, ent.LoggerName)
		}
		if ent.Caller.Defined && cfg.CallerKey != "" {
			enc.AddReflected(cfg.CallerKey, ent.Caller)
		}
		if ent.Message != "" && cfg.MessageKey != "" {
			enc.AddString(cfg.MessageKey, ent.Message)
		}
		if ent.Stack != "" && cfg.StacktraceKey != "" {
			enc.AddString(cfg.StacktraceKey, ent.Stack)
		}
	}
}

func (e *Encoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	enc := e.Encoder.Clone()

	e.parseEntry(enc, ent, e.EncoderConfig)

	fields, ctx := e.extractCtx(fields)
	if ctx != nil {
		fields = append(fields, LogContext(ctx))
	}

	rl := e.ReportLocationFromEntry(ent, fields)
	if rl != nil {
		fields = append(fields, LogReportLocation(rl))
	}

	buf, err := enc.EncodeEntry(ent, fields)
	entry := sdlogging.Entry{
		Timestamp: ent.Time,
		Severity:  parseLevel(ent.Level),
		Payload:   buf.String(),
	}
	e.lg.Log(entry)

	return buf, err
}

func (e *Encoder) extractCtx(fields []zapcore.Field) ([]zapcore.Field, *Context) {
	output := []zapcore.Field{}
	ctx := e.cloneCtx()
	if ctx.IsEmpty() {
		return fields, nil
	}

	for _, f := range fields {
		switch f.Key {
		case keyContextHTTPRequest:
			ctx.HTTPRequest = f.Interface.(*HTTPRequest)
		case keyContextReportLocation:
			ctx.ReportLocation = f.Interface.(*ReportLocation)
		case keyContextUser:
			ctx.User = f.String
		default:
			// output = append(output, f)
		}
	}

	return output, ctx
}

func (e *Encoder) ReportLocationFromEntry(ent zapcore.Entry, fields []zapcore.Field) *ReportLocation {
	if !e.SetReportLocation {
		return nil
	}

	caller := ent.Caller
	if !caller.Defined {
		return nil
	}

	loc := &ReportLocation{
		FilePath:   caller.File,
		LineNumber: caller.Line,
	}
	if fn := runtime.FuncForPC(caller.PC); fn != nil {
		loc.FunctionName = fn.Name()
	}

	return loc
}

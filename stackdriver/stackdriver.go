// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stackdriver

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	sdlogging "cloud.google.com/go/logging"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	oauth2_google "golang.org/x/oauth2/google"
)

func init() {
	ctx := context.Background()
	lg := NewDefaultStackdriverClient(ctx, "", "")
	if err := zap.RegisterEncoder("stackdriver", func(zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return NewStackdriverEncoder(ctx, lg, NewStackdriverEncoderConfig()), nil
	}); err != nil {
		panic(err)
	}
}

const defaultLogID = "app_logs"

// Encoder represents a zap.Encoder with stackdriver logging.
type Encoder struct {
	lg                *sdlogging.Logger
	SetReportLocation bool
	ctx               *LogContext
	pool              buffer.Pool

	zapcore.Encoder
	*zapcore.EncoderConfig
}

func RegisterStackdriverEncoder(ctx context.Context, projectID, logID string) (string, func(zapcore.EncoderConfig) (zapcore.Encoder, error)) {
	return "stackdriver", func(zapcore.EncoderConfig) (zapcore.Encoder, error) {
		lg := NewDefaultStackdriverClient(ctx, projectID, logID)
		return NewStackdriverEncoder(ctx, lg, NewStackdriverEncoderConfig()), nil
	}
}

// NewDefaultStackdriverClient returns the stackdriver logging client with default options.
func NewDefaultStackdriverClient(ctx context.Context, projectID, logID string) *sdlogging.Logger {
	if projectID == "" {
		cred, err := oauth2_google.FindDefaultCredentials(ctx)
		if err != nil {
			panic(fmt.Errorf("failed to find default credentials: %+v", err))
		}
		projectID = cred.ProjectID
	}
	if logID == "" {
		logID = defaultLogID
	}
	fmt.Fprintf(os.Stdout, "projectID: %s, logID: %s\n", projectID, logID)

	sd, err := sdlogging.NewClient(ctx, projectID)
	if err != nil {
		panic(fmt.Errorf("failed to create logging client: %+v", err))
	}
	sd.OnError = func(error) {}

	ctxFn := func() (context.Context, func()) {
		ctx, span := trace.StartSpan(ctx, "this span will not be exported", trace.WithSampler(trace.NeverSample()))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		afterCallFn := func() {
			span.End()
			cancel()
		}
		return ctx, afterCallFn
	}

	return sd.Logger(logID, sdlogging.ContextFunc(ctxFn))
}

// NewLogger returns the new zap.Logger with stackdriver zapcore.Encoder.
func NewLogger(ctx context.Context, lg *sdlogging.Logger, atomlv zap.AtomicLevel, opts ...zap.Option) *zap.Logger {
	var zopts []zap.Option

	cfg := NewStackdriverConfig()
	switch lv := atomlv.Level(); lv {
	default:
		// nothig to do
	case zap.DebugLevel:
		zopts = append(zopts, zap.AddCallerSkip(0), zap.AddStacktrace(lv))
	}
	cfg.Level = atomlv

	fn := func(core zapcore.Core) zapcore.Core {
		enc := NewStackdriverEncoder(ctx, lg, NewStackdriverEncoderConfig())
		ws := &WriteSyncer{lg: lg}
		return zapcore.NewCore(enc, ws, atomlv.Level())
	}
	zopts = append(zopts, zap.WrapCore(fn))

	zopts = append(zopts, opts...)
	zl, err := cfg.Build(zopts...)
	if err != nil {
		panic(zl)
	}

	return zl
}

// NewStackdriverEncoder returns the stackdriver zapcore.Encoder.
func NewStackdriverEncoder(ctx context.Context, lg *sdlogging.Logger, encoderConfig zapcore.EncoderConfig) zapcore.Encoder {
	return &Encoder{
		lg:            lg,
		pool:          buffer.NewPool(),
		Encoder:       zapcore.NewJSONEncoder(encoderConfig),
		EncoderConfig: &encoderConfig,
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

func (e *Encoder) encoder() zapcore.Encoder {
	return e.Encoder.(zapcore.Encoder)
}

func (e *Encoder) primitiveArrayEncoder() zapcore.PrimitiveArrayEncoder {
	return e.Encoder.(zapcore.PrimitiveArrayEncoder)
}

func (e *Encoder) Clone() zapcore.Encoder {
	return &Encoder{
		lg:                e.lg,
		pool:              e.pool,
		SetReportLocation: e.SetReportLocation,
		ctx:               e.ctx,
		Encoder:           e.Encoder.Clone(),
		EncoderConfig:     e.EncoderConfig,
	}
}

func (e *Encoder) cloneCtx() *LogContext {
	if e.ctx == nil {
		return &LogContext{}
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
		fields = append(fields, WithContext(ctx))
	}

	rl := e.ReportLocationFromEntry(ent, fields)
	if rl != nil {
		fields = append(fields, WithReportLocation(rl))
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

const (
	keyServiceContext        = "serviceContext"
	keyContext               = "context"
	keyContextHTTPRequest    = "context.httpRequest"
	keyContextUser           = "context.user"
	keyContextReportLocation = "context.reportLocation"
)

func (e *Encoder) extractCtx(fields []zapcore.Field) ([]zapcore.Field, *LogContext) {
	output := []zapcore.Field{}
	lc := e.cloneCtx()
	if lc.IsEmpty() {
		return fields, nil
	}

	for _, f := range fields {
		switch f.Key {
		case keyContextHTTPRequest:
			lc.HTTPRequest = f.Interface.(*HTTPRequest)
		case keyContextReportLocation:
			lc.ReportLocation = f.Interface.(*ReportLocation)
		case keyContextUser:
			lc.User = f.String
		default:
			// output = append(output, f)
		}
	}

	return output, lc
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

// WriteSyncer represents a zapcore.WriteSyncer with stackdriver logging.
type WriteSyncer struct {
	lg *sdlogging.Logger
}

//pragma: compiler time checks whether the WriteSyncer implemented zapcore.WriteSyncer interface.
var _ zapcore.WriteSyncer = (*WriteSyncer)(nil)

// Write implements zapcore.WriteSyncer.
func (ws *WriteSyncer) Write(b []byte) (int, error) {
	return len(b), nil
}

// Sync implements zapcore.WriteSyncer.
func (ws *WriteSyncer) Sync() error {
	return ws.lg.Flush()
}

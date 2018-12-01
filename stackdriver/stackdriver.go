// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stackdriver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	sdlogging "cloud.google.com/go/logging"
	"go.opencensus.io/trace"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	"github.com/zchee/zap-encoder/pool"
)

const (
	// hex for JSON escaping; see stackdriverEncoder.safeAddString.
	hex = "0123456789abcdef"
)

type stackdriverEncoder struct {
	lg  *sdlogging.Logger
	buf pool.Pooler

	*zapcore.EncoderConfig
	spaced         bool
	openNamespaces int

	reflectBuf pool.Pooler
	reflectEnc *json.Encoder
}

//pragma: compiler time checks whether the stackdriverEncoder implemented zapcore.Encoder interface.
var _ zapcore.Encoder = (*stackdriverEncoder)(nil)

var stackdriverEncoderPool = sync.Pool{
	New: func() interface{} {
		return &stackdriverEncoder{}
	},
}

func getStackdriverEncoder() *stackdriverEncoder {
	return stackdriverEncoderPool.Get().(*stackdriverEncoder)
}

func putStackdriverEncoder(enc *stackdriverEncoder) {
	if enc.reflectBuf != nil {
		enc.reflectBuf.Reset()
	}
	enc.EncoderConfig = nil
	enc.buf = nil
	enc.spaced = false
	enc.openNamespaces = 0
	enc.reflectBuf = nil
	enc.reflectEnc = nil

	stackdriverEncoderPool.Put(enc)
}

// NewStackdriverEncoder creates a fast, low-allocation JSON encoder. The encoder
// appropriately escapes all field keys and values.
//
// Note that the encoder doesn't deduplicate keys, so it's possible to produce a message like
//
//   {"foo":"bar","foo":"baz"}
//
// This is permitted by the JSON specification, but not encouraged.
//
// Many libraries will ignore duplicate key-value pairs (typically keeping the last
// pair) when unmarshaling, but users should attempt to avoid adding duplicate
// keys.
func NewStackdriverEncoder(ctx context.Context, cfg zapcore.EncoderConfig, projectID, logID string) zapcore.Encoder {
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
	sdLogger := client.Logger(logID, sdlogging.ContextFunc(ctxFn))

	return newStackdriverEncoder(cfg, sdLogger, projectID, logID, false)
}

func newStackdriverEncoder(cfg zapcore.EncoderConfig, lg *sdlogging.Logger, projectID, logID string, spaced bool) (sde *stackdriverEncoder) {
	sde = &stackdriverEncoder{
		lg:  lg,
		buf: pool.NewMapPool(),

		EncoderConfig: &cfg,
		spaced:        spaced,
	}

	return sde
}

// New creates a fast, low-allocation JSON encoder.
func New(ctx context.Context, projectID, logID string) zapcore.Encoder {
	client, err := sdlogging.NewClient(ctx, projectID, option.WithGRPCDialOption(grpc.WithTimeout(5*time.Second)))
	if err != nil {
		panic(fmt.Sprintf("Failed to create logging client: %v", err))
	}
	client.OnError = func(err error) {}

	lg := client.Logger(logID)

	return newEncoder(lg)
}

func newEncoder(lg *sdlogging.Logger) (sde *stackdriverEncoder) {
	sde = &stackdriverEncoder{
		lg:  lg,
		buf: pool.NewMapPool(),
	}

	return sde
}

func (enc *stackdriverEncoder) AddArray(key string, arr zapcore.ArrayMarshaler) error {
	enc.addKey(key)
	return enc.AppendArray(arr)
}

func (enc *stackdriverEncoder) AddObject(key string, obj zapcore.ObjectMarshaler) error {
	enc.addKey(key)
	return enc.AppendObject(obj)
}

func (enc *stackdriverEncoder) AddBinary(key string, val []byte) {
	enc.AddString(key, base64.StdEncoding.EncodeToString(val))
}

func (enc *stackdriverEncoder) AddByteString(key string, val []byte) {
	enc.addKey(key)
	enc.AppendByteString(val)
}

func (enc *stackdriverEncoder) AddBool(key string, val bool) {
	enc.addKey(key)
	enc.AppendBool(val)
}

func (enc *stackdriverEncoder) AddComplex128(key string, val complex128) {
	enc.addKey(key)
	enc.AppendComplex128(val)
}

func (enc *stackdriverEncoder) AddDuration(key string, val time.Duration) {
	enc.addKey(key)
	enc.AppendDuration(val)
}

func (enc *stackdriverEncoder) AddFloat64(key string, val float64) {
	enc.addKey(key)
	enc.AppendFloat64(val)
}

func (enc *stackdriverEncoder) AddInt64(key string, val int64) {
	enc.addKey(key)
	enc.AppendInt64(val)
}

func (enc *stackdriverEncoder) resetReflectBuf() {
	if enc.reflectBuf == nil {
		enc.reflectBuf = pool.NewMapPool()
		enc.reflectEnc = json.NewEncoder(enc.reflectBuf)
	} else {
		enc.reflectBuf.Reset()
	}
}

// AddReflected uses reflection to serialize arbitrary objects, so it's slow
// and allocation-heavy.
func (enc *stackdriverEncoder) AddReflected(key string, obj interface{}) error {
	enc.resetReflectBuf()
	err := enc.reflectEnc.Encode(obj)
	if err != nil {
		return err
	}
	enc.reflectBuf.TrimNewline()
	enc.addKey(key)
	_, err = enc.buf.Write(enc.reflectBuf.Bytes())

	return err
}

// OpenNamespace opens an isolated namespace where all subsequent fields will
// be added. Applications can use namespaces to prevent key collisions when
// injecting loggers into sub-components or third-party libraries.
func (enc *stackdriverEncoder) OpenNamespace(key string) {
	enc.addKey(key)
	enc.buf.AppendByte('{')
	enc.openNamespaces++
}

func (enc *stackdriverEncoder) AddString(key, val string) {
	enc.addKey(key)
	enc.AppendString(val)
}

func (enc *stackdriverEncoder) AddTime(key string, val time.Time) {
	enc.addKey(key)
	enc.AppendTime(val)
}

func (enc *stackdriverEncoder) AddUint64(key string, val uint64) {
	enc.addKey(key)
	enc.AppendUint64(val)
}

func (enc *stackdriverEncoder) AppendArray(arr zapcore.ArrayMarshaler) error {
	enc.addElementSeparator()
	enc.buf.AppendByte('[')
	err := arr.MarshalLogArray(enc)
	enc.buf.AppendByte(']')
	return err
}

func (enc *stackdriverEncoder) AppendObject(obj zapcore.ObjectMarshaler) error {
	enc.addElementSeparator()
	enc.buf.AppendByte('{')
	err := obj.MarshalLogObject(enc)
	enc.buf.AppendByte('}')
	return err
}

func (enc *stackdriverEncoder) AppendBool(val bool) {
	enc.addElementSeparator()
	enc.buf.AppendBool(val)
}

func (enc *stackdriverEncoder) AppendByteString(val []byte) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.safeAddByteString(val)
	enc.buf.AppendByte('"')
}

func (enc *stackdriverEncoder) AppendComplex128(val complex128) {
	enc.addElementSeparator()
	// Cast to a platform-independent, fixed-size type.
	r, i := float64(real(val)), float64(imag(val))
	enc.buf.AppendByte('"')
	// Because we're always in a quoted string, we can use strconv without
	// special-casing NaN and +/-Inf.
	enc.buf.AppendFloat(r, 64)
	enc.buf.AppendByte('+')
	enc.buf.AppendFloat(i, 64)
	enc.buf.AppendByte('i')
	enc.buf.AppendByte('"')
}

func (enc *stackdriverEncoder) AppendDuration(val time.Duration) {
	cur := enc.buf.Len()
	enc.EncodeDuration(val, enc)
	if cur == enc.buf.Len() {
		// User-supplied EncodeDuration is a no-op. Fall back to nanoseconds to keep
		// JSON valid.
		enc.AppendInt64(int64(val))
	}
}

func (enc *stackdriverEncoder) AppendInt64(val int64) {
	enc.addElementSeparator()
	enc.buf.AppendInt(val)
}

func (enc *stackdriverEncoder) AppendReflected(val interface{}) error {
	enc.resetReflectBuf()
	err := enc.reflectEnc.Encode(val)
	if err != nil {
		return err
	}
	enc.reflectBuf.TrimNewline()
	enc.addElementSeparator()
	_, err = enc.buf.Write(enc.reflectBuf.Bytes())
	return err
}

func (enc *stackdriverEncoder) AppendString(val string) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.safeAddString(val)
	enc.buf.AppendByte('"')
}

func (enc *stackdriverEncoder) AppendTime(val time.Time) {
	cur := enc.buf.Len()
	enc.EncodeTime(val, enc)
	if cur == enc.buf.Len() {
		// User-supplied EncodeTime is a no-op. Fall back to nanos since epoch to keep
		// output JSON valid.
		enc.AppendInt64(val.UnixNano())
	}
}

func (enc *stackdriverEncoder) AppendUint64(val uint64) {
	enc.addElementSeparator()
	enc.buf.AppendUint(val)
}

func (enc *stackdriverEncoder) AddComplex64(k string, v complex64) {
	enc.AddComplex128(k, complex128(v))
}
func (enc *stackdriverEncoder) AddFloat32(k string, v float32) { enc.AddFloat64(k, float64(v)) }
func (enc *stackdriverEncoder) AddInt(k string, v int)         { enc.AddInt64(k, int64(v)) }
func (enc *stackdriverEncoder) AddInt32(k string, v int32)     { enc.AddInt64(k, int64(v)) }
func (enc *stackdriverEncoder) AddInt16(k string, v int16)     { enc.AddInt64(k, int64(v)) }
func (enc *stackdriverEncoder) AddInt8(k string, v int8)       { enc.AddInt64(k, int64(v)) }
func (enc *stackdriverEncoder) AddUint(k string, v uint)       { enc.AddUint64(k, uint64(v)) }
func (enc *stackdriverEncoder) AddUint32(k string, v uint32)   { enc.AddUint64(k, uint64(v)) }
func (enc *stackdriverEncoder) AddUint16(k string, v uint16)   { enc.AddUint64(k, uint64(v)) }
func (enc *stackdriverEncoder) AddUint8(k string, v uint8)     { enc.AddUint64(k, uint64(v)) }
func (enc *stackdriverEncoder) AddUintptr(k string, v uintptr) { enc.AddUint64(k, uint64(v)) }
func (enc *stackdriverEncoder) AppendComplex64(v complex64)    { enc.AppendComplex128(complex128(v)) }
func (enc *stackdriverEncoder) AppendFloat64(v float64)        { enc.appendFloat(v, 64) }
func (enc *stackdriverEncoder) AppendFloat32(v float32)        { enc.appendFloat(float64(v), 32) }
func (enc *stackdriverEncoder) AppendInt(v int)                { enc.AppendInt64(int64(v)) }
func (enc *stackdriverEncoder) AppendInt32(v int32)            { enc.AppendInt64(int64(v)) }
func (enc *stackdriverEncoder) AppendInt16(v int16)            { enc.AppendInt64(int64(v)) }
func (enc *stackdriverEncoder) AppendInt8(v int8)              { enc.AppendInt64(int64(v)) }
func (enc *stackdriverEncoder) AppendUint(v uint)              { enc.AppendUint64(uint64(v)) }
func (enc *stackdriverEncoder) AppendUint32(v uint32)          { enc.AppendUint64(uint64(v)) }
func (enc *stackdriverEncoder) AppendUint16(v uint16)          { enc.AppendUint64(uint64(v)) }
func (enc *stackdriverEncoder) AppendUint8(v uint8)            { enc.AppendUint64(uint64(v)) }
func (enc *stackdriverEncoder) AppendUintptr(v uintptr)        { enc.AppendUint64(uint64(v)) }

func (enc *stackdriverEncoder) Clone() zapcore.Encoder {
	_ = pool.NewMapPool()
	// for k, v := range enc.buf {
	// 	buf[k] = v
	// }

	return &stackdriverEncoder{
		lg:  enc.lg,
		buf: enc.buf,
	}
}

func (stackdriverEncoder) parseLevel(l zapcore.Level) (sev sdlogging.Severity) {
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

func (enc *stackdriverEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	sev := enc.parseLevel(entry.Level)

	buf := pool.NewMapPool()
	// for k, v := range enc.buf {
	// 	buf[k] = v
	// }

	for _, f := range fields {
		switch f.Type {
		case zapcore.ArrayMarshalerType:
			//TODO:
		case zapcore.ObjectMarshalerType:
			//TODO:
		case zapcore.BinaryType:
			// buf[f.Key] = f.Interface
		case zapcore.BoolType:
			// buf[f.Key] = f.Integer == 1
		case zapcore.ByteStringType:
			// buf[f.Key] = f.Interface
		case zapcore.Complex128Type:
			// buf[f.Key] = f.Interface
		case zapcore.Complex64Type:
			// buf[f.Key] = f.Interface
		case zapcore.DurationType:
			// buf[f.Key] = time.Duration(f.Integer).Seconds()
		case zapcore.Float64Type:
			// buf[f.Key] = math.Float64frombits(uint64(f.Integer))
		case zapcore.Float32Type:
			// buf[f.Key] = math.Float32frombits(uint32(f.Integer))
		case zapcore.Int64Type:
			// buf[f.Key] = f.Integer
		case zapcore.Int32Type:
			// buf[f.Key] = f.Integer
		case zapcore.Int16Type:
			// buf[f.Key] = f.Integer
		case zapcore.Int8Type:
			// buf[f.Key] = f.Integer
		case zapcore.StringType:
			// buf[f.Key] = f.String
		case zapcore.TimeType:
			// if f.Interface != nil {
			// 	buf[f.Key] = time.Unix(0, f.Integer).In(f.Interface.(*time.Location))
			// } else {
			// 	// Fall back to UTC if location is nil.
			// 	buf[f.Key] = time.Unix(0, f.Integer)
			// }
		case zapcore.Uint64Type:
			// buf[f.Key] = f.Integer
		case zapcore.Uint32Type:
			// buf[f.Key] = f.Integer
		case zapcore.Uint16Type:
			// buf[f.Key] = f.Integer
		case zapcore.Uint8Type:
			// buf[f.Key] = f.Integer
		case zapcore.UintptrType:
			// buf[f.Key] = f.Integer
		case zapcore.ReflectType:
			// buf[f.Key] = f.Interface
		case zapcore.NamespaceType:
			//TODO
		case zapcore.StringerType:
			// buf[f.Key] = f.Interface.(fmt.Stringer).String()
		case zapcore.ErrorType:
			// buf[f.Key] = f.Interface.(error).Error()
		case zapcore.SkipType:
			// break

		}
	}
	// buf["msg"] = entry.Message

	e := sdlogging.Entry{
		Timestamp: entry.Time,
		Payload:   buf,
		Severity:  sev,
	}
	enc.lg.Log(e)

	return enc.buf.GetBuffer(), nil
}

func (enc *stackdriverEncoder) truncate() {
	enc.buf.Reset()
}

func (enc *stackdriverEncoder) closeOpenNamespaces() {
	for i := 0; i < enc.openNamespaces; i++ {
		enc.buf.AppendByte('}')
	}
}

func (enc *stackdriverEncoder) addKey(key string) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.safeAddString(key)
	enc.buf.AppendByte('"')
	enc.buf.AppendByte(':')
	if enc.spaced {
		enc.buf.AppendByte(' ')
	}
}

func (enc *stackdriverEncoder) addElementSeparator() {
	last := enc.buf.Len() - 1
	if last < 0 {
		return
	}
	switch enc.buf.Bytes()[last] {
	case '{', '[', ':', ',', ' ':
		return
	default:
		enc.buf.AppendByte(',')
		if enc.spaced {
			enc.buf.AppendByte(' ')
		}
	}
}

func (enc *stackdriverEncoder) appendFloat(val float64, bitSize int) {
	enc.addElementSeparator()
	switch {
	case math.IsNaN(val):
		enc.buf.AppendString(`"NaN"`)
	case math.IsInf(val, 1):
		enc.buf.AppendString(`"+Inf"`)
	case math.IsInf(val, -1):
		enc.buf.AppendString(`"-Inf"`)
	default:
		enc.buf.AppendFloat(val, bitSize)
	}
}

// safeAddString JSON-escapes a string and appends it to the internal buffer.
// Unlike the standard library's encoder, it doesn't attempt to protect the
// user from browser vulnerabilities or JSONP-related problems.
func (enc *stackdriverEncoder) safeAddString(s string) {
	for i := 0; i < len(s); {
		if enc.tryAddRuneSelf(s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if enc.tryAddRuneError(r, size) {
			i++
			continue
		}
		enc.buf.AppendString(s[i : i+size])
		i += size
	}
}

// safeAddByteString is no-alloc equivalent of safeAddString(string(s)) for s []byte.
func (enc *stackdriverEncoder) safeAddByteString(s []byte) {
	for i := 0; i < len(s); {
		if enc.tryAddRuneSelf(s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRune(s[i:])
		if enc.tryAddRuneError(r, size) {
			i++
			continue
		}
		enc.buf.Write(s[i : i+size])
		i += size
	}
}

// tryAddRuneSelf appends b if it is valid UTF-8 character represented in a single byte.
func (enc *stackdriverEncoder) tryAddRuneSelf(b byte) bool {
	if b >= utf8.RuneSelf {
		return false
	}
	if 0x20 <= b && b != '\\' && b != '"' {
		enc.buf.AppendByte(b)
		return true
	}
	switch b {
	case '\\', '"':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte(b)
	case '\n':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte('n')
	case '\r':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte('r')
	case '\t':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte('t')
	default:
		// Encode bytes < 0x20, except for the escape sequences above.
		enc.buf.AppendString(`\u00`)
		enc.buf.AppendByte(hex[b>>4])
		enc.buf.AppendByte(hex[b&0xF])
	}
	return true
}

func (enc *stackdriverEncoder) tryAddRuneError(r rune, size int) bool {
	if r == utf8.RuneError && size == 1 {
		enc.buf.AppendString(`\ufffd`)
		return true
	}
	return false
}

func (enc *stackdriverEncoder) Put() {
	enc.buf = pool.NewMapPool() // no-op
}

type stackdriverWriterSyncer struct {
	lg *sdlogging.Logger
}

//pragma: compiler time checks whether the stackdriverWriterSyncer implemented zapcore interface.
var _ zapcore.WriteSyncer = (*stackdriverWriterSyncer)(nil)

func (g *stackdriverWriterSyncer) Write(b []byte) (int, error) {
	// devnull, the encoder does the work
	return len(b), nil
}

func (g *stackdriverWriterSyncer) Sync() error {
	return g.lg.Flush()
}

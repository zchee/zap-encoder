// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pool provides a thin wrapper around a byte slice.
//
// Unlike the standard library's bytes.Buffer, it supports a portion of the strconv
// package's zero-allocation formatters.
package pool

import (
	"fmt"
	"strconv"
	"sync"

	"go.uber.org/zap/buffer"

	"github.com/zchee/zap-encoder/internal/unsafeutil"
)

type mapKey int

const (
	keyByte mapKey = 1 << iota
	keyString
	keyInt
	keyUint
	keyBool
	keyFloat
)

// String implements fmt.Stringer.
func (i mapKey) String() string {
	switch i {
	case keyByte:
		return "keyByte"
	case keyString:
		return "keyString"
	case keyInt:
		return "keyInt"
	case keyUint:
		return "keyUint"
	case keyBool:
		return "keyBool"
	case keyFloat:
		return "keyFloat"
	default:
		return "mapKey(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}

// MapBuffer is a thin wrapper around a sync.Map.
//
// It is intended to be pooled, so the only way to construct one is via a Pool.
type MapBuffer struct {
	m *sync.Map
	p *sync.Pool
}

//pragma: compiler time checks whether the MapBuffer implemented Pooler interface.
var _ Pooler = (*MapBuffer)(nil)

// NewMapPool constructs a pooled new MapBuffer.
func NewMapPool() Pooler {
	return Pooler(&MapBuffer{p: &sync.Pool{
		New: func() interface{} {
			return &MapBuffer{
				m: getSyncMap(),
			}
		},
	}})
}

// Get retrieves a Buffer from the pool, creating one if necessary.
func (mb *MapBuffer) Get() Pooler {
	buf := mb.p.Get().(*MapBuffer)
	buf.Reset()
	buf.p = mb.p

	return buf
}

// GetBuffer returns the zap *buffer.Buffer.
func (mb *MapBuffer) GetBuffer() (bb *buffer.Buffer) {
	bb.Write(mb.Bytes())

	return bb
}

// Put returns the MapBuffer to its Pool.
//
// Callers must not retain references to the Buffer after calling Free.
func (mb *MapBuffer) Put(b Pooler) {
	mb.p.Put(b)
}

// AppendByte writes a single byte to the Buffer.
func (mb *MapBuffer) AppendByte(v byte) {
	mb.m.Store(keyByte, v)
}

// AppendString writes a string to the Buffer.
func (mb *MapBuffer) AppendString(s string) {
	mb.m.Store(keyString, unsafeutil.UnsafeSlice(s))
}

// AppendInt appends an integer to the underlying buffer (assuming base 10).
func (mb *MapBuffer) AppendInt(i int64) {
	mb.m.Store(keyInt, unsafeutil.UnsafeSlice(strconv.FormatInt(i, 10)))
}

// AppendUint appends an unsigned integer to the underlying buffer (assuming
// base 10).
func (mb *MapBuffer) AppendUint(i uint64) {
	mb.m.Store(keyUint, unsafeutil.UnsafeSlice(strconv.FormatUint(i, 10)))
}

// AppendBool appends a bool to the underlying buffer.
func (mb *MapBuffer) AppendBool(v bool) {
	mb.m.Store(keyBool, unsafeutil.UnsafeSlice(strconv.FormatBool(v)))
}

// AppendFloat appends a float to the underlying buffer. It doesn't quote NaN
// or +/- Inf.
func (mb *MapBuffer) AppendFloat(f float64, bitSize int) {
	mb.m.Store(keyFloat, unsafeutil.UnsafeSlice(strconv.FormatFloat(f, 'f', -1, bitSize)))
}

// Bytes imitations bytes.Buffer.
//
// The returns a mutable reference to the underlying byte slice.
func (mb *MapBuffer) Bytes() (p []byte) {
	mb.m.Range(func(_ interface{}, v interface{}) bool {
		switch v := v.(type) {
		case byte:
			p = append(p, v)
			return true
		case []byte:
			p = append(p, v...)
			return true
		case int:
			p = append(p, unsafeutil.UnsafeSlice(strconv.FormatInt(int64(v), 10))...)
			return true
		default:
			fmt.Printf("v: %T = %#v\n", v, v)
			return false
		}
	})

	return p
}

// String implements fmt.Stringer.
//
// The returns a string copy of the underlying byte slice.
func (mb *MapBuffer) String() (s string) {
	s = unsafeutil.UnsafeString(mb.Bytes())
	return s
}

// Len imitations bytes.Buffer.
//
// The returns the length of the underlying byte slice.
func (mb *MapBuffer) Len() int {
	return len(mb.Bytes())
}

// Truncate imitations bytes.Buffer.
//
// Truncate discards all but the first n unread bytes from the buffer
// but continues to use the same allocated storage.
// It panics if n is negative or greater than the length of the buffer.
func (mb *MapBuffer) Truncate(n int) {
	if n == 0 {
		mb.Reset()
		return
	}
	if n < 0 || n > mb.Len() {
		panic("pool.MapBuffer: truncation out of range")
	}
	mb.m.Range(func(k interface{}, _ interface{}) bool {
		for i := 0; i <= n; i++ {
			mb.m.Delete(k)
		}
		return true
	})
}

// Reset imitations bytes.Buffer.
//
// The resets the underlying byte slice. Subsequent writes re-use the slice's
// backing array.
func (mb *MapBuffer) Reset() {
	mb.m = getSyncMap()
}

// Write implements io.Writer.
//
// That is Write appends the contents of p to the buffer.
func (mb *MapBuffer) Write(p []byte) (int, error) {
	mb.m.Store(keyByte, p)
	return len(p), nil
}

// WriteString implements io.StringWriter.
//
// That is appends the contents of s to the buffer without a heap allocation.
func (mb *MapBuffer) WriteString(s string) (n int, err error) {
	mb.m.Store(keyString, unsafeutil.UnsafeSlice(s))
	return len(s), nil
}

// TrimNewline trims any final `\n` byte from the end of the buffer.
func (mb *MapBuffer) TrimNewline() {
	mb.m.Range(func(k interface{}, v interface{}) bool {
		if i := mb.Len() - 1; i >= 0 {
			switch v := v.(type) {
			case []byte:
				if v[i] == '\n' {
					v = v[:i]
				}
				mb.m.Store(k, v)
			case string:
				if []byte(v)[i] == '\n' {
					v = v[:i]
				}
				mb.m.Store(k, v)
			}
		}
		return true
	})
}

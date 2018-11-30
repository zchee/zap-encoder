// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pool provides a thin wrapper around a byte slice.
//
// Unlike the standard library's bytes.Buffer, it supports a portion of the strconv
// package's zero-allocation formatters.
package pool

import (
	"strconv"
	"sync"

	"go.uber.org/zap/buffer"

	"github.com/zchee/zap-encoder/internal/unsafeutil"
)

const (
	size = 1024 // by default, create 1 KiB buffers
)

// MapBuffer is a thin wrapper around a sync.Map.
//
// It is intended to be pooled, so the only way to construct one is via a Pool.
type MapBuffer struct {
	bs []byte
	p  *sync.Pool
}

//pragma: compiler time checks whether the MapBuffer implemented Pooler interface.
var _ Pooler = (*MapBuffer)(nil)

// NewMapPool constructs a new MapBuffer.
func NewMapPool() Pooler {
	return &MapBuffer{
		p: &sync.Pool{
			New: func() interface{} {
				return &MapBuffer{
					bs: make([]byte, 0, size),
				}
			},
		},
	}
}

// Get retrieves a Buffer from the pool, creating one if necessary.
func (mb *MapBuffer) Get() Pooler {
	buf := mb.p.Get().(*MapBuffer)
	buf.Reset()
	buf.p = mb.p

	return buf
}

// GetBuffer returns the zap *buffer.Buffer.
func (mb *MapBuffer) GetBuffer() *buffer.Buffer {
	// TODO(zchee): implements
	return &buffer.Buffer{}
}

// Put returns the MapBuffer to its Pool.
//
// Callers must not retain references to the Buffer after calling Free.
func (mb *MapBuffer) Put(b Pooler) {
	mb.p.Put(b)
}

// AppendByte writes a single byte to the Buffer.
func (mb *MapBuffer) AppendByte(v byte) {
	mb.bs = append(mb.bs, v)
}

// AppendString writes a string to the Buffer.
func (mb *MapBuffer) AppendString(s string) {
	mb.bs = append(mb.bs, unsafeutil.UnsafeSlice(s)...)
}

// AppendInt appends an integer to the underlying buffer (assuming base 10).
func (mb *MapBuffer) AppendInt(i int64) {
	mb.bs = strconv.AppendInt(mb.bs, i, 10)
}

// AppendUint appends an unsigned integer to the underlying buffer (assuming
// base 10).
func (mb *MapBuffer) AppendUint(i uint64) {
	mb.bs = strconv.AppendUint(mb.bs, i, 10)
}

// AppendBool appends a bool to the underlying buffer.
func (mb *MapBuffer) AppendBool(v bool) {
	mb.bs = strconv.AppendBool(mb.bs, v)
}

// AppendFloat appends a float to the underlying buffer. It doesn't quote NaN
// or +/- Inf.
func (mb *MapBuffer) AppendFloat(f float64, bitSize int) {
	mb.bs = strconv.AppendFloat(mb.bs, f, 'f', -1, bitSize)
}

// Bytes imitations bytes.Buffer.
//
// The returns a mutable reference to the underlying byte slice.
func (mb *MapBuffer) Bytes() []byte {
	return mb.bs
}

// String implements fmt.Stringer.
//
// The returns a string copy of the underlying byte slice.
func (mb *MapBuffer) String() string {
	return unsafeutil.UnsafeString(mb.bs)
}

// Len imitations bytes.Buffer.
//
// The returns the length of the underlying byte slice.
func (mb *MapBuffer) Len() int {
	return len(mb.bs)
}

// Cap imitations bytes.Buffer.
//
// The returns the capacity of the underlying byte slice.
func (mb *MapBuffer) Cap() int {
	return cap(mb.bs)
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
	mb.bs = mb.bs[:n]
}

// Reset imitations bytes.Buffer.
//
// The resets the underlying byte slice. Subsequent writes re-use the slice's
// backing array.
func (mb *MapBuffer) Reset() {
	mb.bs = mb.bs[:0]
}

// Write implements io.Writer.
//
// That is Write appends the contents of p to the buffer.
func (mb *MapBuffer) Write(p []byte) (int, error) {
	mb.bs = append(mb.bs, p...)
	return len(p), nil
}

// WriteString implements io.StringWriter.
//
// That is appends the contents of s to the buffer without a heap allocation.
func (mb *MapBuffer) WriteString(s string) (n int, err error) {
	mb.bs = append(mb.bs, unsafeutil.UnsafeSlice(s)...)
	return len(s), nil
}

// TrimNewline trims any final `\n` byte from the end of the buffer.
func (mb *MapBuffer) TrimNewline() {
	if i := len(mb.bs) - 1; i >= 0 {
		if mb.bs[i] == '\n' {
			mb.bs = mb.bs[:i]
		}
	}
}

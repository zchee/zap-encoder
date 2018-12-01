// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pool

import (
	"sync"

	"go.uber.org/zap/buffer"
)

// Pooler represents a pool any type and returns the zap *buffer.buffer.
type Pooler interface {
	// Get retrieves a Pooler, creating one if necessary.
	Get() Pooler

	// Buffer returns the zap *buffer.Buffer.
	Buffer() *buffer.Buffer

	// Put returns the Pooler to its Pool.
	//
	// Callers must not retain references to the Buffer after calling Free.
	Put(b Pooler)

	// AppendByte writes a single byte to the Buffer.
	AppendByte(v byte)

	// AppendString writes a string to the Buffer.
	AppendString(s string)

	// AppendInt appends an integer to the underlying buffer (assuming base 10).
	AppendInt(i int64)

	// AppendUint appends an unsigned integer to the underlying buffer (assuming
	// base 10).
	AppendUint(i uint64)

	// AppendBool appends a bool to the underlying buffer.
	AppendBool(v bool)

	// AppendFloat appends a float to the underlying buffer. It doesn't quote NaN
	// or +/- Inf.
	AppendFloat(f float64, bitSize int)

	// Bytes imitations bytes.Buffer.
	//
	// The returns a mutable reference to the underlying byte slice.
	Bytes() []byte

	// String implements fmt.Stringer.
	//
	// The returns a string copy of the underlying byte slice.
	String() string

	// Len imitations bytes.Buffer.
	//
	// The returns the length of the underlying byte slice.
	Len() int

	// Truncate imitations bytes.Buffer.
	//
	// Truncate discards all but the first n unread bytes from the buffer
	// but continues to use the same allocated storage.
	// It panics if n is negative or greater than the length of the buffer.
	Truncate(n int)

	// Reset imitations bytes.Buffer.
	//
	// Reset resets the underlying byte slice. Subsequent writes re-use the slice's
	// backing array.
	Reset()

	// Write implements io.Writer.
	Write(p []byte) (int, error)

	// WriteString implements io.StringWriter.
	WriteString(s string) (int, error)

	// TrimNewline trims any final `\n` byte from the end of the buffer.
	TrimNewline()
}

// syncMapPool constructs a new sync.Map.
var syncMapPool = sync.Pool{New: func() interface{} {
	return new(sync.Map)
}}

// getSyncMap gets the new sync.Map from syncMapPool pool.
func getSyncMap() *sync.Map {
	return syncMapPool.Get().(*sync.Map)
}

// putSyncMap puts the used sync.Map to syncMapPool pool.
func putSyncMap(m *sync.Map) {
	m = new(sync.Map)
	syncMapPool.Put(m)
}

func init() {
	for i := 0; i < 32; i++ {
		syncMapPool.Put(new(sync.Map))
	}
}

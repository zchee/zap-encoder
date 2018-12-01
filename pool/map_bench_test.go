// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pool

import (
	"bytes"
	"strings"
	"testing"
)

func BenchmarkByteSlice(b *testing.B) {
	// Because we use the strconv.AppendFoo functions so liberally, we can't
	// use the standard library's bytes.Buffer anyways (without incurring a
	// bunch of extra allocations). Nevertheless, let's make sure that we're
	// not losing any precious nanoseconds.
	b.RunParallel(func(pb *testing.PB) {
		str := strings.Repeat("a", 1024)
		slice := make([]byte, 1024)
		for pb.Next() {
			slice = append(slice, str...)
			slice = slice[:0]
		}
	})
}

func BenchmarkBytesBuffer(b *testing.B) {
	// Because we use the strconv.AppendFoo functions so liberally, we can't
	// use the standard library's bytes.Buffer anyways (without incurring a
	// bunch of extra allocations). Nevertheless, let's make sure that we're
	// not losing any precious nanoseconds.
	b.RunParallel(func(pb *testing.PB) {
		str := strings.Repeat("a", 1024)
		slice := make([]byte, 1024)
		buf := bytes.NewBuffer(slice)
		for pb.Next() {
			buf.Reset()
			buf.WriteString(str)
		}
	})
}

func BenchmarkCustomBuffer(b *testing.B) {
	// Because we use the strconv.AppendFoo functions so liberally, we can't
	// use the standard library's bytes.Buffer anyways (without incurring a
	// bunch of extra allocations). Nevertheless, let's make sure that we're
	// not losing any precious nanoseconds.
	b.RunParallel(func(pb *testing.PB) {
		str := strings.Repeat("a", 1024)
		custom := NewMapPool().Get()
		for pb.Next() {
			custom.Reset()
			custom.AppendString(str)
		}
	})
}

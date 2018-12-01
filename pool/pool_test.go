// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pool

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMapBuffers(t *testing.T) {
	const dummyData = "dummy data"

	t.Run("Truncated buffer", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			buf := NewMapPool().Get()
			if buf.Len() != 0 {
				t.Error("Expected truncated buffer")
				return
			}
			buf.Reset()
		}
	})

	t.Run("non-zero capacity", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			buf := NewMapPool().Get()
			if buf.Cap() == 0 {
				t.Error("Expected non-zero capacity")
				return
			}
			buf.Reset()
		}
	})

	t.Run("contain dummy data in buffer", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			buf := NewMapPool().Get()
			buf.AppendString(dummyData)
			// assert.Equal(t, buf.Len(), len(dummyData), "Expected buffer to contain dummy data")
			if !cmp.Equal(buf.Len(), len(dummyData)) {
				t.Error("Expected buffer to contain dummy data")
			}
			buf.Reset()
		}
	})
}

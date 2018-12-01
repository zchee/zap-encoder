// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pool

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMapPoolWrites(t *testing.T) {
	tests := []struct {
		name string
		buf  Pooler
		f    func(buf Pooler)
		want string
	}{
		{
			name: "AppendByte",
			buf:  NewMapPool().Get(),
			f:    func(buf Pooler) { buf.AppendByte('v') },
			want: "v",
		},
		{
			name: "AppendString",
			buf:  NewMapPool().Get(),
			f:    func(buf Pooler) { buf.AppendString("foo") },
			want: "foo",
		},
		{
			name: "AppendIntPositive",
			buf:  NewMapPool().Get(),
			f:    func(buf Pooler) { buf.AppendInt(42) },
			want: "42",
		},
		{
			name: "AppendIntNegative",
			buf:  NewMapPool().Get(),
			f:    func(buf Pooler) { buf.AppendInt(-42) },
			want: "-42",
		},
		{
			name: "AppendUint",
			buf:  NewMapPool().Get(),
			f:    func(buf Pooler) { buf.AppendUint(42) },
			want: "42",
		},
		{
			name: "AppendBool",
			buf:  NewMapPool().Get(),
			f:    func(buf Pooler) { buf.AppendBool(true) },
			want: "true",
		},
		{
			name: "AppendFloat64",
			buf:  NewMapPool().Get(),
			f:    func(buf Pooler) { buf.AppendFloat(3.14, 64) },
			want: "3.14",
		},
		// Intenationally introduce some floating-point error.
		{
			name: "AppendFloat32",
			buf:  NewMapPool().Get(),
			f:    func(buf Pooler) { buf.AppendFloat(float64(float32(3.14)), 32) },
			want: "3.14",
		},
		{
			name: "AppendWrite",
			buf:  NewMapPool().Get(),
			f:    func(buf Pooler) { buf.Write([]byte("foo")) },
			want: "foo",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.buf.Reset()
			tc.f(tc.buf)
			if diff := cmp.Diff(tc.want, tc.buf.String()); diff != "" {
				t.Errorf("%s: Unexpected buffer.String(): (-got, +want)\n%s\n", tc.name, diff)
				return
			}
			if diff := cmp.Diff(tc.want, string(tc.buf.Bytes())); diff != "" {
				t.Errorf("%s: Unexpected string(buffer.Bytes()): (-got, +want)\n%s\n", tc.name, diff)
				return
			}
			if diff := cmp.Diff(len(tc.want), tc.buf.Len()); diff != "" {
				t.Errorf("%s: Unexpected buffer length: (-got, +want)\n%s\n", tc.name, diff)
				return
			}
			// We're not writing more than a kibibyte in tests.
			if diff := cmp.Diff(size, tc.buf.Cap()); diff != "" {
				t.Errorf("%s: Expected buffer capacity to remain constant: (-got, +want)\n%s\n", tc.name, diff)
			}
		})
	}
}

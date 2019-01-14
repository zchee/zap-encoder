// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stackdriver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/zchee/zap-encoder/internal/testutil"
	"github.com/zchee/zap-encoder/internal/uid"
	"github.com/zchee/zap-encoder/stackdriver"
)

const (
	testLogIDPrefix = "go-logging-client/test-log"
)

func TestStackdriverEncodeEntry(t *testing.T) {
	ctx := context.Background()
	testProjectID := testutil.ProjectID()
	uids := uid.NewSpace(testLogIDPrefix, nil)
	testLogID := uids.New()

	type bar struct {
		Key string  `json:"key"`
		Val float64 `json:"val"`
	}

	type foo struct {
		A string  `json:"aee"`
		B int     `json:"bee"`
		C float64 `json:"cee"`
		D []bar   `json:"dee"`
	}

	tests := []struct {
		name     string
		expected string
		ent      zapcore.Entry
		fields   []zapcore.Field
	}{
		{
			name: "info entry with some fields",
			expected: `{
				"eventTime": "2018-06-19T16:33:42.000Z",
				"severity": "INFO",
				"logger": "bob",
				"message": "lob law",
				"so": "passes",
				"answer": 42,
				"common_pie": 3.14,
				"such": {
					"aee": "lol",
					"bee": 123,
					"cee": 0.9999,
					"dee": [
						{"key": "pi", "val": 3.141592653589793},
						{"key": "tau", "val": 6.283185307179586}
					]
				}
			}`,
			ent: zapcore.Entry{
				Level:      zapcore.InfoLevel,
				Time:       time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
				LoggerName: "bob",
				Message:    "lob law",
			},
			fields: []zapcore.Field{
				zap.String("so", "passes"),
				zap.Int("answer", 42),
				zap.Float64("common_pie", 3.14),
				zap.Reflect("such", foo{
					A: "lol",
					B: 123,
					C: 0.9999,
					D: []bar{
						{"pi", 3.141592653589793},
						{"tau", 6.283185307179586},
					},
				}),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lg := stackdriver.NewDefaultStackdriverClient(ctx, testProjectID, testLogID)
			enc := stackdriver.NewStackdriverEncoder(ctx, lg, stackdriver.NewStackdriverEncoderConfig())
			buf, err := enc.EncodeEntry(tt.ent, tt.fields)
			if err != nil {
				t.Errorf("Unexpected JSON encoding error: %+v", err)
				return
			}
			defer buf.Free()

			var expectedJSONAsInterface, actualJSONAsInterface interface{}
			if err := json.Unmarshal([]byte(tt.expected), &expectedJSONAsInterface); err != nil {
				t.Errorf(fmt.Sprintf("Expected value (%q) is not valid json.\nJSON parsing error: %+v", tt.expected, err))
				return
			}
			if err := json.Unmarshal([]byte(buf.String()), &actualJSONAsInterface); err != nil {
				t.Errorf(fmt.Sprintf("Actual value (%q) is not valid json.\nJSON parsing error: %+v", buf.String(), err))
				return
			}

			if diff := cmp.Diff(&expectedJSONAsInterface, &actualJSONAsInterface); diff != "" {
				t.Errorf("%s: Incorrect encoded JSON entry: (-got, +want)\n%s\n", tt.name, diff)
			}
		})
	}
}

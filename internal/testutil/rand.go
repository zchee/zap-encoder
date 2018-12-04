// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"math/rand"
	"sync"
	"time"
)

// NewRand creates a new *rand.Rand seeded with t. The return value is safe for use
// with multiple goroutines.
func NewRand(t time.Time) *rand.Rand {
	s := &lockedSource{src: rand.NewSource(t.UnixNano())}
	return rand.New(s)
}

// lockedSource makes a rand.Source safe for use by multiple goroutines.
type lockedSource struct {
	mu  sync.Mutex
	src rand.Source
}

func (ls *lockedSource) Int63() int64 {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return ls.src.Int63()
}

func (ls *lockedSource) Seed(int64) {
	panic("shouldn't be calling Seed")
}

// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bufferpool houses zap's shared internal buffer pool.
// Third-party packages can recreate the same functionality with buffer.NewPool.
package bufferpool

import (
	"github.com/zchee/zap-encoder/pool"
)

var (
	mapPool = pool.NewMapPool()
	// Get retrieves a buffer from the pool, creating one if necessary.
	Get = mapPool.Get
)

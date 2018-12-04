// Copyright 2018 The zap-encoder Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package uid supports generating unique IDs. Its chief purpose is to prevent
// multiple test executions from interfering with each other, and to facilitate
// cleanup of old entities that may remain if tests exit early.
package uid

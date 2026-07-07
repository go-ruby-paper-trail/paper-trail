// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

import "time"

// Clock is the time seam stamped onto [Version.CreatedAt]. Production uses
// [SystemClock]; tests inject a fixed clock for deterministic timestamps.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a func to a [Clock].
type ClockFunc func() time.Time

// Now implements [Clock].
func (f ClockFunc) Now() time.Time { return f() }

// SystemClock is the default wall-clock [Clock].
var SystemClock Clock = ClockFunc(time.Now)

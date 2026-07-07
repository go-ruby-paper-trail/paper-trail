// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

// RequestContext is the per-request seam behind PaperTrail.request. It carries
// the whodunnit applied to versions recorded during the request and the
// enabled/disabled toggle (PaperTrail.request.enabled = false suppresses
// versioning). In the gem this is thread/fiber-local; here the host owns a
// RequestContext per logical request and passes it to [New].
type RequestContext struct {
	// Whodunnit is PaperTrail.request.whodunnit — the actor stamped onto each
	// recorded version.
	Whodunnit string
	// Enabled mirrors PaperTrail.request.enabled?; when false, no version is
	// recorded. NewRequest defaults it to true.
	Enabled bool
}

// NewRequest returns an enabled [RequestContext] with an empty whodunnit.
func NewRequest() *RequestContext {
	return &RequestContext{Enabled: true}
}

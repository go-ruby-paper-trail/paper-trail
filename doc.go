// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

// Package papertrail is a pure-Go (CGO=0), MRI-faithful reimplementation of the
// deterministic core of Ruby's [paper_trail] gem — the model-versioning / audit
// engine.
//
// PaperTrail records a [Version] every time a tracked model is created, updated
// or destroyed. Each version carries the item's type and id, the event name,
// who did it ("whodunnit"), a serialized snapshot of the attributes *before* the
// change ("object"), and the per-attribute changeset ("object_changes",
// {attr => [old, new]}). Past model states can be reconstructed with [Reify],
// navigated with previous/next version, and queried at a point in time with
// [Tracker.VersionAt].
//
// # Faithfulness and seams
//
// Everything PaperTrail does *around* persistence is deterministic and needs no
// Ruby interpreter, so it lives here as pure Go: event detection, building the
// object snapshot, computing the changeset from a before/after attribute map,
// applying the only/ignore/skip/on filters, reifying a past state, and resolving
// the whodunnit. The host provides the model's attribute snapshot as a plain
// map[string]any (before and after); the version STORE is a persistence seam
// ([Store]); the CLOCK ([Clock]) and the request context ([RequestContext],
// PaperTrail.request.whodunnit) are injected. Serialization is a seam too
// ([Serializer]); the default [JSONSerializer] matches modern PaperTrail JSON
// columns (see serializer.go for why JSON over YAML). An in-memory store, an
// injected clock and a fixed whodunnit back the tests; an rbgo binding wires the
// same seams to ActiveRecord and Ruby.
//
// [paper_trail]: https://github.com/paper-trail-gem/paper_trail
package papertrail

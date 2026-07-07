// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

import "time"

// Event names recorded on a [Version], mirroring PaperTrail's `event` column.
const (
	EventCreate  = "create"
	EventUpdate  = "update"
	EventDestroy = "destroy"
)

// Version is the audit record PaperTrail writes for each tracked change. It
// maps one-to-one onto the gem's `versions` table columns.
type Version struct {
	// ID is the store-assigned primary key. It is zero until a [Store] persists
	// the version; ordering by ID reproduces PaperTrail's insertion order.
	ID int64
	// ItemType / ItemID identify the tracked record (STI base class name and id
	// in the gem).
	ItemType string
	ItemID   string
	// Event is one of [EventCreate], [EventUpdate], [EventDestroy].
	Event string
	// Whodunnit is the actor responsible for the change, resolved from the
	// [RequestContext] at record time.
	Whodunnit string
	// Object is the serialized snapshot of the attributes *before* the change:
	// empty for create (there was no prior state), the pre-update attributes for
	// update, and the pre-destroy attributes for destroy.
	Object string
	// ObjectChanges is the serialized {attr => [old, new]} changeset. It is set
	// for create and update, and empty for destroy (matching the gem, which
	// records no changeset when a record is wiped).
	ObjectChanges string
	// CreatedAt is the [Clock] time the version was recorded.
	CreatedAt time.Time
}

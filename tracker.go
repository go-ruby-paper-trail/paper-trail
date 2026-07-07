// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

import (
	"errors"
	"time"
)

// ErrNoAttributes is returned by [Tracker.Record] when both the before and
// after attribute maps are nil, so no event can be inferred.
var ErrNoAttributes = errors.New("papertrail: cannot infer event from nil before and after attributes")

// Options configures a [Tracker]. Only ItemType is required; the remaining
// seams default to an in-memory store, the system clock, JSON serialization and
// a fresh enabled request context.
type Options struct {
	ItemType   string
	Config     Config
	Store      Store
	Clock      Clock
	Serializer Serializer
	Request    *RequestContext
}

// Tracker is the has_paper_trail engine for one model class. It builds versions
// from before/after attribute maps, persists them through the [Store] seam, and
// answers version queries. It is the Go surface an rbgo binding drives from the
// model's create/update/destroy callbacks.
type Tracker struct {
	ItemType   string
	Config     Config
	Store      Store
	Clock      Clock
	Serializer Serializer
	Request    *RequestContext
}

// New builds a [Tracker], filling unset seams with defaults.
func New(o Options) *Tracker {
	t := &Tracker{
		ItemType:   o.ItemType,
		Config:     o.Config,
		Store:      o.Store,
		Clock:      o.Clock,
		Serializer: o.Serializer,
		Request:    o.Request,
	}
	if t.Store == nil {
		t.Store = NewMemoryStore()
	}
	if t.Clock == nil {
		t.Clock = SystemClock
	}
	if t.Serializer == nil {
		t.Serializer = JSONSerializer{}
	}
	if t.Request == nil {
		t.Request = NewRequest()
	}
	return t
}

// build assembles the version for an event without persisting it. The second
// result reports whether a version should be recorded at all: false when
// versioning is disabled, the event is filtered out by On, or an update produced
// no notable change (the no-op skip).
func (t *Tracker) build(itemID, event string, before, after map[string]any) (Version, bool, error) {
	if !t.Request.Enabled {
		return Version{}, false, nil
	}
	if !t.Config.enabled(event) {
		return Version{}, false, nil
	}

	changed := changedKeys(before, after)
	if event == EventUpdate && len(t.Config.notable(changed)) == 0 {
		return Version{}, false, nil
	}

	// The `object` snapshot holds the state *before* the change: absent for
	// create, the pre-change attributes for update and destroy.
	var snapshot map[string]any
	if event != EventCreate {
		snapshot = filterSkip(before, t.Config.Skip)
	}
	object, err := t.Serializer.DumpObject(snapshot)
	if err != nil {
		return Version{}, false, err
	}

	// `object_changes` is recorded for create and update, never for destroy.
	var changes map[string]Change
	if event != EventDestroy {
		changes = buildChanges(before, after, changed, t.Config.Skip)
	}
	objectChanges, err := t.Serializer.DumpChanges(changes)
	if err != nil {
		return Version{}, false, err
	}

	return Version{
		ItemType:      t.ItemType,
		ItemID:        itemID,
		Event:         event,
		Whodunnit:     t.Request.Whodunnit,
		Object:        object,
		ObjectChanges: objectChanges,
		CreatedAt:     t.Clock.Now(),
	}, true, nil
}

// record builds and persists a version, returning nil when the change is not
// versioned (disabled, filtered event, or no-op update).
func (t *Tracker) record(itemID, event string, before, after map[string]any) (*Version, error) {
	v, ok, err := t.build(itemID, event, before, after)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	saved, err := t.Store.SaveVersion(v)
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

// RecordCreate records a create version for a newly created record.
func (t *Tracker) RecordCreate(itemID string, after map[string]any) (*Version, error) {
	return t.record(itemID, EventCreate, nil, after)
}

// RecordUpdate records an update version, or returns (nil, nil) when the change
// is a no-op under the only/ignore/skip filters.
func (t *Tracker) RecordUpdate(itemID string, before, after map[string]any) (*Version, error) {
	return t.record(itemID, EventUpdate, before, after)
}

// RecordDestroy records a destroy version for a record being deleted.
func (t *Tracker) RecordDestroy(itemID string, before map[string]any) (*Version, error) {
	return t.record(itemID, EventDestroy, before, nil)
}

// Record infers the event from the presence of before/after attributes
// (before-only ⇒ destroy, after-only ⇒ create, both ⇒ update) and records it.
func (t *Tracker) Record(itemID string, before, after map[string]any) (*Version, error) {
	switch {
	case before == nil && after == nil:
		return nil, ErrNoAttributes
	case before == nil:
		return t.record(itemID, EventCreate, nil, after)
	case after == nil:
		return t.record(itemID, EventDestroy, before, nil)
	default:
		return t.record(itemID, EventUpdate, before, after)
	}
}

// Versions returns every stored version for the item, oldest-first.
func (t *Tracker) Versions(itemID string) ([]Version, error) {
	return t.Store.VersionsFor(t.ItemType, itemID)
}

// PreviousVersion returns the version immediately preceding v for the same item,
// or nil if v is the earliest.
func (t *Tracker) PreviousVersion(v Version) (*Version, error) {
	vs, err := t.Versions(v.ItemID)
	if err != nil {
		return nil, err
	}
	var prev *Version
	for i := range vs {
		if vs[i].ID < v.ID {
			p := vs[i]
			prev = &p
		}
	}
	return prev, nil
}

// NextVersion returns the version immediately following v for the same item, or
// nil if v is the latest (the item is at its live state).
func (t *Tracker) NextVersion(v Version) (*Version, error) {
	vs, err := t.Versions(v.ItemID)
	if err != nil {
		return nil, err
	}
	for i := range vs {
		if vs[i].ID > v.ID {
			n := vs[i]
			return &n, nil
		}
	}
	return nil, nil
}

// VersionAt reconstructs the item's attributes as they were at time ts. It
// returns (attrs, live=false) reified from the first version recorded after ts,
// or (nil, live=true) when ts is at or after the latest change, meaning the
// caller should use the current live record.
func (t *Tracker) VersionAt(itemID string, ts time.Time) (map[string]any, bool, error) {
	vs, err := t.Versions(itemID)
	if err != nil {
		return nil, false, err
	}
	for i := range vs {
		if vs[i].CreatedAt.After(ts) {
			attrs, err := t.Reify(vs[i])
			return attrs, false, err
		}
	}
	return nil, true, nil
}

// Live reports whether the item currently exists as a live record, i.e. its
// latest version is not a destroy. An item with no versions is treated as live.
func (t *Tracker) Live(itemID string) (bool, error) {
	vs, err := t.Versions(itemID)
	if err != nil {
		return false, err
	}
	if len(vs) == 0 {
		return true, nil
	}
	return vs[len(vs)-1].Event != EventDestroy, nil
}

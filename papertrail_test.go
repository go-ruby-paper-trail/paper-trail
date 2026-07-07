// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// fakeClock is a manually-advanced [Clock] for deterministic timestamps.
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time { return c.t }
func (c *fakeClock) advance()       { c.t = c.t.Add(time.Second) }

// errStore is a [Store] that fails on demand, exercising the persistence-seam
// error branches.
type errStore struct {
	saveErr error
	listErr error
}

func (e errStore) SaveVersion(v Version) (Version, error) { return Version{}, e.saveErr }
func (e errStore) VersionsFor(string, string) ([]Version, error) {
	return nil, e.listErr
}

// badValue is a func value; encoding/json cannot marshal it, so it drives the
// serializer error branches.
func badValue() map[string]any { return map[string]any{"f": func() {}} }

func newTracker(cfg Config, clk Clock) *Tracker {
	return New(Options{ItemType: "Widget", Config: cfg, Store: NewMemoryStore(), Clock: clk, Request: NewRequest()})
}

func TestNewDefaults(t *testing.T) {
	tr := New(Options{ItemType: "Widget"})
	if tr.Store == nil || tr.Clock == nil || tr.Serializer == nil || tr.Request == nil {
		t.Fatal("New must fill every seam with a default")
	}
	if _, ok := tr.Serializer.(JSONSerializer); !ok {
		t.Fatalf("default serializer = %T, want JSONSerializer", tr.Serializer)
	}
	// Exercise the default SystemClock through a real record.
	v, err := tr.RecordCreate("1", map[string]any{"name": "a"})
	if err != nil || v == nil || v.CreatedAt.IsZero() {
		t.Fatalf("SystemClock record: v=%v err=%v", v, err)
	}
}

func TestNewProvidedSeams(t *testing.T) {
	clk := &fakeClock{}
	req := NewRequest()
	tr := New(Options{ItemType: "W", Store: NewMemoryStore(), Clock: clk, Serializer: JSONSerializer{}, Request: req})
	if tr.Clock != clk || tr.Request != req {
		t.Fatal("New must keep provided seams")
	}
}

func TestRecordCreate(t *testing.T) {
	clk := &fakeClock{t: time.Unix(100, 0)}
	tr := newTracker(Config{}, clk)
	tr.Request.Whodunnit = "alice"

	v, err := tr.RecordCreate("1", map[string]any{"name": "widget", "qty": float64(3)})
	if err != nil {
		t.Fatal(err)
	}
	if v.Event != EventCreate || v.ItemType != "Widget" || v.ItemID != "1" {
		t.Fatalf("unexpected version %+v", v)
	}
	if v.Whodunnit != "alice" {
		t.Fatalf("whodunnit = %q", v.Whodunnit)
	}
	if v.Object != "" {
		t.Fatalf("create object must be empty, got %q", v.Object)
	}
	changes, err := tr.Serializer.LoadChanges(v.ObjectChanges)
	if err != nil {
		t.Fatal(err)
	}
	if got := changes["name"]; got.Old != nil || got.New != "widget" {
		t.Fatalf("name change = %+v", got)
	}
	if !v.CreatedAt.Equal(time.Unix(100, 0)) {
		t.Fatalf("createdAt = %v", v.CreatedAt)
	}
}

func TestRecordUpdate(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1, 0)}
	tr := newTracker(Config{}, clk)
	before := map[string]any{"name": "old", "qty": float64(1)}
	after := map[string]any{"name": "new", "qty": float64(1)}

	v, err := tr.RecordUpdate("1", before, after)
	if err != nil || v == nil {
		t.Fatalf("v=%v err=%v", v, err)
	}
	if v.Event != EventUpdate {
		t.Fatalf("event = %s", v.Event)
	}
	snap, err := tr.Serializer.LoadObject(v.Object)
	if err != nil {
		t.Fatal(err)
	}
	if snap["name"] != "old" {
		t.Fatalf("object snapshot = %+v", snap)
	}
	changes, _ := tr.Serializer.LoadChanges(v.ObjectChanges)
	if _, ok := changes["qty"]; ok {
		t.Fatalf("qty unchanged, must not appear: %+v", changes)
	}
	if c := changes["name"]; c.Old != "old" || c.New != "new" {
		t.Fatalf("name change = %+v", c)
	}
}

func TestRecordUpdateNoOpSkipped(t *testing.T) {
	tr := newTracker(Config{}, &fakeClock{})
	attrs := map[string]any{"name": "same"}
	v, err := tr.RecordUpdate("1", attrs, map[string]any{"name": "same"})
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Fatalf("no-op update must not create a version, got %+v", v)
	}
	vs, _ := tr.Versions("1")
	if len(vs) != 0 {
		t.Fatalf("expected 0 versions, got %d", len(vs))
	}
}

func TestRecordDestroy(t *testing.T) {
	tr := newTracker(Config{}, &fakeClock{})
	before := map[string]any{"name": "gone"}
	v, err := tr.RecordDestroy("1", before)
	if err != nil || v == nil {
		t.Fatalf("v=%v err=%v", v, err)
	}
	if v.Event != EventDestroy {
		t.Fatalf("event = %s", v.Event)
	}
	if v.ObjectChanges != "" {
		t.Fatalf("destroy must record no changeset, got %q", v.ObjectChanges)
	}
	snap, _ := tr.Serializer.LoadObject(v.Object)
	if snap["name"] != "gone" {
		t.Fatalf("destroy snapshot = %+v", snap)
	}
}

func TestRecordInferEvent(t *testing.T) {
	tr := newTracker(Config{}, &fakeClock{})

	if _, err := tr.Record("1", nil, nil); !errors.Is(err, ErrNoAttributes) {
		t.Fatalf("both-nil must error, got %v", err)
	}
	c, err := tr.Record("1", nil, map[string]any{"a": "1"})
	if err != nil || c.Event != EventCreate {
		t.Fatalf("infer create: %+v %v", c, err)
	}
	u, err := tr.Record("1", map[string]any{"a": "1"}, map[string]any{"a": "2"})
	if err != nil || u.Event != EventUpdate {
		t.Fatalf("infer update: %+v %v", u, err)
	}
	d, err := tr.Record("1", map[string]any{"a": "2"}, nil)
	if err != nil || d.Event != EventDestroy {
		t.Fatalf("infer destroy: %+v %v", d, err)
	}
}

func TestRequestDisabled(t *testing.T) {
	tr := newTracker(Config{}, &fakeClock{})
	tr.Request.Enabled = false
	v, err := tr.RecordCreate("1", map[string]any{"a": "1"})
	if err != nil || v != nil {
		t.Fatalf("disabled request must record nothing: v=%v err=%v", v, err)
	}
}

func TestOnFilter(t *testing.T) {
	tr := newTracker(Config{On: []string{EventCreate}}, &fakeClock{})
	if v, _ := tr.RecordCreate("1", map[string]any{"a": "1"}); v == nil {
		t.Fatal("create must be recorded when on: [create]")
	}
	if v, _ := tr.RecordUpdate("1", map[string]any{"a": "1"}, map[string]any{"a": "2"}); v != nil {
		t.Fatal("update must be skipped when on: [create]")
	}
}

func TestOnlyFilter(t *testing.T) {
	// only: [name] — a qty-only change must not version; a name change must.
	tr := newTracker(Config{Only: []string{"name"}}, &fakeClock{})
	if v, _ := tr.RecordUpdate("1", map[string]any{"name": "a", "qty": float64(1)}, map[string]any{"name": "a", "qty": float64(2)}); v != nil {
		t.Fatal("qty-only change must be skipped under only:[name]")
	}
	if v, _ := tr.RecordUpdate("1", map[string]any{"name": "a"}, map[string]any{"name": "b"}); v == nil {
		t.Fatal("name change must version under only:[name]")
	}
}

func TestIgnoreFilter(t *testing.T) {
	tr := newTracker(Config{Ignore: []string{"updated_at"}}, &fakeClock{})
	// Ignored-only change: no version.
	if v, _ := tr.RecordUpdate("1", map[string]any{"updated_at": "t0"}, map[string]any{"updated_at": "t1"}); v != nil {
		t.Fatal("ignored-only change must be skipped")
	}
	// Mixed change: version created, and the ignored attr is still serialized.
	v, _ := tr.RecordUpdate("1",
		map[string]any{"name": "a", "updated_at": "t0"},
		map[string]any{"name": "b", "updated_at": "t1"})
	if v == nil {
		t.Fatal("mixed change must version")
	}
	changes, _ := tr.Serializer.LoadChanges(v.ObjectChanges)
	if _, ok := changes["updated_at"]; !ok {
		t.Fatalf("ignored attr must still appear in changes: %+v", changes)
	}
}

func TestSkipFilter(t *testing.T) {
	tr := newTracker(Config{Skip: []string{"secret"}}, &fakeClock{})
	// Skip-only change: no version.
	if v, _ := tr.RecordUpdate("1", map[string]any{"secret": "x"}, map[string]any{"secret": "y"}); v != nil {
		t.Fatal("skip-only change must be skipped")
	}
	// Mixed: version created, skip attr excluded from object AND changes.
	v, _ := tr.RecordUpdate("1",
		map[string]any{"name": "a", "secret": "x"},
		map[string]any{"name": "b", "secret": "y"})
	if v == nil {
		t.Fatal("mixed change must version")
	}
	snap, _ := tr.Serializer.LoadObject(v.Object)
	if _, ok := snap["secret"]; ok {
		t.Fatalf("skip attr must be absent from object: %+v", snap)
	}
	changes, _ := tr.Serializer.LoadChanges(v.ObjectChanges)
	if _, ok := changes["secret"]; ok {
		t.Fatalf("skip attr must be absent from changes: %+v", changes)
	}
}

func TestReify(t *testing.T) {
	tr := newTracker(Config{}, &fakeClock{})
	// A create version has no prior state.
	cv, _ := tr.RecordCreate("1", map[string]any{"name": "a"})
	if got, err := tr.Reify(*cv); err != nil || got != nil {
		t.Fatalf("reify(create) = %v, %v; want nil, nil", got, err)
	}
	// An update version reifies to the pre-update snapshot.
	uv, _ := tr.RecordUpdate("1", map[string]any{"name": "a"}, map[string]any{"name": "b"})
	got, err := tr.Reify(*uv)
	if err != nil || got["name"] != "a" {
		t.Fatalf("reify(update) = %v, %v", got, err)
	}
}

func TestReifyUnversionedAttributes(t *testing.T) {
	tr := newTracker(Config{}, &fakeClock{})
	uv, _ := tr.RecordUpdate("1", map[string]any{"name": "a"}, map[string]any{"name": "b"})

	// Default: unversioned current attrs become nil.
	got, _ := tr.Reify(*uv, ReifyWithCurrent(map[string]any{"name": "b", "extra": "z"}))
	if got["name"] != "a" {
		t.Fatalf("versioned attr must win: %+v", got)
	}
	if v, ok := got["extra"]; !ok || v != nil {
		t.Fatalf("unversioned attr must be nil by default: %+v", got)
	}
	// Preserve: unversioned current attrs keep their value.
	got2, _ := tr.Reify(*uv,
		ReifyWithCurrent(map[string]any{"extra": "z"}),
		ReifyUnversioned(UnversionedPreserve))
	if got2["extra"] != "z" {
		t.Fatalf("preserve mode must keep current value: %+v", got2)
	}
}

func TestVersionNavigation(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	tr := newTracker(Config{}, clk)
	clk.advance()
	tr.RecordCreate("1", map[string]any{"n": float64(1)})
	clk.advance()
	tr.RecordUpdate("1", map[string]any{"n": float64(1)}, map[string]any{"n": float64(2)})
	clk.advance()
	tr.RecordUpdate("1", map[string]any{"n": float64(2)}, map[string]any{"n": float64(3)})

	vs, err := tr.Versions("1")
	if err != nil || len(vs) != 3 {
		t.Fatalf("versions: %d %v", len(vs), err)
	}

	// previous/next around the middle version.
	prev, _ := tr.PreviousVersion(vs[1])
	if prev == nil || prev.ID != vs[0].ID {
		t.Fatalf("previous = %+v", prev)
	}
	next, _ := tr.NextVersion(vs[1])
	if next == nil || next.ID != vs[2].ID {
		t.Fatalf("next = %+v", next)
	}
	// Ends: earliest has no previous, latest has no next.
	if p, _ := tr.PreviousVersion(vs[0]); p != nil {
		t.Fatalf("earliest must have no previous: %+v", p)
	}
	if n, _ := tr.NextVersion(vs[2]); n != nil {
		t.Fatalf("latest must have no next: %+v", n)
	}
}

func TestVersionAt(t *testing.T) {
	clk := &fakeClock{t: time.Unix(10, 0)}
	tr := newTracker(Config{}, clk)
	tr.RecordCreate("1", map[string]any{"n": float64(1)}) // at t=10
	clk.t = time.Unix(20, 0)
	tr.RecordUpdate("1", map[string]any{"n": float64(1)}, map[string]any{"n": float64(2)}) // at t=20

	// Between the two changes: reify the update version (state before it, n=1).
	attrs, live, err := tr.VersionAt("1", time.Unix(15, 0))
	if err != nil || live {
		t.Fatalf("mid: live=%v err=%v", live, err)
	}
	if attrs["n"] != float64(1) {
		t.Fatalf("version_at(15).n = %v, want 1", attrs["n"])
	}
	// After the last change: live record, no reification.
	attrs, live, err = tr.VersionAt("1", time.Unix(30, 0))
	if err != nil || !live || attrs != nil {
		t.Fatalf("after: attrs=%v live=%v err=%v", attrs, live, err)
	}
}

func TestLive(t *testing.T) {
	tr := newTracker(Config{}, &fakeClock{})
	// No versions yet: treated as live.
	if ok, _ := tr.Live("1"); !ok {
		t.Fatal("unversioned item must be live")
	}
	tr.RecordCreate("1", map[string]any{"n": float64(1)})
	if ok, _ := tr.Live("1"); !ok {
		t.Fatal("created item must be live")
	}
	tr.RecordDestroy("1", map[string]any{"n": float64(1)})
	if ok, _ := tr.Live("1"); ok {
		t.Fatal("destroyed item must not be live")
	}
}

func TestStoreSaveError(t *testing.T) {
	boom := errors.New("save boom")
	tr := New(Options{ItemType: "W", Store: errStore{saveErr: boom}, Clock: &fakeClock{}})
	if _, err := tr.RecordCreate("1", map[string]any{"a": "1"}); !errors.Is(err, boom) {
		t.Fatalf("save error must propagate, got %v", err)
	}
}

func TestStoreListError(t *testing.T) {
	boom := errors.New("list boom")
	tr := New(Options{ItemType: "W", Store: errStore{listErr: boom}, Clock: &fakeClock{}})
	if _, err := tr.Versions("1"); !errors.Is(err, boom) {
		t.Fatalf("Versions: %v", err)
	}
	if _, err := tr.PreviousVersion(Version{ItemID: "1", ID: 2}); !errors.Is(err, boom) {
		t.Fatalf("PreviousVersion: %v", err)
	}
	if _, err := tr.NextVersion(Version{ItemID: "1", ID: 2}); !errors.Is(err, boom) {
		t.Fatalf("NextVersion: %v", err)
	}
	if _, _, err := tr.VersionAt("1", time.Now()); !errors.Is(err, boom) {
		t.Fatalf("VersionAt: %v", err)
	}
	if _, err := tr.Live("1"); !errors.Is(err, boom) {
		t.Fatalf("Live: %v", err)
	}
}

func TestSerializerErrorBranches(t *testing.T) {
	tr := newTracker(Config{}, &fakeClock{})
	// DumpObject error: an unmarshalable value in the pre-update snapshot.
	if _, err := tr.RecordUpdate("1", badValue(), map[string]any{"f": "x"}); err == nil {
		t.Fatal("DumpObject must error on func value")
	}
	// DumpChanges error: create has an empty object (no DumpObject error) but the
	// changeset carries the unmarshalable value.
	if _, err := tr.RecordCreate("1", badValue()); err == nil {
		t.Fatal("DumpChanges must error on func value")
	}
	// LoadObject error in Reify.
	if _, err := tr.Reify(Version{Object: "{not json"}); err == nil {
		t.Fatal("Reify must propagate LoadObject error")
	}
}

func TestRecordCreateEmptyAttrs(t *testing.T) {
	// A create with no attributes yields a version whose changeset is empty
	// (buildChanges collapses to nil).
	tr := newTracker(Config{}, &fakeClock{})
	v, err := tr.RecordCreate("1", map[string]any{})
	if err != nil || v == nil {
		t.Fatalf("v=%v err=%v", v, err)
	}
	if v.ObjectChanges != "" {
		t.Fatalf("empty create changeset = %q, want empty", v.ObjectChanges)
	}
}

func TestRecordDestroyNilAttrs(t *testing.T) {
	// Destroying with no known attributes: the snapshot is absent (filterSkip
	// of a nil map is nil).
	tr := newTracker(Config{Skip: []string{"secret"}}, &fakeClock{})
	v, err := tr.RecordDestroy("1", nil)
	if err != nil || v == nil {
		t.Fatalf("v=%v err=%v", v, err)
	}
	if v.Object != "" {
		t.Fatalf("nil-attr destroy object = %q, want empty", v.Object)
	}
}

func TestJSONSerializerDirect(t *testing.T) {
	s := JSONSerializer{}
	// nil / empty round-trips.
	if out, _ := s.DumpObject(nil); out != "" {
		t.Fatalf("DumpObject(nil) = %q", out)
	}
	if m, _ := s.LoadObject(""); m != nil {
		t.Fatalf("LoadObject(\"\") = %v", m)
	}
	if out, _ := s.DumpChanges(nil); out != "" {
		t.Fatalf("DumpChanges(nil) = %q", out)
	}
	if m, _ := s.LoadChanges(""); m != nil {
		t.Fatalf("LoadChanges(\"\") = %v", m)
	}
	// Round-trip an object.
	str, err := s.DumpObject(map[string]any{"a": "b"})
	if err != nil {
		t.Fatal(err)
	}
	obj, _ := s.LoadObject(str)
	if !reflect.DeepEqual(obj, map[string]any{"a": "b"}) {
		t.Fatalf("object round-trip = %+v", obj)
	}
	// Round-trip a changeset.
	cstr, _ := s.DumpChanges(map[string]Change{"a": {Old: "1", New: "2"}})
	ch, _ := s.LoadChanges(cstr)
	if ch["a"].Old != "1" || ch["a"].New != "2" {
		t.Fatalf("changes round-trip = %+v", ch)
	}
	// Error branches on load.
	if _, err := s.LoadObject("{bad"); err == nil {
		t.Fatal("LoadObject must error on bad json")
	}
	if _, err := s.LoadChanges("{bad"); err == nil {
		t.Fatal("LoadChanges must error on bad json")
	}
	// Short changeset arrays: empty [] leaves Old/New nil; single-element sets Old.
	one, _ := s.LoadChanges(`{"a":[],"b":["x"]}`)
	if one["a"].Old != nil || one["a"].New != nil {
		t.Fatalf("empty pair must be nil/nil: %+v", one["a"])
	}
	if one["b"].Old != "x" || one["b"].New != nil {
		t.Fatalf("single pair = %+v", one["b"])
	}
}

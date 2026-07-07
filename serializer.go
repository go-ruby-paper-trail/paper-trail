// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

import "encoding/json"

// Serializer is the seam PaperTrail uses to (de)serialize the `object` snapshot
// and the `object_changes` changeset for storage. PaperTrail::Serializers::YAML
// is the gem's historical default (Rails text columns), but modern PaperTrail on
// Postgres/MySQL uses native JSON/JSONB columns via
// PaperTrail::Serializers::JSON. We adopt JSON as the default ([JSONSerializer]):
// it is stdlib-only (encoding/json, CGO=0), round-trips deterministically
// (map keys are emitted sorted), and matches the JSON-column deployment. The
// seam lets an rbgo binding plug a YAML serializer when a legacy YAML column
// must be matched byte-for-byte.
type Serializer interface {
	// DumpObject serializes an attribute snapshot. A nil map serializes to the
	// empty string (an absent snapshot, as for a create event).
	DumpObject(map[string]any) (string, error)
	// LoadObject is the inverse of DumpObject; the empty string loads to nil.
	LoadObject(string) (map[string]any, error)
	// DumpChanges serializes a {attr => [old, new]} changeset. A nil map
	// serializes to the empty string.
	DumpChanges(map[string]Change) (string, error)
	// LoadChanges is the inverse of DumpChanges; the empty string loads to nil.
	LoadChanges(string) (map[string]Change, error)
}

// Change is the [old, new] pair PaperTrail records per attribute in
// `object_changes`.
type Change struct {
	Old any
	New any
}

// JSONSerializer is the default [Serializer]; it stores snapshots and changesets
// as JSON, matching PaperTrail's JSON/JSONB columns.
type JSONSerializer struct{}

// DumpObject implements [Serializer].
func (JSONSerializer) DumpObject(m map[string]any) (string, error) {
	if m == nil {
		return "", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// LoadObject implements [Serializer].
func (JSONSerializer) LoadObject(s string) (map[string]any, error) {
	if s == "" {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// DumpChanges implements [Serializer]. The changeset is emitted as
// {"attr": [old, new]}, PaperTrail's on-disk shape.
func (JSONSerializer) DumpChanges(m map[string]Change) (string, error) {
	if m == nil {
		return "", nil
	}
	pairs := make(map[string][]any, len(m))
	for k, c := range m {
		pairs[k] = []any{c.Old, c.New}
	}
	b, err := json.Marshal(pairs)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// LoadChanges implements [Serializer].
func (JSONSerializer) LoadChanges(s string) (map[string]Change, error) {
	if s == "" {
		return nil, nil
	}
	var pairs map[string][]any
	if err := json.Unmarshal([]byte(s), &pairs); err != nil {
		return nil, err
	}
	m := make(map[string]Change, len(pairs))
	for k, p := range pairs {
		var c Change
		if len(p) > 0 {
			c.Old = p[0]
		}
		if len(p) > 1 {
			c.New = p[1]
		}
		m[k] = c
	}
	return m, nil
}

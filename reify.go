// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

// Unversioned-attribute handling for [Reify], mirroring PaperTrail's
// reify(unversioned_attributes:) option.
const (
	// UnversionedNil sets attributes present on the current record but absent
	// from the version's snapshot to nil (PaperTrail's default).
	UnversionedNil = "nil"
	// UnversionedPreserve keeps such attributes at their current value.
	UnversionedPreserve = "preserve"
)

type reifyConfig struct {
	current     map[string]any
	unversioned string
}

// ReifyOption customizes [Tracker.Reify].
type ReifyOption func(*reifyConfig)

// ReifyWithCurrent supplies the record's current attributes so that
// [Tracker.Reify] can decide how to handle attributes that were not versioned
// (see [ReifyUnversioned]).
func ReifyWithCurrent(current map[string]any) ReifyOption {
	return func(c *reifyConfig) { c.current = current }
}

// ReifyUnversioned selects how attributes present on the current record but
// absent from the snapshot are handled: [UnversionedNil] (default) or
// [UnversionedPreserve].
func ReifyUnversioned(mode string) ReifyOption {
	return func(c *reifyConfig) { c.unversioned = mode }
}

// Reify reconstructs the attribute state captured by v's `object` snapshot —
// the record as it was *before* v's change. A create version has no prior state
// and reifies to nil. When [ReifyWithCurrent] is supplied, attributes on the
// current record that were not versioned are set to nil, or preserved under
// [ReifyUnversioned](UnversionedPreserve).
func (t *Tracker) Reify(v Version, opts ...ReifyOption) (map[string]any, error) {
	cfg := reifyConfig{unversioned: UnversionedNil}
	for _, o := range opts {
		o(&cfg)
	}

	snap, err := t.Serializer.LoadObject(v.Object)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return nil, nil
	}

	out := make(map[string]any, len(snap))
	for k, val := range snap {
		out[k] = val
	}
	for k, cur := range cfg.current {
		if _, ok := out[k]; ok {
			continue
		}
		if cfg.unversioned == UnversionedPreserve {
			out[k] = cur
		} else {
			out[k] = nil
		}
	}
	return out, nil
}

// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

// Config mirrors the has_paper_trail options that govern *when* a version is
// recorded. It affects version triggering only; the skip list additionally
// excludes attributes from serialization.
type Config struct {
	// Only, when non-empty, restricts version-triggering to changes on these
	// attributes (has_paper_trail only:). Changes outside the list do not, on
	// their own, create an update version.
	Only []string
	// Ignore lists attributes whose changes do not, on their own, trigger an
	// update version (has_paper_trail ignore:). Ignored attributes are still
	// serialized if another attribute triggers the version.
	Ignore []string
	// Skip lists attributes excluded entirely from the `object` snapshot and the
	// `object_changes` changeset, and which never trigger a version
	// (has_paper_trail skip:).
	Skip []string
	// On restricts which lifecycle events are versioned (has_paper_trail
	// on: [...]). Empty means all of create/update/destroy.
	On []string
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// enabled reports whether the given event is versioned under this config.
func (c Config) enabled(event string) bool {
	if len(c.On) == 0 {
		return true
	}
	return contains(c.On, event)
}

// notable filters a set of changed attribute names down to those that should
// trigger an update version: skip and ignore are removed, and if Only is set the
// result is intersected with it.
func (c Config) notable(changed []string) []string {
	var out []string
	for _, k := range changed {
		if contains(c.Skip, k) {
			continue
		}
		if contains(c.Ignore, k) {
			continue
		}
		if len(c.Only) > 0 && !contains(c.Only, k) {
			continue
		}
		out = append(out, k)
	}
	return out
}

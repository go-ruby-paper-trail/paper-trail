// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

import (
	"reflect"
	"sort"
)

// changedKeys returns the sorted set of attribute names whose value differs
// between before and after (present-in-one-only counts as changed). It is the
// raw diff, before any only/ignore/skip filtering.
func changedKeys(before, after map[string]any) []string {
	seen := map[string]bool{}
	var keys []string
	for k := range before {
		seen[k] = true
	}
	for k := range after {
		seen[k] = true
	}
	for k := range seen {
		bv, bok := before[k]
		av, aok := after[k]
		if bok != aok || !reflect.DeepEqual(bv, av) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// buildChanges computes the {attr => [old, new]} changeset over the given
// changed keys, dropping any skipped attribute.
func buildChanges(before, after map[string]any, changed, skip []string) map[string]Change {
	out := map[string]Change{}
	for _, k := range changed {
		if contains(skip, k) {
			continue
		}
		out[k] = Change{Old: before[k], New: after[k]}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// filterSkip returns a copy of attrs with skipped keys removed. A nil input
// yields nil (an absent snapshot).
func filterSkip(attrs map[string]any, skip []string) map[string]any {
	if attrs == nil {
		return nil
	}
	out := make(map[string]any, len(attrs))
	for k, v := range attrs {
		if contains(skip, k) {
			continue
		}
		out[k] = v
	}
	return out
}

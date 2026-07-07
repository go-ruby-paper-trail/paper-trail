// Copyright (c) 2026, the go-ruby-paper-trail/paper-trail authors
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the conditions of the BSD 3-Clause
// License (see the LICENSE file) are met.

package papertrail

import "sort"

// Store is the persistence seam for versions. In production an rbgo binding
// backs it with ActiveRecord (the `versions` table); tests use [MemoryStore].
type Store interface {
	// SaveVersion persists v, assigns and returns it with its ID populated.
	SaveVersion(v Version) (Version, error)
	// VersionsFor returns every stored version for the given item, ordered
	// oldest-first (ascending ID).
	VersionsFor(itemType, itemID string) ([]Version, error)
}

// MemoryStore is an in-memory [Store] for tests and non-persistent hosts. It is
// safe for the sequential access the tests perform.
type MemoryStore struct {
	seq      int64
	versions []Version
}

// NewMemoryStore returns an empty [MemoryStore].
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

// SaveVersion implements [Store], assigning a monotonically increasing ID.
func (s *MemoryStore) SaveVersion(v Version) (Version, error) {
	s.seq++
	v.ID = s.seq
	s.versions = append(s.versions, v)
	return v, nil
}

// VersionsFor implements [Store].
func (s *MemoryStore) VersionsFor(itemType, itemID string) ([]Version, error) {
	var out []Version
	for _, v := range s.versions {
		if v.ItemType == itemType && v.ItemID == itemID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

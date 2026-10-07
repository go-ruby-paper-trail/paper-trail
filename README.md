<p align="center"><img src="https://go-ruby-paper-trail.github.io/logo.png" alt="go-ruby-paper-trail/paper-trail" width="720"></p>

# paper-trail — go-ruby-paper-trail

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-paper-trail.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27.1%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the deterministic core of Ruby's
[`paper_trail`](https://github.com/paper-trail-gem/paper_trail) gem** — the model
versioning / audit engine. It records a `Version` on every create, update and
destroy, computes the per-attribute changeset, reifies past model states,
navigates an item's version history, and resolves the whodunnit — **without any
Ruby runtime**.

It is the audit/versioning engine for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but a
**standalone, reusable** module — a sibling of
[go-ruby-openbao](https://github.com/go-ruby-openbao/openbao).

> **What it is — and isn't.** Everything `paper_trail` does *around* persistence
> is deterministic and needs **no interpreter**, so it lives here as pure Go:
> detecting the event, building the `object` snapshot of the attributes *before*
> the change, computing the `object_changes` changeset (`{attr => [old, new]}`)
> from a before/after attribute map, applying the `only` / `ignore` / `skip` /
> `on` filters, reifying a past state, and resolving the whodunnit. The **model
> attribute snapshot** is provided by the host as a plain `map[string]any`
> (before and after). The **version store is a host seam** (`Store`), wired to
> ActiveRecord in the binding; the **clock** (`Clock`) and the **request
> context** (`RequestContext`, i.e. `PaperTrail.request.whodunnit`) are injected.
> **Serialization is a seam** too: the default `JSONSerializer` matches modern
> PaperTrail JSON/JSONB columns. Tests use an in-memory store, an injected clock
> and a fixed whodunnit — the core opens no database of its own. A future rbgo
> binding wires the seams to Ruby.

## Features

Faithful port of the `paper_trail` versioning core:

- **Version model** — `Version{ItemType, ItemID, Event, Whodunnit, Object,
  ObjectChanges, CreatedAt}`, one-to-one with the gem's `versions` table.
- **Recording** — `Tracker.RecordCreate` / `RecordUpdate` / `RecordDestroy`, or
  `Record` which infers the event from the presence of the before/after maps.
  `object` snapshots the state *before* the change (empty on create); an update
  with no notable change records nothing (the no-op skip); destroy records no
  changeset (matching the gem).
- **Changeset** — `object_changes` is the `{attr => [old, new]}` diff of the
  before/after attribute maps.
- **Filters** — `Config{Only, Ignore, Skip, On}` mirrors `has_paper_trail
  only:/ignore:/skip:/on:`: `only`/`ignore` govern which changes trigger a
  version, `skip` additionally excludes attributes from serialization, `on`
  restricts the versioned lifecycle events.
- **Reify** — `Tracker.Reify(version, opts...)` reconstructs the pre-change
  attributes; `ReifyWithCurrent` + `ReifyUnversioned` reproduce
  `reify(unversioned_attributes: :nil | :preserve)`.
- **Querying** — `Versions`, `VersionAt(t)` (with a `live` result), 
  `PreviousVersion` / `NextVersion`, and `Live` (`live?`).
- **Whodunnit** — `RequestContext{Whodunnit, Enabled}` is the
  `PaperTrail.request` seam; `Enabled = false` suppresses versioning.

## Usage

```go
tr := papertrail.New(papertrail.Options{ItemType: "Widget"})
tr.Request.Whodunnit = "alice@example.com"

// create
tr.RecordCreate("42", map[string]any{"name": "gadget", "price": 9.99})

// update — records the pre-change snapshot + the {attr:[old,new]} diff
v, _ := tr.RecordUpdate("42",
    map[string]any{"name": "gadget", "price": 9.99},
    map[string]any{"name": "gadget", "price": 12.50})

// reconstruct the state before that update
prev, _ := tr.Reify(*v) // => {"name":"gadget","price":9.99}
```

## Ruby surface (rbgo binding)

The seams above back the familiar `paper_trail` API:
`has_paper_trail(only:, ignore:, skip:, on:)`, `model.versions`,
`version.reify(...)`, `version.object` / `version.object_changes`,
`model.paper_trail.previous_version` / `.next_version` / `.live?`,
`Model.paper_trail.version_at(t)`, and `PaperTrail.request.whodunnit`.

## Tests & coverage

`go test -race` with a **100% statement-coverage gate**, on three host OSes,
across six 64-bit architectures (amd64/arm64 native, riscv64/loong64/ppc64le/s390x
under qemu), plus `js/wasm` and `wasip1/wasm` build checks — all `CGO_ENABLED=0`.

## License

BSD-3-Clause — see [LICENSE](LICENSE).

// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-hiera/hiera authors

// Package hiera is a pure-Go (no cgo) adapter that presents the Ruby Hiera API
// surface over the hierarchical data-lookup engine github.com/go-hiera/hiera.
//
// The go-hiera engine already implements everything that is Hiera semantics:
// parsing hiera.yaml, walking the configured hierarchy, the four merge
// behaviours (first / unique / hash / deep), per-key lookup_options, dotted-key
// digging into structured data and the full %{...} interpolation grammar. This
// package does not reimplement any of that; it wraps the engine and adds the
// thin, Ruby-facing conveniences the engine deliberately omits so that a
// consumer such as go-embedded-ruby (rbgo) can expose a Ruby "Hiera" class:
//
//   - a ResolutionType enum mirroring Ruby's resolution_type / Puppet's lookup
//     merge behaviours (:priority, :array, :hash, :deep), mapped onto the
//     engine's MergeStrategy;
//   - a Hiera value that binds one engine to one scope, constructed either from
//     a hiera.yaml on disk ([New]) or from inline configuration bytes
//     ([NewFromConfig]);
//   - a LookupOptions struct carrying the Ruby lookup() knobs (merge override,
//     default value, per-key default hash);
//   - [NewFactScope], which builds a [Scope] from Puppet-style fact namespaces,
//     placing facts both at top level (legacy %{osfamily}) and under a "facts"
//     key (%{facts.os.family}), plus trusted data under "trusted".
//
// The package has no dependency on any Ruby runtime: the surface is Go-typed,
// and a Ruby binding layer marshals Ruby values onto these Go types.
package hiera

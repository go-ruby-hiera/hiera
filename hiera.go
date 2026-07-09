// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-hiera/hiera authors

package hiera

import (
	gohiera "github.com/go-hiera/hiera"
)

// Value is any value a lookup can yield; it mirrors the engine's Value (= any).
type Value = gohiera.Value

// Scope is the pluggable variable provider interpolation resolves against. It
// is re-exported so callers depend on a single import.
type Scope = gohiera.Scope

// MapScope is a [Scope] backed by a nested map of variables, re-exported from
// the engine so callers depend on a single import.
type MapScope = gohiera.MapScope

// ResolutionType mirrors Ruby Hiera's resolution_type and Puppet's lookup merge
// behaviour, selecting how values found at several hierarchy levels combine.
type ResolutionType int

const (
	// Priority (:priority) returns the highest-priority value found.
	Priority ResolutionType = iota
	// Unique (:array) flattens every found array/scalar into one deduplicated
	// array.
	Unique
	// Hash (:hash) shallow-merges every found hash, higher priority winning.
	Hash
	// Deep (:deep) recursively merges every found hash, higher priority winning.
	Deep
)

// mergeStrategy maps a ResolutionType onto the engine's *MergeStrategy. An
// unrecognised value degrades to Priority (MergeFirst), matching Hiera's
// default merge behaviour.
func (rt ResolutionType) mergeStrategy() *gohiera.MergeStrategy {
	switch rt {
	case Priority:
		return &gohiera.MergeStrategy{Kind: gohiera.MergeFirst}
	case Unique:
		return &gohiera.MergeStrategy{Kind: gohiera.MergeUnique}
	case Hash:
		return &gohiera.MergeStrategy{Kind: gohiera.MergeHash}
	case Deep:
		return &gohiera.MergeStrategy{Kind: gohiera.MergeDeep}
	default:
		return &gohiera.MergeStrategy{Kind: gohiera.MergeFirst}
	}
}

// LookupOptions carries the knobs of a single Ruby-style lookup() call. A nil
// Merge means "let the configuration and per-key lookup_options decide"; a
// non-nil Merge overrides the merge behaviour for this call only.
type LookupOptions struct {
	// Merge, when non-nil, overrides the merge behaviour for this call.
	Merge *ResolutionType
	// Default supplies the value returned when the key is not found.
	Default Value
	// HasDefault enables Default (so a nil default is distinguishable).
	HasDefault bool
	// DefaultValuesHash supplies per-key defaults consulted, by looked-up root
	// key, when the key is not found and HasDefault is false.
	DefaultValuesHash map[string]any
}

// Hiera is the Ruby Hiera class surface: one configured go-hiera engine bound
// to one scope.
type Hiera struct {
	engine *gohiera.Hiera
	scope  Scope
}

// New loads the hiera.yaml at configPath and returns a Hiera resolving
// interpolation against scope. The configuration's directory is the base for
// relative datadirs. It returns an error if the file cannot be read or parsed.
func New(configPath string, scope Scope) (*Hiera, error) {
	engine, err := gohiera.Load(configPath, scope)
	if err != nil {
		return nil, err
	}
	return &Hiera{engine: engine, scope: scope}, nil
}

// NewFromConfig builds a Hiera from inline hiera.yaml bytes, using baseDir as
// the base directory for relative datadirs, resolving interpolation against
// scope. It returns an error if the bytes cannot be parsed.
func NewFromConfig(configYAML []byte, baseDir string, scope Scope) (*Hiera, error) {
	cfg, err := gohiera.ParseConfig(configYAML, baseDir)
	if err != nil {
		return nil, err
	}
	return &Hiera{engine: gohiera.New(cfg, scope), scope: scope}, nil
}

// Lookup resolves key and returns its value, whether it was found, and any
// error. key may be dotted (e.g. "profile.ntp.servers.0") to dig into
// structured data after the root key is looked up and merged.
func (h *Hiera) Lookup(key string, opts LookupOptions) (Value, bool, error) {
	engOpts := &gohiera.Options{
		Default:           opts.Default,
		HasDefault:        opts.HasDefault,
		DefaultValuesHash: opts.DefaultValuesHash,
	}
	if opts.Merge != nil {
		engOpts.Merge = opts.Merge.mergeStrategy()
	}
	return h.engine.Lookup(key, engOpts)
}

// Config returns the engine's parsed configuration.
func (h *Hiera) Config() *gohiera.Config { return h.engine.Config() }

// RegisterDataHash registers (or replaces) a custom data_hash backend under
// name, which a hierarchy level selects via "data_hash: name". This is the seam
// for eyaml/hocon and other Ruby Hiera backends.
func (h *Hiera) RegisterDataHash(name string, fn func(data []byte, path string) (map[string]any, error)) {
	h.engine.RegisterDataHash(name, fn)
}

// NewFactScope builds a [Scope] from Puppet-style fact namespaces. Each fact is
// placed both at top level (so legacy references such as %{osfamily} resolve)
// and under a "facts" key (so structured references such as %{facts.os.family}
// resolve); trusted data is placed under "trusted". facts or trusted may be
// nil.
func NewFactScope(facts, trusted map[string]any) Scope {
	s := MapScope{}
	for k, v := range facts {
		s[k] = v
	}
	s["facts"] = mapCopy(facts)
	s["trusted"] = mapCopy(trusted)
	return s
}

// mapCopy returns a shallow copy of m as a map[string]any, or an empty map when
// m is nil, so the scope always carries a mapping under facts/trusted.
func mapCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

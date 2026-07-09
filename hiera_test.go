// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2026, the go-ruby-hiera/hiera authors

package hiera

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gohiera "github.com/go-hiera/hiera"
)

// writeFixture materialises a hiera.yaml plus data files in a fresh temp dir
// and returns the path to hiera.yaml.
func writeFixture(t *testing.T, hieraYAML string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "hiera.yaml")
	if err := os.WriteFile(cfgPath, []byte(hieraYAML), 0o644); err != nil {
		t.Fatalf("write hiera.yaml: %v", err)
	}
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return cfgPath
}

// mergeFixture is a two-level hierarchy where a scalar, an array and a hash key
// are present at both levels so the four merge behaviours diverge.
const mergeConfig = `version: 5
defaults:
  datadir: data
  data_hash: yaml_data
hierarchy:
  - name: "Node"
    path: "node.yaml"
  - name: "Common"
    path: "common.yaml"
`

var mergeFiles = map[string]string{
	"data/node.yaml": `simple: "node-value"
arr:
  - a
  - b
hsh:
  x: 1
  nested:
    p: 10
interp: "%{facts.os.family}-%{role}"
`,
	"data/common.yaml": `simple: "common-value"
arr:
  - b
  - c
hsh:
  y: 2
  nested:
    q: 20
`,
}

func TestNewHappyPath(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	v, found, err := h.Lookup("simple", LookupOptions{})
	if err != nil || !found {
		t.Fatalf("Lookup(simple) = %v, %v, %v", v, found, err)
	}
	if v != "node-value" {
		t.Fatalf("simple = %v, want node-value", v)
	}
}

func TestNewErrorMissingConfig(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	if _, err := New(missing, MapScope{}); err == nil {
		t.Fatal("New with missing config: want error, got nil")
	}
}

func TestNewFromConfigHappyPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "common.yaml"), []byte("key: value\n"), 0o644); err != nil {
		t.Fatalf("write common.yaml: %v", err)
	}
	cfgYAML := []byte(`version: 5
defaults:
  datadir: data
  data_hash: yaml_data
hierarchy:
  - name: "Common"
    path: "common.yaml"
`)
	h, err := NewFromConfig(cfgYAML, dir, MapScope{})
	if err != nil {
		t.Fatalf("NewFromConfig: %v", err)
	}
	v, found, err := h.Lookup("key", LookupOptions{})
	if err != nil || !found || v != "value" {
		t.Fatalf("Lookup(key) = %v, %v, %v", v, found, err)
	}
}

func TestNewFromConfigError(t *testing.T) {
	// A top-level sequence is valid YAML but not a valid hiera.yaml mapping,
	// so ParseConfig returns an error.
	if _, err := NewFromConfig([]byte("- a\n- b\n"), t.TempDir(), MapScope{}); err == nil {
		t.Fatal("NewFromConfig with non-mapping config: want error, got nil")
	}
}

func TestLookupNotFound(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	v, found, err := h.Lookup("no-such-key", LookupOptions{})
	if err != nil {
		t.Fatalf("Lookup error: %v", err)
	}
	if found || v != nil {
		t.Fatalf("Lookup(no-such-key) = %v, %v, want nil,false", v, found)
	}
}

func TestLookupDefault(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	v, found, err := h.Lookup("absent", LookupOptions{Default: "fallback", HasDefault: true})
	if err != nil || !found {
		t.Fatalf("Lookup with default = %v, %v, %v", v, found, err)
	}
	if v != "fallback" {
		t.Fatalf("default = %v, want fallback", v)
	}
}

func TestLookupDefaultValuesHash(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	v, found, err := h.Lookup("absent", LookupOptions{
		DefaultValuesHash: map[string]any{"absent": "from-hash"},
	})
	if err != nil || !found {
		t.Fatalf("Lookup with DefaultValuesHash = %v, %v, %v", v, found, err)
	}
	if v != "from-hash" {
		t.Fatalf("DefaultValuesHash value = %v, want from-hash", v)
	}
}

func TestLookupMergePriority(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rt := Priority
	v, found, err := h.Lookup("arr", LookupOptions{Merge: &rt})
	if err != nil || !found {
		t.Fatalf("Lookup(arr, priority) = %v, %v, %v", v, found, err)
	}
	want := []any{"a", "b"}
	if !reflect.DeepEqual(v, want) {
		t.Fatalf("priority arr = %v, want %v", v, want)
	}
}

func TestLookupMergeUnique(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rt := Unique
	v, found, err := h.Lookup("arr", LookupOptions{Merge: &rt})
	if err != nil || !found {
		t.Fatalf("Lookup(arr, unique) = %v, %v, %v", v, found, err)
	}
	got, ok := v.([]any)
	if !ok {
		t.Fatalf("unique arr type = %T, want []any", v)
	}
	// Unique flattens both levels and deduplicates the shared "b".
	set := map[any]bool{}
	for _, e := range got {
		set[e] = true
	}
	for _, want := range []any{"a", "b", "c"} {
		if !set[want] {
			t.Fatalf("unique arr %v missing %v", got, want)
		}
	}
	if len(got) != 3 {
		t.Fatalf("unique arr = %v, want 3 unique elements", got)
	}
}

func TestLookupMergeHash(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rt := Hash
	v, found, err := h.Lookup("hsh", LookupOptions{Merge: &rt})
	if err != nil || !found {
		t.Fatalf("Lookup(hsh, hash) = %v, %v, %v", v, found, err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("hash hsh type = %T, want map", v)
	}
	// Shallow hash merge unions the top-level keys x and y.
	if m["x"] == nil || m["y"] == nil {
		t.Fatalf("hash hsh = %v, want both x and y", m)
	}
	// Shallow merge keeps only the higher-priority nested map (p, not q).
	nested, ok := m["nested"].(map[string]any)
	if !ok {
		t.Fatalf("hash hsh.nested type = %T", m["nested"])
	}
	if _, hasQ := nested["q"]; hasQ {
		t.Fatalf("shallow hash merge leaked deep key q: %v", nested)
	}
}

func TestLookupMergeDeep(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rt := Deep
	v, found, err := h.Lookup("hsh", LookupOptions{Merge: &rt})
	if err != nil || !found {
		t.Fatalf("Lookup(hsh, deep) = %v, %v, %v", v, found, err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("deep hsh type = %T, want map", v)
	}
	nested, ok := m["nested"].(map[string]any)
	if !ok {
		t.Fatalf("deep hsh.nested type = %T", m["nested"])
	}
	// Deep merge combines both nested maps (p and q).
	if nested["p"] == nil || nested["q"] == nil {
		t.Fatalf("deep hsh.nested = %v, want both p and q", nested)
	}
}

func TestLookupInterpolation(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	scope := NewFactScope(
		map[string]any{
			"os":   map[string]any{"family": "Debian"},
			"role": "web",
		},
		nil,
	)
	h, err := New(cfg, scope)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	v, found, err := h.Lookup("interp", LookupOptions{})
	if err != nil || !found {
		t.Fatalf("Lookup(interp) = %v, %v, %v", v, found, err)
	}
	// %{facts.os.family} resolves under the facts namespace; %{role} resolves
	// via the legacy top-level placement.
	if v != "Debian-web" {
		t.Fatalf("interp = %v, want Debian-web", v)
	}
}

func TestNewFactScopeTrusted(t *testing.T) {
	scope := NewFactScope(
		map[string]any{"osfamily": "RedHat"},
		map[string]any{"certname": "node1.example.com"},
	)
	if v, ok := scope.Lookup("osfamily"); !ok || v != "RedHat" {
		t.Fatalf("legacy fact osfamily = %v, %v", v, ok)
	}
	if v, ok := scope.Lookup("facts.osfamily"); !ok || v != "RedHat" {
		t.Fatalf("facts.osfamily = %v, %v", v, ok)
	}
	if v, ok := scope.Lookup("trusted.certname"); !ok || v != "node1.example.com" {
		t.Fatalf("trusted.certname = %v, %v", v, ok)
	}
}

func TestRegisterDataHash(t *testing.T) {
	cfg := writeFixture(t, `version: 5
defaults:
  datadir: data
hierarchy:
  - name: "Custom"
    path: "custom.dat"
    data_hash: custom_backend
`, map[string]string{
		"data/custom.dat": "hello-payload",
	})
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h.RegisterDataHash("custom_backend", func(data []byte, path string) (map[string]any, error) {
		return map[string]any{"custom_key": strings.TrimSpace(string(data))}, nil
	})
	v, found, err := h.Lookup("custom_key", LookupOptions{})
	if err != nil || !found {
		t.Fatalf("Lookup(custom_key) = %v, %v, %v", v, found, err)
	}
	if v != "hello-payload" {
		t.Fatalf("custom_key = %v, want hello-payload", v)
	}
}

func TestConfigAccessor(t *testing.T) {
	cfg := writeFixture(t, mergeConfig, mergeFiles)
	h, err := New(cfg, MapScope{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c := h.Config()
	if c == nil {
		t.Fatal("Config() = nil")
	}
	if c.Version != 5 {
		t.Fatalf("Config().Version = %d, want 5", c.Version)
	}
	if len(c.Hierarchy) != 2 {
		t.Fatalf("Config().Hierarchy len = %d, want 2", len(c.Hierarchy))
	}
}

func TestMergeStrategyMapping(t *testing.T) {
	cases := []struct {
		rt   ResolutionType
		kind gohiera.MergeKind
	}{
		{Priority, gohiera.MergeFirst},
		{Unique, gohiera.MergeUnique},
		{Hash, gohiera.MergeHash},
		{Deep, gohiera.MergeDeep},
		{ResolutionType(42), gohiera.MergeFirst}, // unknown -> default (first)
	}
	for _, tc := range cases {
		got := tc.rt.mergeStrategy()
		if got == nil {
			t.Fatalf("mergeStrategy(%d) = nil", tc.rt)
		}
		if got.Kind != tc.kind {
			t.Fatalf("mergeStrategy(%d).Kind = %v, want %v", tc.rt, got.Kind, tc.kind)
		}
	}
}

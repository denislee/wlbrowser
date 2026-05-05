package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Home == "" || cfg.Width == 0 {
		t.Fatalf("defaults not populated: %+v", cfg)
	}
	if got := cfg.Keys["normal"]["j"].Name; got != "scroll-down" {
		t.Errorf(`keys.normal.j = %q, want "scroll-down"`, got)
	}
	yy := cfg.Keys["normal"]["yy"]
	if yy.Name != "copy-url" {
		t.Errorf("yy binding malformed: %+v", yy)
	}
}

func TestUserOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	user := `
home = "https://example.com"
[keys.normal]
"j" = "scroll-up"
"<C-q>" = { action = "exec", command = ["true"] }
`
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Home != "https://example.com" {
		t.Errorf("home not overridden: %q", cfg.Home)
	}
	if cfg.Keys["normal"]["j"].Name != "scroll-up" {
		t.Errorf("j override lost: %+v", cfg.Keys["normal"]["j"])
	}
	if cfg.Keys["normal"]["k"].Name != "scroll-up" {
		t.Errorf("default k binding lost: %+v", cfg.Keys["normal"]["k"])
	}
	cq := cfg.Keys["normal"]["<C-q>"]
	if cq.Name != "exec" || len(cq.Command) != 1 {
		t.Errorf("table-form override malformed: %+v", cq)
	}
}

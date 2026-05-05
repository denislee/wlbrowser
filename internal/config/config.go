// Package config loads wlbrowser's TOML configuration. Defaults are baked in
// via go:embed and merged with any user-provided override file.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

//go:embed default.toml
var defaultTOML []byte

// DefaultTOML returns the embedded default configuration bytes (used by
// --write-config and as the baseline for merge).
func DefaultTOML() []byte { return defaultTOML }

// Config is the merged result of defaults + user overrides.
type Config struct {
	Home          string                  `toml:"home"`
	Title         string                  `toml:"title"`
	Width         int                     `toml:"width"`
	Height        int                     `toml:"height"`
	SeqTimeoutMs  int                     `toml:"seq_timeout_ms"`
	Keys          map[string]BindingTable `toml:"keys"`
}

// BindingTable maps a vim-style key string to an Action.
type BindingTable map[string]Action

// Action is the side effect attached to a key binding. Built-in actions are
// referenced by name (e.g. "scroll-down"); free-form actions like exec/js/open-url
// carry extra fields.
type Action struct {
	Name    string   `toml:"action"`
	URL     string   `toml:"url"`
	Script  string   `toml:"script"`
	Command []string `toml:"command"`
	Stdin   string   `toml:"stdin"`
}

// UnmarshalTOML lets a binding value be either a bare string ("scroll-down")
// or a table ({action = "exec", command = [...]}).
func (a *Action) UnmarshalTOML(v any) error {
	switch x := v.(type) {
	case string:
		a.Name = x
		return nil
	case map[string]any:
		if name, ok := x["action"].(string); ok {
			a.Name = name
		} else {
			return errors.New("action table missing `action` key")
		}
		if u, ok := x["url"].(string); ok {
			a.URL = u
		}
		if s, ok := x["script"].(string); ok {
			a.Script = s
		}
		if s, ok := x["stdin"].(string); ok {
			a.Stdin = s
		}
		if c, ok := x["command"].([]any); ok {
			for _, item := range c {
				s, ok := item.(string)
				if !ok {
					return fmt.Errorf("command entry %v is not a string", item)
				}
				a.Command = append(a.Command, s)
			}
		}
		return nil
	default:
		return fmt.Errorf("expected string or table, got %T", v)
	}
}

// Load reads the embedded defaults, then layers the user file on top if it
// exists. A missing user file is not an error. Pass an empty string for path
// to use the default location ($XDG_CONFIG_HOME/wlbrowser/config.toml).
func Load(path string) (*Config, error) {
	cfg := &Config{Keys: map[string]BindingTable{}}
	if _, err := toml.Decode(string(defaultTOML), cfg); err != nil {
		return nil, fmt.Errorf("default config: %w", err)
	}
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	user := &Config{Keys: map[string]BindingTable{}}
	if _, err := toml.Decode(string(data), user); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	merge(cfg, user)
	return cfg, nil
}

// Save writes the current config to the specified path (or default path if empty).
func (cfg *Config) Save(path string) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func merge(dst, src *Config) {
	if src.Home != "" {
		dst.Home = src.Home
	}
	if src.Title != "" {
		dst.Title = src.Title
	}
	if src.Width != 0 {
		dst.Width = src.Width
	}
	if src.Height != 0 {
		dst.Height = src.Height
	}
	if src.SeqTimeoutMs != 0 {
		dst.SeqTimeoutMs = src.SeqTimeoutMs
	}
	for mode, bindings := range src.Keys {
		dstMode, ok := dst.Keys[mode]
		if !ok {
			dstMode = BindingTable{}
			dst.Keys[mode] = dstMode
		}
		for k, v := range bindings {
			dstMode[k] = v
		}
	}
}

// DefaultPath returns the canonical config file location.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "wlbrowser", "config.toml"), nil
}

// WriteDefault writes the embedded defaults to path, creating parent dirs.
// Refuses to overwrite an existing file.
func WriteDefault(path string) error {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; not overwriting", path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, defaultTOML, 0o644)
}

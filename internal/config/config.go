// Package config loads and persists mytunes settings from an XDG TOML file.
// On first run an empty config is written; the values are filled in by the
// setup wizard (see internal/tui) or by editing the file directly.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config mirrors ~/.config/mytunes/config.toml. No secrets are stored; SSH auth
// is delegated entirely to the user's ssh binary.
type Config struct {
	RemoteUser     string   `toml:"remote_user"`
	RemoteHost     string   `toml:"remote_host"`
	RemotePath     string   `toml:"remote_path"`
	LastDest       string   `toml:"last_dest"`
	RsyncExtraArgs []string `toml:"rsync_extra_args"`

	path string // where this config was loaded from / will be saved
}

// defaults is an empty config; required fields are collected by the setup
// wizard on first run (see Config.NeedsSetup).
func defaults() Config {
	return Config{
		RsyncExtraArgs: []string{},
	}
}

// Remote returns the user@host SSH target.
func (c Config) Remote() string { return c.RemoteUser + "@" + c.RemoteHost }

// NeedsSetup reports whether required fields are unset, meaning the setup
// wizard should run (first launch, or an incomplete config file).
func (c Config) NeedsSetup() bool { return c.RemoteHost == "" || c.RemotePath == "" }

// Path returns the config file path that was resolved for this Config.
func (c Config) Path() string { return c.path }

// configPath resolves $XDG_CONFIG_HOME/mytunes/config.toml (falling back to
// ~/.config).
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mytunes", "config.toml"), nil
}

// Load reads the config file, creating it with defaults on first run.
func Load() (Config, error) {
	p, err := configPath()
	if err != nil {
		return Config{}, err
	}

	c := defaults()
	c.path = p

	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		if err := c.Save(); err != nil {
			return c, err
		}
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if _, err := toml.Decode(string(data), &c); err != nil {
		return c, err
	}
	c.path = p
	return c, nil
}

// Save writes the config back to its file, creating parent dirs as needed.
func (c Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

// Overrides are CLI flag values that take precedence over the file. Empty
// strings leave the loaded value untouched.
type Overrides struct {
	Remote string // user@host
	Path   string // remote music path
	Dest   string // destination dir
}

// Apply layers non-empty overrides onto the config and returns it.
func (c Config) Apply(o Overrides) Config {
	if o.Remote != "" {
		c.RemoteUser, c.RemoteHost = splitRemote(o.Remote)
	}
	if o.Path != "" {
		c.RemotePath = o.Path
	}
	if o.Dest != "" {
		c.LastDest = o.Dest
	}
	return c
}

func splitRemote(s string) (user, host string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			return s[:i], s[i+1:]
		}
	}
	return "", s
}

package settings

import (
	"errors"
	"io/fs"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// ParseTOML parses TOML configuration into a [Settings] seeded with defaults.
func ParseTOML(data []byte) (Settings, []string, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Settings{}, nil, err
	}
	return Parse(raw)
}

// Load reads and parses the TOML config file at path.
// A missing file yields defaults without error.
func Load(path string) (Settings, []string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil, nil
	}
	if err != nil {
		return Default(), nil, err
	}
	return ParseTOML(data)
}

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ServerConfig represents the TOML config for a single server directory.
type ServerConfig struct {
	ServerID string   `toml:"server_id"`
	Worlds   []string `toml:"worlds"`
	Name     string   `toml:"name"`
}

// LoadedServer holds a parsed config along with its directory path.
type LoadedServer struct {
	Dir    string
	Config ServerConfig
}

// LoadAll scans the given base directory for subdirectories containing a
// config.toml and returns all parsed configs.
func LoadAll(baseDir string) ([]LoadedServer, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("reading base directory %s: %w", baseDir, err)
	}

	var servers []LoadedServer
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		configPath := filepath.Join(baseDir, entry.Name(), "config.toml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		var cfg ServerConfig
		if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", configPath, err)
		}

		if cfg.ServerID == "" {
			return nil, fmt.Errorf("%s: server_id is required", configPath)
		}
		if len(cfg.Worlds) == 0 {
			return nil, fmt.Errorf("%s: at least one world is required", configPath)
		}

		servers = append(servers, LoadedServer{
			Dir:    filepath.Join(baseDir, entry.Name()),
			Config: cfg,
		})
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("no config.toml found in any subdirectory of %s", baseDir)
	}

	return servers, nil
}

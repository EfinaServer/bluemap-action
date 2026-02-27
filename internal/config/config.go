package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	ServerTypeVanilla = "vanilla"
	ServerTypePlugin  = "plugin"

	// DownloadMode constants control how the backup is downloaded.
	DownloadModeAuto     = "auto"     // Probe the server and choose the best mode.
	DownloadModeParallel = "parallel" // Force parallel multi-connection download.
	DownloadModeSingle   = "single"   // Force single-connection streaming download.
)

// ServerConfig represents the TOML config for a single server directory.
type ServerConfig struct {
	ServerID            string `toml:"server_id"`
	ServerType          string `toml:"server_type"`
	WorldName           string `toml:"world_name"`
	Name                string `toml:"name"`
	MinecraftVersion    string `toml:"mc_version"`
	BlueMapVersion      string `toml:"bluemap_version"`
	DownloadMode        string `toml:"download_mode"`        // "auto" (default) | "parallel" | "single"
	DownloadConnections int    `toml:"download_connections"` // 0 = auto (scale by file size) | 1-32 = fixed count
}

// ResolveDownloadMode returns the effective download mode, defaulting to
// DownloadModeAuto when the field is not set in config.toml.
func (c *ServerConfig) ResolveDownloadMode() string {
	if c.DownloadMode == "" {
		return DownloadModeAuto
	}
	return c.DownloadMode
}

// ResolveDownloadConnections returns the configured number of download
// connections. A value of 0 means automatic (scale by file size).
func (c *ServerConfig) ResolveDownloadConnections() int {
	return c.DownloadConnections
}

// ResolveWorlds returns the list of world folder names to extract from the
// backup, derived from ServerType and WorldName.
//
// For vanilla servers, dimensions are stored as subdirectories within a single
// world folder (world/DIM-1, world/DIM1), so only one folder is needed.
//
// For plugin servers (Bukkit/Spigot/Paper), each dimension is a separate
// top-level folder (world, world_nether, world_the_end).
func (c *ServerConfig) ResolveWorlds() []string {
	name := c.WorldName
	if name == "" {
		name = "world"
	}

	switch c.ServerType {
	case ServerTypeVanilla:
		return []string{name}
	case ServerTypePlugin:
		return []string{
			name,
			name + "_nether",
			name + "_the_end",
		}
	default:
		return []string{name}
	}
}

// LoadedServer holds a parsed config along with its directory path.
type LoadedServer struct {
	Dir    string
	Config ServerConfig
}

// Load reads and validates a single config.toml from the given directory.
func Load(dir string) (LoadedServer, error) {
	configPath := filepath.Join(dir, "config.toml")

	var cfg ServerConfig
	if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
		return LoadedServer{}, fmt.Errorf("parsing %s: %w", configPath, err)
	}

	if cfg.ServerID == "" {
		return LoadedServer{}, fmt.Errorf("%s: server_id is required", configPath)
	}
	if cfg.ServerType == "" {
		return LoadedServer{}, fmt.Errorf("%s: server_type is required (\"vanilla\" or \"plugin\")", configPath)
	}
	if cfg.ServerType != ServerTypeVanilla && cfg.ServerType != ServerTypePlugin {
		return LoadedServer{}, fmt.Errorf("%s: server_type must be \"vanilla\" or \"plugin\", got %q", configPath, cfg.ServerType)
	}
	if cfg.WorldName == "" {
		return LoadedServer{}, fmt.Errorf("%s: world_name is required", configPath)
	}
	if cfg.MinecraftVersion == "" {
		return LoadedServer{}, fmt.Errorf("%s: mc_version is required", configPath)
	}
	if cfg.BlueMapVersion == "" {
		return LoadedServer{}, fmt.Errorf("%s: bluemap_version is required", configPath)
	}
	if cfg.DownloadMode != "" &&
		cfg.DownloadMode != DownloadModeAuto &&
		cfg.DownloadMode != DownloadModeParallel &&
		cfg.DownloadMode != DownloadModeSingle {
		return LoadedServer{}, fmt.Errorf(
			"%s: download_mode must be %q, %q, or %q, got %q",
			configPath, DownloadModeAuto, DownloadModeParallel, DownloadModeSingle, cfg.DownloadMode)
	}
	if cfg.DownloadConnections < 0 || cfg.DownloadConnections > 32 {
		return LoadedServer{}, fmt.Errorf(
			"%s: download_connections must be between 0 and 32, got %d",
			configPath, cfg.DownloadConnections)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return LoadedServer{}, fmt.Errorf("resolving path %s: %w", dir, err)
	}

	return LoadedServer{Dir: absDir, Config: cfg}, nil
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

		srv, err := Load(filepath.Join(baseDir, entry.Name()))
		if err != nil {
			return nil, err
		}

		servers = append(servers, srv)
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("no config.toml found in any subdirectory of %s", baseDir)
	}

	return servers, nil
}

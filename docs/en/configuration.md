# Configuration

> **[繁體中文](../configuration.md)**

This document describes all configuration options for bluemap-action, including server config files, BlueMap config files, and environment variables.

## Server Config (`config.toml`)

Each server directory must contain a `config.toml` at its root. All fields are **required** except `name`:

```toml
# Pterodactyl server identifier (from panel URL or API)
server_id = "8e22b0c9"

# Server type: "vanilla" or "plugin"
server_type = "vanilla"

# Base world folder name
world_name = "world"

# Minecraft version (used for BlueMap rendering)
mc_version = "1.21.11"

# BlueMap CLI version (downloaded from GitHub Releases)
bluemap_version = "5.16"

# Display name (optional, defaults to directory name)
name = "My Server"
```

### Field Reference

| Field | Required | Description |
|---|---|---|
| `server_id` | **Yes** | Pterodactyl server identifier, used to access backups via API |
| `server_type` | **Yes** | `"vanilla"` or `"plugin"`, determines world folder structure (see below) |
| `world_name` | **Yes** | Base world folder name in the backup (usually `"world"`) |
| `mc_version` | **Yes** | Minecraft version number, required by BlueMap CLI for correct rendering |
| `bluemap_version` | **Yes** | BlueMap CLI version to download and use |
| `name` | No | Project display name, shown in the language file footer |

### Server Types

The server type determines how the tool extracts world files from backups:

#### `vanilla`

Vanilla servers store all dimensions as subdirectories within a single world folder:

```
world/           # Overworld
world/DIM-1/     # Nether
world/DIM1/      # The End
```

When `server_type = "vanilla"`, the tool extracts only one folder (the name specified by `world_name`).

#### `plugin`

Plugin servers (Bukkit/Spigot/Paper, etc.) store each dimension as a separate top-level folder:

```
world/           # Overworld
world_nether/    # Nether
world_the_end/   # The End
```

When `server_type = "plugin"`, the tool extracts three folders:
- `{world_name}`
- `{world_name}_nether`
- `{world_name}_the_end`

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `PTERODACTYL_PANEL_URL` | **Yes** | Pterodactyl panel base URL (e.g. `https://panel.example.com`) |
| `PTERODACTYL_API_KEY` | **Yes** | Pterodactyl client API key |

Both environment variables are validated at startup. If either is missing, the tool terminates immediately.

## BlueMap Config Files

In addition to `config.toml`, the server directory must contain BlueMap config files in a `config/` subdirectory. These files are read directly by the BlueMap CLI.

### Directory Structure

```
onlinemap-01/
├── config.toml              # bluemap-action config
└── config/
    ├── core.conf             # BlueMap core settings
    ├── webapp.conf           # Web interface settings
    ├── maps/
    │   ├── overworld.conf    # Overworld map config
    │   ├── nether.conf       # Nether map config
    │   └── end.conf          # End map config
    └── storages/
        └── file.conf         # File storage config
```

### `core.conf`

BlueMap core behavior settings:

```conf
accept-download: true       # Accept Mojang EULA
data: "data"                # Data directory
render-thread-count: 1      # Number of render threads
scan-for-mod-resources: true
metrics: true               # Usage reporting
```

> In CI environments, set `render-thread-count` to 1 or adjust based on available CPU cores.

### `webapp.conf`

BlueMap web interface settings:

```conf
enabled: true               # Enable web application
webroot: "web"              # Web output root directory
update-settings-file: true  # Auto-update settings.json
use-cookies: true           # Use cookies to remember user preferences
```

### Map Config (`maps/*.conf`)

Each map requires a separate config file. Key settings include:

```conf
world: "world"              # World folder path
dimension: "minecraft:overworld"  # Dimension identifier
name: "Overworld"           # Map display name
sorting: 0                  # Map sort order

# Render bounds (optional)
min-x: -4000
max-x: 4000
min-z: -4000
max-z: 4000
```

Dimension values for each dimension:
- Overworld: `minecraft:overworld`
- Nether: `minecraft:the_nether`
- The End: `minecraft:the_end`

### Storage Config (`storages/*.conf`)

Defines how rendered output is stored. Default uses file storage:

```conf
storage-type: FILE
root: "web/maps"            # Map tile output directory
compression: GZIP           # Compression method
```

## Language File Placeholders

Language files have the following placeholders substituted at deployment:

| Placeholder | Description | Example |
|---|---|---|
| `{toolVersion}` | Git version of bluemap-action | `v1.0.0` |
| `{minecraftVersion}` | Minecraft version (from `mc_version`) | `1.21.11` |
| `{projectName}` | Project name (from `name` field or directory name) | `My Server` |
| `{renderTime}` | Render execution timestamp (Asia/Taipei timezone) | `2025-01-15 14:30 CST` |

Bundled BlueMap translation files:
- English (`en.conf`)
- Simplified Chinese (`zh-CN.conf`)
- Traditional Chinese, Taiwan (`zh-TW.conf`)
- Traditional Chinese, Hong Kong (`zh-HK.conf`)
- Language settings (`settings.conf`) — Defines the default locale and available language list

These files are sourced from BlueMap's own translations. Only the languages listed above are kept; all other unused language settings are removed. `settings.conf` sets the default language to English and enables automatic browser language detection.

## Adding a New Server

1. Create a new server directory (e.g. `onlinemap-02/`)
2. Add a `config.toml` with all required fields
3. Copy BlueMap config files to the `config/` subdirectory and adjust map settings (world paths, dimensions, render bounds, etc.)
4. Add a corresponding job in your workflow (see [multi-server example](../../README.en.md#multi-server) in the README)

# bluemap-action

A reusable action to deploy minecraft maps into Netlify.

## Pterodactyl Backup Downloader

A Go tool that downloads the latest backup from Pterodactyl panel servers and extracts specified world directories.

### Directory Structure

```
onlinemap-01/
  config.toml    # Server config
  world/         # Extracted after download
  world_nether/
  world_the_end/
onlinemap-02/
  config.toml
  world/
  ...
```

### Config Format (TOML)

Each server subdirectory must contain a `config.toml`:

```toml
# Pterodactyl server identifier
server_id = "a1b2c3d4"

# World directories to extract from the backup
worlds = ["world", "world_nether", "world_the_end"]
```

### Environment Variables

| Variable | Description |
|---|---|
| `PTERODACTYL_PANEL_URL` | Pterodactyl panel base URL (e.g. `https://panel.example.com`) |
| `PTERODACTYL_API_KEY` | Pterodactyl client API key |

### Usage

```bash
# Build
go build -o backup-downloader ./cmd/backup-downloader/

# Run (scans current directory for subdirectories with config.toml)
export PTERODACTYL_PANEL_URL="https://panel.example.com"
export PTERODACTYL_API_KEY="your-api-key"
./backup-downloader

# Or specify a different base directory
./backup-downloader -dir /path/to/server-configs
```

### How It Works

1. Scans the base directory for subdirectories containing `config.toml`
2. For each server, fetches the latest successful backup via Pterodactyl Client API
3. Downloads the backup archive (tar.gz)
4. Extracts only the specified world directories into the server's subdirectory

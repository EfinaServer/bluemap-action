# CLAUDE.md

Guide for AI assistants working on **bluemap-action**.

## Project Overview

**bluemap-action** is a Go CLI tool that automates Minecraft 3D map rendering and deployment. It downloads world backups from a Pterodactyl panel, renders them with BlueMap CLI, and produces a static web site ready for Netlify hosting.

## Repository Structure

```
bluemap-action/
├── cmd/bluemap-action/
│   └── main.go                  # CLI entry point (9-step pipeline)
├── internal/
│   ├── analyzer/analyzer.go     # World and web output size reporting
│   ├── assets/assets.go         # Rewrites web asset references to compressed variants
│   ├── bluemap/
│   │   ├── download.go          # BlueMap CLI jar download from GitHub Releases
│   │   ├── render.go            # Executes BlueMap CLI via java -jar
│   │   └── scripts.go           # Runs custom scripts from scripts/ directory
│   ├── config/config.go         # TOML config parsing and validation
│   ├── extractor/extractor.go   # tar.gz backup download and world extraction
│   ├── lang/
│   │   ├── lang.go              # Embedded language file deployment
│   │   └── files/               # Embedded .conf language files (en, settings, zh-CN, zh-TW, zh-HK)
│   ├── netlify/deploy.go        # Generates netlify.toml for static hosting
│   └── pterodactyl/client.go    # Pterodactyl panel Client API integration
├── onlinemap-01/                # Example server configuration
│   ├── config.toml              # Server-specific config
│   ├── config/                  # BlueMap configuration files
│   └── web/                     # BlueMap web output directory
├── .github/workflows/           # CI/CD workflows
├── go.mod                       # Go 1.24.7, single dependency (BurntSushi/toml)
└── go.sum
```

## Build & Run

```bash
# Build
go build -o bluemap-action ./cmd/bluemap-action/

# Build with version tag
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" \
  -o bluemap-action ./cmd/bluemap-action/

# Run against a server directory
export PTERODACTYL_PANEL_URL="https://panel.example.com"
export PTERODACTYL_API_KEY="your-api-key"
./bluemap-action -dir onlinemap-01
```

### Version resolution

The binary version is determined in this order:
1. `-ldflags "-X main.version=..."` at build time
2. Git revision from `debug.ReadBuildInfo()` (truncated to 7 chars)
3. Fallback: `"dev"`

## Execution Pipeline

The tool runs a sequential 9-step pipeline (`cmd/bluemap-action/main.go`):

1. **Download & extract** — Fetch latest successful backup from Pterodactyl, extract world directories from tar.gz
2. **Analyze worlds** — Report extracted world sizes (dimension breakdown for vanilla, per-folder for plugin)
3. **Download BlueMap CLI** — Fetch the jar from GitHub Releases (cached if already present)
4. **Deploy language files** — Copy embedded `.conf` files to `web/lang/`, substituting placeholders
5. **Deploy netlify.toml** — Write static site config (SPA redirect, gzip headers)
6. **Run custom scripts** — If a `scripts/` directory exists in the server directory, execute all `.py` and `.sh` scripts in alphabetical order (optional, skipped if directory absent)
7. **Render** — Execute `java -jar bluemap-cli.jar -v <mcVersion> -r`
8. **Rewrite asset refs** — Rewrite `.prbm` → `.prbm.gz` and `/textures.json` → `/textures.json.gz` in the generated JS bundle so Netlify serves pre-compressed files directly
9. **Analyze output** — Report total `web/` directory size

## Configuration

Each server directory needs a `config.toml`:

```toml
server_id       = "8e22b0c9"    # Pterodactyl server identifier
server_type     = "vanilla"      # "vanilla" or "plugin"
world_name      = "world"        # Base world folder name
mc_version      = "1.21.11"      # Minecraft version for rendering
bluemap_version = "5.16"         # BlueMap CLI version to download
name            = "My Server"    # Optional display name (defaults to directory name)
# download_mode = "auto"         # Optional: "auto" (default) | "parallel" | "single"
```

### Server types

- **vanilla** — Dimensions are subdirectories: `world/`, `world/DIM-1/`, `world/DIM1/`. Only one folder extracted.
- **plugin** — Dimensions are separate folders: `world/`, `world_nether/`, `world_the_end/`. All three extracted.

### Required environment variables

| Variable | Description |
|---|---|
| `PTERODACTYL_PANEL_URL` | Panel base URL (e.g. `https://panel.example.com`) |
| `PTERODACTYL_API_KEY` | Pterodactyl client API key |

## Key Design Decisions

- **Single dependency** — Only `github.com/BurntSushi/toml` for config parsing. Everything else uses the Go standard library.
- **Embedded language files** — Language `.conf` files are compiled into the binary via `//go:embed`. Placeholders (`{toolVersion}`, `{minecraftVersion}`, `{projectName}`, `{renderTime}`) are substituted at runtime.
- **Three download modes** — Controlled by `download_mode` in `config.toml` (`auto` / `parallel` / `single`). In `auto` mode the server is probed with a `GET Range: bytes=0-0` request: if it responds with `206 Partial Content` and the backup is ≥ 64 MB, 4 parallel HTTP Range connections are used (temp file required); otherwise the response body is streamed directly into the tar reader (no temp file). Using GET instead of HEAD for probing ensures compatibility with S3 Presigned URLs, which are typically signed for GET only. `parallel` forces multi-connection and errors if Range or Content-Length is absent. `single` forces streaming. The log line always states which mode was chosen and the reason.
- **Temp-file extraction (parallel only)** — Parallel download pre-allocates a temporary `.backup-*.tar.gz` file (same filesystem as the output directory to avoid cross-device rename issues), each worker writes its chunk via `WriteAt`, then the file is re-opened for sequential tar.gz extraction. The temp file is removed on completion.
- **Path traversal protection** — The extractor validates that all extracted paths stay within the output directory.
- **Atomic file writes** — BlueMap CLI jar downloads use a `.tmp` file with rename to prevent partial files.
- **Timezone** — Render timestamps use `Asia/Taipei` timezone.

## Runtime Requirements

- **Go 1.24.7+** for building
- **Java runtime** for BlueMap CLI execution
- **Python 3** (optional) — only needed if a server's `scripts/` directory contains `.py` scripts
- Network access to: Pterodactyl panel API, GitHub Releases (BlueMap CLI download)

## Code Conventions

- Standard Go project layout: `cmd/` for entry points, `internal/` for private packages
- No test files exist yet — when adding tests, follow Go convention (`*_test.go` alongside source)
- Error handling uses `fmt.Errorf` with `%w` wrapping throughout
- All packages are under `internal/` — not importable by external projects
- Config validation is fail-fast: missing required fields cause immediate `log.Fatal`
- Logging goes to stdout; warnings go to stderr

## Adding a New Server

1. Create a directory (e.g. `onlinemap-02/`)
2. Add a `config.toml` with required fields
3. Copy BlueMap config files into `config/` subdirectory (maps, storages, core.conf, webapp.conf)
4. Run the tool with `-dir onlinemap-02`

## Gitignore

- `/bluemap-action` — compiled binary
- `onlinemap-*/web/lang/` — generated language files (recreated each run)

# Architecture

> **[繁體中文](../architecture.md)**

This document describes the internal architecture, execution pipeline, and design decisions of bluemap-action.

## Project Structure

```
bluemap-action/
├── cmd/bluemap-action/
│   └── main.go                  # CLI entry point (execution pipeline)
├── internal/
│   ├── analyzer/analyzer.go     # World and web output size analysis
│   ├── assets/assets.go         # Static asset compression reference rewriting
│   ├── bluemap/
│   │   ├── download.go          # Download BlueMap CLI jar from GitHub Releases
│   │   └── render.go            # Execute BlueMap CLI rendering via java -jar
│   ├── config/config.go         # TOML config parsing and validation
│   ├── extractor/extractor.go   # tar.gz backup download and world directory extraction
│   ├── lang/
│   │   ├── lang.go              # Embedded language file deployment
│   │   └── files/               # Embedded .conf language files (en, zh-CN, zh-TW, zh-HK)
│   ├── netlify/deploy.go        # Generate netlify.toml for static site hosting
│   └── pterodactyl/client.go    # Pterodactyl panel Client API integration
├── test/
│   └── test-onlinemap/          # Example server configuration for testing
├── .github/workflows/           # CI/CD workflows
├── go.mod                       # Go 1.24.7, single dependency: BurntSushi/toml
└── go.sum
```

## Execution Pipeline

`cmd/bluemap-action/main.go` defines a sequential pipeline that processes a single server directory:

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. Load Config                                                  │
│    Read config.toml and validate required fields                │
├─────────────────────────────────────────────────────────────────┤
│ 2. Download & Extract Worlds                                    │
│    Get latest successful backup from Pterodactyl → stream       │
│    decompress tar.gz → extract only matching world directories  │
├─────────────────────────────────────────────────────────────────┤
│ 3. Analyze World Sizes                                          │
│    Report extracted world sizes                                 │
│    (vanilla: dimension breakdown / plugin: per-folder)          │
├─────────────────────────────────────────────────────────────────┤
│ 4. Download BlueMap CLI                                         │
│    Fetch jar from GitHub Releases (skip if already cached)      │
├─────────────────────────────────────────────────────────────────┤
│ 5. Deploy Language Files                                        │
│    Copy embedded .conf files to web/lang/, substitute           │
│    placeholders                                                 │
├─────────────────────────────────────────────────────────────────┤
│ 6. Deploy netlify.toml                                          │
│    Write static site config (SPA redirect, gzip headers)        │
├─────────────────────────────────────────────────────────────────┤
│ 7. Render                                                       │
│    Execute java -jar bluemap-cli.jar -v <mcVersion> -r          │
├─────────────────────────────────────────────────────────────────┤
│ 8. Rewrite Asset References                                     │
│    Rewrite .prbm → .prbm.gz, .json → .json.gz                  │
├─────────────────────────────────────────────────────────────────┤
│ 9. Analyze Output                                               │
│    Report total web/ directory size                              │
└─────────────────────────────────────────────────────────────────┘
```

### GitHub Step Summary

When running in a CI environment (`CI=true`), the pipeline writes a build summary to `$GITHUB_STEP_SUMMARY` at the end of execution, producing a Markdown report that includes:

- **Server Configuration** — Project name, server ID, type, world name, Minecraft version, BlueMap version, render timestamp
- **Backup** — Backup name, UUID, file size, download and extraction duration
- **Render** — BlueMap CLI render duration
- **World Sizes** — Size breakdown by dimension/world folder
- **Web Output** — Total `web/` directory size

This step is automatically skipped when not running in CI.

## Module Reference

### `internal/pterodactyl`

Encapsulates Pterodactyl panel Client API interactions:

- `ListBackups()` — Retrieve all backups for a server, sorted by creation time (newest first)
- `GetLatestBackup()` — Return the most recent successful backup
- `GetBackupDownloadURL()` — Get a signed download URL

### `internal/extractor`

Handles backup file download and decompression. Supports three download modes controlled by `download_mode` in `config.toml`:

- **`auto` (default)** — probes the server with a `GET Range: bytes=0-0` request and chooses automatically: uses 4 parallel connections (temp file required) when the server responds with `206 Partial Content` and size ≥ 64 MB; otherwise falls back to single-connection streaming (no temp file). Compatible with S3 Presigned URLs.
- **`parallel`** — forces 4-connection parallel download; returns an error if the server does not support Range requests or does not return `Content-Length`
- **`single`** — forces single-connection streaming, piping the HTTP response directly into the tar reader with no temp file written to disk

Common features:
- Filters extraction by world names, extracting only matching directories
- Includes path traversal protection, ensuring all extracted paths stay within the output directory
- Per-file size limit: 10 GB

### `internal/config`

Parses TOML config files and validates required fields:

- `Load()` — Load and validate a single `config.toml`
- `LoadAll()` — Scan a directory for all subdirectories containing `config.toml`
- `ResolveWorlds()` — Derive world folder list based on server type

### `internal/bluemap`

Manages BlueMap CLI download and execution:

- `EnsureCLI()` — Download jar if not present, using `.tmp` file with rename (atomic write to prevent incomplete files)
- `Render()` — Execute `java -jar <jar> -v <mcVersion> -r`, streaming stdout/stderr in real time

### `internal/lang`

BlueMap translation file deployment:

- Bundles BlueMap's own translation files into the binary via `//go:embed`
- Keeps only the required languages (en, zh-CN, zh-TW, zh-HK) and removes unused language settings
- Placeholders substituted at deploy time: `{toolVersion}`, `{minecraftVersion}`, `{projectName}`, `{renderTime}`

### `internal/netlify`

Generates Netlify static site configuration:

- SPA fallback redirect: `/*` → `/index.html` (200 status code)
- Gzip headers: applied to `*.json.gz` and `*.prbm.gz`

### `internal/assets`

Handles static asset compression reference rewriting:

- Scans `web/assets/index-*.js` files
- Rewrites `.prbm` to `.prbm.gz`, `/textures.json` to `/textures.json.gz`

> Netlify does not support wildcard content-encoding rewrites, so the JavaScript bundle must reference compressed file paths directly rather than relying on server-side content negotiation.

### `internal/analyzer`

World file and output size analysis:

- `AnalyzeVanillaWorld()` — Analyze vanilla server world sizes (overworld, nether, end)
- `AnalyzeWorlds()` — Analyze plugin server world folder sizes
- `AnalyzeWebOutput()` — Calculate total `web/` directory size
- `FormatSize()` — Human-readable size formatting (B, KB, MB, GB)

## Design Decisions

### Single Dependency

The project depends only on `github.com/BurntSushi/toml` for config parsing. Everything else uses the Go standard library. This reduces supply chain risk and simplifies the build process.

### Three Download Modes

The backup download strategy is configured via `download_mode` in `config.toml`:

- **`auto` (default)** — probes the server and selects the best strategy automatically
- **`parallel`** — forces 4-connection parallel download (best for large backups)
- **`single`** — forces streaming with no temp file (lowest disk I/O)

Parallel download requires a temp file on the same filesystem as the output directory (to avoid cross-device rename issues); each worker writes to its byte offset via `WriteAt`, then the file is re-opened for sequential extraction. Single/streaming mode pipes the HTTP response body directly into the tar reader and never touches the local disk for the archive.

### Embedded Language Files

Language files are compiled into the binary via Go's `//go:embed` directive. This allows the tool to be distributed as a single executable without additional resource files.

### Atomic File Writes

BlueMap CLI jar downloads use a `.tmp` temporary file with rename, ensuring incomplete jar files are never left behind. If a download is interrupted, no corrupted file remains.

### Path Traversal Protection

The extractor validates all paths extracted from tar archives, ensuring they remain within the output directory. This prevents malicious backup files from overwriting system files.

### Timezone

Render timestamps use the `Asia/Taipei` timezone to match the project's primary user base.

## Version Resolution

Binary version is determined in the following order:

1. Build-time via `-ldflags "-X main.version=..."`
2. Git revision from `debug.ReadBuildInfo()` (truncated to 7 characters)
3. Fallback: `"dev"`

## Runtime Requirements

- **Go 1.24.7+** — Building the tool
- **Java runtime** — Executing BlueMap CLI
- Network access to: Pterodactyl panel API, GitHub Releases (BlueMap CLI download)

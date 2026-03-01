# Development

> **[繁體中文](../development.md)**

This document covers building bluemap-action from source, code conventions, and the CI/CD process.

## Building

### Prerequisites

- **Go 1.24.7+**
- **Java runtime** — For running BlueMap CLI (during development/testing)

### Build from Source

```bash
# Basic build
go build -o bluemap-action ./cmd/bluemap-action/

# Build with version tag
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" \
  -o bluemap-action ./cmd/bluemap-action/
```

### Running Locally

```bash
export PTERODACTYL_PANEL_URL="https://panel.example.com"
export PTERODACTYL_API_KEY="your-api-key"

# Specify server directory
./bluemap-action -dir test/test-onlinemap
```

### CLI Arguments

| Argument | Default | Description |
|---|---|---|
| `-dir` | `.` | Server directory containing `config.toml` |

## Code Conventions

### Project Layout

- `cmd/` — Executable entry points
- `internal/` — Internal packages (not importable by external projects)
- Standard Go project layout

### Error Handling

- Uses `fmt.Errorf` with `%w` for error wrapping
- Config validation is fail-fast: missing required fields trigger `log.Fatal` immediately
- Logging goes to stdout; warnings go to stderr

### Dependency Management

The project depends only on `github.com/BurntSushi/toml`. Everything else uses the Go standard library. Carefully evaluate necessity before adding new dependencies.

### Testing

No test files exist yet. When adding tests, follow Go conventions by placing `*_test.go` files alongside source files.

## Release Process

Pushing a tag matching the `v*` pattern triggers the release pipeline via GitHub Actions (`.github/workflows/release.yml`):

1. Checkout with full Git history
2. Set up Go environment (version from `go.mod`)
3. Extract version from Git tag
4. Cross-compile multi-platform binaries:
   - `linux/amd64`, `linux/arm64`
   - `darwin/amd64`, `darwin/arm64`
   - `windows/amd64`
5. Generate SHA256 checksums
6. Create GitHub Release and upload all files

### Creating a New Release

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions will automatically handle building and publishing.

## Reusable Workflow Development

The reusable workflow is defined in `.github/workflows/build-map.yml` and can be called directly by other repositories.

### Workflow Jobs

The workflow consists of two jobs:

#### 1. `check-cache` — Cache Detection & Runner Selection

Runs on the `runs-on-cache-hit` runner (lightweight cache lookup only). Uses `actions/cache/restore` with `lookup-only: true` to probe for an existing `web/maps` cache without downloading files.

- **Cache found** → selects the smaller `runs-on-cache-hit` runner for the build job
- **No cache** → selects the larger `runs-on-cache-miss` runner for the build job

#### 2. `build-map` — Build & Deploy

Runs on the runner selected by `check-cache`. Steps:

1. **Checkout** — Check out the caller repository
2. **Set up Java** — Install Temurin JDK (default 21)
3. **Download bluemap-action** — Download binary from GitHub Releases
4. **Restore cache** — Restore `web/maps` cache (incremental rendering)
5. **Build map** — Run bluemap-action
6. **Deploy to Netlify** — Conditional deployment (controlled via `deploy-to-netlify`)

### Incremental Rendering

The workflow uses `actions/cache` to cache the `web/maps` directory. BlueMap CLI supports incremental rendering — previously rendered chunks are not reprocessed, significantly reducing subsequent render times.

Cache key format:
- Primary key: `bluemap-maps-{server-directory}-{run-id}`
- Restore key: `bluemap-maps-{server-directory}-` (matches the most recent cache)

### Cost Optimization

The `check-cache` job automatically selects the appropriate runner size based on cache availability:

- **Cache hit (incremental render)** — Uses the smaller `runs-on-cache-hit` runner (default: 2 vCPU), since only changed chunks need re-rendering
- **Cache miss (full render)** — Uses the larger `runs-on-cache-miss` runner (default: 8 vCPU), needed for full map rendering

The `check-cache` probe job runs on the `runs-on-cache-hit` runner since the cache lookup is lightweight and does not require the larger machine.

### `refresh-cache.yml` — Cache Refresh

GitHub Actions caches are automatically deleted after 7 days of not being accessed. While `build-map.yml` saves a new cache entry with a `run_id` key on each successful build (resetting the timer), a weekly render schedule sits right at the 7-day boundary — the cache could be evicted moments before the build starts, triggering an unexpected full render.

`refresh-cache.yml` is designed to run every 5 days, proactively refreshing the cache before expiry:

1. Restores the most recent `web/maps` cache using `restore-keys` (downloads to the runner)
2. Saves the cache with a new `run_id` as the primary key at job end → creates a fresh entry, definitively resetting the 7-day timer
3. Requires no checkout, Java, or Pterodactyl credentials — the smallest available runner suffices

> **Why not just use `actions/cache/restore`?** GitHub's documentation states caches expire after "7 days of no access," but whether a restore (download) counts as "access" is not officially guaranteed. Using the full `actions/cache` (with save) ensures a new entry is created, explicitly resetting the timer without relying on undefined behavior.

The cache key format is identical to `build-map.yml`, so caches are fully interchangeable between the two workflows:

- Primary key: `bluemap-maps-{server-directory}-{run-id}`
- Restore key: `bluemap-maps-{server-directory}-`

### Testing the Reusable Workflow

The project provides `.github/workflows/test-reusable-workflow.yml`, which can be manually triggered to test the reusable workflow:

```bash
gh workflow run test-reusable-workflow.yml
```

This will use the `test/test-onlinemap` directory for a test build.

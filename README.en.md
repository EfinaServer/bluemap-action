# bluemap-action

> **[繁體中文](README.md)**

An automated Minecraft 3D map rendering and deployment tool. Downloads world backups from [Pterodactyl](https://pterodactyl.io/) panel, renders 3D maps using [BlueMap](https://bluemap.bluecolored.de/) CLI, and deploys static sites to [Netlify](https://www.netlify.com/).

## Features

- **Fully Automated** — From backup download to map deployment, everything is automated
- **Reusable Workflow** — Call directly from other repositories, no need to write complex CI pipelines
- **Incremental Rendering** — Only re-renders changed chunks via caching
- **Multi-Server Support** — Build maps for multiple servers in a single workflow file
- **Multi-Language UI** — Supports English, Simplified Chinese, Traditional Chinese (Taiwan), Traditional Chinese (Hong Kong)

## Quick Start

### 1. Prepare Server Directory

Create a server directory in your repository with bluemap-action config and BlueMap config files:

```
onlinemap-01/
├── config.toml              # bluemap-action config (see below)
└── config/
    ├── core.conf             # BlueMap core settings
    ├── webapp.conf           # Web interface settings
    ├── maps/
    │   ├── overworld.conf    # Overworld map
    │   ├── nether.conf       # Nether map
    │   └── end.conf          # End map
    └── storages/
        └── file.conf         # File storage settings
```

`config.toml` contents:

```toml
server_id       = "8e22b0c9"     # Pterodactyl server identifier
server_type     = "vanilla"      # "vanilla" or "plugin"
world_name      = "world"        # World folder name
mc_version      = "1.21.11"      # Minecraft version
bluemap_version = "5.16"         # BlueMap CLI version
name            = "My Server"    # Display name (optional)
```

> See [docs/en/configuration.md](docs/en/configuration.md) for full configuration reference.

### 2. Configure GitHub Secrets

In your repository's **Settings → Secrets and variables → Actions**, add:

| Secret | Description |
|---|---|
| `PTERODACTYL_PANEL_URL` | Pterodactyl panel URL (e.g. `https://panel.example.com`) |
| `PTERODACTYL_API_KEY` | Pterodactyl client API key |
| `NETLIFY_AUTH_TOKEN` | Netlify auth token (required for Netlify deployment) |

### 3. Create Workflow

Create `.github/workflows/build-map.yml` in your repository:

```yaml
name: Build Map

on:
  schedule:
    - cron: "0 0 * * *"    # Run daily
  workflow_dispatch:         # Allow manual trigger

jobs:
  build:
    uses: EfinaServer/bluemap-action/.github/workflows/build-map.yml@main
    with:
      server-directory: onlinemap-01
      netlify-site-id: your-netlify-site-id
    secrets:
      PTERODACTYL_PANEL_URL: ${{ secrets.PTERODACTYL_PANEL_URL }}
      PTERODACTYL_API_KEY: ${{ secrets.PTERODACTYL_API_KEY }}
      NETLIFY_AUTH_TOKEN: ${{ secrets.NETLIFY_AUTH_TOKEN }}
```

## Reusable Workflow Reference

### Inputs

| Name | Required | Default | Description |
|---|---|---|---|
| `server-directory` | **Yes** | — | Server directory containing `config.toml` |
| `bluemap-action-version` | No | `latest` | bluemap-action release tag (e.g. `v1.0.0`) |
| `java-version` | No | `21` | Java version for BlueMap CLI rendering |
| `deploy-to-netlify` | No | `true` | Whether to deploy to Netlify |
| `netlify-site-id` | No | — | Netlify site ID (required when deploying) |

### Secrets

| Name | Required | Description |
|---|---|---|
| `PTERODACTYL_PANEL_URL` | **Yes** | Pterodactyl panel URL |
| `PTERODACTYL_API_KEY` | **Yes** | Pterodactyl client API key |
| `NETLIFY_AUTH_TOKEN` | Conditional | Netlify auth token (required when `deploy-to-netlify` is `true`) |

### Workflow Steps

```
Checkout → Set up Java → Download bluemap-action → Restore cache → Build map → Deploy to Netlify
```

1. **Checkout** — Check out the caller repository
2. **Set up Java** — Install Temurin JDK (default version 21)
3. **Download bluemap-action** — Download the specified version binary from GitHub Releases
4. **Restore web/maps cache** — Restore previous render cache for incremental rendering
5. **Build map** — Run bluemap-action (download backup → extract worlds → render map)
6. **Deploy to Netlify** — Deploy rendered static site to Netlify (optional)

## Usage Examples

### All Options

Specify all available options:

```yaml
name: Build and Deploy Map

on:
  schedule:
    - cron: "0 4 * * 1"    # Every Monday at 04:00 UTC
  workflow_dispatch:

jobs:
  build:
    uses: EfinaServer/bluemap-action/.github/workflows/build-map.yml@main
    with:
      server-directory: onlinemap-01
      bluemap-action-version: v1.0.0
      java-version: "21"
      deploy-to-netlify: true
      netlify-site-id: your-netlify-site-id
    secrets:
      PTERODACTYL_PANEL_URL: ${{ secrets.PTERODACTYL_PANEL_URL }}
      PTERODACTYL_API_KEY: ${{ secrets.PTERODACTYL_API_KEY }}
      NETLIFY_AUTH_TOKEN: ${{ secrets.NETLIFY_AUTH_TOKEN }}
```

### Multi-Server

Build maps for multiple servers, with each job running in parallel:

```yaml
name: Build All Maps

on:
  schedule:
    - cron: "0 0 * * *"
  workflow_dispatch:

jobs:
  server-01:
    uses: EfinaServer/bluemap-action/.github/workflows/build-map.yml@main
    with:
      server-directory: onlinemap-01
      netlify-site-id: site-id-for-server-01
    secrets:
      PTERODACTYL_PANEL_URL: ${{ secrets.PTERODACTYL_PANEL_URL }}
      PTERODACTYL_API_KEY: ${{ secrets.PTERODACTYL_API_KEY }}
      NETLIFY_AUTH_TOKEN: ${{ secrets.NETLIFY_AUTH_TOKEN }}

  server-02:
    uses: EfinaServer/bluemap-action/.github/workflows/build-map.yml@main
    with:
      server-directory: onlinemap-02
      netlify-site-id: site-id-for-server-02
    secrets:
      PTERODACTYL_PANEL_URL: ${{ secrets.PTERODACTYL_PANEL_URL }}
      PTERODACTYL_API_KEY: ${{ secrets.PTERODACTYL_API_KEY }}
      NETLIFY_AUTH_TOKEN: ${{ secrets.NETLIFY_AUTH_TOKEN }}
```

### Build Only (No Deployment)

Render the map without deploying to Netlify. The output will be uploaded as an artifact:

```yaml
jobs:
  build:
    uses: EfinaServer/bluemap-action/.github/workflows/build-map.yml@main
    with:
      server-directory: onlinemap-01
      deploy-to-netlify: false
    secrets:
      PTERODACTYL_PANEL_URL: ${{ secrets.PTERODACTYL_PANEL_URL }}
      PTERODACTYL_API_KEY: ${{ secrets.PTERODACTYL_API_KEY }}
```

## Standalone Usage

bluemap-action can also be used as a standalone CLI tool without GitHub Actions:

```bash
# Download the latest release
gh release download --repo EfinaServer/bluemap-action \
  --pattern "bluemap-action-linux-amd64"
chmod +x bluemap-action-linux-amd64

# Set environment variables
export PTERODACTYL_PANEL_URL="https://panel.example.com"
export PTERODACTYL_API_KEY="your-api-key"

# Run
./bluemap-action-linux-amd64 -dir onlinemap-01
```

Supported platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`

## Documentation

| Document | Description |
|---|---|
| [docs/en/architecture.md](docs/en/architecture.md) | Project architecture, execution pipeline, modules, and design decisions |
| [docs/en/configuration.md](docs/en/configuration.md) | Full configuration reference: config.toml, BlueMap configs, environment variables |
| [docs/en/development.md](docs/en/development.md) | Building from source, code conventions, release process |

## License

[MIT](LICENSE)

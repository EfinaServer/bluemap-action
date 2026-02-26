# bluemap-action

> **[English](README.en.md)**

自動化 Minecraft 3D 地圖渲染與部署工具。從 [Pterodactyl](https://pterodactyl.io/) 面板下載世界備份，使用 [BlueMap](https://bluemap.bluecolored.de/) CLI 渲染 3D 地圖，並部署為靜態網站至 [Netlify](https://www.netlify.com/)。

## 特色

- **一鍵自動化** — 從備份下載到地圖部署，全程自動
- **Reusable Workflow** — 其他 repository 直接呼叫，無需自行撰寫複雜 CI 流程
- **增量渲染** — 透過快取機制，僅渲染變動的區塊
- **多伺服器支援** — 單一 workflow 檔案可同時建置多個伺服器的地圖
- **內建翻譯檔** — 預先打包 BlueMap 翻譯檔，僅保留所需語言並移除未使用的語言設定
- **GitHub Step Summary** — 在 CI 環境中自動產生建置摘要，包含伺服器設定、備份資訊、世界大小與渲染時間

## 快速開始

### 1. 準備伺服器目錄

在你的 repository 中建立伺服器目錄，包含 bluemap-action 設定檔與 BlueMap 設定檔：

```
onlinemap-01/
├── config.toml              # bluemap-action 設定（見下方）
└── config/
    ├── core.conf             # BlueMap 核心設定
    ├── webapp.conf           # Web 介面設定
    ├── maps/
    │   ├── overworld.conf    # 主世界地圖
    │   ├── nether.conf       # 地獄地圖
    │   └── end.conf          # 終界地圖
    └── storages/
        └── file.conf         # 檔案儲存設定
```

`config.toml` 內容：

```toml
server_id       = "8e22b0c9"     # Pterodactyl 伺服器識別碼
server_type     = "vanilla"      # "vanilla" 或 "plugin"
world_name      = "world"        # 世界資料夾名稱
mc_version      = "1.21.11"      # Minecraft 版本
bluemap_version = "5.16"         # BlueMap CLI 版本
name            = "My Server"    # 顯示名稱（選填）
# download_mode = "auto"         # 下載模式（選填）："auto" | "parallel" | "single"
```

> 完整設定說明見 [docs/configuration.md](docs/configuration.md)。

### 2. 設定 GitHub Secrets

在你的 repository 的 **Settings → Secrets and variables → Actions** 中設定：

| Secret | 說明 |
|---|---|
| `PTERODACTYL_PANEL_URL` | Pterodactyl 面板網址（例如 `https://panel.example.com`） |
| `PTERODACTYL_API_KEY` | Pterodactyl client API key |
| `NETLIFY_AUTH_TOKEN` | Netlify 認證 token（部署至 Netlify 時需要） |

### 3. 建立 Workflow

在你的 repository 建立 `.github/workflows/build-map.yml`：

```yaml
name: Build Map

on:
  schedule:
    - cron: "0 0 * * *"    # 每天執行
  workflow_dispatch:         # 允許手動觸發

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

## Reusable Workflow 參考

### Inputs

| 名稱 | 必填 | 預設值 | 說明 |
|---|---|---|---|
| `server-directory` | **是** | — | 包含 `config.toml` 的伺服器目錄名稱 |
| `runs-on-cache-hit` | 否 | `blacksmith-2vcpu-ubuntu-2404` | 有快取時使用的 runner（增量渲染，較小機器） |
| `runs-on-cache-miss` | 否 | `blacksmith-8vcpu-ubuntu-2404` | 無快取時使用的 runner（完整渲染，較大機器） |
| `bluemap-action-version` | 否 | `latest` | bluemap-action 的 release tag（例如 `v1.0.0`） |
| `java-version` | 否 | `21` | 用於 BlueMap CLI 渲染的 Java 版本 |
| `deploy-to-netlify` | 否 | `true` | 是否部署至 Netlify（設為 `false` 僅供測試渲染用） |
| `netlify-site-id` | 否 | — | Netlify site ID（部署時必填） |

### Secrets

| 名稱 | 必填 | 說明 |
|---|---|---|
| `PTERODACTYL_PANEL_URL` | **是** | Pterodactyl 面板網址 |
| `PTERODACTYL_API_KEY` | **是** | Pterodactyl client API key |
| `NETLIFY_AUTH_TOKEN` | 條件性 | Netlify 認證 token（`deploy-to-netlify` 為 `true` 時必填） |

### 工作流程執行步驟

```
Checkout → 安裝 Java → 下載 bluemap-action → 還原快取 → 建置地圖 → 部署至 Netlify
```

1. **Checkout** — 取出呼叫方的 repository
2. **Set up Java** — 安裝 Temurin JDK（預設版本 21）
3. **Download bluemap-action** — 從 GitHub Releases 下載指定版本的二進位檔
4. **Restore web/maps cache** — 還原上次渲染的快取，實現增量渲染
5. **Build map** — 執行 bluemap-action（下載備份 → 擷取世界 → 渲染地圖）
6. **Deploy to Netlify** — 將渲染完成的靜態網站部署至 Netlify（可選）

## 使用範例

### 完整選項

指定所有可用選項：

```yaml
name: Build and Deploy Map

on:
  schedule:
    - cron: "0 4 * * 1"    # 每週一 04:00 UTC
  workflow_dispatch:

jobs:
  build:
    uses: EfinaServer/bluemap-action/.github/workflows/build-map.yml@main
    with:
      runs-on-cache-hit: blacksmith-2vcpu-ubuntu-2404
      runs-on-cache-miss: blacksmith-8vcpu-ubuntu-2404
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

### 多伺服器

為多個伺服器分別建置地圖，各 job 平行執行：

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

## 獨立使用

bluemap-action 也可以作為獨立 CLI 工具使用，不透過 GitHub Actions：

```bash
# 下載最新版本
gh release download --repo EfinaServer/bluemap-action \
  --pattern "bluemap-action-linux-amd64"
chmod +x bluemap-action-linux-amd64

# 設定環境變數
export PTERODACTYL_PANEL_URL="https://panel.example.com"
export PTERODACTYL_API_KEY="your-api-key"

# 執行
./bluemap-action-linux-amd64 -dir onlinemap-01
```

支援平台：`linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64`、`windows/amd64`

## 文件

| 文件 | 說明 |
|---|---|
| [docs/architecture.md](docs/architecture.md) | 專案架構、執行管線、模組說明與設計決策 |
| [docs/configuration.md](docs/configuration.md) | 完整設定參考：config.toml、BlueMap 設定檔、環境變數 |
| [docs/development.md](docs/development.md) | 從原始碼建置、程式碼規範、Release 流程 |

## License

[MIT](LICENSE)

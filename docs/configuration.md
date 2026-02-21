# Configuration

本文件說明 bluemap-action 所有設定選項，包含伺服器設定檔、BlueMap 設定檔與環境變數。

## 伺服器設定檔 (`config.toml`)

每個伺服器目錄的根目錄必須包含一個 `config.toml`，所有欄位皆為**必填**（`name` 除外）：

```toml
# Pterodactyl 伺服器識別碼（從面板 URL 或 API 取得）
server_id = "8e22b0c9"

# 伺服器類型："vanilla" 或 "plugin"
server_type = "vanilla"

# 基礎世界資料夾名稱
world_name = "world"

# Minecraft 版本（用於 BlueMap 渲染）
mc_version = "1.21.11"

# BlueMap CLI 版本（從 GitHub Releases 下載）
bluemap_version = "5.16"

# 顯示名稱（選填，預設為目錄名稱）
name = "My Server"
```

### 欄位說明

| 欄位 | 必填 | 說明 |
|---|---|---|
| `server_id` | **是** | Pterodactyl 伺服器識別碼，用於透過 API 存取備份 |
| `server_type` | **是** | `"vanilla"` 或 `"plugin"`，決定世界資料夾結構（見下方說明） |
| `world_name` | **是** | 備份中基礎世界資料夾的名稱（通常為 `"world"`） |
| `mc_version` | **是** | Minecraft 版本號，BlueMap CLI 需要此資訊來正確渲染 |
| `bluemap_version` | **是** | 要下載使用的 BlueMap CLI 版本 |
| `name` | 否 | 專案顯示名稱，會出現在語言檔案的頁尾資訊中 |

### 伺服器類型

伺服器類型決定了工具如何從備份中擷取世界檔案：

#### `vanilla`

Vanilla 伺服器將所有維度儲存在單一世界資料夾的子目錄中：

```
world/           # 主世界 (Overworld)
world/DIM-1/     # 地獄 (Nether)
world/DIM1/      # 終界 (The End)
```

設定 `server_type = "vanilla"` 時，工具僅擷取一個資料夾（`world_name` 指定的名稱）。

#### `plugin`

Plugin 伺服器（Bukkit/Spigot/Paper 等）將每個維度儲存為獨立的頂層資料夾：

```
world/           # 主世界 (Overworld)
world_nether/    # 地獄 (Nether)
world_the_end/   # 終界 (The End)
```

設定 `server_type = "plugin"` 時，工具會擷取三個資料夾：
- `{world_name}`
- `{world_name}_nether`
- `{world_name}_the_end`

## 環境變數

| 變數 | 必填 | 說明 |
|---|---|---|
| `PTERODACTYL_PANEL_URL` | **是** | Pterodactyl 面板基底 URL（例如 `https://panel.example.com`） |
| `PTERODACTYL_API_KEY` | **是** | Pterodactyl client API key |

這兩個環境變數在啟動時驗證，若缺少任一個，工具會立即終止。

## BlueMap 設定檔

除了 `config.toml`，伺服器目錄還需要包含 BlueMap 的設定檔，放在 `config/` 子目錄中。這些檔案直接由 BlueMap CLI 讀取。

### 目錄結構

```
onlinemap-01/
├── config.toml              # bluemap-action 設定
└── config/
    ├── core.conf             # BlueMap 核心設定
    ├── webapp.conf           # Web 介面設定
    ├── maps/
    │   ├── overworld.conf    # 主世界地圖設定
    │   ├── nether.conf       # 地獄地圖設定
    │   └── end.conf          # 終界地圖設定
    └── storages/
        └── file.conf         # 檔案儲存設定
```

### `core.conf`

BlueMap 核心行為設定：

```conf
accept-download: true       # 接受 Mojang EULA
data: "data"                # 資料目錄
render-thread-count: 1      # 渲染執行緒數
scan-for-mod-resources: true
metrics: true               # 使用量回報
```

> `render-thread-count` 在 CI 環境中建議設為 1 或根據可用 CPU 核心調整。

### `webapp.conf`

BlueMap Web 介面設定：

```conf
enabled: true               # 啟用 Web 應用程式
webroot: "web"              # Web 輸出根目錄
update-settings-file: true  # 自動更新 settings.json
use-cookies: true           # 使用 cookies 記住使用者偏好
```

### 地圖設定 (`maps/*.conf`)

每個地圖需要一個獨立的設定檔。關鍵設定包含：

```conf
world: "world"              # 世界資料夾路徑
dimension: "minecraft:overworld"  # 維度識別碼
name: "Overworld"           # 地圖顯示名稱
sorting: 0                  # 地圖排序順序

# 渲染範圍（可選）
min-x: -4000
max-x: 4000
min-z: -4000
max-z: 4000
```

各維度的 `dimension` 值：
- 主世界：`minecraft:overworld`
- 地獄：`minecraft:the_nether`
- 終界：`minecraft:the_end`

### 儲存設定 (`storages/*.conf`)

定義渲染結果的儲存方式。預設使用檔案儲存：

```conf
storage-type: FILE
root: "web/maps"            # 地圖瓷磚輸出目錄
compression: GZIP           # 壓縮方式
```

## 語言檔案佔位符

語言檔案在部署時會替換以下佔位符：

| 佔位符 | 說明 | 範例 |
|---|---|---|
| `{toolVersion}` | bluemap-action 的 Git 版本 | `v1.0.0` |
| `{minecraftVersion}` | Minecraft 版本（來自 `mc_version`） | `1.21.11` |
| `{projectName}` | 專案名稱（來自 `name` 欄位或目錄名稱） | `My Server` |
| `{renderTime}` | 渲染執行時間戳（Asia/Taipei 時區） | `2025-01-15 14:30 CST` |

支援語言：
- English (`en.conf`)
- 简体中文 (`zh-CN.conf`)
- 繁體中文 台灣 (`zh-TW.conf`)
- 繁體中文 香港 (`zh-HK.conf`)

## 新增伺服器

1. 建立新的伺服器目錄（例如 `onlinemap-02/`）
2. 加入 `config.toml` 並填寫所有必填欄位
3. 將 BlueMap 設定檔複製到 `config/` 子目錄，並調整地圖設定（世界路徑、維度、渲染範圍等）
4. 在 workflow 中加入對應的 job（見 [README.md](../README.md) 中的多伺服器範例）

# Development

> **[English](en/development.md)**

本文件說明如何從原始碼建置 bluemap-action、程式碼規範，以及 CI/CD 流程。

## 建置

### 前置需求

- **Go 1.24.7+**
- **Java runtime** — 執行 BlueMap CLI（開發時測試用）

### 從原始碼建置

```bash
# 基本建置
go build -o bluemap-action ./cmd/bluemap-action/

# 附帶版本標籤的建置
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" \
  -o bluemap-action ./cmd/bluemap-action/
```

### 本地執行

```bash
export PTERODACTYL_PANEL_URL="https://panel.example.com"
export PTERODACTYL_API_KEY="your-api-key"

# 指定伺服器目錄
./bluemap-action -dir test/test-onlinemap
```

### CLI 參數

| 參數 | 預設值 | 說明 |
|---|---|---|
| `-dir` | `.` | 包含 `config.toml` 的伺服器目錄 |

## 程式碼規範

### 專案佈局

- `cmd/` — 可執行檔進入點
- `internal/` — 內部套件（不可被外部專案引用）
- 標準 Go 專案佈局

### 錯誤處理

- 使用 `fmt.Errorf` 搭配 `%w` 進行錯誤包裝
- 設定驗證採 fail-fast 模式：缺少必填欄位時透過 `log.Fatal` 立即終止
- 日誌輸出至 stdout；警告輸出至 stderr

### 依賴管理

專案僅依賴 `github.com/BurntSushi/toml`，其餘功能皆使用 Go 標準函式庫。新增依賴前請謹慎評估必要性。

### 測試

目前尚未有測試檔案。新增測試時，請遵循 Go 慣例，將 `*_test.go` 放在對應原始碼旁。

## Release 流程

推送符合 `v*` 格式的 tag 時，GitHub Actions 會自動觸發 release 流程（`.github/workflows/release.yml`）：

1. Checkout 完整 Git 歷史
2. 設定 Go 環境（版本來自 `go.mod`）
3. 從 Git tag 取得版本號
4. 交叉編譯多平台二進位檔：
   - `linux/amd64`、`linux/arm64`
   - `darwin/amd64`、`darwin/arm64`
   - `windows/amd64`
5. 產生 SHA256 checksum
6. 建立 GitHub Release 並上傳所有檔案

### 建立新 Release

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions 會自動完成建置與發佈。

## Reusable Workflow 開發

Reusable workflow 定義在 `.github/workflows/build-map.yml`，其他 repository 可以直接呼叫。

### 工作流程 Jobs

工作流程由兩個 job 組成：

#### 1. `check-cache` — 快取偵測與 Runner 選擇

在 `runs-on-cache-hit` runner 上執行（快取查詢為輕量操作，不需要較大的機器）。使用 `actions/cache/restore` 搭配 `lookup-only: true` 探測是否存在 `web/maps` 快取，不會下載檔案。

- **有快取** → 選擇較小的 `runs-on-cache-hit` runner 執行建置 job
- **無快取** → 選擇較大的 `runs-on-cache-miss` runner 執行建置 job

#### 2. `build-map` — 建置與部署

在 `check-cache` 選定的 runner 上執行。步驟：

1. **Checkout** — 取出呼叫方的 repository
2. **Set up Java** — 安裝 Temurin JDK（預設 21）
3. **Download bluemap-action** — 從 GitHub Releases 下載二進位檔
4. **Restore cache** — 還原 `web/maps` 快取（增量渲染）
5. **Build map** — 執行 bluemap-action
6. **Deploy to Netlify** — 條件性部署（可透過 `deploy-to-netlify` 控制）

### 增量渲染

工作流程使用 `actions/cache` 快取 `web/maps` 目錄。BlueMap CLI 支援增量渲染，已渲染過的區塊不會重新處理，大幅縮短後續渲染所需時間。

快取鍵格式：
- 主要鍵：`bluemap-maps-{server-directory}-{run-id}`
- 還原鍵：`bluemap-maps-{server-directory}-`（匹配最近一次的快取）

### 成本優化

`check-cache` job 會根據快取狀態自動選擇合適的 runner 規格：

- **有快取（增量渲染）** — 使用較小的 `runs-on-cache-hit` runner（預設：2 vCPU），僅需重新渲染變動的區塊
- **無快取（完整渲染）** — 使用較大的 `runs-on-cache-miss` runner（預設：8 vCPU），需要完整渲染地圖

`check-cache` 探測 job 在 `runs-on-cache-hit` runner 上執行，因為快取查詢為輕量操作，不需要較大的機器。

### `refresh-cache.yml` — 快取刷新

GitHub Actions 快取超過 7 天未被存取即會被自動清除。`build-map.yml` 每次成功建置後會儲存含 `run_id` 的新快取 entry，理論上每次 build 都會重設計時器。但若渲染週期恰為 7 天，快取可能在 build 前的邊界時刻被清除，導致非預期的完整渲染。

`refresh-cache.yml` 設計為每 5 天執行一次，在到期前主動刷新快取：

1. 使用 `restore-keys` 還原最近一次的 `web/maps` 快取（下載至 runner）
2. Job 結束時以新的 `run_id` 為主要 key 儲存快取 → 確實建立新 entry，重設 7 天計時器
3. 不需要 checkout、Java 或 Pterodactyl 憑證，使用最小 runner 即可

> **為何不只用 `actions/cache/restore`？** GitHub 文件指出快取「7 天未被存取」即過期，但「存取」是否包含 restore（下載）操作並無官方保證。使用完整的 `actions/cache`（含 save）可確保建立新 entry，明確重設計時器，不依賴未定義行為。

快取 key 格式與 `build-map.yml` 相同，兩個 workflow 的快取可互相存取：

- 主要鍵：`bluemap-maps-{server-directory}-{run-id}`
- 還原鍵：`bluemap-maps-{server-directory}-`

### 測試 Reusable Workflow

專案提供 `.github/workflows/test-reusable-workflow.yml`，可手動觸發來測試 reusable workflow：

```bash
gh workflow run test-reusable-workflow.yml
```

這會使用 `test/test-onlinemap` 目錄進行測試建置。

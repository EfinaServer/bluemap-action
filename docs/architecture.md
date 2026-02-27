# Architecture

> **[English](en/architecture.md)**

本文件說明 bluemap-action 的內部架構、執行流程與設計決策。

## 專案結構

```
bluemap-action/
├── cmd/bluemap-action/
│   └── main.go                  # CLI 進入點（執行管線）
├── internal/
│   ├── analyzer/analyzer.go     # 世界檔案與輸出大小分析
│   ├── assets/assets.go         # 靜態資源壓縮參照改寫
│   ├── bluemap/
│   │   ├── download.go          # 從 GitHub Releases 下載 BlueMap CLI jar
│   │   ├── render.go            # 透過 java -jar 執行 BlueMap CLI 渲染
│   │   └── scripts.go           # 執行 scripts/ 目錄中的自訂腳本
│   ├── config/config.go         # TOML 設定檔解析與驗證
│   ├── extractor/extractor.go   # tar.gz 備份下載與世界目錄擷取
│   ├── lang/
│   │   ├── lang.go              # 嵌入式語言檔案部署
│   │   └── files/               # 嵌入的 .conf 語言檔 (en, zh-CN, zh-TW, zh-HK)
│   ├── netlify/deploy.go        # 產生 netlify.toml 靜態網站設定
│   └── pterodactyl/client.go    # Pterodactyl 面板 Client API 整合
├── test/
│   └── test-onlinemap/          # 測試用伺服器設定範例
├── .github/workflows/           # CI/CD 工作流程
├── go.mod                       # Go 1.24.7，唯一依賴：BurntSushi/toml
└── go.sum
```

## 執行管線

`cmd/bluemap-action/main.go` 定義了一個循序執行的管線，處理單一伺服器目錄：

```
┌─────────────────────────────────────────────────────────┐
│ 1. 下載並擷取世界檔案                                       │
│    從 Pterodactyl 取得最新成功備份 → 串流解壓 tar.gz          │
│    → 僅擷取對應的世界目錄                                    │
├─────────────────────────────────────────────────────────┤
│ 2. 分析世界大小                                            │
│    報告擷取的世界大小（vanilla: 維度細分 / plugin: 各資料夾）   │
├─────────────────────────────────────────────────────────┤
│ 3. 下載 BlueMap CLI                                       │
│    從 GitHub Releases 取得 jar（若已快取則跳過）              │
├─────────────────────────────────────────────────────────┤
│ 4. 部署語言檔案                                            │
│    複製嵌入的 .conf 到 web/lang/，替換佔位符                 │
├─────────────────────────────────────────────────────────┤
│ 5. 部署 netlify.toml                                      │
│    寫入靜態網站設定（SPA 重導、gzip 標頭）                    │
├─────────────────────────────────────────────────────────┤
│ 6. 執行自訂腳本                                            │
│    依字母順序執行 scripts/ 中的 .py 與 .sh 腳本              │
│    （若 scripts/ 目錄不存在則自動略過）                       │
├─────────────────────────────────────────────────────────┤
│ 7. 渲染                                                   │
│    執行 java -jar bluemap-cli.jar -v <mcVersion> -r       │
├─────────────────────────────────────────────────────────┤
│ 8. 改寫資源參照                                            │
│    .prbm → .prbm.gz、/textures.json → /textures.json.gz  │
├─────────────────────────────────────────────────────────┤
│ 9. 分析輸出                                               │
│    報告 web/ 目錄總大小                                     │
└─────────────────────────────────────────────────────────┘
```

### GitHub Step Summary

在 CI 環境中（`CI=true`），管線結束時會將建置摘要寫入 `$GITHUB_STEP_SUMMARY`，產生 Markdown 格式的報告，包含：

- **伺服器設定** — 專案名稱、伺服器 ID、類型、世界名稱、Minecraft 版本、BlueMap 版本、渲染時間
- **備份資訊** — 備份名稱、UUID、檔案大小、下載與擷取所需時間
- **渲染** — BlueMap CLI 渲染所需時間
- **世界大小** — 各維度/世界的檔案大小明細
- **Web 輸出** — `web/` 目錄總大小

在非 CI 環境中，此步驟會自動略過。

## 各模組說明

### `internal/pterodactyl`

封裝 Pterodactyl 面板 Client API 的互動邏輯：

- `ListBackups()` — 取得伺服器的所有備份，依建立時間降序排列
- `GetLatestBackup()` — 回傳最近一次成功的備份
- `GetBackupDownloadURL()` — 取得簽署過的下載 URL

### `internal/extractor`

處理備份檔案的下載與解壓，支援三種下載模式（由 `config.toml` 的 `download_mode` 控制）：

- **`auto`（預設）** — 以 `GET Range: bytes=0-0` 請求探測伺服器後自動選擇：伺服器回應 `206 Partial Content` 且 ≥ 64 MB 時使用平行下載（連線數依檔案大小自動調整：< 256 MiB 用 2 條、256 MiB–1 GiB 用 4 條、1–4 GiB 用 8 條、≥ 4 GiB 用 12 條；可透過 `download_connections` 覆寫），否則退回串流單線程（無暫存檔案）。相容 S3 Presigned URL。
- **`parallel`** — 強制平行下載，連線數同樣依檔案大小自動調整；若伺服器不支援 Range 請求或無 `Content-Length` 則報錯
- **`single`** — 強制單線程串流，HTTP 回應直接導入 tar reader，完全不寫入暫存檔案

通用特性：
- 透過世界名稱過濾，僅擷取匹配的目錄
- 包含路徑遍歷保護，確保所有擷取路徑在輸出目錄內
- 單一檔案上限 10 GB

### `internal/config`

解析 TOML 設定檔並驗證必填欄位：

- `Load()` — 載入並驗證單一 `config.toml`
- `LoadAll()` — 掃描目錄下所有含 `config.toml` 的子目錄
- `ResolveWorlds()` — 依據伺服器類型推算世界資料夾列表

### `internal/bluemap`

管理 BlueMap CLI 的下載、執行與自訂腳本執行：

- `EnsureCLI()` — 若 jar 不存在則下載，使用 `.tmp` 暫存再 rename（原子寫入，避免不完整檔案）
- `Render()` — 執行 `java -jar <jar> -v <mcVersion> -r`，即時串流 stdout/stderr
- `RunScripts()` — 依字母順序探索並執行 `scripts/` 子目錄中的 `.py` 與 `.sh` 腳本；若目錄不存在則自動略過

### `internal/lang`

BlueMap 翻譯檔案部署：

- 將 BlueMap 本身的翻譯檔案透過 `//go:embed` 編譯進二進位檔
- 僅保留所需語言 (en, zh-CN, zh-TW, zh-HK)，移除未使用的語言設定
- 部署時替換佔位符：`{toolVersion}`、`{minecraftVersion}`、`{projectName}`、`{renderTime}`

### `internal/netlify`

產生 Netlify 靜態網站設定：

- SPA 回退重導：`/*` → `/index.html`（200 狀態碼）
- gzip 標頭：套用於 `*.json.gz` 與 `*.prbm.gz`

### `internal/assets`

處理靜態資源壓縮參照：

- 掃描 `web/assets/index-*.js` 檔案
- 將 `.prbm` 改寫為 `.prbm.gz`，`/textures.json` 改寫為 `/textures.json.gz`

> Netlify 不支援 wildcard content-encoding rewrite，因此 JS bundle 必須直接參照已壓縮的檔案路徑，而非由伺服器動態協商。

### `internal/analyzer`

世界檔案與輸出大小分析：

- `AnalyzeVanillaWorld()` — 分析 vanilla 伺服器的世界大小（主世界、地獄、終界）
- `AnalyzeWorlds()` — 分析 plugin 伺服器的各世界資料夾大小
- `AnalyzeWebOutput()` — 計算 `web/` 目錄總大小
- `FormatSize()` — 人類可讀的大小格式化（B、KB、MB、GB）

## 設計決策

### 單一依賴

專案僅依賴 `github.com/BurntSushi/toml` 進行設定檔解析，其餘功能皆使用 Go 標準函式庫。這降低了供應鏈風險，並簡化建置流程。

### 三種下載模式

備份下載策略透過 `config.toml` 的 `download_mode` 欄位設定：

- **`auto`（預設）** — 探測伺服器後自動選擇最佳策略
- **`parallel`** — 強制平行下載，連線數依檔案大小自動調整（適合大型備份）
- **`single`** — 強制串流，不寫入暫存檔案（最低磁碟 I/O）

平行下載需要暫存檔案（同一檔案系統以避免跨裝置 rename 問題），各 worker 透過 `WriteAt` 寫入對應偏移量，下載完成後循序讀取進行解壓。串流模式則直接將 HTTP 回應導入 tar reader，完全不接觸磁碟。

### 嵌入式語言檔案

語言檔案透過 Go 的 `//go:embed` 指令編譯進二進位檔。這使得工具可以作為單一可執行檔分發，無需額外的資源檔案。

### 原子檔案寫入

BlueMap CLI jar 下載使用 `.tmp` 暫存檔案加上 rename 的方式，確保不會產生不完整的 jar 檔。若下載中斷，不會留下損壞的檔案。

### 路徑遍歷保護

擷取器驗證所有從 tar 歸檔中擷取的路徑，確保它們位於輸出目錄內。這防止惡意的備份檔案覆寫系統檔案。

### 時區

渲染時間戳使用 `Asia/Taipei` 時區，以配合專案的主要使用者群。

## 版本解析

二進位檔版本依以下順序決定：

1. 建置時透過 `-ldflags "-X main.version=..."` 設定
2. 從 `debug.ReadBuildInfo()` 取得 Git revision（截斷至 7 字元）
3. 回退值：`"dev"`

## 執行環境需求

- **Go 1.24.7+** — 建置工具
- **Java runtime** — 執行 BlueMap CLI
- 網路存取：Pterodactyl 面板 API、GitHub Releases（BlueMap CLI 下載）

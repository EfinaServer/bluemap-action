package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/EfinaServer/bluemap-action/internal/analyzer"
	"github.com/EfinaServer/bluemap-action/internal/assets"
	"github.com/EfinaServer/bluemap-action/internal/bluemap"
	"github.com/EfinaServer/bluemap-action/internal/config"
	"github.com/EfinaServer/bluemap-action/internal/extractor"
	"github.com/EfinaServer/bluemap-action/internal/lang"
	"github.com/EfinaServer/bluemap-action/internal/netlify"
	"github.com/EfinaServer/bluemap-action/internal/pterodactyl"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func getVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				if len(s.Value) > 7 {
					return s.Value[:7]
				}
				return s.Value
			}
		}
	}
	return version
}

// fmtDuration formats a duration as a human-readable string (e.g. "1m 23s").
func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// buildSummary collects data during the run for the GitHub Step Summary.
type buildSummary struct {
	toolVersion      string
	projectName      string
	serverID         string
	serverType       string
	worldName        string
	worlds           []string
	mcVersion        string
	blueMapVersion   string
	renderTime       string
	backupName       string
	backupUUID       string
	backupSize       int64
	downloadDur      time.Duration
	worldRows        []analyzer.WorldSummaryRow
	worldTotal       int64
	webOutputSize    int64
}

// writeGitHubSummary writes a Markdown summary to $GITHUB_STEP_SUMMARY when running
// inside GitHub Actions. It is a no-op when the environment variable is not set.
func writeGitHubSummary(sum *buildSummary) {
	summaryPath := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryPath == "" {
		return
	}

	f, err := os.OpenFile(summaryPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  could not open GITHUB_STEP_SUMMARY: %v\n", err)
		return
	}
	defer f.Close()

	var sb strings.Builder

	sb.WriteString("## 🗺 BlueMap Build Summary\n\n")

	// Server configuration table.
	sb.WriteString("### 📋 Server Configuration\n\n")
	sb.WriteString("| Property | Value |\n")
	sb.WriteString("|:---|:---|\n")
	sb.WriteString(fmt.Sprintf("| **Project** | `%s` |\n", sum.projectName))
	sb.WriteString(fmt.Sprintf("| **Server ID** | `%s` |\n", sum.serverID))
	sb.WriteString(fmt.Sprintf("| **Server Type** | `%s` |\n", sum.serverType))
	sb.WriteString(fmt.Sprintf("| **World** | `%s` |\n", sum.worldName))
	sb.WriteString(fmt.Sprintf("| **Minecraft** | `%s` |\n", sum.mcVersion))
	sb.WriteString(fmt.Sprintf("| **BlueMap CLI** | `v%s` |\n", sum.blueMapVersion))
	sb.WriteString(fmt.Sprintf("| **Rendered At** | %s |\n", sum.renderTime))
	sb.WriteString("\n")

	// Backup section.
	sb.WriteString("### 💾 Backup\n\n")
	sb.WriteString("| Property | Value |\n")
	sb.WriteString("|:---|:---|\n")
	sb.WriteString(fmt.Sprintf("| **Name** | %s |\n", sum.backupName))
	sb.WriteString(fmt.Sprintf("| **UUID** | `%s` |\n", sum.backupUUID))
	sb.WriteString(fmt.Sprintf("| **Size** | %s |\n", analyzer.FormatSize(sum.backupSize)))
	sb.WriteString(fmt.Sprintf("| **Download + Extraction** | %s |\n", fmtDuration(sum.downloadDur)))
	sb.WriteString("\n")

	// World sizes section.
	sb.WriteString("### 🌍 World Sizes\n\n")
	sb.WriteString("| World | Size |\n")
	sb.WriteString("|:---|---:|\n")
	for _, row := range sum.worldRows {
		if row.Found {
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", row.Label, analyzer.FormatSize(row.Size)))
		} else {
			sb.WriteString(fmt.Sprintf("| %s | *(not found)* |\n", row.Label))
		}
	}
	sb.WriteString(fmt.Sprintf("| **TOTAL** | **%s** |\n", analyzer.FormatSize(sum.worldTotal)))
	sb.WriteString("\n")

	// Web output section.
	sb.WriteString("### 📊 Web Output\n\n")
	sb.WriteString("| Property | Value |\n")
	sb.WriteString("|:---|---:|\n")
	sb.WriteString(fmt.Sprintf("| **Total Size** | %s |\n", analyzer.FormatSize(sum.webOutputSize)))
	sb.WriteString("\n")

	if _, err := f.WriteString(sb.String()); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  could not write to GITHUB_STEP_SUMMARY: %v\n", err)
	}
}

func main() {
	serverDir := flag.String("dir", ".", "server directory containing config.toml (e.g. onlinemap-01)")
	flag.Parse()

	panelURL := os.Getenv("PTERODACTYL_PANEL_URL")
	apiKey := os.Getenv("PTERODACTYL_API_KEY")

	if panelURL == "" {
		log.Fatal("PTERODACTYL_PANEL_URL environment variable is required")
	}
	if apiKey == "" {
		log.Fatal("PTERODACTYL_API_KEY environment variable is required")
	}

	toolVersion := getVersion()
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		panic(err)
	}

	renderTime := time.Now().In(loc).Format("2006-01-02 15:04 MST")

	fmt.Printf("🗺  bluemap-action %s\n\n", toolVersion)

	// Load config from the server directory.
	srv, err := config.Load(*serverDir)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	worlds := srv.Config.ResolveWorlds()

	projectName := filepath.Base(srv.Dir)
	if srv.Config.Name != "" {
		projectName = srv.Config.Name
	}

	sum := &buildSummary{
		toolVersion:    toolVersion,
		projectName:    projectName,
		serverID:       srv.Config.ServerID,
		serverType:     srv.Config.ServerType,
		worldName:      srv.Config.WorldName,
		worlds:         worlds,
		mcVersion:      srv.Config.MinecraftVersion,
		blueMapVersion: srv.Config.BlueMapVersion,
		renderTime:     renderTime,
	}

	fmt.Printf("📋  %s  (server: %s)\n", projectName, srv.Config.ServerID)
	fmt.Printf("    server type:        %s\n", srv.Config.ServerType)
	fmt.Printf("    world name:         %s\n", srv.Config.WorldName)
	fmt.Printf("    worlds:             %v\n", worlds)
	fmt.Printf("    minecraft version:  %s\n", srv.Config.MinecraftVersion)
	fmt.Printf("    bluemap version:    %s\n\n", srv.Config.BlueMapVersion)

	// Step 1: Download and extract world data from Pterodactyl backup.
	client := pterodactyl.NewClient(panelURL, apiKey)

	backup, err := client.GetLatestBackup(srv.Config.ServerID)
	if err != nil {
		log.Fatalf("💥  error getting latest backup: %v", err)
	}

	sum.backupName = backup.Name
	sum.backupUUID = backup.UUID
	sum.backupSize = backup.Bytes

	fmt.Printf("💾  Latest backup: %s (%s, %s)\n", backup.Name, backup.UUID, analyzer.FormatSize(backup.Bytes))

	downloadURL, err := client.GetBackupDownloadURL(srv.Config.ServerID, backup.UUID)
	if err != nil {
		log.Fatalf("💥  error getting download URL: %v", err)
	}

	fmt.Printf("⬇️   Downloading and extracting worlds: %v\n", worlds)

	downloadStart := time.Now()
	if err := extractor.DownloadAndExtractWorlds(downloadURL, srv.Dir, worlds); err != nil {
		log.Fatalf("💥  error extracting worlds: %v", err)
	}
	downloadDur := time.Since(downloadStart)

	sum.downloadDur = downloadDur
	fmt.Printf("⏱   Download + extraction took %s\n", fmtDuration(downloadDur))

	// Step 2: Analyze extracted world sizes.
	fmt.Println()
	worldTotal, worldRows := analyzer.PrintWorldAnalysis(srv.Config.ServerType, srv.Dir, worlds)
	sum.worldRows = worldRows
	sum.worldTotal = worldTotal

	// Step 3: Download BlueMap CLI.
	fmt.Println()
	fmt.Printf("📦  BlueMap CLI v%s\n", srv.Config.BlueMapVersion)
	jarPath, err := bluemap.EnsureCLI(srv.Dir, srv.Config.BlueMapVersion)
	if err != nil {
		log.Fatalf("💥  error downloading BlueMap CLI: %v", err)
	}

	// Step 4: Deploy language files before rendering.
	langDir := filepath.Join(srv.Dir, "web", "lang")
	langCfg := lang.DeployConfig{
		ToolVersion:      toolVersion,
		MinecraftVersion: srv.Config.MinecraftVersion,
		ProjectName:      projectName,
		RenderTime:       renderTime,
	}

	fmt.Printf("\n📝  Deploying language files → %s\n", langDir)
	if err := lang.Deploy(langDir, langCfg); err != nil {
		log.Fatalf("💥  error deploying lang files: %v", err)
	}

	// Step 5: Deploy netlify.toml for static site hosting.
	fmt.Printf("📝  Deploying netlify.toml → %s\n", filepath.Join(srv.Dir, "web"))
	if err := netlify.DeployConfig(srv.Dir); err != nil {
		log.Fatalf("💥  error deploying netlify.toml: %v", err)
	}

	// Step 6: Execute BlueMap CLI rendering.
	fmt.Printf("\n🔨  Running BlueMap CLI render...\n")
	if err := bluemap.Render(jarPath, srv.Dir, srv.Config.MinecraftVersion); err != nil {
		log.Fatalf("💥  error during rendering: %v", err)
	}

	// Step 7: Rewrite asset references to compressed variants.
	fmt.Printf("\n✏️   Rewriting asset references to compressed variants...\n")
	if err := assets.RewriteCompressedRefs(srv.Dir); err != nil {
		log.Fatalf("💥  error rewriting asset references: %v", err)
	}

	// Step 8: Analyze web output size after rendering.
	fmt.Println()
	webSize, err := analyzer.AnalyzeWebOutput(srv.Dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  could not analyze web output: %v\n", err)
	} else {
		sum.webOutputSize = webSize
		fmt.Printf("📊  Web Output Analysis\n")
		fmt.Printf("    web/ total size:  %s\n", analyzer.FormatSize(webSize))
	}

	// Write GitHub Step Summary (no-op if not running inside GitHub Actions).
	writeGitHubSummary(sum)

	fmt.Printf("\n✅  Done!\n")
}

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/EfinaServer/bluemap-action/internal/analyzer"
	"github.com/EfinaServer/bluemap-action/internal/bluemap"
	"github.com/EfinaServer/bluemap-action/internal/config"
	"github.com/EfinaServer/bluemap-action/internal/extractor"
	"github.com/EfinaServer/bluemap-action/internal/lang"
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
	renderTime := time.Now().UTC().Format("2006-01-02 15:04 UTC")

	fmt.Printf("bluemap-action %s\n\n", toolVersion)

	// Load config from the server directory directly.
	srv, err := config.Load(*serverDir)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	worlds := srv.Config.ResolveWorlds()

	fmt.Printf("=== %s (server: %s) ===\n", filepath.Base(srv.Dir), srv.Config.ServerID)
	fmt.Printf("  server type:     %s\n", srv.Config.ServerType)
	fmt.Printf("  world name:      %s\n", srv.Config.WorldName)
	fmt.Printf("  worlds:          %v\n", worlds)
	fmt.Printf("  bluemap version: %s\n\n", srv.Config.BlueMapVersion)

	// Step 1: Download and extract world data from Pterodactyl backup.
	client := pterodactyl.NewClient(panelURL, apiKey)

	backup, err := client.GetLatestBackup(srv.Config.ServerID)
	if err != nil {
		log.Fatalf("error getting latest backup: %v", err)
	}

	fmt.Printf("  latest backup: %s (%s, %s)\n", backup.Name, backup.UUID, analyzer.FormatSize(int64(backup.Bytes)))

	downloadURL, err := client.GetBackupDownloadURL(srv.Config.ServerID, backup.UUID)
	if err != nil {
		log.Fatalf("error getting download URL: %v", err)
	}

	fmt.Printf("  downloading and extracting worlds: %v\n", worlds)

	if err := extractor.DownloadAndExtractWorlds(downloadURL, srv.Dir, worlds); err != nil {
		log.Fatalf("error extracting worlds: %v", err)
	}

	// Step 2: Analyze extracted world sizes.
	fmt.Println()
	analyzer.PrintWorldAnalysis(srv.Config.ServerType, srv.Dir, worlds)

	// Step 3: Download BlueMap CLI.
	fmt.Println()
	jarPath, err := bluemap.EnsureCLI(srv.Dir, srv.Config.BlueMapVersion)
	if err != nil {
		log.Fatalf("error downloading BlueMap CLI: %v", err)
	}

	// Step 4: Deploy language files before rendering.
	projectName := filepath.Base(srv.Dir)
	if srv.Config.Name != "" {
		projectName = srv.Config.Name
	}

	langDir := filepath.Join(srv.Dir, "web", "lang")
	langCfg := lang.DeployConfig{
		ToolVersion: toolVersion,
		ProjectName: projectName,
		RenderTime:  renderTime,
	}

	fmt.Printf("\n  deploying language files to %s\n", langDir)
	if err := lang.Deploy(langDir, langCfg); err != nil {
		log.Fatalf("error deploying lang files: %v", err)
	}

	// Step 5: Execute BlueMap CLI rendering.
	fmt.Println()
	if err := bluemap.Render(jarPath, srv.Dir); err != nil {
		log.Fatalf("error during rendering: %v", err)
	}

	// Step 6: Analyze web output size after rendering.
	fmt.Println()
	webSize, err := analyzer.AnalyzeWebOutput(srv.Dir)
	if err != nil {
		log.Printf("warning: could not analyze web output: %v", err)
	} else {
		fmt.Printf("  --- Web Output Analysis ---\n")
		fmt.Printf("    web/ total size:  %s\n", analyzer.FormatSize(webSize))
		fmt.Printf("  ---------------------------\n")
	}

	fmt.Printf("\n  done!\n")
}

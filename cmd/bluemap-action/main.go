package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

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
	baseDir := flag.String("dir", ".", "base directory containing server subdirectories with config.toml")
	flag.Parse()

	panelURL := os.Getenv("PTERODACTYL_PANEL_URL")
	apiKey := os.Getenv("PTERODACTYL_API_KEY")

	if panelURL == "" {
		log.Fatal("PTERODACTYL_PANEL_URL environment variable is required")
	}
	if apiKey == "" {
		log.Fatal("PTERODACTYL_API_KEY environment variable is required")
	}

	servers, err := config.LoadAll(*baseDir)
	if err != nil {
		log.Fatalf("loading configs: %v", err)
	}

	client := pterodactyl.NewClient(panelURL, apiKey)

	toolVersion := getVersion()
	renderTime := time.Now().UTC().Format("2006-01-02 15:04 UTC")

	fmt.Printf("bluemap-action %s\n\n", toolVersion)

	var failed bool
	for _, srv := range servers {
		fmt.Printf("=== %s (server: %s) ===\n", srv.Dir, srv.Config.ServerID)

		backup, err := client.GetLatestBackup(srv.Config.ServerID)
		if err != nil {
			log.Printf("error getting latest backup for %s: %v", srv.Dir, err)
			failed = true
			continue
		}

		fmt.Printf("  latest backup: %s (%s, %d bytes)\n", backup.Name, backup.UUID, backup.Bytes)

		downloadURL, err := client.GetBackupDownloadURL(srv.Config.ServerID, backup.UUID)
		if err != nil {
			log.Printf("error getting download URL for %s: %v", srv.Dir, err)
			failed = true
			continue
		}

		fmt.Printf("  downloading and extracting worlds: %v\n", srv.Config.Worlds)

		if err := extractor.DownloadAndExtractWorlds(downloadURL, srv.Dir, srv.Config.Worlds); err != nil {
			log.Printf("error extracting worlds for %s: %v", srv.Dir, err)
			failed = true
			continue
		}

		// Deploy shared language files with project-specific info.
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

		fmt.Printf("  deploying language files to %s\n", langDir)
		if err := lang.Deploy(langDir, langCfg); err != nil {
			log.Printf("error deploying lang files for %s: %v", srv.Dir, err)
			failed = true
			continue
		}

		fmt.Printf("  done!\n\n")
	}

	if failed {
		os.Exit(1)
	}
}

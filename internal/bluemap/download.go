package bluemap

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// CLIJarName returns the expected jar filename for the given version.
func CLIJarName(version string) string {
	return fmt.Sprintf("bluemap-%s-cli.jar", version)
}

// DownloadURL returns the GitHub release download URL for the given version.
func DownloadURL(version string) string {
	return fmt.Sprintf(
		"https://github.com/BlueMap-Minecraft/BlueMap/releases/download/v%s/%s",
		version, CLIJarName(version),
	)
}

// EnsureCLI downloads the BlueMap CLI jar into serverDir if it doesn't already
// exist. Returns the absolute path to the jar file.
func EnsureCLI(serverDir, version string) (string, error) {
	jarPath := filepath.Join(serverDir, CLIJarName(version))

	if info, err := os.Stat(jarPath); err == nil && info.Size() > 0 {
		fmt.Printf("  ✔  BlueMap CLI %s already cached (%s)\n", version, formatSize(info.Size()))
		return jarPath, nil
	}

	url := DownloadURL(version)
	fmt.Printf("  ⬇️  downloading BlueMap CLI %s\n", version)
	fmt.Printf("     URL: %s\n", url)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading BlueMap CLI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned status %d for %s", resp.StatusCode, url)
	}

	tmpPath := jarPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}

	written, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("writing jar file: %w", err)
	}

	if err := os.Rename(tmpPath, jarPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("renaming temp file: %w", err)
	}

	fmt.Printf("  ✔  downloaded %s\n", formatSize(written))
	return jarPath, nil
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

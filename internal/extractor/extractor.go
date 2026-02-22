package extractor

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadAndExtractWorlds downloads a backup from the given URL and extracts
// only the specified world directories into outputDir.
//
// The backup is expected to be a tar.gz archive. World folders are matched by
// checking if a tar entry path starts with one of the world names (e.g.
// "world/", "world_nether/").
//
// Each world is extracted into outputDir/<worldName>/.
func DownloadAndExtractWorlds(downloadURL, outputDir string, worlds []string) error {
	client := &http.Client{Timeout: 30 * time.Minute}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("downloading backup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	// Build a set for quick lookup of world prefixes.
	worldSet := make(map[string]bool, len(worlds))
	for _, w := range worlds {
		worldSet[w] = true
	}

	extracted := make(map[string]int)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		// Determine which world this entry belongs to.
		matchedWorld := matchWorld(header.Name, worldSet)
		if matchedWorld == "" {
			continue
		}

		targetPath := filepath.Join(outputDir, header.Name)

		// Prevent path traversal.
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(outputDir)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("creating parent directory for %s: %w", targetPath, err)
			}
			if err := writeFile(targetPath, tr, header.FileInfo().Mode()); err != nil {
				return fmt.Errorf("writing file %s: %w", targetPath, err)
			}
			extracted[matchedWorld]++
		}
	}

	// Verify all worlds were found.
	for _, w := range worlds {
		if extracted[w] == 0 {
			fmt.Fprintf(os.Stderr, "  ⚠️  world %q was not found in the backup\n", w)
		} else {
			fmt.Printf("  ✔  extracted %d files for world %q\n", extracted[w], w)
		}
	}

	return nil
}

// matchWorld returns the world name if the tar entry path begins with one of
// the world names followed by a slash, or is the world directory itself.
func matchWorld(entryPath string, worldSet map[string]bool) string {
	// Normalize: remove leading "./" if present.
	clean := strings.TrimPrefix(entryPath, "./")

	// Check each world prefix.
	for w := range worldSet {
		if clean == w || clean == w+"/" || strings.HasPrefix(clean, w+"/") {
			return w
		}
	}
	return ""
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	// Limit copy size to 10 GB as a safety measure.
	_, err = io.Copy(f, io.LimitReader(r, 10<<30))
	return err
}

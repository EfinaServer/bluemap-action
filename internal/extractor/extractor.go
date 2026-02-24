package extractor

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// downloadWorkers is the number of parallel HTTP connections used when the
	// server supports Range requests.
	downloadWorkers = 4

	// minParallelSize is the minimum backup size to trigger parallel download.
	// Small files are not worth the overhead of spawning multiple connections.
	minParallelSize = 64 << 20 // 64 MB
)

// DownloadAndExtractWorlds downloads a backup from the given URL and extracts
// only the specified world directories into outputDir.
//
// When the server advertises Accept-Ranges: bytes and a known Content-Length,
// the download is split across multiple parallel connections for higher
// throughput. Otherwise it falls back to a single streaming connection.
//
// The backup is expected to be a tar.gz archive. World folders are matched by
// checking if a tar entry path starts with one of the world names (e.g.
// "world/", "world_nether/").
func DownloadAndExtractWorlds(downloadURL, outputDir string, worlds []string) error {
	contentLength, rangeOK, err := probeDownload(downloadURL)
	if err != nil {
		return fmt.Errorf("probing download URL: %w", err)
	}

	// Create a temp file in outputDir for the downloaded archive.
	// Using the same filesystem avoids cross-device rename issues and keeps
	// disk usage predictable.
	tmpFile, err := os.CreateTemp(outputDir, ".backup-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if rangeOK && contentLength >= minParallelSize {
		fmt.Printf("  → parallel download (%d connections, %s)\n",
			downloadWorkers, formatBytes(contentLength))
		if err := downloadParallel(downloadURL, tmpFile, contentLength, downloadWorkers); err != nil {
			tmpFile.Close()
			return fmt.Errorf("parallel download: %w", err)
		}
	} else {
		if contentLength > 0 {
			fmt.Printf("  → downloading %s\n", formatBytes(contentLength))
		}
		if err := downloadSingle(downloadURL, tmpFile); err != nil {
			tmpFile.Close()
			return fmt.Errorf("downloading backup: %w", err)
		}
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Re-open the temp file for extraction.
	f, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("opening downloaded archive: %w", err)
	}
	defer f.Close()

	return extractWorlds(f, outputDir, worlds)
}

// probeDownload sends a HEAD request to discover the content length and whether
// the server supports HTTP Range requests. Returns (0, false, nil) on any
// failure so the caller can fall back to single-threaded download.
func probeDownload(url string) (contentLength int64, rangeSupported bool, err error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0, false, nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, false, nil
	}

	cl := resp.ContentLength
	ar := strings.ToLower(resp.Header.Get("Accept-Ranges"))
	return cl, ar == "bytes" && cl > 0, nil
}

// downloadParallel downloads the resource at url using numWorkers parallel
// HTTP Range requests and writes the result into f (pre-truncated to
// contentLength bytes). A progress line is printed every 5 seconds.
func downloadParallel(url string, f *os.File, contentLength int64, numWorkers int) error {
	// Pre-allocate the file so each worker can WriteAt its own section
	// without interfering with others.
	if err := f.Truncate(contentLength); err != nil {
		return fmt.Errorf("pre-allocating %s: %w", formatBytes(contentLength), err)
	}

	chunkSize := contentLength / int64(numWorkers)

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		firstErr   error
		downloaded atomic.Int64
	)

	// Progress reporter goroutine.
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				got := downloaded.Load()
				pct := float64(got) / float64(contentLength) * 100
				fmt.Printf("  → %s / %s (%.0f%%)\n",
					formatBytes(got), formatBytes(contentLength), pct)
			case <-progressDone:
				return
			}
		}
	}()
	defer close(progressDone)

	sharedClient := &http.Client{Timeout: 30 * time.Minute}

	for i := 0; i < numWorkers; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == numWorkers-1 {
			end = contentLength - 1
		}

		wg.Add(1)
		go func(workerID int, start, end int64) {
			defer wg.Done()
			if err := downloadChunk(sharedClient, url, f, start, end, &downloaded); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("worker %d (bytes %d-%d): %w", workerID, start, end, err)
				}
				mu.Unlock()
			}
		}(i, start, end)
	}

	wg.Wait()
	return firstErr
}

// downloadChunk fetches bytes [start, end] from url using a Range request and
// writes them into f at the correct offset. downloaded is updated atomically
// as bytes arrive.
func downloadChunk(client *http.Client, url string, f *os.File, start, end int64, downloaded *atomic.Int64) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("expected 206 Partial Content, got %d", resp.StatusCode)
	}

	buf := make([]byte, 256<<10) // 256 KB read buffer per worker
	offset := start
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.WriteAt(buf[:n], offset); writeErr != nil {
				return writeErr
			}
			offset += int64(n)
			downloaded.Add(int64(n))
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

// downloadSingle downloads url to f using a single connection (fallback for
// servers that do not support Range requests).
func downloadSingle(url string, f *os.File) error {
	client := &http.Client{Timeout: 30 * time.Minute}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	const limit = 10 << 30 // 10 GB safety cap
	_, err = io.Copy(f, io.LimitReader(resp.Body, limit))
	return err
}

// extractWorlds reads a tar.gz archive from r and extracts only the world
// directories listed in worlds into outputDir.
func extractWorlds(r io.Reader, outputDir string, worlds []string) error {
	gz, err := gzip.NewReader(r)
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
		if errors.Is(err, io.EOF) {
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

	// Limit copy size to 10 GB as a safety measure against malformed archives.
	const limit = 10 << 30 // 10 GB
	lr := &io.LimitedReader{R: r, N: limit}
	if _, err = io.Copy(f, lr); err != nil {
		return err
	}
	if lr.N == 0 {
		return fmt.Errorf("file exceeds maximum allowed size of %d bytes", limit)
	}
	return nil
}

// formatBytes formats a byte count as a human-readable string (e.g. "1.5 GiB").
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

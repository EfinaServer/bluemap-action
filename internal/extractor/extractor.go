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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// minParallelSize is the minimum backup size to trigger parallel download.
	// Small files are not worth the overhead of spawning multiple connections.
	minParallelSize = 64 << 20 // 64 MB
)

// DownloadOptions configures the download behavior.
type DownloadOptions struct {
	Mode        string // "auto", "parallel", "single"
	Connections int    // 0 = auto (size-based scaling), >0 = manual override (1-32)
}

// connectionCount returns the number of parallel download connections to use
// based on the file size. Larger files benefit from more connections because a
// single HTTP stream rarely saturates a high-bandwidth link.
func connectionCount(contentLength int64) int {
	switch {
	case contentLength < 256<<20: // < 256 MiB
		return 2
	case contentLength < 1<<30: // 256 MiB – 1 GiB
		return 4
	case contentLength < 4<<30: // 1 GiB – 4 GiB
		return 8
	default: // >= 4 GiB
		return 12
	}
}

// DownloadAndExtractWorlds downloads a backup from the given URL and extracts
// only the specified world directories into outputDir.
//
// opts.Mode selects the download strategy:
//   - "auto"     — probe the server; use parallel if it supports Range requests
//     and the file is ≥ 64 MB, otherwise stream directly (no temp file).
//   - "parallel" — force parallel download; returns an error if the server does
//     not support Range requests or does not report Content-Length.
//   - "single"   — force a single HTTP connection and stream the response body
//     directly into the tar reader without writing a temp file to disk.
//
// opts.Connections overrides the automatic connection count when > 0.
//
// The backup is expected to be a tar.gz archive. World folders are matched by
// checking if a tar entry path starts with one of the world names (e.g.
// "world/", "world_nether/").
func DownloadAndExtractWorlds(downloadURL, outputDir string, worlds []string, opts DownloadOptions) error {
	switch opts.Mode {
	case "parallel":
		return downloadParallelExtract(downloadURL, outputDir, worlds, opts.Connections)
	case "single":
		fmt.Println("  → single-connection download (streaming, forced)")
		return downloadStreamExtract(downloadURL, outputDir, worlds)
	default: // "auto"
		return downloadAutoExtract(downloadURL, outputDir, worlds, opts.Connections)
	}
}

// downloadAutoExtract probes the server and chooses the best strategy:
// parallel (temp file) when Range is supported and size ≥ 64 MB, otherwise
// a single streaming connection (no temp file).
func downloadAutoExtract(downloadURL, outputDir string, worlds []string, connOverride int) error {
	contentLength, rangeOK, err := probeDownload(downloadURL)
	if err != nil {
		return fmt.Errorf("probing download URL: %w", err)
	}

	if rangeOK && contentLength >= minParallelSize {
		numWorkers := connectionCount(contentLength)
		if connOverride > 0 {
			numWorkers = connOverride
		}
		fmt.Printf("  → parallel download (%d connections, %s)\n",
			numWorkers, formatBytes(contentLength))
		return parallelDownloadAndExtract(downloadURL, outputDir, worlds, contentLength, numWorkers)
	}

	// Log why we are falling back to a single connection.
	if !rangeOK {
		if contentLength > 0 {
			fmt.Printf("  → single-connection download (%s, server does not support Range requests)\n",
				formatBytes(contentLength))
		} else {
			fmt.Println("  → single-connection download (size unknown, server does not support Range requests)")
		}
	} else {
		// rangeOK but file is below the parallel threshold.
		fmt.Printf("  → single-connection download (%s, below %s parallel threshold)\n",
			formatBytes(contentLength), formatBytes(minParallelSize))
	}
	return downloadStreamExtract(downloadURL, outputDir, worlds)
}

// downloadParallelExtract forces parallel download. It probes the server first
// and returns an error if Range requests or Content-Length are not available.
func downloadParallelExtract(downloadURL, outputDir string, worlds []string, connOverride int) error {
	contentLength, rangeOK, err := probeDownload(downloadURL)
	if err != nil {
		return fmt.Errorf("probing download URL: %w", err)
	}
	if !rangeOK {
		return fmt.Errorf("server does not support HTTP Range requests; cannot use parallel download mode")
	}
	if contentLength <= 0 {
		return fmt.Errorf("server did not return Content-Length; cannot use parallel download mode")
	}

	numWorkers := connectionCount(contentLength)
	if connOverride > 0 {
		numWorkers = connOverride
	}

	fmt.Printf("  → parallel download (%d connections, %s, forced)\n",
		numWorkers, formatBytes(contentLength))
	return parallelDownloadAndExtract(downloadURL, outputDir, worlds, contentLength, numWorkers)
}

// parallelDownloadAndExtract downloads the file in parallel into a temp file,
// then extracts worlds from it. The temp file is removed on return.
func parallelDownloadAndExtract(downloadURL, outputDir string, worlds []string, contentLength int64, numWorkers int) error {
	// Create a temp file in outputDir for the downloaded archive.
	// Using the same filesystem avoids cross-device rename issues and keeps
	// disk usage predictable.
	tmpFile, err := os.CreateTemp(outputDir, ".backup-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if err := downloadParallel(downloadURL, tmpFile, contentLength, numWorkers); err != nil {
		tmpFile.Close()
		return fmt.Errorf("parallel download: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Re-open the temp file for sequential extraction.
	f, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("opening downloaded archive: %w", err)
	}
	defer f.Close()

	return extractWorlds(f, outputDir, worlds)
}

// downloadStreamExtract downloads via a single HTTP connection and pipes the
// response body directly into the tar reader — no temp file is written to disk.
func downloadStreamExtract(downloadURL, outputDir string, worlds []string) error {
	client := &http.Client{Timeout: 30 * time.Minute}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	const limit = 10 << 30 // 10 GB safety cap
	return extractWorlds(io.LimitReader(resp.Body, limit), outputDir, worlds)
}

// probeDownload sends a GET request with Range: bytes=0-0 to discover whether
// the server supports HTTP Range requests and to determine the total content
// length. Using GET instead of HEAD ensures compatibility with S3 Presigned
// URLs, which are typically signed only for the GET method.
//
// Returns (0, false, nil) on any non-fatal failure so the caller can
// gracefully fall back to single-connection download.
func probeDownload(url string) (contentLength int64, rangeSupported bool, err error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, false, nil
	}
	req.Header.Set("Range", "bytes=0-0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, false, nil
	}
	defer resp.Body.Close()

	// Drain the response body so the underlying TCP connection can be reused.
	// Capped at 1 KiB to avoid accidentally streaming a large body when the
	// server ignores the Range header and returns 200 with the full file.
	io.CopyN(io.Discard, resp.Body, 1024)

	switch resp.StatusCode {
	case http.StatusPartialContent: // 206 — server supports Range requests
		cl, ok := parseContentRange(resp.Header.Get("Content-Range"))
		if !ok || cl <= 0 {
			return 0, false, nil
		}
		return cl, true, nil

	case http.StatusOK: // 200 — server ignored Range header; no Range support
		cl := resp.ContentLength
		if cl <= 0 {
			return 0, false, nil
		}
		return cl, false, nil

	default:
		return 0, false, nil
	}
}

// parseContentRange extracts the total size from a Content-Range header value.
// The expected format is "bytes START-END/TOTAL" (e.g. "bytes 0-0/123456789").
// Returns (total, true) on success, or (0, false) if the header is missing,
// malformed, or uses "*" for the total.
func parseContentRange(header string) (int64, bool) {
	if header == "" {
		return 0, false
	}
	slashIdx := strings.LastIndex(header, "/")
	if slashIdx < 0 || slashIdx == len(header)-1 {
		return 0, false
	}
	totalStr := header[slashIdx+1:]
	if totalStr == "*" {
		return 0, false
	}
	total, err := strconv.ParseInt(totalStr, 10, 64)
	if err != nil {
		return 0, false
	}
	return total, true
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
	// Use limit+1 so a file of exactly limit bytes is not falsely rejected.
	const limit = 10 << 30 // 10 GB
	n, err := io.Copy(f, io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
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

package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
)

// DirSize calculates the total size of all files in a directory recursively.
func DirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// WorldReport holds size information for a single world directory.
type WorldReport struct {
	Name   string
	Size   int64
	Exists bool
}

// AnalyzeWorlds reports the size of each world directory and returns the total.
func AnalyzeWorlds(serverDir string, worlds []string) ([]WorldReport, int64) {
	var reports []WorldReport
	var total int64

	for _, w := range worlds {
		worldPath := filepath.Join(serverDir, w)
		info, err := os.Stat(worldPath)
		if err != nil || !info.IsDir() {
			reports = append(reports, WorldReport{Name: w, Exists: false})
			continue
		}

		size, err := DirSize(worldPath)
		if err != nil {
			reports = append(reports, WorldReport{Name: w, Exists: true, Size: 0})
			continue
		}

		reports = append(reports, WorldReport{Name: w, Exists: true, Size: size})
		total += size
	}

	return reports, total
}

// PrintWorldAnalysis prints world size analysis to stdout.
func PrintWorldAnalysis(reports []WorldReport, total int64) {
	fmt.Println("  --- World Size Analysis ---")
	for _, r := range reports {
		if !r.Exists {
			fmt.Printf("    %-20s  (not found)\n", r.Name)
		} else {
			fmt.Printf("    %-20s  %s\n", r.Name, FormatSize(r.Size))
		}
	}
	fmt.Printf("    %-20s  %s\n", "TOTAL", FormatSize(total))
	fmt.Println("  ---------------------------")
}

// AnalyzeWebOutput reports the total size of the web output directory.
func AnalyzeWebOutput(serverDir string) (int64, error) {
	webDir := filepath.Join(serverDir, "web")
	info, err := os.Stat(webDir)
	if err != nil {
		return 0, fmt.Errorf("web directory not found: %w", err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("web path is not a directory")
	}
	return DirSize(webDir)
}

// FormatSize formats bytes into human-readable size string.
func FormatSize(bytes int64) string {
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

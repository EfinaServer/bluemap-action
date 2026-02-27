package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EfinaServer/bluemap-action/internal/config"
)

// WorldSummaryRow is a single row for the GitHub Step Summary world table.
type WorldSummaryRow struct {
	Label string
	Size  int64
	Found bool
}

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

// dirSizeExcluding calculates the total size of all files in a directory,
// excluding any subdirectories whose names match the given prefixes.
func dirSizeExcluding(root string, excludeDirs []string) (int64, error) {
	var total int64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != root {
			rel, _ := filepath.Rel(root, path)
			// Only check top-level subdirectories.
			if !strings.Contains(rel, string(os.PathSeparator)) {
				for _, exc := range excludeDirs {
					if rel == exc {
						return filepath.SkipDir
					}
				}
			}
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

// DimensionReport holds size information broken down by dimension for a vanilla world.
type DimensionReport struct {
	WorldName string
	Overworld WorldReport
	Nether    WorldReport
	End       WorldReport
	Total     int64
}

// AnalyzeWorlds reports the size of each world directory for plugin-type servers.
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

// AnalyzeVanillaWorld reports the size of a vanilla world directory, broken
// down by dimension (overworld files, DIM-1, DIM1).
func AnalyzeVanillaWorld(serverDir, worldName string) (*DimensionReport, error) {
	worldPath := filepath.Join(serverDir, worldName)
	info, err := os.Stat(worldPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("world directory %q not found", worldPath)
	}

	report := &DimensionReport{
		WorldName: worldName,
	}

	// Nether: DIM-1/
	netherPath := filepath.Join(worldPath, "DIM-1")
	if info, err := os.Stat(netherPath); err == nil && info.IsDir() {
		size, _ := DirSize(netherPath)
		report.Nether = WorldReport{Name: worldName + "/DIM-1", Size: size, Exists: true}
		report.Total += size
	}

	// End: DIM1/
	endPath := filepath.Join(worldPath, "DIM1")
	if info, err := os.Stat(endPath); err == nil && info.IsDir() {
		size, _ := DirSize(endPath)
		report.End = WorldReport{Name: worldName + "/DIM1", Size: size, Exists: true}
		report.Total += size
	}

	// Overworld: everything in the world folder except DIM-1/ and DIM1/
	overworldSize, _ := dirSizeExcluding(worldPath, []string{"DIM-1", "DIM1"})
	report.Overworld = WorldReport{Name: worldName, Size: overworldSize, Exists: true}
	report.Total += overworldSize

	return report, nil
}

// PrintWorldAnalysis prints world size analysis to stdout based on server type.
// It also returns a slice of rows for use in the GitHub Step Summary.
func PrintWorldAnalysis(serverType, serverDir string, worlds []string) (int64, []WorldSummaryRow) {
	fmt.Println("🌍  World Size Analysis")

	var grandTotal int64
	var rows []WorldSummaryRow

	switch serverType {
	case config.ServerTypeVanilla:
		for _, w := range worlds {
			report, err := AnalyzeVanillaWorld(serverDir, w)
			if err != nil {
				fmt.Printf("    %-25s  (not found)\n", w)
				rows = append(rows, WorldSummaryRow{Label: w + " (overworld)", Found: false})
				continue
			}

			fmt.Printf("    %-25s  %s\n", report.Overworld.Name+" (overworld)", FormatSize(report.Overworld.Size))
			rows = append(rows, WorldSummaryRow{Label: report.Overworld.Name + " (overworld)", Size: report.Overworld.Size, Found: true})
			if report.Nether.Exists {
				fmt.Printf("    %-25s  %s\n", report.Nether.Name+" (nether)", FormatSize(report.Nether.Size))
				rows = append(rows, WorldSummaryRow{Label: report.Nether.Name + " (nether)", Size: report.Nether.Size, Found: true})
			}
			if report.End.Exists {
				fmt.Printf("    %-25s  %s\n", report.End.Name+" (end)", FormatSize(report.End.Size))
				rows = append(rows, WorldSummaryRow{Label: report.End.Name + " (end)", Size: report.End.Size, Found: true})
			}
			grandTotal += report.Total
		}

	default: // plugin
		reports, total := AnalyzeWorlds(serverDir, worlds)
		for _, r := range reports {
			if !r.Exists {
				fmt.Printf("    %-25s  (not found)\n", r.Name)
				rows = append(rows, WorldSummaryRow{Label: r.Name, Found: false})
			} else {
				fmt.Printf("    %-25s  %s\n", r.Name, FormatSize(r.Size))
				rows = append(rows, WorldSummaryRow{Label: r.Name, Size: r.Size, Found: true})
			}
		}
		grandTotal = total
	}

	fmt.Printf("    %-25s  %s\n", "TOTAL", FormatSize(grandTotal))

	return grandTotal, rows
}

// WebOutputReport holds statistics for the web output directory.
type WebOutputReport struct {
	TotalSize   int64
	FileCount   int64
	MaxFileSize int64
}

// AnalyzeWebOutput reports statistics for the web output directory.
func AnalyzeWebOutput(serverDir string) (*WebOutputReport, error) {
	webDir := filepath.Join(serverDir, "web")
	info, err := os.Stat(webDir)
	if err != nil {
		return nil, fmt.Errorf("web directory not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("web path is not a directory")
	}

	var report WebOutputReport
	err = filepath.Walk(webDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			report.TotalSize += info.Size()
			report.FileCount++
			if info.Size() > report.MaxFileSize {
				report.MaxFileSize = info.Size()
			}
		}
		return nil
	})
	return &report, err
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

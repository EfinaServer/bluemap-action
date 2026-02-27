package bluemap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func GenerateMarkers(serverDir string) error {
	const markerFile = "generate_markers.py"

	markerPath := filepath.Join(serverDir, markerFile)
	if _, err := os.Stat(markerPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("  %s not found in %s; skipping\n", markerFile, serverDir)
			return nil
		}
		return fmt.Errorf("error checking for %s: %w", markerFile, err)
	}

	cmd := exec.Command("python3", markerFile)
	cmd.Dir = serverDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("  executing: python3 %s\n", markerFile)
	fmt.Printf("  working dir: %s\n", serverDir)
	fmt.Println()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", markerFile, err)
	}

	return nil
}

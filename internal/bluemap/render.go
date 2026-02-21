package bluemap

import (
	"fmt"
	"os"
	"os/exec"
)

// Render executes the BlueMap CLI jar in render mode.
// It runs: java -jar <jarPath> -v <mcVersion> -r
// The working directory is set to serverDir so BlueMap picks up the config/ directory.
// Stdout and stderr are streamed directly to the terminal so progress is visible.
func Render(jarPath, serverDir, mcVersion string) error {
	cmd := exec.Command("java", "-jar", jarPath, "-v", mcVersion, "-r")
	cmd.Dir = serverDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("  executing: java -jar %s -v %s -r\n", jarPath, mcVersion)
	fmt.Printf("  working directory: %s\n", serverDir)
	fmt.Println()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("BlueMap render failed: %w", err)
	}

	fmt.Println()
	fmt.Println("  BlueMap render completed successfully.")
	return nil
}

package bluemap

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Render executes the BlueMap CLI jar in render mode.
// It runs: java -jar <jarPath> -v <mcVersion> -r
// The working directory is set to serverDir so BlueMap picks up the config/ directory.
// Stdout and stderr are streamed directly to the terminal so progress is visible.
// It returns the wall-clock duration of the render process.
func Render(jarPath, serverDir, mcVersion string) (time.Duration, error) {
	cmd := exec.Command("java", "-jar", jarPath, "-v", mcVersion, "-r")
	cmd.Dir = serverDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("  executing: java -jar %s -v %s -r\n", jarPath, mcVersion)
	fmt.Printf("  working dir: %s\n", serverDir)
	fmt.Println()

	start := time.Now()
	if err := cmd.Run(); err != nil {
		return time.Since(start), fmt.Errorf("BlueMap render failed: %w", err)
	}
	elapsed := time.Since(start)

	fmt.Println()
	fmt.Printf("  ✔  BlueMap render completed in %s\n", elapsed.Round(time.Second))
	return elapsed, nil
}

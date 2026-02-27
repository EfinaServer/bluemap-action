package bluemap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// scriptsDir is the conventional subdirectory name scanned for custom scripts.
const scriptsDir = "scripts"

// interpreters maps file extensions to the interpreter used to execute them.
var interpreters = map[string]string{
	".py": "python3",
	".sh": "sh",
}

// RunScripts discovers and executes custom scripts from the scripts/
// subdirectory of serverDir. Scripts are executed in alphabetical order with
// the working directory set to serverDir. If no scripts/ directory exists, the
// step is silently skipped.
func RunScripts(serverDir string) error {
	dir := filepath.Join(serverDir, scriptsDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("  no %s/ directory found; skipping\n", scriptsDir)
			return nil
		}
		return fmt.Errorf("reading %s: %w", dir, err)
	}

	// Collect script files, skip directories.
	var scripts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if _, ok := interpreters[ext]; !ok {
			fmt.Fprintf(os.Stderr, "  ⚠  skipping %s (unsupported extension %q)\n", e.Name(), ext)
			continue
		}
		scripts = append(scripts, e.Name())
	}

	sort.Strings(scripts)

	if len(scripts) == 0 {
		fmt.Printf("  no scripts found in %s/\n", scriptsDir)
		return nil
	}

	for _, name := range scripts {
		ext := strings.ToLower(filepath.Ext(name))
		interpreter := interpreters[ext]
		scriptPath := filepath.Join(scriptsDir, name)

		fmt.Printf("  executing: %s %s\n", interpreter, scriptPath)
		fmt.Printf("  working dir: %s\n", serverDir)
		fmt.Println()

		cmd := exec.Command(interpreter, scriptPath)
		cmd.Dir = serverDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed: %w", scriptPath, err)
		}
	}

	return nil
}

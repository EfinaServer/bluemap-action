package assets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RewriteCompressedRefs finds web/assets/index-*.js in the given server
// directory and rewrites asset references to point to their gzip-compressed
// variants (.prbm → .prbm.gz, /textures.json → /textures.json.gz).
//
// This is necessary because Netlify does not support wildcard rewrites,
// so the JavaScript must reference the compressed files directly.
func RewriteCompressedRefs(serverDir string) error {
	pattern := filepath.Join(serverDir, "web", "assets", "index-*.js")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing %s: %w", pattern, err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no files matching %s", pattern)
	}

	for _, path := range matches {
		if err := rewriteFile(path); err != nil {
			return fmt.Errorf("rewriting %s: %w", path, err)
		}
	}

	return nil
}

func rewriteFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	content := string(data)
	original := content

	content = strings.ReplaceAll(content, ".prbm", ".prbm.gz")
	content = strings.ReplaceAll(content, "/textures.json", "/textures.json.gz")

	if content == original {
		fmt.Printf("    %s: no changes needed\n", filepath.Base(path))
		return nil
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	fmt.Printf("    %s: rewritten .prbm → .prbm.gz, /textures.json → /textures.json.gz\n", filepath.Base(path))
	return nil
}

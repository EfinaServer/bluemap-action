package lang

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed files/*.conf
var langFiles embed.FS

// DeployConfig holds the values to substitute into language file placeholders.
type DeployConfig struct {
	ToolVersion string
	MinecraftVersion string
	ProjectName string
	RenderTime  string
}

// Deploy copies all embedded language files into targetDir, replacing
// placeholders {toolVersion}, {minecraftVersion}, {projectName}, and
// {renderTime} with the corresponding values from cfg.
func Deploy(targetDir string, cfg DeployConfig) error {
	entries, err := fs.ReadDir(langFiles, "files")
	if err != nil {
		return fmt.Errorf("reading embedded lang files: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating lang directory %s: %w", targetDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := fs.ReadFile(langFiles, "files/"+entry.Name())
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", entry.Name(), err)
		}

		content := string(data)
		content = strings.ReplaceAll(content, "{toolVersion}", cfg.ToolVersion)
		content = strings.ReplaceAll(content, "{minecraftVersion}", cfg.MinecraftVersion)
		content = strings.ReplaceAll(content, "{projectName}", cfg.ProjectName)
		content = strings.ReplaceAll(content, "{renderTime}", cfg.RenderTime)

		targetPath := filepath.Join(targetDir, entry.Name())
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", targetPath, err)
		}
	}

	return nil
}

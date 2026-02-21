package netlify

import (
	"fmt"
	"os"
	"path/filepath"
)

const netlifyToml = `[[redirects]]
from = "/*"
to = "/index.html"
status = 200

[[headers]]
  for = "/*.json.gz"
  [headers.values]
    Content-Encoding = "gzip"

[[headers]]
  for = "/*.prbm.gz"
  [headers.values]
    Content-Encoding = "gzip"
`

// DeployConfig writes a netlify.toml into the web/ directory under serverDir.
func DeployConfig(serverDir string) error {
	webDir := filepath.Join(serverDir, "web")
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		return fmt.Errorf("creating web directory %s: %w", webDir, err)
	}

	targetPath := filepath.Join(webDir, "netlify.toml")
	if err := os.WriteFile(targetPath, []byte(netlifyToml), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", targetPath, err)
	}

	return nil
}

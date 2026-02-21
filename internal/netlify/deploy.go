package netlify

import (
	"fmt"
	"os"
	"path/filepath"
)

const netlifyToml = `# Rewrite .prbm requests to .prbm.gz files
[[redirects]]
from = "/*.prbm"
to = "/:splat.prbm.gz"
status = 200

# Rewrite textures.json requests to textures.json.gz
[[redirects]]
from = "/*/textures.json"
to = "/:splat/textures.json.gz"
status = 200

# SPA fallback (must be last — Netlify matches redirects in order)
[[redirects]]
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

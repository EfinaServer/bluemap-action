package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFileBytes creates a file at path (with parent dirs) containing n bytes.
func writeFileBytes(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestAnalyzeUnifiedWorld(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")

	// Vanilla dimensions under dimensions/minecraft/.
	writeFileBytes(t, filepath.Join(world, "dimensions", "minecraft", "overworld", "region", "r.0.0.mca"), 100)
	writeFileBytes(t, filepath.Join(world, "dimensions", "minecraft", "the_nether", "region", "r.0.0.mca"), 50)
	writeFileBytes(t, filepath.Join(world, "dimensions", "minecraft", "the_end", "region", "r.0.0.mca"), 25)
	// A datapack/mod custom dimension under a different namespace.
	writeFileBytes(t, filepath.Join(world, "dimensions", "mymod", "mydim", "region", "r.0.0.mca"), 10)
	// Non-dimension content that should be grouped into OtherSize.
	writeFileBytes(t, filepath.Join(world, "level.dat"), 7)
	writeFileBytes(t, filepath.Join(world, "players", "uuid.dat"), 3)

	report, err := AnalyzeUnifiedWorld(dir, "world")
	if err != nil {
		t.Fatalf("AnalyzeUnifiedWorld: %v", err)
	}

	wantDims := map[string]int64{
		"minecraft:overworld":  100,
		"minecraft:the_nether": 50,
		"minecraft:the_end":    25,
		"mymod:mydim":          10,
	}
	if len(report.Dimensions) != len(wantDims) {
		t.Fatalf("got %d dimensions, want %d: %+v", len(report.Dimensions), len(wantDims), report.Dimensions)
	}
	for _, d := range report.Dimensions {
		want, ok := wantDims[d.Key]
		if !ok {
			t.Errorf("unexpected dimension key %q", d.Key)
			continue
		}
		if d.Size != want {
			t.Errorf("dimension %q size = %d, want %d", d.Key, d.Size, want)
		}
	}

	if report.OtherSize != 10 {
		t.Errorf("OtherSize = %d, want 10 (level.dat + players)", report.OtherSize)
	}
	if report.Total != 195 {
		t.Errorf("Total = %d, want 195", report.Total)
	}
}

func TestAnalyzeUnifiedWorldMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := AnalyzeUnifiedWorld(dir, "world"); err == nil {
		t.Fatal("expected error for missing world directory, got nil")
	}
}

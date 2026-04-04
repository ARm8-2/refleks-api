package runsync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRefleksFile_SampleFiles(t *testing.T) {
	t.Parallel()

	samplesDir := filepath.Join("..", "..", "testdata", "refleks-samples")
	entries, err := os.ReadDir(samplesDir)
	if err != nil {
		samplesDir = filepath.Join("..", "..", "references", "runs")
		entries, err = os.ReadDir(samplesDir)
		if err != nil {
			t.Skip("no refleks sample fixtures found in testdata/refleks-samples or references/runs")
		}
	}

	found := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".refleks" {
			continue
		}
		found++

		path := filepath.Join(samplesDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read sample %s: %v", path, err)
		}

		parsed, err := ParseRefleksFile(raw)
		if err != nil {
			t.Fatalf("parse sample %s: %v", path, err)
		}
		if parsed.FileName == "" {
			t.Fatalf("sample %s has empty filename", path)
		}
	}

	if found == 0 {
		t.Fatalf("no .refleks samples found under %s", samplesDir)
	}
}

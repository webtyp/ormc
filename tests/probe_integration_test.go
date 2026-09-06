//go:build !wasm

package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"webtyp.com/ormc"
)

// TestProbeIntegration_RealGoRun is the ONE integration test (plan §Stage 4)
// that runs the real dependency probe (`go run`) against a local fixture
// kind package implementing model.Kind, instead of an injected test double.
func TestProbeIntegration_RealGoRun(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fixtureDir := filepath.Join(cwd, "fixtureuse")
	defPath := filepath.Join(fixtureDir, "definitions.go")
	outPath := filepath.Join(fixtureDir, "definitions_orm.go")
	defer os.Remove(outPath)

	g := ormc.New()
	g.SetRootDir(fixtureDir)

	if err := g.GenerateForStruct("Fixture", defPath); err != nil {
		t.Fatalf("GenerateForStruct with real probe failed: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	s := string(content)

	// fixturekind.Custom().Storage() returns model.FieldText, so the probe
	// must resolve the "note" field to a plain string.
	if !strings.Contains(s, "Note string") {
		t.Errorf("expected probed field resolved to string (FieldText), got:\n%s", s)
	}
}

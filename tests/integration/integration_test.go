//go:build integration
// +build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func Test_Generation(t *testing.T) {
	// Find project root
	cwd, err := os.Getwd()
	assert.NilError(t, err)
	
	// Assume we are in /opt/codes/workspace/protoc-gen-typescript-http/tests/integration
	root := filepath.Join(cwd, "..", "..")
	absRoot, err := filepath.Abs(root)
	assert.NilError(t, err)
	
	binDir := filepath.Join(absRoot, "bin")
	pluginPath := filepath.Join(binDir, "protoc-gen-typescript-http")
	
	// Ensure the plugin is built
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Fatal("plugin binary not found, please run 'mage build' first")
	}
	
	// Prepare environment
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)
	
	// Run buf generate in examples/proto
	cmd := exec.Command("buf", "generate")
	cmd.Dir = filepath.Join(absRoot, "examples", "proto")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	err = cmd.Run()
	assert.NilError(t, err, "buf generate failed")
	
	// Format the generated code with deno fmt
	cmd = exec.Command("deno", "fmt", "gen/typescript")
	cmd.Dir = filepath.Join(absRoot, "examples", "proto")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	err = cmd.Run()
	assert.NilError(t, err, "deno fmt failed")
	
	// Verify that the generated code is clean (no git diff)
	verifyClean(t, absRoot)
}

func verifyClean(t *testing.T, root string) {
	t.Helper()
	
	// Run git diff to ensure no changes in examples/proto/gen/typescript
	cmd := exec.Command("git", "diff", "--exit-code", "examples/proto/gen/typescript")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated code is out of sync with committed code:\n%s", string(out))
	}
}

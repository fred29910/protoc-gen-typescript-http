//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = Build

// Build builds the plugin binary to the bin directory.
func Build() error {
	fmt.Println("Building...")
	if err := os.MkdirAll("bin", 0755); err != nil {
		return err
	}
	return sh.Run("go", "build", "--trimpath", "-o", filepath.Join("bin", "protoc-gen-typescript-http"), ".")
}

// Test runs the unit tests (excludes integration tests via build tag).
func Test() error {
	fmt.Println("Running unit tests...")
	return sh.RunV("go", "test", "-v", "-count=1", "./...")
}

// Vet runs go vet on the project.
func Vet() error {
	fmt.Println("Vetting...")
	return sh.RunV("go", "vet", "./...")
}

// Fmt runs go fmt on the project and reports any formatting issues.
func Fmt() error {
	fmt.Println("Formatting...")
	return sh.RunV("go", "fmt", "./...")
}

// Integration runs the integration tests.
// Requires buf to be installed and available in PATH.
// Depends on: Build
func Integration() error {
	mg.Deps(Build)
	fmt.Println("Running integration tests...")

	// Verify buf is available before running tests
	if _, err := exec.LookPath("buf"); err != nil {
		return fmt.Errorf("buf not found in PATH — please install buf first (e.g. 'make install-buf' or 'go install github.com/bufbuild/buf/cmd/buf@latest')")
	}

	// Add current bin directory to PATH so the test can find the plugin binary
	binDir, err := filepath.Abs("bin")
	if err != nil {
		return err
	}

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	fmt.Println("Running integration test suite...")
	return sh.RunV("go", "test", "-v", "-count=1", "-tags=integration", "./tests/integration/...")
}

// Clean cleans the project.
func Clean() error {
	fmt.Println("Cleaning...")
	sh.Run("go", "clean")
	sh.Rm("bin")
	// Keep generated code for now, or use buf clean if available
	return nil
}

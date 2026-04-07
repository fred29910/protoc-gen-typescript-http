//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
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
	return sh.Run("go", "build", "-o", filepath.Join("bin", "protoc-gen-typescript-http"), ".")
}

// Test runs the unit tests.
func Test() error {
	fmt.Println("Running unit tests...")
	return sh.RunV("go", "test", "-v", "./...")
}

// Integration runs the integration tests.
// This requires the plugin to be built first.
func Integration() error {
	mg.Deps(Build)
	fmt.Println("Running integration tests...")
	
	// Add current bin to PATH
	binDir, err := filepath.Abs("bin")
	if err != nil {
		return err
	}
	
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	defer os.Setenv("PATH", oldPath)

	fmt.Println("Running integration test suite...")
	return sh.RunV("go", "test", "-v", "-tags=integration", "./tests/integration/...")
}

// Clean cleans the project.
func Clean() error {
	fmt.Println("Cleaning...")
	sh.Run("go", "clean")
	sh.Rm("bin")
	// Keep generated code for now, or use buf clean if available
	return nil
}

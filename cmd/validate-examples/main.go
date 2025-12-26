package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/kube-zen/zen-gc/pkg/api/v1alpha1"
	"github.com/kube-zen/zen-gc/pkg/validation"
)

func main() {
	examplesDir := flag.String("dir", "examples", "Directory containing example YAML files")
	flag.Parse()

	files, err := filepath.Glob(filepath.Join(*examplesDir, "*.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding files: %v\n", err)
		os.Exit(1)
	}

	var errors []string
	var warnings []string
	validCount := 0

	for _, file := range files {
		// Skip README if it's named as YAML
		if strings.Contains(file, "README") {
			continue
		}

		data, err := os.ReadFile(file)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: failed to read file: %v", file, err))
			continue
		}

		var policy v1alpha1.GarbageCollectionPolicy
		if err := yaml.Unmarshal(data, &policy); err != nil {
			errors = append(errors, fmt.Sprintf("%s: YAML parse error: %v", file, err))
			continue
		}

		// Validate using the validation package
		if err := validation.ValidatePolicy(&policy); err != nil {
			errors = append(errors, fmt.Sprintf("%s: validation error: %v", file, err))
			continue
		}

		fmt.Printf("✅ %s\n", filepath.Base(file))
		validCount++
	}

	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "\n❌ Validation errors:\n")
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "  %s\n", err)
		}
		os.Exit(1)
	}

	if len(warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings:\n")
		for _, warn := range warnings {
			fmt.Printf("  %s\n", warn)
		}
	}

	fmt.Printf("\n✅ All %d example files are valid!\n", validCount)
}

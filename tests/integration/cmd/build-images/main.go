//go:build integration
// +build integration

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultRegistry = "ghcr.io"
	defaultOwner    = "amartani"
	defaultImageTag = "latest"
)

var (
	registry  string
	imageBase string
	imageTag  string
)

func main() {
	// Get configuration from environment
	registry = getEnv("REGISTRY", defaultRegistry)
	owner := getEnv("GITHUB_REPOSITORY_OWNER", defaultOwner)
	imageBase = getEnv("TEST_IMAGE_BASE", fmt.Sprintf("%s/%s/oci-extract-test", registry, owner))
	imageTag = getEnv("TEST_IMAGE_TAG", defaultImageTag)

	fmt.Printf("Registry: %s\n", registry)
	fmt.Printf("Image base: %s\n", imageBase)
	fmt.Printf("Image tag: %s\n", imageTag)

	// Generate test data
	if err := generateTestData(); err != nil {
		fmt.Printf("Error generating test data: %v\n", err)
		os.Exit(1)
	}

	// Build and push test images
	if err := buildTestImages(); err != nil {
		fmt.Printf("Error building test images: %v\n", err)
		os.Exit(1)
	}

	// Convert to eStargz format
	if err := convertToEstargz(); err != nil {
		fmt.Printf("Error converting to eStargz: %v\n", err)
		os.Exit(1)
	}

	// Create SOCI indices
	if err := createSociIndices(); err != nil {
		fmt.Printf("Error creating SOCI indices: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ All test images built and pushed successfully!")
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// runCommand prints and executes a command
func runCommand(name string, args ...string) error {
	fmt.Printf("$ %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// generateTestData creates test files needed for the test images
func generateTestData() error {
	fmt.Println("=== Generating Test Data ===")

	// Generate large.bin (1MB)
	largeBinPath := "../../test-images/base/testdata/large.bin"
	if err := os.MkdirAll(filepath.Dir(largeBinPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create 1MB file with deterministic pattern
	data := bytes.Repeat([]byte("b"), 1024*1024)
	if err := os.WriteFile(largeBinPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write large.bin: %w", err)
	}

	fmt.Println("✓ Generated large.bin (1MB)")
	return nil
}

// buildTestImages builds and pushes standard test images
func buildTestImages() error {
	fmt.Println("\n=== Building Test Images ===")

	images := []struct {
		name    string
		context string
		tags    []string
	}{
		{
			name:    "base",
			context: "../../test-images/base",
			tags: []string{
				fmt.Sprintf("%s:standard", imageBase),
				fmt.Sprintf("%s:standard-%s", imageBase, imageTag),
			},
		},
		{
			name:    "multilayer",
			context: "../../test-images/multilayer",
			tags: []string{
				fmt.Sprintf("%s:multilayer-standard", imageBase),
				fmt.Sprintf("%s:multilayer-standard-%s", imageBase, imageTag),
			},
		},
	}

	for _, img := range images {
		fmt.Printf("\nBuilding %s image...\n", img.name)

		// Build the image
		args := []string{"build", "-t", img.tags[0]}
		for _, tag := range img.tags[1:] {
			args = append(args, "-t", tag)
		}
		args = append(args, img.context)

		cmd := exec.Command("docker", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build %s image: %w", img.name, err)
		}

		// Push all tags
		for _, tag := range img.tags {
			fmt.Printf("Pushing %s...\n", tag)
			cmd = exec.Command("docker", "push", tag)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to push %s: %w", tag, err)
			}
		}

		fmt.Printf("✓ Built and pushed %s image\n", img.name)
	}

	return nil
}

// convertToEstargz converts standard images to eStargz format using nerdctl
func convertToEstargz() error {
	fmt.Println("\n=== Converting to eStargz Format ===")

	// Resolve full path to nerdctl
	nerdctlPath, err := exec.LookPath("nerdctl")
	if err != nil {
		fmt.Println("⚠ nerdctl not found, skipping eStargz conversion")
		return nil
	}

	// Get absolute path
	nerdctlPath, err = filepath.Abs(nerdctlPath)
	if err != nil {
		nerdctlPath, _ = exec.LookPath("nerdctl") // fallback to original path
	}

	fmt.Printf("Using nerdctl: %s\n", nerdctlPath)

	images := []struct {
		source string
		target string
	}{
		{
			source: fmt.Sprintf("%s:standard", imageBase),
			target: fmt.Sprintf("%s:estargz", imageBase),
		},
		{
			source: fmt.Sprintf("%s:multilayer-standard", imageBase),
			target: fmt.Sprintf("%s:multilayer-estargz", imageBase),
		},
	}

	for _, img := range images {
		fmt.Printf("\nConverting %s to eStargz...\n", img.source)

		// Pull the source image
		if err := runCommand("sudo", nerdctlPath, "pull", img.source); err != nil {
			return fmt.Errorf("failed to pull %s: %w", img.source, err)
		}

		// Convert to eStargz
		if err := runCommand("sudo", nerdctlPath, "image", "convert",
			"--estargz",
			"--oci",
			img.source,
			img.target); err != nil {
			return fmt.Errorf("failed to convert %s to eStargz: %w", img.source, err)
		}

		// Push the eStargz image
		if err := runCommand("sudo", nerdctlPath, "push", img.target); err != nil {
			return fmt.Errorf("failed to push %s: %w", img.target, err)
		}

		// Also tag with image tag
		targetWithTag := fmt.Sprintf("%s-%s", img.target, imageTag)
		if err := runCommand("sudo", nerdctlPath, "tag", img.target, targetWithTag); err != nil {
			return fmt.Errorf("failed to tag %s: %w", img.target, err)
		}

		if err := runCommand("sudo", nerdctlPath, "push", targetWithTag); err != nil {
			return fmt.Errorf("failed to push %s: %w", targetWithTag, err)
		}

		fmt.Printf("✓ Converted and pushed %s\n", img.target)
	}

	return nil
}

// createSociIndices creates SOCI indices for standard images
func createSociIndices() error {
	fmt.Println("\n=== Creating SOCI Indices ===")

	// Resolve full path to soci
	sociPath, err := exec.LookPath("soci")
	if err != nil {
		fmt.Println("⚠ soci not found, skipping SOCI index creation")
		return nil
	}

	// Get absolute path
	sociPath, err = filepath.Abs(sociPath)
	if err != nil {
		sociPath, _ = exec.LookPath("soci") // fallback to original path
	}

	fmt.Printf("Using soci: %s\n", sociPath)

	images := []string{
		fmt.Sprintf("%s:standard", imageBase),
		fmt.Sprintf("%s:multilayer-standard", imageBase),
	}

	for _, img := range images {
		fmt.Printf("\nCreating SOCI index for %s...\n", img)

		// Create SOCI index
		if err := runCommand("sudo", sociPath, "create", "--min-layer-size", "0", img); err != nil {
			return fmt.Errorf("failed to create SOCI index for %s: %w", img, err)
		}

		// Push SOCI index
		if err := runCommand("sudo", sociPath, "push", img); err != nil {
			return fmt.Errorf("failed to push SOCI index for %s: %w", img, err)
		}

		fmt.Printf("✓ Created and pushed SOCI index for %s\n", img)
	}

	return nil
}

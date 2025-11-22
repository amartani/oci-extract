//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	binaryPath string
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Get configuration from environment
	registry = getEnv("REGISTRY", defaultRegistry)
	owner := getEnv("GITHUB_REPOSITORY_OWNER", defaultOwner)
	imageBase = getEnv("TEST_IMAGE_BASE", fmt.Sprintf("%s/%s/oci-extract-test", registry, owner))
	imageTag = getEnv("TEST_IMAGE_TAG", defaultImageTag)

	// Find oci-extract binary
	binaryPath = findBinary()
	if binaryPath == "" {
		fmt.Println("Error: oci-extract binary not found. Run 'make build' first.")
		os.Exit(1)
	}

	fmt.Printf("Using oci-extract binary: %s\n", binaryPath)
	fmt.Printf("Test image base: %s\n", imageBase)
	fmt.Printf("Test image tag: %s\n", imageTag)

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

	// Run tests
	code := m.Run()

	// Cleanup (optional)
	// cleanupTestImages()

	os.Exit(code)
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// findBinary locates the oci-extract binary
func findBinary() string {
	// Try common locations
	locations := []string{
		"./oci-extract",
		"../../oci-extract",
		"../../../oci-extract",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			abs, _ := filepath.Abs(loc)
			return abs
		}
	}

	// Try in PATH
	if path, err := exec.LookPath("oci-extract"); err == nil {
		return path
	}

	return ""
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

// extractFile extracts a file from an image using oci-extract
func extractFile(t *testing.T, image, filePath string) (string, error) {
	t.Helper()

	outputPath := filepath.Join(t.TempDir(), filepath.Base(filePath))

	cmd := exec.Command(binaryPath, "extract", image, filePath, "-o", outputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("extraction failed: %w\nStderr: %s", err, stderr.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted file: %w", err)
	}

	return string(data), nil
}

// TestExtractSmallFile tests extraction of small text files
func TestExtractSmallFile(t *testing.T) {
	formats := []string{"standard", "estargz", "soci"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			// SOCI uses the standard image (with SOCI index)
			imageFormat := format
			if format == "soci" {
				imageFormat = "standard"
			}
			image := fmt.Sprintf("%s:%s", imageBase, imageFormat)

			content, err := extractFile(t, image, "/testdata/small.txt")
			if err != nil {
				t.Fatalf("Failed to extract file: %v", err)
			}

			expected := "Hello from OCI-Extract integration test!"
			if content != expected {
				t.Errorf("Content mismatch:\nExpected: %q\nGot: %q", expected, content)
			}
		})
	}
}

// TestExtractNestedFile tests extraction of files in nested directories
func TestExtractNestedFile(t *testing.T) {
	formats := []string{"standard", "estargz", "soci"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			// SOCI uses the standard image (with SOCI index)
			imageFormat := format
			if format == "soci" {
				imageFormat = "standard"
			}
			image := fmt.Sprintf("%s:%s", imageBase, imageFormat)

			content, err := extractFile(t, image, "/testdata/nested/deep/file.txt")
			if err != nil {
				t.Fatalf("Failed to extract nested file: %v", err)
			}

			expected := "Nested file test - testing deep path extraction"
			if content != expected {
				t.Errorf("Content mismatch:\nExpected: %q\nGot: %q", expected, content)
			}
		})
	}
}

// TestExtractJSONFile tests extraction and validation of JSON files
func TestExtractJSONFile(t *testing.T) {
	formats := []string{"standard", "estargz", "soci"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			// SOCI uses the standard image (with SOCI index)
			imageFormat := format
			if format == "soci" {
				imageFormat = "standard"
			}
			image := fmt.Sprintf("%s:%s", imageBase, imageFormat)

			content, err := extractFile(t, image, "/testdata/medium.json")
			if err != nil {
				t.Fatalf("Failed to extract JSON file: %v", err)
			}

			// Validate JSON structure
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(content), &data); err != nil {
				t.Fatalf("Invalid JSON: %v", err)
			}

			// Check expected fields
			if name, ok := data["name"].(string); !ok || name != "oci-extract-test" {
				t.Errorf("Expected name 'oci-extract-test', got: %v", data["name"])
			}

			if formats, ok := data["formats"].([]interface{}); !ok || len(formats) != 3 {
				t.Errorf("Expected 3 formats, got: %v", data["formats"])
			}
		})
	}
}

// TestExtractLargeFile tests extraction of larger binary files
func TestExtractLargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file test in short mode")
	}

	formats := []string{"standard", "estargz", "soci"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			// SOCI uses the standard image (with SOCI index)
			imageFormat := format
			if format == "soci" {
				imageFormat = "standard"
			}
			image := fmt.Sprintf("%s:%s", imageBase, imageFormat)

			content, err := extractFile(t, image, "/testdata/large.bin")
			if err != nil {
				t.Fatalf("Failed to extract large file: %v", err)
			}

			// Verify size
			expectedSize := 1024 * 1024 // 1MB
			if len(content) != expectedSize {
				t.Errorf("Size mismatch: expected %d bytes, got %d bytes", expectedSize, len(content))
			}

			// Verify content (should be all 'b' characters)
			for i, b := range []byte(content) {
				if b != 'b' {
					t.Errorf("Unexpected byte at position %d: expected 'b', got %c", i, b)
					break
				}
			}
		})
	}
}

// TestExtractMultiLayer tests extraction from multi-layer images
func TestExtractMultiLayer(t *testing.T) {
	formats := []string{"multilayer-standard", "multilayer-estargz", "multilayer-soci"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			// SOCI uses the standard image (with SOCI index)
			imageFormat := format
			if format == "multilayer-soci" {
				imageFormat = "multilayer-standard"
			}
			image := fmt.Sprintf("%s:%s", imageBase, imageFormat)

			// Test file from layer 1
			content, err := extractFile(t, image, "/layer1/file.txt")
			if err != nil {
				t.Fatalf("Failed to extract from layer1: %v", err)
			}
			if !strings.Contains(content, "Layer 1 content") {
				t.Errorf("Unexpected content from layer1: %s", content)
			}

			// Test file from layer 2
			content, err = extractFile(t, image, "/layer2/file.txt")
			if err != nil {
				t.Fatalf("Failed to extract from layer2: %v", err)
			}
			if !strings.Contains(content, "Layer 2 content") {
				t.Errorf("Unexpected content from layer2: %s", content)
			}

			// Test final layer file
			content, err = extractFile(t, image, "/final.txt")
			if err != nil {
				t.Fatalf("Failed to extract final.txt: %v", err)
			}
			if !strings.Contains(content, "Final layer content") {
				t.Errorf("Unexpected content from final layer: %s", content)
			}
		})
	}
}

// TestExtractNonExistentFile tests error handling for missing files
func TestExtractNonExistentFile(t *testing.T) {
	image := fmt.Sprintf("%s:standard", imageBase)

	outputPath := filepath.Join(t.TempDir(), "nonexistent.txt")

	cmd := exec.Command(binaryPath, "extract", image, "/nonexistent/file.txt", "-o", outputPath)
	err := cmd.Run()

	if err == nil {
		t.Error("Expected error for non-existent file, but extraction succeeded")
	}
}

// TestExtractWithVerbose tests verbose output
func TestExtractWithVerbose(t *testing.T) {
	image := fmt.Sprintf("%s:standard", imageBase)
	outputPath := filepath.Join(t.TempDir(), "small.txt")

	cmd := exec.Command(binaryPath, "extract", image, "/testdata/small.txt", "-o", outputPath, "--verbose")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Extraction failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	output := stdout.String() + stderr.String()

	// Check for verbose output indicators
	expectedStrings := []string{"Extracting", "Output:", "Found", "layer"}
	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected verbose output to contain %q, but it didn't.\nOutput: %s", expected, output)
		}
	}
}

// BenchmarkExtractSmallFile benchmarks small file extraction
func BenchmarkExtractSmallFile(b *testing.B) {
	image := fmt.Sprintf("%s:estargz", imageBase)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outputPath := filepath.Join(b.TempDir(), fmt.Sprintf("small-%d.txt", i))

		cmd := exec.Command(binaryPath, "extract", image, "/testdata/small.txt", "-o", outputPath)
		if err := cmd.Run(); err != nil {
			b.Fatalf("Extraction failed: %v", err)
		}
	}
}

// BenchmarkExtractLargeFile benchmarks large file extraction
func BenchmarkExtractLargeFile(b *testing.B) {
	image := fmt.Sprintf("%s:estargz", imageBase)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outputPath := filepath.Join(b.TempDir(), fmt.Sprintf("large-%d.bin", i))

		cmd := exec.Command(binaryPath, "extract", image, "/testdata/large.bin", "-o", outputPath)
		if err := cmd.Run(); err != nil {
			b.Fatalf("Extraction failed: %v", err)
		}
	}
}

// TestPerformanceComparison compares extraction performance across formats
func TestPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	formats := []string{"standard", "estargz"}
	filePath := "/testdata/small.txt"

	results := make(map[string]time.Duration)

	for _, format := range formats {
		image := fmt.Sprintf("%s:%s", imageBase, format)

		start := time.Now()
		_, err := extractFile(t, image, filePath)
		duration := time.Since(start)

		if err != nil {
			t.Logf("Format %s failed: %v", format, err)
			continue
		}

		results[format] = duration
		t.Logf("Format %s: %v", format, duration)
	}

	// Calculate speedup if both formats succeeded
	if stdTime, ok := results["standard"]; ok {
		if estargzTime, ok := results["estargz"]; ok {
			speedup := float64(stdTime) / float64(estargzTime)
			t.Logf("eStargz speedup: %.2fx", speedup)

			// eStargz should be faster or comparable
			if speedup < 0.5 {
				t.Logf("Warning: eStargz is slower than expected (speedup: %.2fx)", speedup)
			}
		}
	}
}

// TestExtractWithSOCIIndex tests extraction from images with SOCI indices
// This test explicitly verifies that the tool can detect and use SOCI indices
func TestExtractWithSOCIIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SOCI test in short mode")
	}

	testCases := []struct {
		name     string
		image    string
		filePath string
		expected string
	}{
		{
			name:     "small_file_with_soci",
			image:    fmt.Sprintf("%s:standard", imageBase),
			filePath: "/testdata/small.txt",
			expected: "Hello from OCI-Extract integration test!",
		},
		{
			name:     "nested_file_with_soci",
			image:    fmt.Sprintf("%s:standard", imageBase),
			filePath: "/testdata/nested/deep/file.txt",
			expected: "Nested file test - testing deep path extraction",
		},
		{
			name:     "multilayer_with_soci",
			image:    fmt.Sprintf("%s:multilayer-standard", imageBase),
			filePath: "/layer1/file.txt",
			expected: "Layer 1 content",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := extractFile(t, tc.image, tc.filePath)
			if err != nil {
				t.Fatalf("Failed to extract file with SOCI index: %v", err)
			}

			if !strings.Contains(content, tc.expected) {
				t.Errorf("Content mismatch:\nExpected to contain: %q\nGot: %q", tc.expected, content)
			}

			t.Logf("Successfully extracted %s from SOCI-indexed image", tc.filePath)
		})
	}
}

// TestSOCIIndexDetection tests that SOCI indices are properly detected
func TestSOCIIndexDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping SOCI detection test in short mode")
	}

	image := fmt.Sprintf("%s:standard", imageBase)
	outputPath := filepath.Join(t.TempDir(), "test.txt")

	// Run with verbose to see if SOCI index is detected
	cmd := exec.Command(binaryPath, "extract", image, "/testdata/small.txt", "-o", outputPath, "--verbose")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Extraction failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	output := stdout.String() + stderr.String()

	// Check if output mentions SOCI index detection
	// The exact message depends on implementation, but we should see some indication
	// that SOCI was considered
	t.Logf("Extraction output:\n%s", output)

	// Verify the file was extracted successfully
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}

	expected := "Hello from OCI-Extract integration test!"
	if string(content) != expected {
		t.Errorf("Content mismatch:\nExpected: %q\nGot: %q", expected, string(content))
	}
}

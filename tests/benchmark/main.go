package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultRegistry = "ghcr.io"
	defaultOwner    = "amartani"
	defaultImageTag = "latest"
)

var (
	runs     int
	registry string
	owner    string
	imageTag string
	verbose  bool
)

type benchmarkResult struct {
	method   string
	format   string
	file     string
	duration time.Duration
	err      error
}

func main() {
	flag.IntVar(&runs, "runs", 1, "Number of times to run each benchmark")
	flag.StringVar(&registry, "registry", defaultRegistry, "Container registry")
	flag.StringVar(&owner, "owner", defaultOwner, "Repository owner")
	flag.StringVar(&imageTag, "tag", defaultImageTag, "Image tag")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.Parse()

	imageBase := fmt.Sprintf("%s/%s/oci-extract-test", registry, owner)

	// Find oci-extract binary
	binaryPath := findBinary()
	if binaryPath == "" {
		fmt.Fprintln(os.Stderr, "Error: oci-extract binary not found. Run 'mise run build' first.")
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Using oci-extract binary: %s\n", binaryPath)
		fmt.Printf("Test image base: %s\n", imageBase)
		fmt.Printf("Test image tag: %s\n", imageTag)
		fmt.Printf("Runs per test: %d\n\n", runs)
	}

	// Define test cases
	testCases := []struct {
		method   string
		format   string
		imageTag string
		file     string
		desc     string
	}{
		// Small file tests
		{
			method:   "docker",
			format:   "standard",
			imageTag: "standard",
			file:     "/testdata/small.txt",
			desc:     "Small file via docker pull + cp",
		},
		{
			method:   "oci-extract",
			format:   "standard",
			imageTag: "standard",
			file:     "/testdata/small.txt",
			desc:     "Small file (standard format)",
		},
		{
			method:   "oci-extract",
			format:   "estargz",
			imageTag: "estargz",
			file:     "/testdata/small.txt",
			desc:     "Small file (eStargz format)",
		},
		{
			method:   "oci-extract",
			format:   "soci",
			imageTag: "standard",
			file:     "/testdata/small.txt",
			desc:     "Small file (SOCI format)",
		},
		// Large file tests
		{
			method:   "docker",
			format:   "standard",
			imageTag: "standard",
			file:     "/testdata/large.bin",
			desc:     "Large file via docker pull + cp",
		},
		{
			method:   "oci-extract",
			format:   "standard",
			imageTag: "standard",
			file:     "/testdata/large.bin",
			desc:     "Large file (standard format)",
		},
		{
			method:   "oci-extract",
			format:   "estargz",
			imageTag: "estargz",
			file:     "/testdata/large.bin",
			desc:     "Large file (eStargz format)",
		},
		{
			method:   "oci-extract",
			format:   "soci",
			imageTag: "standard",
			file:     "/testdata/large.bin",
			desc:     "Large file (SOCI format)",
		},
	}

	fmt.Println("Running Extraction Performance Benchmark")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Check if docker is available
	dockerAvailable := checkDocker()
	if !dockerAvailable {
		fmt.Println("Warning: docker not found, skipping docker pull benchmarks")
		fmt.Println()
	}

	var results []benchmarkResult

	for _, tc := range testCases {
		if tc.method == "docker" && !dockerAvailable {
			continue
		}

		image := fmt.Sprintf("%s:%s", imageBase, tc.imageTag)

		if verbose {
			fmt.Printf("Running: %s\n", tc.desc)
			fmt.Printf("  Image: %s\n", image)
			fmt.Printf("  File: %s\n", tc.file)
		} else {
			fmt.Printf("%-50s ", tc.desc+"...")
		}

		var totalDuration time.Duration
		var lastErr error

		for i := 0; i < runs; i++ {
			if verbose && runs > 1 {
				fmt.Printf("  Run %d/%d...\n", i+1, runs)
			}

			var duration time.Duration
			var err error

			if tc.method == "docker" {
				duration, err = benchmarkDocker(image, tc.file)
			} else {
				duration, err = benchmarkOCIExtract(binaryPath, image, tc.file)
			}

			if err != nil {
				lastErr = err
				if verbose {
					fmt.Printf("  Error: %v\n", err)
				}
				break
			}

			totalDuration += duration

			if verbose {
				fmt.Printf("  Time: %v\n", duration)
			}
		}

		avgDuration := totalDuration
		if runs > 1 && lastErr == nil {
			avgDuration = totalDuration / time.Duration(runs)
		}

		results = append(results, benchmarkResult{
			method:   tc.method,
			format:   tc.format,
			file:     tc.file,
			duration: avgDuration,
			err:      lastErr,
		})

		if !verbose {
			if lastErr != nil {
				fmt.Printf("FAILED: %v\n", lastErr)
			} else {
				fmt.Printf("%.3fs\n", avgDuration.Seconds())
			}
		} else {
			fmt.Println()
		}
	}

	// Print summary
	fmt.Println()
	printSummary(results, runs)
}

func findBinary() string {
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

	if path, err := exec.LookPath("oci-extract"); err == nil {
		return path
	}

	return ""
}

func checkDocker() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func benchmarkOCIExtract(binaryPath, image, filePath string) (time.Duration, error) {
	tmpDir, err := os.MkdirTemp("", "oci-extract-bench-*")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	outputPath := filepath.Join(tmpDir, filepath.Base(filePath))

	start := time.Now()
	cmd := exec.Command(binaryPath, "extract", image, filePath, "-o", outputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("extraction failed: %w\nStderr: %s", err, stderr.String())
	}
	duration := time.Since(start)

	// Verify file exists
	if _, err := os.Stat(outputPath); err != nil {
		return duration, fmt.Errorf("output file not found: %w", err)
	}

	return duration, nil
}

func benchmarkDocker(image, filePath string) (time.Duration, error) {
	tmpDir, err := os.MkdirTemp("", "docker-bench-*")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Remove image if it exists (to avoid using cached layers)
	_ = exec.Command("docker", "rmi", "-f", image).Run()

	containerName := fmt.Sprintf("oci-extract-bench-%d", time.Now().UnixNano())

	// Cleanup image after test
	defer func() {
		_ = exec.Command("docker", "rmi", "-f", image).Run()
	}()

	start := time.Now()

	// Pull the image
	pullCmd := exec.Command("docker", "pull", image)
	var pullStderr bytes.Buffer
	pullCmd.Stderr = &pullStderr
	if err := pullCmd.Run(); err != nil {
		return 0, fmt.Errorf("docker pull failed: %w\nStderr: %s", err, pullStderr.String())
	}

	// Create container
	createCmd := exec.Command("docker", "create", "--name", containerName, image)
	var createStderr bytes.Buffer
	createCmd.Stderr = &createStderr
	if err := createCmd.Run(); err != nil {
		return 0, fmt.Errorf("docker create failed: %w\nStderr: %s", err, createStderr.String())
	}
	defer func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	}()

	// Copy file
	outputPath := filepath.Join(tmpDir, filepath.Base(filePath))
	cpCmd := exec.Command("docker", "cp", containerName+":"+filePath, outputPath)
	var cpStderr bytes.Buffer
	cpCmd.Stderr = &cpStderr
	if err := cpCmd.Run(); err != nil {
		return 0, fmt.Errorf("docker cp failed: %w\nStderr: %s", err, cpStderr.String())
	}

	duration := time.Since(start)

	// Verify file exists
	if _, err := os.Stat(outputPath); err != nil {
		return duration, fmt.Errorf("output file not found: %w", err)
	}

	return duration, nil
}

func printSummary(results []benchmarkResult, runs int) {
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("BENCHMARK SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	if runs > 1 {
		fmt.Printf("All times are averaged over %d runs\n\n", runs)
	}

	// Group by file
	fileGroups := make(map[string][]benchmarkResult)
	for _, result := range results {
		fileGroups[result.file] = append(fileGroups[result.file], result)
	}

	// Print results for each file
	files := []string{"/testdata/small.txt", "/testdata/large.bin"}
	for _, file := range files {
		group, ok := fileGroups[file]
		if !ok || len(group) == 0 {
			continue
		}

		fileDesc := "Small File"
		if file == "/testdata/large.bin" {
			fileDesc = "Large File (1MB)"
		}

		fmt.Printf("%s Extraction (%s)\n", fileDesc, file)
		fmt.Println(strings.Repeat("-", 80))

		// Print header
		fmt.Printf("%-20s %-15s %-15s\n", "Method", "Format", "Time")
		fmt.Println(strings.Repeat("-", 80))

		// Find docker baseline time
		var dockerTime time.Duration
		dockerOk := false
		for _, r := range group {
			if r.method == "docker" && r.err == nil {
				dockerTime = r.duration
				dockerOk = true
				break
			}
		}

		// Print results
		for _, r := range group {
			method := r.method
			if method == "oci-extract" {
				method = "oci-extract"
			} else {
				method = "docker pull+cp"
			}

			timeStr := "FAILED"
			speedup := ""
			if r.err == nil {
				timeStr = fmt.Sprintf("%.3fs", r.duration.Seconds())
				if dockerOk && r.method != "docker" && dockerTime > 0 {
					ratio := float64(dockerTime) / float64(r.duration)
					speedup = fmt.Sprintf(" (%.2fx faster)", ratio)
				}
			}

			fmt.Printf("%-20s %-15s %-15s%s\n", method, r.format, timeStr, speedup)
		}

		fmt.Println()
	}

	// Print detailed comparison
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("PERFORMANCE COMPARISON")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	for _, file := range files {
		group, ok := fileGroups[file]
		if !ok || len(group) == 0 {
			continue
		}

		fileDesc := "Small File"
		if file == "/testdata/large.bin" {
			fileDesc = "Large File (1MB)"
		}

		// Collect times
		var dockerTime, standardTime, estargzTime, sociTime time.Duration
		dockerOk, standardOk, estargzOk, sociOk := false, false, false, false

		for _, r := range group {
			if r.err != nil {
				continue
			}
			if r.method == "docker" {
				dockerTime = r.duration
				dockerOk = true
			} else if r.format == "standard" {
				standardTime = r.duration
				standardOk = true
			} else if r.format == "estargz" {
				estargzTime = r.duration
				estargzOk = true
			} else if r.format == "soci" {
				sociTime = r.duration
				sociOk = true
			}
		}

		fmt.Printf("%s:\n", fileDesc)

		if dockerOk {
			fmt.Printf("  docker pull+cp:       %.3fs (baseline)\n", dockerTime.Seconds())
		}
		if standardOk {
			fmt.Printf("  oci-extract standard: %.3fs", standardTime.Seconds())
			if dockerOk {
				fmt.Printf(" (%.2fx faster than docker)", float64(dockerTime)/float64(standardTime))
			}
			fmt.Println()
		}
		if estargzOk {
			fmt.Printf("  oci-extract estargz:  %.3fs", estargzTime.Seconds())
			if dockerOk {
				fmt.Printf(" (%.2fx faster than docker)", float64(dockerTime)/float64(estargzTime))
			}
			if standardOk {
				fmt.Printf(", %.2fx faster than standard", float64(standardTime)/float64(estargzTime))
			}
			fmt.Println()
		}
		if sociOk {
			fmt.Printf("  oci-extract soci:     %.3fs", sociTime.Seconds())
			if dockerOk {
				fmt.Printf(" (%.2fx faster than docker)", float64(dockerTime)/float64(sociTime))
			}
			if standardOk {
				fmt.Printf(", %.2fx faster than standard", float64(standardTime)/float64(sociTime))
			}
			fmt.Println()
		}

		fmt.Println()
	}
}

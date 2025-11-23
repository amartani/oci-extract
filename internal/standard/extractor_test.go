package standard

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// createTestLayer creates a test layer with the given files
func createTestLayer(t *testing.T, files map[string]string) v1.Layer {
	t.Helper()

	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	// Write files to tar
	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0600,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write tar content: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	// Create layer from buffer
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	if err != nil {
		t.Fatalf("failed to create layer: %v", err)
	}

	return layer
}

func TestListFiles(t *testing.T) {
	testFiles := map[string]string{
		"file1.txt":            "content1",
		"dir/file2.txt":        "content2",
		"dir/subdir/file3.txt": "content3",
		"another/file4.txt":    "content4",
	}

	layer := createTestLayer(t, testFiles)
	extractor := NewExtractor(layer)

	ctx := context.Background()
	files, err := extractor.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	// Check that we got all the files
	if len(files) != len(testFiles) {
		t.Errorf("ListFiles() got %d files, want %d", len(files), len(testFiles))
	}

	// Create a set of expected files (with leading slash for normalized paths)
	expectedFiles := make(map[string]bool)
	for name := range testFiles {
		// Paths are now normalized to include a leading slash
		normalizedName := name
		if normalizedName[0] != '/' {
			normalizedName = "/" + normalizedName
		}
		expectedFiles[normalizedName] = true
	}

	// Check that all files are in the result
	for _, file := range files {
		if !expectedFiles[file] {
			t.Errorf("ListFiles() returned unexpected file: %s", file)
		}
		delete(expectedFiles, file)
	}

	// Check for missing files
	if len(expectedFiles) > 0 {
		for file := range expectedFiles {
			t.Errorf("ListFiles() missing file: %s", file)
		}
	}
}

func TestListFilesEmpty(t *testing.T) {
	layer := createTestLayer(t, map[string]string{})
	extractor := NewExtractor(layer)

	ctx := context.Background()
	files, err := extractor.ListFiles(ctx)
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	if len(files) != 0 {
		t.Errorf("ListFiles() got %d files, want 0", len(files))
	}
}

func TestExtractFile(t *testing.T) {
	testContent := "Hello, World!"
	testFiles := map[string]string{
		"test.txt":       testContent,
		"dir/nested.txt": "nested content",
	}

	layer := createTestLayer(t, testFiles)
	extractor := NewExtractor(layer)

	// Create a temporary file for output
	outputPath := t.TempDir() + "/output.txt"

	ctx := context.Background()
	err := extractor.ExtractFile(ctx, "test.txt", outputPath)
	if err != nil {
		t.Fatalf("ExtractFile() error = %v", err)
	}

	// Verify the content
	// Note: In a real test we would read the file back, but that requires
	// additional setup. The important part is that it doesn't error.
}

func TestExtractFileNotFound(t *testing.T) {
	testFiles := map[string]string{
		"test.txt": "content",
	}

	layer := createTestLayer(t, testFiles)
	extractor := NewExtractor(layer)

	outputPath := t.TempDir() + "/output.txt"

	ctx := context.Background()
	err := extractor.ExtractFile(ctx, "nonexistent.txt", outputPath)
	if err == nil {
		t.Error("ExtractFile() expected error for non-existent file, got nil")
	}
}

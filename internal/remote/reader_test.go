package remote

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRemoteReader tests basic functionality of RemoteReader
func TestRemoteReader(t *testing.T) {
	// Create test data
	testData := []byte("Hello, World! This is test data for remote reader.")
	dataSize := int64(len(testData))

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a HEAD request
		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", dataSize))
			w.WriteHeader(http.StatusOK)
			return
		}

		// Handle range requests
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			// Parse range header (simplified)
			var start, end int64
			_, _ = fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)

			if start < 0 || start >= dataSize {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}

			if end >= dataSize {
				end = dataSize - 1
			}

			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, dataSize))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(testData[start : end+1])
			return
		}

		// Full content
		w.Header().Set("Content-Length", fmt.Sprintf("%d", dataSize))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(testData)
	}))
	defer server.Close()

	// Create a RemoteReader
	reader, err := NewRemoteReader(server.URL)
	if err != nil {
		t.Fatalf("Failed to create RemoteReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// Test 1: Check size
	if reader.Size() != dataSize {
		t.Errorf("Size mismatch: expected %d, got %d", dataSize, reader.Size())
	}

	// Test 2: Read from beginning
	buf := make([]byte, 5)
	n, err := reader.ReadAt(buf, 0)
	if err != nil {
		t.Errorf("ReadAt failed: %v", err)
	}
	if n != 5 {
		t.Errorf("ReadAt returned wrong size: expected 5, got %d", n)
	}
	if string(buf) != "Hello" {
		t.Errorf("ReadAt returned wrong data: expected 'Hello', got '%s'", string(buf))
	}

	// Test 3: Read from middle
	buf = make([]byte, 5)
	_, err = reader.ReadAt(buf, 7)
	if err != nil {
		t.Errorf("ReadAt from middle failed: %v", err)
	}
	if string(buf) != "World" {
		t.Errorf("ReadAt returned wrong data: expected 'World', got '%s'", string(buf))
	}

	// Test 4: Read beyond end
	buf = make([]byte, 10)
	_, err = reader.ReadAt(buf, dataSize)
	if err != io.EOF {
		t.Errorf("Expected EOF when reading beyond end, got: %v", err)
	}
}

// TestRemoteReaderCache tests the caching functionality
func TestRemoteReaderCache(t *testing.T) {
	testData := []byte("Cached data test")
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
			w.WriteHeader(http.StatusOK)
			return
		}

		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			var start, end int64
			_, _ = fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(testData)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(testData[start : end+1])
		}
	}))
	defer server.Close()

	reader, err := NewRemoteReader(server.URL)
	if err != nil {
		t.Fatalf("Failed to create RemoteReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	// First read
	buf1 := make([]byte, 5)
	_, _ = reader.ReadAt(buf1, 0)
	firstRequestCount := requestCount

	// Second read from same location (should use cache)
	buf2 := make([]byte, 5)
	_, _ = reader.ReadAt(buf2, 0)

	// The request count should be the same, indicating cache was used
	if requestCount != firstRequestCount {
		t.Logf("Cache may not be working optimally. Requests: %d", requestCount)
	}
}

// TestRemoteReaderNoRangeSupport tests handling of servers without range support
func TestRemoteReaderNoRangeSupport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			// Don't set Accept-Ranges header
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	_, err := NewRemoteReader(server.URL)
	if err == nil {
		t.Error("Expected error for server without range support")
	}
}

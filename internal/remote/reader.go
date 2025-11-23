package remote

import (
	"fmt"
	"io"
	"net/http"
	"sync"
)

// RemoteReader implements io.ReaderAt for remote HTTP resources using Range requests
type RemoteReader struct {
	URL    string
	Client *http.Client
	size   int64

	// Simple cache for small reads
	cacheMu    sync.RWMutex
	cacheStart int64
	cacheEnd   int64 // Tracks the end of valid cached data
	cacheData  []byte
	cacheSize  int
	cacheValid bool // Tracks if cache contains valid data
}

// NewRemoteReader creates a new RemoteReader for the given URL
func NewRemoteReader(url string) (*RemoteReader, error) {
	client := &http.Client{}

	// Get the content length
	resp, err := client.Head(url)
	if err != nil {
		return nil, fmt.Errorf("failed to HEAD %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HEAD request failed with status: %d", resp.StatusCode)
	}

	// Check if server supports range requests
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return nil, fmt.Errorf("server does not support range requests")
	}

	return &RemoteReader{
		URL:       url,
		Client:    client,
		size:      resp.ContentLength,
		cacheSize: 1024 * 1024, // 1MB cache
		cacheData: make([]byte, 1024*1024),
	}, nil
}

// ReadAt implements io.ReaderAt
func (r *RemoteReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, fmt.Errorf("negative offset")
	}

	if off >= r.size {
		return 0, io.EOF
	}

	// Check cache first
	r.cacheMu.RLock()
	if r.cacheValid && off >= r.cacheStart && off+int64(len(p)) <= r.cacheEnd {
		cacheOffset := off - r.cacheStart
		n = copy(p, r.cacheData[cacheOffset:cacheOffset+int64(len(p))])
		r.cacheMu.RUnlock()
		return n, nil
	}
	r.cacheMu.RUnlock()

	// Prepare range request
	end := off + int64(len(p)) - 1
	if end >= r.size {
		end = r.size - 1
	}

	req, err := http.NewRequest("GET", r.URL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, end))

	resp, err := r.Client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute range request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("range request failed with status: %d", resp.StatusCode)
	}

	// Read response body
	n, err = io.ReadFull(resp.Body, p)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return n, fmt.Errorf("failed to read response: %w", err)
	}

	// Update cache if this was a small read
	if n > 0 && n <= r.cacheSize {
		r.cacheMu.Lock()
		r.cacheStart = off
		r.cacheEnd = off + int64(n)
		copy(r.cacheData, p[:n])
		r.cacheValid = true
		r.cacheMu.Unlock()
	}

	return n, nil
}

// Size returns the total size of the remote resource
func (r *RemoteReader) Size() int64 {
	return r.size
}

// Close cleans up resources (currently no-op, but included for future extensibility)
func (r *RemoteReader) Close() error {
	return nil
}

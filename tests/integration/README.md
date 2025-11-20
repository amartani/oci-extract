# Integration Tests

Integration tests for OCI-Extract that validate extraction functionality against real OCI images in all supported formats.

## Overview

These tests validate:
- Format detection (Standard, eStargz, SOCI)
- File extraction from all formats
- Multi-layer image handling
- Error cases (file not found, invalid images)
- Performance characteristics

## Running Tests

### Prerequisites

1. **Test Images**: Images must be available in GitHub Container Registry
   ```bash
   # Images are built automatically by CI, or build manually:
   cd ../../test-images
   docker build -t ghcr.io/YOUR_USERNAME/oci-extract-test:standard ./base
   docker push ghcr.io/YOUR_USERNAME/oci-extract-test:standard
   ```

2. **Authentication**: Login to GHCR if using private images
   ```bash
   echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin
   ```

3. **OCI-Extract Binary**: Build the binary
   ```bash
   cd ../..
   make build
   ```

### Run All Tests

```bash
# From repository root
./tests/integration/run-integration-tests.sh
```

### Run Specific Tests

```bash
# Run only Go integration tests
go test -v -tags=integration ./tests/integration/...

# Run specific test
go test -v -tags=integration ./tests/integration/... -run TestExtractSmallFile

# Run with custom image
TEST_IMAGE_BASE=ghcr.io/myuser/oci-extract-test go test -v -tags=integration ./tests/integration/...
```

## Test Structure

```
tests/integration/
├── README.md                 # This file
├── run-integration-tests.sh  # Test runner script
├── main_test.go             # Test setup and teardown
├── fixtures.go              # Expected test data
├── helpers.go               # Test utilities
├── standard_test.go         # Standard format tests
├── estargz_test.go          # eStargz format tests
├── soci_test.go             # SOCI format tests
└── performance_test.go      # Performance benchmarks
```

## Test Cases

### Format Detection Tests
- Correctly identifies Standard OCI layers
- Correctly identifies eStargz layers (magic footer detection)
- Correctly identifies SOCI-indexed layers (index discovery)

### Extraction Tests

#### Small Files (< 1KB)
- Extract `/testdata/small.txt` from all formats
- Verify content matches expected
- Validate performance (eStargz/SOCI should use minimal bandwidth)

#### Medium Files (~10KB)
- Extract `/testdata/medium.json` from all formats
- Parse and validate JSON structure
- Verify all nested fields

#### Large Files (1MB+)
- Extract `/testdata/large.bin` from all formats
- Verify checksum matches expected
- Measure download efficiency

#### Nested Paths
- Extract `/testdata/nested/deep/file.txt`
- Verify path handling and normalization

### Multi-Layer Tests
- Extract files from different layers
- Verify correct layer selection (last layer wins)
- Test file overwrites

### Error Handling Tests
- File not found in image
- Invalid image reference
- Network errors
- Authentication failures
- Corrupted images

### Performance Tests
- Benchmark extraction time for each format
- Measure bytes downloaded vs file size
- Compare speedup: eStargz vs Standard, SOCI vs Standard

## Environment Variables

- `TEST_IMAGE_BASE`: Base image name (default: `ghcr.io/$USER/oci-extract-test`)
- `TEST_IMAGE_TAG`: Specific tag to test (default: `latest`)
- `TEST_TIMEOUT`: Test timeout duration (default: `30m`)
- `TEST_VERBOSE`: Enable verbose output (default: `false`)

## CI/CD Integration

Tests run automatically via GitHub Actions:
- On every push
- On pull requests
- Daily scheduled runs
- Manual workflow dispatch

See `.github/workflows/integration-tests.yml` for details.

## Adding New Tests

1. Add test data to `test-images/base/testdata/`
2. Update expected values in `fixtures.go`
3. Write test function in appropriate `*_test.go` file
4. Add to test runner script if needed
5. Update this README

Example:
```go
func TestExtractNewFile(t *testing.T) {
    formats := []string{"standard", "estargz", "soci"}

    for _, format := range formats {
        t.Run(format, func(t *testing.T) {
            image := fmt.Sprintf("%s:%s", testImageBase, format)
            output := extractFile(t, image, "/testdata/newfile.txt")
            assert.Equal(t, expectedContent, output)
        })
    }
}
```

## Troubleshooting

### Tests Fail with "image not found"
- Ensure test images are built and pushed to registry
- Check authentication with `docker login ghcr.io`
- Verify image names match expected format

### Format Detection Fails
- Check that eStargz images have proper magic footer
- Verify SOCI indices were created and pushed
- Use `--verbose` flag for debugging

### Extraction Fails
- Verify file exists in test image: `docker run IMAGE ls -la /testdata`
- Check registry connectivity
- Review logs with `-v` flag

### Performance Tests Don't Show Speedup
- Ensure eStargz/SOCI images are properly created
- Check network conditions (tests measure actual download)
- Verify HTTP Range support on registry

## Performance Expectations

Based on architecture and real-world testing:

| Format | Small File (< 1KB) | Large File (1MB) | Speedup |
|--------|-------------------|------------------|---------|
| Standard | ~2s (full layer) | ~2-5s | 1x baseline |
| eStargz | ~0.3s (~150KB) | ~1.1s (~1.2MB) | 3-4x |
| SOCI | ~0.4s (~200KB) | ~1.2s (~1.3MB) | 2-3x |

*Actual results depend on network speed, registry performance, and layer size.*

## References

- [Integration Test Plan](../../INTEGRATION_TEST_PLAN.md)
- [Test Images](../../test-images/README.md)
- [OCI Image Spec](https://github.com/opencontainers/image-spec)
- [eStargz Spec](https://github.com/containerd/stargz-snapshotter)
- [SOCI Spec](https://github.com/awslabs/soci-snapshotter)

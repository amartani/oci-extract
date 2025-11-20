# Integration Tests

Integration tests for OCI-Extract that validate extraction functionality against real OCI images in all supported formats.

## Overview

These tests validate:
- Format detection (Standard, eStargz, SOCI)
- File extraction from all formats
- Multi-layer image handling
- Error cases (file not found, invalid images)
- Performance characteristics

## Architecture

The integration tests are written in Go and automatically:
1. Generate test data (including large binary files)
2. Build Docker test images
3. Convert images to eStargz format (using nerdctl)
4. Create SOCI indices (using soci CLI)
5. Push all variants to GitHub Container Registry
6. Run extraction tests against all formats

This approach keeps the test logic in Go and makes the tests easier to maintain.

## Running Tests

### Unit Tests vs Integration Tests

**Unit Tests** (fast, no external dependencies):
```bash
# Run all unit tests
go test ./...

# Run unit tests with coverage
go test -v -race -coverprofile=coverage.out ./...
```

**Integration Tests** (slower, requires Docker and registry access):
```bash
# Run integration tests
go test -v -tags=integration ./tests/integration/...

# Run with timeout for long-running tests
go test -v -tags=integration -timeout=30m ./tests/integration/...

# Run specific integration test
go test -v -tags=integration ./tests/integration/... -run TestExtractSmallFile

# Skip long-running tests
go test -v -tags=integration -short ./tests/integration/...
```

### Prerequisites

1. **Docker**: Required for building test images
   ```bash
   docker --version
   ```

2. **OCI-Extract Binary**: Build the binary first
   ```bash
   cd ../..
   make build
   ```

3. **Authentication**: Login to GHCR
   ```bash
   echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin
   ```

4. **Optional Tools** (for full format support):
   - **nerdctl**: For eStargz conversion (tests will skip if not available)
   - **soci**: For SOCI index creation (tests will skip if not available)

### Installing Optional Tools

**nerdctl** (for eStargz):
```bash
NERDCTL_VERSION=1.7.6
curl -LO "https://github.com/containerd/nerdctl/releases/download/v${NERDCTL_VERSION}/nerdctl-${NERDCTL_VERSION}-linux-amd64.tar.gz"
sudo tar Cxzvf /usr/local/bin nerdctl-${NERDCTL_VERSION}-linux-amd64.tar.gz
sudo nerdctl --version

# Note: nerdctl will use Docker's containerd, no separate installation needed
```

**soci** (for SOCI indices):
```bash
SOCI_VERSION=0.11.1
curl -LO "https://github.com/awslabs/soci-snapshotter/releases/download/v${SOCI_VERSION}/soci-snapshotter-${SOCI_VERSION}-linux-amd64.tar.gz"
tar -xzf soci-snapshotter-${SOCI_VERSION}-linux-amd64.tar.gz
sudo install soci /usr/local/bin/
soci --version

# Create content store directory
sudo mkdir -p /var/lib/soci-snapshotter-grpc
sudo chown -R $USER:$USER /var/lib/soci-snapshotter-grpc
```

## Test Structure

```
tests/integration/
├── README.md                  # This file
├── integration_test.go        # All integration tests (Go)
└── run-integration-tests.sh   # Legacy shell script (deprecated)
```

All integration tests are now in `integration_test.go` which includes:
- `TestMain`: Setup (generates test data, builds images, converts formats)
- `TestExtractSmallFile`: Tests extraction of small text files
- `TestExtractNestedFile`: Tests nested directory paths
- `TestExtractJSONFile`: Tests JSON extraction and validation
- `TestExtractLargeFile`: Tests large binary file extraction
- `TestExtractMultiLayer`: Tests multi-layer image handling
- `TestExtractNonExistentFile`: Tests error handling
- `TestExtractWithVerbose`: Tests verbose output
- `TestPerformanceComparison`: Compares performance across formats
- Benchmark tests for performance measurement

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

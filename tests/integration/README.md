# Integration Tests

Integration tests for OCI-Extract that validate extraction functionality against real OCI images in all supported formats.

## Overview

These tests validate:
- Format detection (Standard, eStargz, SOCI, zstd, zstd:chunked)
- File extraction from all formats
- Multi-layer image handling
- Error cases (file not found, invalid images)
- Performance characteristics

## Architecture

The integration tests use **prebuilt test images** hosted in GitHub Container Registry. This design allows developers to run integration tests locally without requiring Docker, nerdctl, or soci tools.

### Two-Part Design

**1. Image Builder** (`cmd/build-images/main.go` - CI only):
- Generates test data (including large binary files)
- Builds Docker test images
- Converts images to eStargz format (using nerdctl)
- Creates SOCI indices (using soci CLI)
- Converts images to zstd format (using nerdctl)
- Converts images to zstd:chunked format (using nerdctl)
- Pushes all variants to GitHub Container Registry
- **Runs only in CI** (requires write permissions to ghcr.io)

**2. Integration Tests** (`integration_test.go` - Local & CI):
- Uses prebuilt images from the registry
- Tests extraction against all formats
- **Can run locally** without special tools or permissions

This separation makes local development easier while ensuring CI validates the complete workflow.

## Running Tests

### Unit Tests vs Integration Tests

**Unit Tests** (fast, no external dependencies):
```bash
# Run all unit tests
go test ./...

# Run unit tests with coverage
go test -v -race -coverprofile=coverage.out ./...
```

**Integration Tests** (use prebuilt images from registry):
```bash
# Run integration tests (uses prebuilt images)
mise run integration-test

# Or directly with go test
go test -v -tags=integration -timeout=30m ./tests/integration/...

# Run specific integration test
go test -v -tags=integration ./tests/integration/... -run TestExtractSmallFile

# Skip long-running tests
go test -v -tags=integration -short ./tests/integration/...
```

**Note:** Local runs use prebuilt images from `ghcr.io/amartani/oci-extract-test`. No Docker, nerdctl, or soci tools required!

### Prerequisites

#### For Running Tests Locally

1. **OCI-Extract Binary**: Build the binary first
   ```bash
   mise run build
   ```

2. **That's it!** Tests use prebuilt images from the registry. No Docker, nerdctl, or soci tools needed.

#### For Building Test Images (CI Only)

Building test images requires:
1. **Docker**: For building base images
2. **nerdctl**: For eStargz conversion (installed by mise)
3. **soci**: For SOCI index creation (installed by mise)
4. **Registry Write Access**: Credentials to push to ghcr.io

**Note:** Most developers don't need to build images. The CI automatically builds and pushes images on every commit. Use the prebuilt images for local testing.

If you need to build images for development:
```bash
# Install tools (mise handles this automatically)
mise install

# Build and push images (requires ghcr.io write access)
export GITHUB_TOKEN=your_token
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin
mise run integration-test-build-images
```

## Test Structure

```
tests/integration/
├── README.md              # This file
├── integration_test.go    # Integration tests (uses prebuilt images)
└── cmd/
    └── build-images/
        └── main.go        # Image builder (CI only)
```

**integration_test.go** includes:
- `TestMain`: Setup (finds binary, configures registry)
- `TestExtractSmallFile`: Tests extraction of small text files
- `TestExtractNestedFile`: Tests nested directory paths
- `TestExtractJSONFile`: Tests JSON extraction and validation
- `TestExtractLargeFile`: Tests large binary file extraction
- `TestExtractMultiLayer`: Tests multi-layer image handling
- `TestExtractNonExistentFile`: Tests error handling
- `TestExtractWithVerbose`: Tests verbose output
- `TestPerformanceComparison`: Compares performance across formats
- Benchmark tests for performance measurement

**cmd/build-images/main.go** (CI only):
- Generates test data
- Builds and pushes Docker images
- Converts to eStargz format
- Creates SOCI indices

## Test Cases

### Format Detection Tests
- Correctly identifies Standard OCI layers
- Correctly identifies eStargz layers (magic footer detection)
- Correctly identifies SOCI-indexed layers (index discovery)
- Correctly identifies zstd compressed layers
- Correctly identifies zstd:chunked layers

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

The tests use the following environment variables:

- `REGISTRY`: Container registry (default: `ghcr.io`)
- `GITHUB_REPOSITORY_OWNER`: GitHub username/org (default: current user)
- `TEST_IMAGE_BASE`: Full image base name (default: `ghcr.io/{owner}/oci-extract-test`)
- `TEST_IMAGE_TAG`: Image tag to use (default: `latest`)

Example:
```bash
export TEST_IMAGE_BASE=ghcr.io/myuser/oci-extract-test
export TEST_IMAGE_TAG=v1.0.0
go test -v -tags=integration ./tests/integration/...
```

## CI/CD Integration

Tests run automatically via GitHub Actions (`.github/workflows/ci.yml`):
- On every push to any branch
- On all pull requests
- Integration tests must pass before releases are created

The CI workflow uses a two-step process:
1. **Build Images** (`mise run integration-test-build-images`):
   - Installs Docker, nerdctl, and soci
   - Generates test data
   - Builds test images in all formats
   - Pushes to ghcr.io with commit SHA tag

2. **Run Tests** (`mise run integration-test`):
   - Uses images built in step 1
   - Runs all integration tests
   - Gates version generation and releases on test success

This design ensures CI validates the complete workflow while allowing developers to run tests locally using prebuilt images.

## Adding New Tests

To add a new test case:

1. **Add test data** to `test-images/base/testdata/` or `test-images/multilayer/`
2. **Rebuild the Dockerfile** if needed to include new files
3. **Add test function** to `integration_test.go`:

```go
func TestExtractNewFile(t *testing.T) {
    formats := []string{"standard", "estargz"}

    for _, format := range formats {
        t.Run(format, func(t *testing.T) {
            image := fmt.Sprintf("%s:%s", imageBase, format)

            content, err := extractFile(t, image, "/testdata/newfile.txt")
            if err != nil {
                t.Fatalf("Failed to extract file: %v", err)
            }

            expected := "expected content"
            if content != expected {
                t.Errorf("Content mismatch:\nExpected: %q\nGot: %q", expected, content)
            }
        })
    }
}
```

4. **Update this README** with new test case documentation

## Troubleshooting

### Tests Fail with "image not found"
- **Local runs**: Prebuilt images should be available at `ghcr.io/amartani/oci-extract-test`
- **CI runs**: Ensure the "Build and push test images" step completed successfully
- Check registry connectivity: `docker pull ghcr.io/amartani/oci-extract-test:standard`
- Verify image names match expected format

### Tests Fail with "oci-extract binary not found"
- Build the binary first: `mise run build`
- The binary must be at the project root: `./oci-extract`

### Format Detection Fails
- Check that eStargz images have proper magic footer
- Verify SOCI indices were created and pushed (during image build)
- Use `--verbose` flag for debugging

### Extraction Fails
- Verify file exists in test image: `./oci-extract list ghcr.io/amartani/oci-extract-test:standard`
- Check registry connectivity
- Review logs with `-v` flag

### Performance Tests Don't Show Speedup
- Ensure eStargz/SOCI images are properly created (this happens in CI)
- Check network conditions (tests measure actual download)
- Verify HTTP Range support on registry

### Need to Rebuild Images
- Only required if modifying test image structure or test data
- Requires ghcr.io write permissions (maintainers only or CI)
- Run: `mise run integration-test-build-images`

## Performance Expectations

Based on architecture and real-world testing:

| Format | Small File (< 1KB) | Large File (1MB) | Speedup |
|--------|-------------------|------------------|---------|
| Standard | ~2s (full layer) | ~2-5s | 1x baseline |
| eStargz | ~0.3s (~150KB) | ~1.1s (~1.2MB) | 3-4x |
| SOCI | ~0.4s (~200KB) | ~1.2s (~1.3MB) | 2-3x |
| zstd | ~2s (full layer) | ~2-5s | 1x (better compression) |
| zstd:chunked | ~0.3s (~150KB) | ~1.1s (~1.2MB) | 3-4x |

*Actual results depend on network speed, registry performance, and layer size.*
*Note: zstd provides better compression than gzip but requires streaming full layer like standard format.*
*zstd:chunked combines zstd compression with chunked access similar to eStargz.*

## References

- [Test Images Documentation](../../test-images/README.md)
- [Main README](../../README.md)
- [OCI Image Spec](https://github.com/opencontainers/image-spec)
- [eStargz Spec](https://github.com/containerd/stargz-snapshotter)
- [SOCI Spec](https://github.com/awslabs/soci-snapshotter)

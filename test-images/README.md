# OCI-Extract Test Images

This directory contains test images used for integration testing of OCI-Extract.

## Directory Structure

```
test-images/
├── base/              # Simple single-layer test image
│   ├── Dockerfile
│   └── testdata/      # Test files of various sizes
├── multilayer/        # Multi-layer test image
│   └── Dockerfile
└── README.md
```

## Test Images

### Base Image
- **Purpose**: Test basic extraction functionality
- **Contents**:
  - `small.txt` - Small text file (< 1KB)
  - `medium.json` - JSON configuration (~10KB)
  - `large.bin` - Binary file (1MB)
  - `nested/deep/file.txt` - Nested file for path testing
  - `build-info.txt` - Generated at build time

### Multi-Layer Image
- **Purpose**: Test layer traversal and file overwriting
- **Layers**: 4 distinct layers
- **Contents**: Files spread across multiple layers

## Building Test Images Locally

### Prerequisites
- Docker or Podman
- Access to GitHub Container Registry (for pushing)

### Build Commands

```bash
# Build base image
docker build -t ghcr.io/YOUR_USERNAME/oci-extract-test:standard ./base

# Build multi-layer image
docker build -t ghcr.io/YOUR_USERNAME/oci-extract-test:multilayer-standard ./multilayer

# Push to registry
docker push ghcr.io/YOUR_USERNAME/oci-extract-test:standard
docker push ghcr.io/YOUR_USERNAME/oci-extract-test:multilayer-standard
```

### Converting to eStargz

Using nerdctl:
```bash
nerdctl image convert \
  --estargz \
  --oci \
  ghcr.io/YOUR_USERNAME/oci-extract-test:standard \
  ghcr.io/YOUR_USERNAME/oci-extract-test:estargz

nerdctl push ghcr.io/YOUR_USERNAME/oci-extract-test:estargz
```

### Creating SOCI Indices

Using SOCI CLI:
```bash
# Create and push SOCI index
soci create ghcr.io/YOUR_USERNAME/oci-extract-test:standard
soci push ghcr.io/YOUR_USERNAME/oci-extract-test:standard

# This creates SOCI-indexed version
```

## Testing Locally

```bash
# Build oci-extract
cd ../..
make build

# Test extraction
./oci-extract extract ghcr.io/YOUR_USERNAME/oci-extract-test:standard \
  /testdata/small.txt \
  -o /tmp/small.txt \
  --verbose

# Verify content
cat /tmp/small.txt
```

## Automated Testing

These images are automatically built and pushed by the GitHub Actions workflow:
- `.github/workflows/integration-tests.yml`

The workflow:
1. Builds standard images
2. Converts to eStargz format
3. Creates SOCI indices
4. Pushes all variants to GHCR
5. Runs integration tests

## Image Tags

Each test image is tagged with:
- `standard` - Latest standard OCI image
- `estargz` - Latest eStargz-converted image
- `soci` - Latest SOCI-indexed image (same as standard, but with index)
- `multilayer-standard` - Multi-layer standard image
- `multilayer-estargz` - Multi-layer eStargz image
- `multilayer-soci` - Multi-layer SOCI-indexed image
- `{format}-{git-sha}` - Specific commit version

## Maintenance

- Test images are rebuilt on every push to ensure freshness
- Old SHA-tagged images are cleaned up after 30 days
- Base images are updated monthly to track Alpine security updates

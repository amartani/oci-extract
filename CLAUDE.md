# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

This project uses [mise](https://mise.jdx.dev/) for tool management and task running.

```bash
# Install all development tools (Go, golangci-lint, nerdctl, soci)
mise install

# Build the binary
mise run build

# Run unit tests
mise run test

# Run unit tests with coverage
mise run test-coverage

# Run integration tests (uses prebuilt images from ghcr.io)
mise run integration-test

# Build and push test images (CI only - requires ghcr.io write access)
mise run integration-test-build-images

# Run linter
mise run lint

# Format code
mise run fmt

# Check for dead code
mise run deadcode

# Clean build artifacts
mise run clean

# See all available tasks
mise tasks
```

### Running Single Tests

```bash
# Run a specific test function
go test -v ./internal/estargz -run TestExtractFile

# Run tests in a specific package
go test -v ./internal/soci/...

# Run with race detector
go test -race ./...
```

## Architecture Overview

oci-extract extracts files from OCI/Docker images **without mounting them or downloading entire images**. The core innovation is using **HTTP Range requests** to fetch only the necessary bytes.

### The "No-Mount" Strategy

```
Traditional:        docker pull → mount → copy file → 500MB downloaded
oci-extract:        fetch metadata → range request → ~50KB downloaded
```

The tool processes image layers from **top to bottom** (respecting OCI overlay semantics) and tries increasingly efficient extraction strategies with graceful fallback:

1. **eStargz** (fastest): Read TOC → Range request specific chunks
2. **SOCI** (fast): Read zTOC → Range request compressed ranges
3. **zstd:chunked** (fast): Read TOC → Range request zstd-compressed chunks
4. **zstd** (fallback): Stream entire zstd-compressed layer
5. **Standard** (fallback): Stream entire gzip-compressed layer

### High-Level Flow

```
CLI Command (cmd/)
    ↓
Orchestrator (extractor/orchestrator.go)
    ├─ Registry Client → Fetch manifest, construct blob URLs
    ├─ SOCI Discovery → Find SOCI indices (optional)
    └─ For each layer (bottom-up):
        ├─ Format Detection → Minimal detection, mostly try-and-fallback
        ├─ Try eStargz extraction
        ├─ Try SOCI extraction (if index exists)
        ├─ Try zstd:chunked extraction
        ├─ Try zstd extraction
        └─ Fallback to standard extraction
            ↓
Remote Reader (remote/reader.go)
    └─ HTTP Range requests to blob URLs
```

## Key Code Architecture

### Critical Components

#### 1. **Orchestrator** (`internal/extractor/orchestrator.go`)
The brain of the operation. Coordinates format detection, layer iteration, and extractor selection.

**Important patterns:**
- Processes layers in **reverse order** (index len-1 to 0) to respect overlay semantics
- Uses **optimistic trying**: attempts efficient formats first, degrades gracefully
- Returns on **first successful extraction** (top layer wins)

#### 2. **Remote Reader** (`internal/remote/reader.go`)
Implements `io.ReaderAt` for HTTP Range requests. This is the **secret sauce** that enables efficient extraction.

**Key insight:** The reader bypasses go-containerregistry's streaming `io.ReadCloser` API to enable random access:

```go
// ❌ What we DON'T do:
layer.Compressed() // Must stream from start, no seeking

// ✅ What we DO:
RemoteReader(blobURL).ReadAt(offset, length) // Random access!
```

Includes a simple 1MB cache to reduce redundant requests for metadata reads.

#### 3. **Registry Client** (`internal/registry/client.go`)
Handles OCI registry operations and constructs direct blob URLs.

**Critical function:** `GetLayerURL()` constructs the blob URL that RemoteReader uses:
```
https://{registry}/v2/{repository}/blobs/{digest}
```

**Important:** Handles Docker Hub's registry domain mapping (`index.docker.io` → `registry-1.docker.io`).

#### 4. **EnhancedLayerInfo** (`internal/registry/client.go`)
Bundles layer metadata with its direct blob URL. This structure is the handoff between registry operations and extraction.

```go
type EnhancedLayerInfo struct {
    Layer     v1.Layer  // go-containerregistry layer object
    Digest    v1.Hash
    Size      int64
    MediaType string
    BlobURL   string    // Critical: enables RemoteReader
}
```

#### 5. **Format Extractors**
Five packages implement the same conceptual interface (not formally defined):

- `internal/estargz/extractor.go`: eStargz format (TOC-based, gzip)
- `internal/soci/extractor.go`: SOCI format (zTOC-based)
- `internal/zstd/chunked_extractor.go`: zstd:chunked format (TOC-based, zstd)
- `internal/zstd/extractor.go`: Standard tar+zstd layers
- `internal/standard/extractor.go`: Standard tar+gzip layers

Each provides:
```go
ExtractFile(ctx, targetPath, outputPath) error
ListFiles(ctx) ([]string, error)
```

**Design decision:** No explicit Go interface. This is intentional pragmatism - each extractor has different constructor needs and format-specific optimizations.

#### 6. **SOCI Discovery** (`internal/soci/discovery.go`)
Finds SOCI indices via two methods:

1. **OCI 1.1 Referrers API** (modern, standard)
2. **Tag-based naming** (fallback: `sha256-{digest}.soci`)

Supporting both maximizes registry compatibility.

### Data Flow: Extract Command

```
1. cmd/extract.go:runExtract()
   └─ Parse args, create Orchestrator

2. orchestrator.Extract()
   ├─ client.GetEnhancedLayers(imageRef)
   │  ├─ Authenticate via Docker keychain
   │  ├─ Fetch manifest
   │  └─ Construct blob URLs for each layer
   │
   ├─ soci.DiscoverSOCIIndex() [if applicable]
   │
   └─ For each layer (reverse order):
      ├─ Try eStargz:
      │  └─ RemoteReader → Read footer → Read TOC → Range request chunks
      │
      ├─ Try SOCI:
      │  └─ RemoteReader → Read zTOC → Range request compressed ranges
      │
      ├─ Try zstd:chunked:
      │  └─ RemoteReader → Try TOC → Range request zstd chunks (fallback to stream)
      │
      ├─ Try zstd:
      │  └─ Stream layer → Decompress zstd → Iterate tar → Extract file
      │
      └─ Fallback Standard:
         └─ Stream layer → Decompress gzip → Iterate tar → Extract file
```

### Data Flow: List Command

Same as Extract, but:
- Calls `ListFiles()` instead of `ExtractFile()`
- Accumulates files across all layers
- De-duplicates (upper layers override lower)
- No early exit (must check all layers)

## Important Design Decisions

### 1. Separation of Metadata and Blob Access
The orchestrator uses `v1.Layer` for metadata but accesses actual data via `RemoteReader(blobURL)`. This bypasses the streaming-only `v1.Layer` interface to enable random access.

### 2. Format Detection is Minimal
`detector/format.go` does basic detection, but the real strategy is **try-and-fallback**. Failed format attempts are cheap (just TOC/zTOC header checks), so optimistic trying is efficient.

### 3. Bottom-Up Layer Processing
```go
for i := len(layers) - 1; i >= 0; i-- {
    // Process from top to bottom
}
```
This respects OCI overlay semantics: upper layers override lower layers.

### 4. Authentication Piggybacks on Initial Fetch
- Initial manifest/layer fetch authenticates via Docker keychain
- OAuth/Bearer tokens cached in HTTP client
- Subsequent Range requests automatically include auth headers
- No re-authentication needed for blob URLs

### 5. No Explicit Extractor Interface
Extractors follow a common pattern but don't implement a formal Go interface. This allows format-specific optimizations and different constructor signatures while keeping the code pragmatic.

### 6. Simple 1MB Cache in RemoteReader
Instead of complex LRU caching, RemoteReader uses a single 1MB buffer cache. This is sufficient for typical TOC/zTOC reads and keeps the implementation simple.

## Working with Extractors

When adding support for a new format:

1. Create package under `internal/newformat/`
2. Implement `ExtractFile(ctx, targetPath, outputPath)` and `ListFiles(ctx)`
3. Add detection logic in `internal/detector/format.go`
4. Wire into orchestrator in `internal/extractor/orchestrator.go:extractFromLayer()`
5. Add to the try-and-fallback chain with appropriate priority

**Testing strategy:**
- Unit tests: Mock layers, test extraction logic
- Integration tests: Use real prebuilt images from ghcr.io (see `tests/integration/`)
- Image building: CI builds test images in all formats (standard, eStargz, SOCI) using nerdctl and soci
- Local testing: No special tools required - tests use prebuilt images from the registry

## Important External Dependencies

- **google/go-containerregistry**: OCI registry client, manifest/layer operations
- **containerd/stargz-snapshotter**: eStargz format support (TOC parsing)
- **awslabs/soci-snapshotter**: SOCI format support (zTOC parsing)
- **spf13/cobra**: CLI framework

The tool is a thin orchestration layer that combines these libraries with custom HTTP Range request handling.

## Common Gotchas

### Docker Hub Registry Domain
Docker Hub uses different domains for API and blob storage:
- API/Auth: `index.docker.io`
- Blob storage: `registry-1.docker.io`

The registry client handles this mapping in `GetLayerURL()`.

### Layer Order Matters
Always process layers from **high index to low index** (reverse order of the slice). This is the opposite of what might seem intuitive but matches overlay filesystem semantics.

### BlobURL is Critical
The `EnhancedLayerInfo.BlobURL` must be correct for RemoteReader to work. If you see "404 Not Found" errors, check the blob URL construction logic.

### SOCI Indices Are Optional
The tool works without SOCI indices (falls back to eStargz or standard). Don't treat missing SOCI indices as errors unless the user explicitly requested `--format soci`.

### Range Request Requirements
Some registries might not support HTTP Range requests (rare but possible). The standard extractor is the fallback that works everywhere because it streams the entire layer.

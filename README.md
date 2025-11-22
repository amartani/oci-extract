# OCI-Extract

A CLI tool for extracting specific files from OCI/Docker images without mounting them or requiring root privileges.

## Features

- **No Root Required**: Extract files without needing privileged access or container runtime
- **Efficient**: Uses HTTP Range requests to fetch only necessary bytes
- **Format Support**: Automatically detects and handles multiple image formats:
  - Standard OCI/Docker layers
  - eStargz (seekable tar.gz with Table of Contents)
  - SOCI (Seekable OCI with zTOC indices)
- **Remote-First**: Works directly with remote registries without pulling entire images
- **File Listing**: List all files in an image without downloading it

## Installation

### Download Binary

Download the latest release for your platform from the [GitHub releases page](https://github.com/amartani/oci-extract/releases).

### Using mise

```bash
mise use --global github:amartani/oci-extract
```

### Using Go Install

```bash
go install github.com/amartani/oci-extract@latest
```

## Usage

### Basic Extraction

Extract a single file from an image:

```bash
oci-extract extract alpine:latest /bin/sh -o ./sh
```

### Extract Configuration Files

```bash
oci-extract extract nginx:latest /etc/nginx/nginx.conf -o ./nginx.conf
```

### Verbose Output

See detailed information about the extraction process:

```bash
oci-extract extract ubuntu:latest /etc/passwd -o ./passwd --verbose
```

### Force Specific Format

If you know the image format, you can skip auto-detection:

```bash
oci-extract extract myimage:latest /app/config.json --format estargz -o ./config.json
```

### Extract from Private Registries

The tool uses Docker's credential helper by default:

```bash
# Authenticate with your registry first
docker login registry.example.com

# Then extract
oci-extract extract registry.example.com/myapp:v1.0 /app/binary -o ./binary
```

### List Files in an Image

List all files in an image without downloading it:

```bash
# List all files
oci-extract list alpine:latest

# List with verbose output
oci-extract list nginx:latest --verbose

# Force a specific format
oci-extract list myimage:latest --format estargz
```

## How It Works

### Architecture

```
┌─────────────┐
│   CLI Tool  │
└──────┬──────┘
       │
       ├──────────────┐
       │              │
┌──────▼──────┐  ┌───▼────────┐
│  Registry   │  │  Format    │
│   Client    │  │  Detector  │
└──────┬──────┘  └───┬────────┘
       │             │
       │      ┌──────▼──────┐
       │      │ Orchestrator│
       │      └──────┬──────┘
       │             │
       │     ┌───────┼───────┐
       │     │       │       │
    ┌──▼─────▼─┐  ┌─▼────┐  ┌▼─────┐
    │ eStargz  │  │ SOCI │  │ Std  │
    │Extractor │  │ Extr.│  │ Extr.│
    └──────────┘  └──────┘  └──────┘
```

### The "No-Mount" Approach

Instead of mounting the image, oci-extract:

1. **Authenticates** with the OCI registry
2. **Fetches Manifest** to discover available layers
3. **Detects Format** (eStargz, SOCI, or Standard)
4. **Fetches Metadata** (TOC/zTOC) using small HTTP Range requests
5. **Locates File** in the metadata to find exact byte offsets
6. **Surgical Download** of only the required compressed chunks
7. **Decompresses** and writes the file to disk

### Format-Specific Behavior

#### eStargz

- Reads the footer (last 47 bytes) to locate the TOC
- Fetches the TOC to get file offsets
- Downloads only the specific chunk containing the file
- Decompresses on-the-fly

#### SOCI

- Queries the Referrers API or tag-based index
- Downloads the zTOC (compression info) for relevant layers
- Maps file paths to compressed byte ranges
- Fetches and decompresses specific ranges

#### Standard Layers

- Falls back to streaming decompression (less efficient)
- Still avoids pulling the entire image into local storage

## Performance Comparison

### File Extraction

For extracting a 10KB file from a 500MB image:

| Method | Downloaded | Time |
|--------|------------|------|
| docker pull + cp | 500 MB | ~2 min |
| oci-extract (eStargz) | ~50 KB | ~2 sec |
| oci-extract (SOCI) | ~100 KB | ~3 sec |
| oci-extract (Standard) | ~20 MB* | ~15 sec |

*Standard format requires downloading the entire layer containing the file

### File Listing

For listing all files in a typical image:

| Format | Downloaded | Time |
|--------|------------|------|
| eStargz | ~50-100 KB (TOC) | ~2-3 sec |
| SOCI | ~100-200 KB (zTOC + index) | ~3-4 sec |
| Standard | Full layer size | ~10-30 sec |

## Limitations

- Standard (non-seekable) layers require downloading entire layer containing the target file
- SOCI support requires the image to have SOCI indices generated beforehand
- Some registries may not support HTTP Range requests (though most do)
- Large files in highly compressed layers may still require significant downloads

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed development setup, workflow, and guidelines.

## License

[MIT License](LICENSE)

## Acknowledgments

Built on top of:
- [google/go-containerregistry](https://github.com/google/go-containerregistry) - OCI registry client
- [containerd/stargz-snapshotter](https://github.com/containerd/stargz-snapshotter) - eStargz support
- [awslabs/soci-snapshotter](https://github.com/awslabs/soci-snapshotter) - SOCI support
- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework

## References

- [OCI Image Specification](https://github.com/opencontainers/image-spec)
- [eStargz: Standard-Compatible Extensions to Tar.gz Layers](https://github.com/containerd/stargz-snapshotter)
- [SOCI: Seekable OCI](https://github.com/awslabs/soci-snapshotter)
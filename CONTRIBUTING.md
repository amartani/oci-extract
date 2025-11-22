# Contributing to OCI-Extract

Thank you for your interest in contributing to OCI-Extract!

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git
- (Optional) Docker for testing with real images

### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/oci-extract
   cd oci-extract
   ```

3. Install dependencies:
   ```bash
   go mod download
   ```

4. Build the project:
   ```bash
   make build
   ```

## Project Structure

```
oci-extract/
├── cmd/                    # CLI commands (Cobra)
│   ├── root.go            # Root command and global flags
│   └── extract.go         # Main extract command
├── internal/
│   ├── remote/            # HTTP Range request client
│   ├── registry/          # OCI registry interactions
│   ├── estargz/           # eStargz format support
│   ├── soci/              # SOCI format support
│   ├── detector/          # Format detection logic
│   └── extractor/         # Main orchestration logic
├── main.go                # Entry point
└── go.mod                 # Go module definition
```

## Development Workflow

### Making Changes

1. Create a new branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes
3. Format your code:
   ```bash
   make fmt
   ```

4. Run tests:
   ```bash
   make test
   ```

5. Check for issues:
   ```bash
   make vet
   ```

### Commit Guidelines

- Use clear, descriptive commit messages
- Follow conventional commits format: `type(scope): description`
- Examples:
  - `feat(estargz): add support for compressed TOC`
  - `fix(remote): handle connection timeouts properly`
  - `docs: update README with new examples`

### Pull Request Process

1. Update documentation if needed
2. Add tests for new functionality
3. Ensure all tests pass
4. Update CHANGELOG.md (if applicable)
5. Submit PR with clear description of changes

## Adding Support for New Formats

To add support for a new OCI layer format:

1. Create a new package under `internal/`:
   ```bash
   mkdir internal/myformat
   ```

2. Implement the extraction logic:
   ```go
   package myformat

   type Extractor struct {
       reader io.ReaderAt
   }

   func (e *Extractor) ExtractFile(ctx context.Context, path, output string) error {
       // Implementation
   }
   ```

3. Add format detection in `internal/detector/format.go`:
   ```go
   const FormatMyFormat Format = ...

   func detectMyFormat(layer v1.Layer) (bool, error) {
       // Detection logic
   }
   ```

4. Wire into the extraction logic in `cmd/extract.go`:
   ```go
   // In extractFromLayer function, add a case for your format:
   if formatToUse == "auto" || formatToUse == "myformat" {
       reader, err := remote.NewRemoteReader(layerInfo.BlobURL)
       if err != nil {
           return false, fmt.Errorf("failed to create remote reader: %w", err)
       }
       defer reader.Close()

       extractor := myformat.NewExtractor(reader, layerInfo.Size)
       err = extractor.ExtractFile(ctx, filePath, outputPath)
       if err == nil {
           return true, nil
       }
       // Handle error...
   }
   ```

5. Add tests in `internal/myformat/extractor_test.go`

## Testing

### Unit Tests

```bash
make test
```

### Integration Tests

Integration tests require network access to registries:

```bash
go test -tags=integration ./...
```

### Manual Testing

Test with a real image:

```bash
go run . extract alpine:latest /bin/sh -o ./sh -v
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting (automatically applied by `make fmt`)
- Keep functions small and focused
- Add comments for exported functions
- Handle errors explicitly

## Questions?

- Open an issue for bugs or feature requests
- Join discussions in GitHub Discussions
- Review existing issues and PRs for context

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

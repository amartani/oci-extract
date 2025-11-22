# Contributing to OCI-Extract

Thank you for your interest in contributing to OCI-Extract!

## Development Setup

### Prerequisites

- [mise](https://mise.jdx.dev/) - Development tool management (recommended)
  - mise will automatically install Go, golangci-lint, and other required tools
- Alternatively: Go 1.24+ and Git

### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/oci-extract
   cd oci-extract
   ```

3. Install development tools using mise:
   ```bash
   mise install
   ```

4. Download dependencies:
   ```bash
   mise run deps
   ```

5. Build the project:
   ```bash
   mise run build
   ```

## Project Structure

```
oci-extract/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command
│   ├── extract.go         # Extract command
│   └── list.go            # List command
├── internal/
│   ├── remote/            # HTTP Range request handler
│   │   └── reader.go
│   ├── registry/          # Registry client
│   │   └── client.go
│   ├── estargz/           # eStargz support
│   │   └── extractor.go
│   ├── soci/              # SOCI support
│   │   ├── discovery.go
│   │   └── extractor.go
│   ├── detector/          # Format detection
│   │   └── format.go
│   └── extractor/         # Orchestration logic
│       └── orchestrator.go
├── main.go
└── go.mod
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
   mise run fmt
   ```

4. Run tests:
   ```bash
   mise run test
   ```

5. Run the linter:
   ```bash
   mise run lint
   ```

6. Check for dead code:
   ```bash
   mise run deadcode
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

1. Create a new package under `internal/`
2. Implement the extraction logic
3. Add format detection in `internal/detector/`
4. Wire it into the orchestrator in `internal/extractor/orchestrator.go`
5. Add comprehensive tests

## Testing

### Unit Tests

Run fast unit tests with no external dependencies:

```bash
# Run all unit tests
mise run test

# Run with coverage report
mise run test-coverage
```

### Integration Tests

Integration tests use prebuilt images from GitHub Container Registry:

```bash
# Run integration tests (uses prebuilt images)
mise run integration-test
```

**Note:** Tests use prebuilt images from `ghcr.io/amartani/oci-extract-test`. The CI automatically builds and pushes images in all formats (standard, eStargz, SOCI) on every commit.

If you need to modify test images:
- Edit files in `test-images/` directory
- Update `tests/integration/cmd/build-images/main.go` if needed
- Images are rebuilt automatically in CI
- For local image builds (requires ghcr.io write access): `mise run integration-test-build-images`

See `tests/integration/README.md` for more details on integration testing.

### Manual Testing

Test with a real image:

```bash
# Build and run
mise run build
./oci-extract extract alpine:latest /bin/sh -o ./sh -v

# Or run directly with go
go run . extract alpine:latest /bin/sh -o ./sh -v
```

## Available Tasks

Run `mise tasks` to see all available development tasks:

```bash
mise tasks
```

Common tasks:
- `mise run build` - Build the binary for development
- `mise run build-release` - Build with version stamping and optimizations
- `mise run install` - Install the binary with optimizations
- `mise run test` - Run unit tests
- `mise run test-coverage` - Run tests with coverage report
- `mise run integration-test` - Run integration tests (uses prebuilt images)
- `mise run integration-test-build-images` - Build and push test images (CI only)
- `mise run lint` - Run golangci-lint
- `mise run fmt` - Format code with gofmt
- `mise run clean` - Remove build artifacts
- `mise run deps` - Download and tidy dependencies
- `mise run deadcode` - Check for unreachable functions

## Code Style

- Follow standard Go conventions
- Use `mise run fmt` for formatting
- Keep functions small and focused
- Add comments for exported functions and types
- Handle errors explicitly
- Run `mise run lint` before committing to catch issues early

## Questions?

- Open an issue for bugs or feature requests
- Join discussions in GitHub Discussions
- Review existing issues and PRs for context

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

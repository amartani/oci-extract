#!/bin/bash
set -e

# Build script for OCI-Extract
# This script handles dependency download with retries and builds the binary

BINARY_NAME="oci-extract"
VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')}"
BUILD_DATE="${BUILD_DATE:-$(date -u '+%Y-%m-%d_%H:%M:%S')}"

LDFLAGS="-ldflags \"-X github.com/amartani/oci-extract/cmd.version=${VERSION} \
-X github.com/amartani/oci-extract/cmd.commit=${COMMIT} \
-X github.com/amartani/oci-extract/cmd.date=${BUILD_DATE}\""

echo "Building ${BINARY_NAME}..."
echo "Version: ${VERSION}"
echo "Commit: ${COMMIT}"
echo "Build Date: ${BUILD_DATE}"
echo ""

# Function to download dependencies with retries
download_deps() {
    local max_retries=4
    local retry_delay=2

    for i in $(seq 1 $max_retries); do
        echo "Downloading dependencies (attempt $i/$max_retries)..."

        if go mod download; then
            echo "Dependencies downloaded successfully!"
            return 0
        fi

        if [ $i -lt $max_retries ]; then
            echo "Download failed, retrying in ${retry_delay}s..."
            sleep $retry_delay
            retry_delay=$((retry_delay * 2))
        fi
    done

    echo "Warning: Failed to download all dependencies after $max_retries attempts"
    echo "You may need to check your network connection or try again later"
    return 1
}

# Download dependencies
download_deps || true

# Tidy up go.mod
echo "Tidying go.mod..."
go mod tidy || true

# Build the binary
echo "Building binary..."
eval go build ${LDFLAGS} -o ${BINARY_NAME} . || {
    echo "Build failed. This might be due to missing dependencies."
    echo "Try running 'go mod download' manually and then 'go build'."
    exit 1
}

echo ""
echo "Build successful! Binary: ./${BINARY_NAME}"
echo "Run with: ./${BINARY_NAME} --help"

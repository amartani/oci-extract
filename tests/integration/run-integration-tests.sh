#!/bin/bash
# Integration test runner for OCI-Extract
# This script builds the binary and runs integration tests against test images

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REGISTRY="${REGISTRY:-ghcr.io}"
OWNER="${GITHUB_REPOSITORY_OWNER:-$(git config user.name | tr '[:upper:]' '[:lower:]')}"
IMAGE_BASE="${TEST_IMAGE_BASE:-${REGISTRY}/${OWNER}/oci-extract-test}"
IMAGE_TAG="${TEST_IMAGE_TAG:-latest}"

# Test images to use
declare -a IMAGES=(
    "${IMAGE_BASE}:standard"
    "${IMAGE_BASE}:estargz"
    "${IMAGE_BASE}:multilayer-standard"
    "${IMAGE_BASE}:multilayer-estargz"
)

# Add SOCI images if soci is available
if command -v soci &> /dev/null; then
    IMAGES+=("${IMAGE_BASE}:soci")
    IMAGES+=("${IMAGE_BASE}:multilayer-soci")
fi

# Temporary directory for test outputs
TEST_OUTPUT_DIR="/tmp/oci-extract-integration-tests-$$"
mkdir -p "${TEST_OUTPUT_DIR}"

# Cleanup function
cleanup() {
    echo -e "\n${BLUE}Cleaning up...${NC}"
    rm -rf "${TEST_OUTPUT_DIR}"
}
trap cleanup EXIT

# Print header
print_header() {
    echo -e "\n${BLUE}╔════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  OCI-Extract Integration Tests                ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════╝${NC}\n"
}

# Print section
print_section() {
    echo -e "\n${YELLOW}▶ $1${NC}"
}

# Print success
print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

# Print error
print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    print_section "Checking prerequisites"

    local missing_tools=()

    if ! command -v docker &> /dev/null && ! command -v podman &> /dev/null; then
        missing_tools+=("docker or podman")
    fi

    if ! command -v go &> /dev/null; then
        missing_tools+=("go")
    fi

    if [ ${#missing_tools[@]} -ne 0 ]; then
        print_error "Missing required tools: ${missing_tools[*]}"
        exit 1
    fi

    print_success "All required tools found"
}

# Build oci-extract binary
build_binary() {
    print_section "Building oci-extract binary"

    cd "$(git rev-parse --show-toplevel)"

    if make build; then
        print_success "Binary built successfully"

        # Verify binary works
        if ./oci-extract --version &> /dev/null; then
            print_success "Binary is functional"
        else
            print_error "Binary exists but doesn't run"
            exit 1
        fi
    else
        print_error "Build failed"
        exit 1
    fi
}

# Check if test images are available
check_images() {
    print_section "Checking test image availability"

    local images_available=0
    local images_missing=0

    for image in "${IMAGES[@]}"; do
        # Try to inspect the image
        if docker manifest inspect "${image}" &> /dev/null || \
           podman manifest inspect "${image}" &> /dev/null; then
            print_success "Image available: ${image}"
            ((images_available++))
        else
            print_error "Image not found: ${image}"
            ((images_missing++))
        fi
    done

    if [ ${images_missing} -gt 0 ]; then
        echo -e "\n${YELLOW}Warning: ${images_missing} test image(s) not found${NC}"
        echo -e "Build and push test images with:"
        echo -e "  cd test-images/base && docker build -t ${IMAGE_BASE}:standard ."
        echo -e "  docker push ${IMAGE_BASE}:standard"

        if [ ${images_available} -eq 0 ]; then
            print_error "No test images available, cannot run tests"
            exit 1
        fi
    fi

    echo -e "\n${GREEN}Found ${images_available} test image(s)${NC}"
}

# Run Go integration tests
run_go_tests() {
    print_section "Running Go integration tests"

    cd "$(git rev-parse --show-toplevel)"

    if [ -f "tests/integration/main_test.go" ]; then
        export TEST_IMAGE_BASE="${IMAGE_BASE}"
        export TEST_IMAGE_TAG="${IMAGE_TAG}"

        if go test -v -tags=integration ./tests/integration/... -timeout=30m; then
            print_success "Go integration tests passed"
        else
            print_error "Go integration tests failed"
            return 1
        fi
    else
        echo -e "${YELLOW}No Go integration tests found (tests/integration/main_test.go)${NC}"
        echo -e "${YELLOW}Skipping Go tests${NC}"
    fi
}

# Run CLI integration tests
run_cli_tests() {
    print_section "Running CLI integration tests"

    cd "$(git rev-parse --show-toplevel)"

    local tests_passed=0
    local tests_failed=0

    # Only test with standard and estargz images (most common)
    local cli_test_images=(
        "${IMAGE_BASE}:standard"
        "${IMAGE_BASE}:estargz"
    )

    for image in "${cli_test_images[@]}"; do
        # Skip if image doesn't exist
        if ! docker manifest inspect "${image}" &> /dev/null && \
           ! podman manifest inspect "${image}" &> /dev/null; then
            echo -e "${YELLOW}Skipping ${image} (not available)${NC}"
            continue
        fi

        echo -e "\n${BLUE}Testing image: ${image}${NC}"

        # Test 1: Extract small.txt
        if ./oci-extract extract "${image}" /testdata/small.txt \
           -o "${TEST_OUTPUT_DIR}/small.txt" &> /dev/null; then

            # Verify content
            expected="Hello from OCI-Extract integration test!"
            actual=$(cat "${TEST_OUTPUT_DIR}/small.txt")

            if [ "$actual" = "$expected" ]; then
                print_success "small.txt extraction and verification"
                ((tests_passed++))
            else
                print_error "small.txt content mismatch"
                echo "  Expected: ${expected}"
                echo "  Got: ${actual}"
                ((tests_failed++))
            fi
        else
            print_error "small.txt extraction failed"
            ((tests_failed++))
        fi

        # Test 2: Extract nested file
        if ./oci-extract extract "${image}" /testdata/nested/deep/file.txt \
           -o "${TEST_OUTPUT_DIR}/nested.txt" &> /dev/null; then

            expected="Nested file test - testing deep path extraction"
            actual=$(cat "${TEST_OUTPUT_DIR}/nested.txt")

            if [ "$actual" = "$expected" ]; then
                print_success "nested file extraction and verification"
                ((tests_passed++))
            else
                print_error "nested file content mismatch"
                ((tests_failed++))
            fi
        else
            print_error "nested file extraction failed"
            ((tests_failed++))
        fi

        # Test 3: Extract JSON and validate
        if ./oci-extract extract "${image}" /testdata/medium.json \
           -o "${TEST_OUTPUT_DIR}/medium.json" &> /dev/null; then

            # Check if it's valid JSON
            if jq empty "${TEST_OUTPUT_DIR}/medium.json" 2>/dev/null; then
                print_success "medium.json extraction and JSON validation"
                ((tests_passed++))
            else
                print_error "medium.json is not valid JSON"
                ((tests_failed++))
            fi
        else
            print_error "medium.json extraction failed"
            ((tests_failed++))
        fi

        # Test 4: File not found (should fail gracefully)
        if ./oci-extract extract "${image}" /nonexistent/file.txt \
           -o "${TEST_OUTPUT_DIR}/nonexistent.txt" &> /dev/null; then
            print_error "Should have failed for non-existent file"
            ((tests_failed++))
        else
            print_success "Correctly handled non-existent file"
            ((tests_passed++))
        fi
    done

    echo -e "\n${BLUE}CLI Test Results:${NC}"
    echo -e "  Passed: ${GREEN}${tests_passed}${NC}"
    echo -e "  Failed: ${RED}${tests_failed}${NC}"

    if [ ${tests_failed} -gt 0 ]; then
        return 1
    fi

    return 0
}

# Print summary
print_summary() {
    local exit_code=$1

    echo -e "\n${BLUE}╔════════════════════════════════════════════════╗${NC}"

    if [ ${exit_code} -eq 0 ]; then
        echo -e "${BLUE}║${GREEN}  ✓ All Integration Tests Passed!             ${BLUE}║${NC}"
    else
        echo -e "${BLUE}║${RED}  ✗ Some Integration Tests Failed              ${BLUE}║${NC}"
    fi

    echo -e "${BLUE}╚════════════════════════════════════════════════╝${NC}\n"
}

# Main execution
main() {
    local final_exit_code=0

    print_header

    check_prerequisites
    build_binary
    check_images

    # Run Go tests if they exist
    if ! run_go_tests; then
        final_exit_code=1
    fi

    # Run CLI tests
    if ! run_cli_tests; then
        final_exit_code=1
    fi

    print_summary ${final_exit_code}

    exit ${final_exit_code}
}

# Run main function
main "$@"

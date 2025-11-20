FROM alpine:3.19

# Set working directory
WORKDIR /testdata

# Copy test data files
COPY testdata/ /testdata/

# Generate build info with timestamp
RUN echo "Build timestamp: $(date)" > /testdata/build-info.txt && \
    echo "Image format: base" >> /testdata/build-info.txt && \
    chmod -R 644 /testdata/*

# Add labels for testing
LABEL org.opencontainers.image.title="OCI-Extract Test Image" \
      org.opencontainers.image.description="Base test image for OCI-Extract integration tests" \
      org.opencontainers.image.vendor="OCI-Extract" \
      test.purpose="integration-testing"

# Default command
CMD ["cat", "/testdata/build-info.txt"]

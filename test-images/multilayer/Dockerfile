FROM alpine:3.19

# Layer 1: First application layer
RUN mkdir -p /layer1 && \
    echo "Layer 1 content - initial setup" > /layer1/file.txt && \
    echo "timestamp: $(date)" >> /layer1/file.txt

# Layer 2: Configuration layer
RUN mkdir -p /layer2 && \
    echo "Layer 2 content - configuration" > /layer2/file.txt && \
    echo '{"config": "layer2"}' > /layer2/config.json

# Layer 3: Data layer
RUN mkdir -p /layer3 && \
    echo "Layer 3 content - data" > /layer3/file.txt && \
    dd if=/dev/zero bs=1024 count=100 2>/dev/null | tr '\0' '\143' > /layer3/data.bin

# Layer 4: Final layer with overwrite test
RUN echo "Final layer content - should override if file exists in earlier layers" > /final.txt && \
    mkdir -p /app && \
    echo "Application binary placeholder" > /app/binary

# Add labels
LABEL org.opencontainers.image.title="OCI-Extract Multi-Layer Test Image" \
      org.opencontainers.image.description="Multi-layer test image for OCI-Extract" \
      test.purpose="multilayer-testing" \
      test.layers="4"

CMD ["sh", "-c", "echo 'Multi-layer test image ready'"]

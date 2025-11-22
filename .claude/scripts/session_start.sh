#!/usr/bin/env bash

echo "Running session start script..."

# Check if mise is installed
if ! command -v mise &> /dev/null; then
    echo "mise not found. Installing mise..."
    npm install -g @jdxcode/mise
    mise --version
    echo "mise installed successfully."
else
    echo "mise is already installed."
fi

mise trust 2>/dev/null || true
mise install > /dev/null 2>&1

# Determine target environment file
TARGET_ENV_FILE="${CLAUDE_ENV_FILE:-$HOME/.bashrc}"

MISE_ACTIVATION='eval "$(mise activate bash)"'
GOPROXY_EXPORT='export GOPROXY=direct'

# Ensure directory exists
mkdir -p "$(dirname "$TARGET_ENV_FILE")"

# Add mise activation if not already present
if ! grep -qF "$MISE_ACTIVATION" "$TARGET_ENV_FILE" 2>/dev/null; then
    echo "" >> "$TARGET_ENV_FILE"
    echo "# mise activation" >> "$TARGET_ENV_FILE"
    echo "$MISE_ACTIVATION" >> "$TARGET_ENV_FILE"
    echo "mise activation added to $TARGET_ENV_FILE."
else
    echo "mise activation already present in $TARGET_ENV_FILE."
fi

# Add GOPROXY=direct if not already present
if ! grep -qF "$GOPROXY_EXPORT" "$TARGET_ENV_FILE" 2>/dev/null; then
    echo "" >> "$TARGET_ENV_FILE"
    echo "# Go proxy configuration for faster module downloads" >> "$TARGET_ENV_FILE"
    echo "$GOPROXY_EXPORT" >> "$TARGET_ENV_FILE"
    echo "GOPROXY configuration added to $TARGET_ENV_FILE."
else
    echo "GOPROXY configuration already present in $TARGET_ENV_FILE."
fi

# Immediately export variables for current session and child processes
export GOPROXY=direct

echo "Session start script completed."

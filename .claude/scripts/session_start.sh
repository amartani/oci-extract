#!/usr/bin/env bash

set -e

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

# Append mise activation to CLAUDE_ENV_FILE if not already present
if [ -n "$CLAUDE_ENV_FILE" ]; then
    MISE_ACTIVATION='eval "$(mise activate bash)"'
    if ! grep -qsF "$MISE_ACTIVATION" "$CLAUDE_ENV_FILE" 2>/dev/null; then
        echo "Adding mise activation to $CLAUDE_ENV_FILE..."
        echo "$MISE_ACTIVATION" >> "$CLAUDE_ENV_FILE"
        echo "mise activation added to environment file."
    else
        echo "mise activation already present in $CLAUDE_ENV_FILE."
    fi
else
    # Ensure mise activation is present in ~/.bashrc
    BASHRC="$HOME/.bashrc"
    MISE_ACTIVATION='eval "$(mise activate bash)"'

    if [ -f "$BASHRC" ]; then
        if ! grep -qF "$MISE_ACTIVATION" "$BASHRC"; then
            echo "Adding mise activation to $BASHRC..."
            echo "" >> "$BASHRC"
            echo "$MISE_ACTIVATION" >> "$BASHRC"
            echo "mise activation added to $BASHRC."
        else
            echo "mise activation already present in $BASHRC."
        fi
    else
        echo "$BASHRC not found. Creating and adding mise activation..."
        echo "$MISE_ACTIVATION" > "$BASHRC"
        echo "mise activation added to new $BASHRC."
    fi
fi

echo "Session start script completed."

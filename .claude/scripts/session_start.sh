#!/usr/bin/env bash

set -e

# Check if mise is installed
if ! command -v mise &> /dev/null; then
    echo "mise not found. Installing mise..."
    npm install -g @jdxcode/mise
    mise --version
fi

mise trust 2>/dev/null || true
mise install > /dev/null 2>&1
mise generate git-pre-commit --write --task=pre-commit

if [ -n "$CLAUDE_ENV_FILE" ]; then
    # In Claude Code, there is frequently throttling when downloading Go packages.
    # Setting GOPROXY to direct can help mitigate this issue.
    echo 'export GOPROXY_EXPORT=direct' >> "$CLAUDE_ENV_FILE"
else
    # Ensure mise activation is present in ~/.bashrc
    BASHRC="$HOME/.bashrc"
    # shellcheck disable=SC2016
    MISE_ACTIVATION='eval "$(mise activate bash)"'

    touch "$BASHRC"
    if ! grep -qF "$MISE_ACTIVATION" "$BASHRC"; then
        echo "Adding mise activation to $BASHRC..."
        echo "" >> "$BASHRC"
        echo "$MISE_ACTIVATION" >> "$BASHRC"
    fi
fi

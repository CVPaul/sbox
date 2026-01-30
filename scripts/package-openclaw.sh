#!/bin/bash
# Package sbox-openclaw template for distribution
# Usage: ./scripts/package-openclaw.sh [output_dir]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="${1:-$PROJECT_ROOT/dist}"
TEMPLATE_DIR="$PROJECT_ROOT/releases/sbox-openclaw"
ARCHIVE_NAME="sbox-openclaw.tar.gz"

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

# Verify template exists
if [ ! -d "$TEMPLATE_DIR" ]; then
    echo "Error: Template directory not found: $TEMPLATE_DIR"
    exit 1
fi

# Create archive with explicit excludes
# - No absolute paths (using -C for relative paths)
# - No runtime state or caches
# - Deterministic (sorted, no timestamps in content)
echo "Packaging sbox-openclaw template..."

tar -czf "$OUTPUT_DIR/$ARCHIVE_NAME" \
    -C "$PROJECT_ROOT/releases" \
    --exclude='*.pyc' \
    --exclude='__pycache__' \
    --exclude='node_modules' \
    --exclude='.npm' \
    --exclude='.cache' \
    --exclude='.venv' \
    --exclude='venv' \
    --exclude='.env' \
    --exclude='*.log' \
    --exclude='.DS_Store' \
    --exclude='Thumbs.db' \
    --exclude='*.tar.gz' \
    --exclude='*.zip' \
    --exclude='.sbox/env' \
    --exclude='.sbox/mamba' \
    --exclude='sbox.lock' \
    sbox-openclaw

# Show result
ARCHIVE_PATH="$OUTPUT_DIR/$ARCHIVE_NAME"
ARCHIVE_SIZE=$(du -h "$ARCHIVE_PATH" | cut -f1)

echo ""
echo "Created: $ARCHIVE_PATH"
echo "Size: $ARCHIVE_SIZE"
echo ""
echo "Contents:"
tar -tzf "$ARCHIVE_PATH" | head -20

echo ""
echo "To test:"
echo "  mkdir /tmp/test-openclaw && cd /tmp/test-openclaw"
echo "  tar -xzf $ARCHIVE_PATH"
echo "  ls -la sbox-openclaw/"

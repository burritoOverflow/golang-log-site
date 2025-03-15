#!/usr/bin/env bash

set -xe

# Usage: ./deploy.sh username@hostname

if [ -z "$1" ]; then
    echo "Error: Destination is required"
    echo "Usage: ./deploy.sh username@hostname"
    exit 1
fi

DESTINATION=$1
BINARY_NAME="logwatcher-arm64"
REMOTE_DIR="~/.local/logwatcher"

echo "Building for Raspberry Pi (ARM)..."

ARM64_CC=aarch64-linux-gnu-gcc
GOOS=linux GOARCH=arm64 CC=$ARM64_CC go build -o $BINARY_NAME

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo "Build successful. Transferring to $DESTINATION..."

ssh "$DESTINATION" "mkdir -p $REMOTE_DIR"

scp $BINARY_NAME "$DESTINATION":$REMOTE_DIR/

scp -r static "$DESTINATION":$REMOTE_DIR/

if [ $? -eq 0 ]; then
    echo "Deployment complete! Binary transferred to $DESTINATION:~/$BINARY_NAME"

    ssh "$DESTINATION" "chmod +x $REMOTE_DIR/$BINARY_NAME && ls -latsh $REMOTE_DIR/"
    echo "Binary permissions set to executable"
else
    echo "Transfer failed!"
    exit 1
fi
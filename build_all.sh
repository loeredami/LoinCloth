#!/bin/bash

APP_NAME="loin"
BUILD_DIR="builds"

mkdir -p $BUILD_DIR

targets=(
    "linux/amd64"
    "linux/arm64"
    "linux/386"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
    "windows/386"
)

echo "Starting builds..."

for target in "${targets[@]}"; do
    IFS="/" read -r OS ARCH <<< "$target"

    OUTPUT_NAME="${APP_NAME}_${OS}_${ARCH}"
    if [ "$OS" == "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi

    echo "Building for $OS ($ARCH)..."

    env GOOS=$OS GOARCH=$ARCH go build -o "${BUILD_DIR}/${OUTPUT_NAME}" .

    if [ $? -ne 0 ]; then
        echo "Error: Build failed for $target"
    else
        echo "Done: ${OUTPUT_NAME}"
    fi
done

echo "---------------------------------"
echo "All builds completed in ./${BUILD_DIR}"

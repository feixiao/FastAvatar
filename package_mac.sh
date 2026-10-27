#!/usr/bin/env bash
set -euo pipefail

echo "=== Building Avatar Native App for macOS ==="

APP_NAME="Avatar"
BUILD_DIR="build/bin"
APP_DIR="${BUILD_DIR}/${APP_NAME}.app"
CONTENTS_DIR="${APP_DIR}/Contents"
MACOS_DIR="${CONTENTS_DIR}/MacOS"
RESOURCES_DIR="${CONTENTS_DIR}/Resources"

mkdir -p "${MACOS_DIR}" "${RESOURCES_DIR}"

echo "1. Compiling Go binary..."
go build -tags production -ldflags="-w -s" -o "${MACOS_DIR}/${APP_NAME}" .

echo "1b. Compiling native avatar-asr helper..."
mkdir -p bin
swiftc -O -framework Speech -framework AVFoundation cmd/avatar-asr/main.swift -o bin/avatar-asr
cp "bin/avatar-asr" "${MACOS_DIR}/avatar-asr"

echo "2. Generating Info.plist..."
sed \
  -e "s/{{.Info.ProductName}}/${APP_NAME}/g" \
  -e "s/{{.Name}}/${APP_NAME}/g" \
  -e "s/{{.Info.ProductVersion}}/1.0.0/g" \
  -e "s/{{.Info.Comments}}/Desktop AI Companion/g" \
  -e "s/{{.Info.Copyright}}/Copyright 2026/g" \
  build/darwin/Info.plist > "${CONTENTS_DIR}/Info.plist"

if [ -f "build/appicon.png" ]; then
  cp "build/appicon.png" "${RESOURCES_DIR}/iconfile.png"
fi

echo "=== Build Complete: ${APP_DIR} ==="
ls -lh "${MACOS_DIR}/${APP_NAME}"

#!/bin/bash
set -euo pipefail

# Script to generate Helm chart index.yaml from GitHub releases (assumes public repository)
# Usage: generate-helm-index.sh <github_repo> <workspace> <chart_package_path> <index_url>

GITHUB_REPOSITORY="${1:-}"
WORKSPACE="${2:-}"
CHART_PACKAGE_PATH="${3:-}"
INDEX_URL="${4:-}"

if [ -z "$GITHUB_REPOSITORY" ] || [ -z "$WORKSPACE" ]; then
  echo "Error: Missing required parameters"
  echo "Usage: $0 <github_repo> <workspace> <chart_package_path> <index_url>"
  exit 1
fi

# Create a temporary directory for charts
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

cd "$TEMP_DIR"

echo "Generating index.yaml from GitHub releases"
echo "Repository: $GITHUB_REPOSITORY (public)"
echo "Working directory: $TEMP_DIR"

# Fetch all GitHub releases (public API, no authentication needed)
# Handle pagination to get all releases
echo "Fetching all GitHub releases..."
RELEASES="[]"
PAGE=1
PER_PAGE=100

while true; do
  PAGE_RELEASES=$(curl -s "https://api.github.com/repos/${GITHUB_REPOSITORY}/releases?page=${PAGE}&per_page=${PER_PAGE}")
  
  # Check if we got valid releases for this page
  if [ -z "$PAGE_RELEASES" ] || [ "$PAGE_RELEASES" == "null" ] || [ "$(echo "$PAGE_RELEASES" | jq -r 'if type=="array" then length else 0 end')" -eq 0 ]; then
    break
  fi
  
  # Merge releases from this page
  RELEASES=$(echo "$RELEASES" | jq ". + $PAGE_RELEASES")
  PAGE_COUNT=$(echo "$PAGE_RELEASES" | jq -r 'length')
  
  if [ "$PAGE_COUNT" -lt "$PER_PAGE" ]; then
    break
  fi
  
  PAGE=$((PAGE + 1))
done

# Check if we got valid releases
if [ -z "$RELEASES" ] || [ "$RELEASES" == "null" ] || [ "$RELEASES" == "[]" ]; then
  echo "Warning: No releases found in repository"
else
  RELEASE_COUNT=$(echo "$RELEASES" | jq -r 'length')
  echo "Found $RELEASE_COUNT release(s)"
fi

# Download chart packages from all releases
echo "$RELEASES" | jq -r '.[] | .tag_name' | while read -r tag; do
  echo "Processing release: $tag"
  # Find chart asset in release - get both URL and name from release API
  ASSET_INFO=$(echo "$RELEASES" | jq -r ".[] | select(.tag_name == \"$tag\") | .assets[] | select(.name | endswith(\".tgz\")) | \"\(.browser_download_url)|\(.name)\"" | head -n 1)
  
  if [ -n "$ASSET_INFO" ] && [ "$ASSET_INFO" != "null" ] && [ "$ASSET_INFO" != "" ]; then
    ASSET_URL=$(echo "$ASSET_INFO" | cut -d'|' -f1)
    ASSET_NAME=$(echo "$ASSET_INFO" | cut -d'|' -f2)
    echo "Downloading chart from release asset: $ASSET_NAME"
    # For public repositories, no authentication needed for downloading assets
    # Use the original asset name to preserve version information
    curl -L -f -o "$ASSET_NAME" "$ASSET_URL" 2>/dev/null || echo "Failed to download chart for $tag"
  else
    echo "No chart asset found for release $tag, skipping..."
  fi
done

# Also include the current chart package if provided
if [ -n "$CHART_PACKAGE_PATH" ] && [ -f "$CHART_PACKAGE_PATH" ]; then
  cp "$CHART_PACKAGE_PATH" "$TEMP_DIR/"
  echo "Included current chart package: $(basename "$CHART_PACKAGE_PATH")"
fi

# Check if we have any chart packages
CHART_COUNT=$(find . -name "*.tgz" -type f | wc -l)
if [ "$CHART_COUNT" -eq 0 ]; then
  echo "Warning: No chart packages found. index.yaml will be empty."
fi

# Generate index.yaml
echo "Generating index.yaml..."
if [ -n "$INDEX_URL" ]; then
  helm repo index . --url "$INDEX_URL"
else
  helm repo index . --url "https://github.com/${GITHUB_REPOSITORY}/releases/download"
fi

# Copy index.yaml to workspace
if [ -f "index.yaml" ]; then
  cp index.yaml "$WORKSPACE/index.yaml"
  echo "index.yaml generated successfully at $WORKSPACE/index.yaml"
  echo "Found $CHART_COUNT chart package(s)"
else
  echo "Error: Failed to generate index.yaml"
  exit 1
fi

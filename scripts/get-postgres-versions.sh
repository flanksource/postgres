#!/bin/bash
set -e

# Script to fetch latest PostgreSQL versions from built Docker images
# Used by the release workflow to tag images with correct version numbers

echo "🔍 Detecting PostgreSQL versions from built images..."

# Function to get version from built Docker image
get_version_from_image() {
    local major_version=$1
    local image_tag="postgres-upgrade:$major_version"
    
    echo "🔧 Detecting PostgreSQL $major_version version from image $image_tag..." >&2
    
    # Run the built image to get the PostgreSQL version
    version=$(docker run --rm --entrypoint="" "$image_tag" \
        /usr/lib/postgresql/$major_version/bin/postgres --version 2>/dev/null | \
        grep -oE 'PostgreSQL [0-9]+\.[0-9]+' | \
        awk '{print $2}' || echo "")
    
    if [ -z "$version" ]; then
        echo "❌ Failed to detect version for PostgreSQL $major_version" >&2
        exit 1
    fi
    
    echo "✅ Detected PostgreSQL $major_version version: $version" >&2
    echo "$version"
}

# Function for backward compatibility - try to get from image first, fallback to hardcoded
get_latest_version() {
    local major_version=$1
    
    # Try to get from built image first
    if docker image inspect "postgres-upgrade:$major_version" >/dev/null 2>&1; then
        get_version_from_image "$major_version"
    else
        echo "⚠️  Image postgres-upgrade:$major_version not found, using hardcoded version" >&2
        # Fallback to hardcoded versions
        case $major_version in
            14)
                echo "14.19"
                ;;
            15)
                echo "15.14"
                ;;
            16)
                echo "16.10"
                ;;
            17)
                echo "17.6"
                ;;
            *)
                echo "Unknown version: $major_version" >&2
                exit 1
                ;;
        esac
    fi
}

# Output for GitHub Actions if running in CI
if [ "${GITHUB_ACTIONS}" = "true" ] && [ -n "${GITHUB_OUTPUT}" ]; then
    echo "pg14_version=$(get_latest_version 14)" >> $GITHUB_OUTPUT
    echo "pg15_version=$(get_latest_version 15)" >> $GITHUB_OUTPUT
    echo "pg16_version=$(get_latest_version 16)" >> $GITHUB_OUTPUT
    echo "pg17_version=$(get_latest_version 17)" >> $GITHUB_OUTPUT
fi

# Human readable output
echo "PostgreSQL Version Information:"
echo "  PostgreSQL 14: $(get_latest_version 14)"
echo "  PostgreSQL 15: $(get_latest_version 15)"
echo "  PostgreSQL 16: $(get_latest_version 16)"
echo "  PostgreSQL 17: $(get_latest_version 17)"

echo "✅ Version information retrieved successfully"
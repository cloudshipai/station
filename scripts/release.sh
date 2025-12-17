#!/bin/bash
set -e

# Station Release Automation Script
# Usage: ./scripts/release.sh [patch|minor|major]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION_FILE="$PROJECT_ROOT/VERSION"
CHANGELOG_FILE="$PROJECT_ROOT/CHANGELOG.md"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check if git working directory is clean
check_git_clean() {
    if [[ -n $(git status -s) ]]; then
        log_error "Working directory is not clean. Commit or stash changes first."
        exit 1
    fi
}

# Get current version
get_current_version() {
    if [[ -f "$VERSION_FILE" ]]; then
        cat "$VERSION_FILE"
    else
        # Try to get from git tags
        git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0"
    fi
}

# Bump version
bump_version() {
    local current=$1
    local bump_type=$2

    IFS='.' read -r -a parts <<< "$current"
    major="${parts[0]}"
    minor="${parts[1]}"
    patch="${parts[2]}"

    case $bump_type in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
        *)
            log_error "Invalid bump type: $bump_type. Use: major, minor, or patch"
            exit 1
            ;;
    esac

    echo "$major.$minor.$patch"
}

# Generate changelog from commits
generate_changelog() {
    local from_tag=$1
    local to_ref=${2:-HEAD}
    local version=$3

    log_info "Generating changelog from $from_tag to $to_ref..."

    # Get commits since last tag
    local commits=$(git log "$from_tag..$to_ref" --pretty=format:"%s|||%h" --no-merges)

    # Categorize commits
    local features=""
    local fixes=""
    local docs=""
    local refactors=""
    local chores=""
    local others=""

    while IFS='|||' read -r message hash; do
        if [[ $message =~ ^feat(\(.+\))?: ]]; then
            features+="* $message ($hash)\n"
        elif [[ $message =~ ^fix(\(.+\))?: ]]; then
            fixes+="* $message ($hash)\n"
        elif [[ $message =~ ^docs(\(.+\))?: ]]; then
            docs+="* $message ($hash)\n"
        elif [[ $message =~ ^refactor(\(.+\))?: ]]; then
            refactors+="* $message ($hash)\n"
        elif [[ $message =~ ^chore(\(.+\))?: ]]; then
            chores+="* $message ($hash)\n"
        else
            others+="* $message ($hash)\n"
        fi
    done <<< "$commits"

    # Build changelog entry
    local changelog="## [$version] - $(date +%Y-%m-%d)\n\n"

    if [[ -n $features ]]; then
        changelog+="### 🚀 Features\n$features\n"
    fi

    if [[ -n $fixes ]]; then
        changelog+="### 🐛 Bug Fixes\n$fixes\n"
    fi

    if [[ -n $docs ]]; then
        changelog+="### 📚 Documentation\n$docs\n"
    fi

    if [[ -n $refactors ]]; then
        changelog+="### ♻️ Refactoring\n$refactors\n"
    fi

    if [[ -n $chores ]]; then
        changelog+="### 🔧 Chores\n$chores\n"
    fi

    if [[ -n $others ]]; then
        changelog+="### 📝 Other Changes\n$others\n"
    fi

    echo -e "$changelog"
}

# Update CHANGELOG.md
update_changelog_file() {
    local new_entry=$1

    if [[ -f "$CHANGELOG_FILE" ]]; then
        # Insert new entry after the header
        local temp_file=$(mktemp)
        head -n 3 "$CHANGELOG_FILE" > "$temp_file"
        echo -e "$new_entry" >> "$temp_file"
        tail -n +4 "$CHANGELOG_FILE" >> "$temp_file"
        mv "$temp_file" "$CHANGELOG_FILE"
    else
        # Create new CHANGELOG.md
        cat > "$CHANGELOG_FILE" <<EOF
# Changelog

All notable changes to Station will be documented in this file.

$new_entry
EOF
    fi
}

# Create VERSION file
create_version_file() {
    local version=$1
    echo "$version" > "$VERSION_FILE"
}

# Main release process
main() {
    local bump_type=${1:-patch}

    log_info "Starting release process..."

    # Check prerequisites
    if ! command -v git &> /dev/null; then
        log_error "git is required but not installed"
        exit 1
    fi

    if ! command -v gh &> /dev/null; then
        log_error "gh (GitHub CLI) is required but not installed"
        exit 1
    fi

    # Check git status
    check_git_clean

    # Pull latest changes
    log_info "Pulling latest changes from origin/main..."
    git checkout main
    git pull origin main

    # Get current and new version
    local current_version=$(get_current_version)
    local new_version=$(bump_version "$current_version" "$bump_type")

    log_info "Current version: v$current_version"
    log_info "New version: v$new_version"

    # Get last tag
    local last_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    if [[ -z $last_tag ]]; then
        last_tag="$(git rev-list --max-parents=0 HEAD)" # First commit
    fi

    # Generate changelog
    log_info "Generating changelog..."
    local changelog_entry=$(generate_changelog "$last_tag" "HEAD" "$new_version")

    # Preview
    echo ""
    log_info "Changelog Preview:"
    echo "---"
    echo -e "$changelog_entry"
    echo "---"
    echo ""

    # Confirm
    read -p "Proceed with release v$new_version? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_warning "Release cancelled"
        exit 0
    fi

    # Update VERSION file
    log_info "Updating VERSION file..."
    create_version_file "$new_version"

    # Update CHANGELOG.md
    log_info "Updating CHANGELOG.md..."
    update_changelog_file "$changelog_entry"

    # Commit changes
    log_info "Committing version bump..."
    git add "$VERSION_FILE" "$CHANGELOG_FILE"
    git commit -m "chore: Release v$new_version" || true

    # Create and push tag
    log_info "Creating tag v$new_version..."
    git tag -a "v$new_version" -m "Release v$new_version

$(echo -e "$changelog_entry")"

    log_info "Pushing changes and tag..."
    git push origin main
    git push origin "v$new_version"

    log_success "Release v$new_version created successfully!"
    log_info "GitHub Actions will now build and publish the release"
    log_info "Monitor progress: https://github.com/cloudshipai/station/actions"

    # Wait a bit for GitHub to register the tag
    sleep 5

    # Show workflow status
    log_info "Checking workflow status..."
    gh run list --workflow=release.yml --limit 1

    echo ""
    log_success "Release process complete!"
    echo ""
    echo "Next steps:"
    echo "  1. Watch GitHub Actions: gh run watch"
    echo "  2. Verify release: gh release view v$new_version"
    echo "  3. Test installation: curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash"
    echo "  4. Update project board and close milestone"
}

# Show help
show_help() {
    cat <<EOF
Station Release Automation

Usage: $0 [patch|minor|major]

Arguments:
  patch     Bump patch version (0.0.X) - bug fixes
  minor     Bump minor version (0.X.0) - new features
  major     Bump major version (X.0.0) - breaking changes

Examples:
  $0 patch   # 0.16.1 -> 0.16.2
  $0 minor   # 0.16.1 -> 0.17.0
  $0 major   # 0.16.1 -> 1.0.0

This script will:
  1. Check git status (must be clean)
  2. Pull latest changes from origin/main
  3. Generate changelog from commits
  4. Update VERSION and CHANGELOG.md files
  5. Create git tag
  6. Push changes and tag (triggers GitHub Actions)

GitHub Actions will then:
  - Build binaries for all platforms
  - Create multi-arch Docker images
  - Generate GitHub release
  - Update 'latest' tag
EOF
}

# Parse arguments
case ${1:-} in
    -h|--help)
        show_help
        exit 0
        ;;
    patch|minor|major)
        main "$1"
        ;;
    "")
        log_error "Missing argument. Use: patch, minor, or major"
        echo ""
        show_help
        exit 1
        ;;
    *)
        log_error "Invalid argument: $1"
        echo ""
        show_help
        exit 1
        ;;
esac

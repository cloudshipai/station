#!/bin/bash

# Generate coverage badge for Station
# This script runs tests, extracts coverage percentage, and generates a badge URL

set -e

echo "🧪 Running tests to calculate coverage..."

# Run tests and capture coverage
COVERAGE=$(go test -cover ./internal/services 2>&1 | grep "coverage:" | awk '{print $2}' | tr -d '%')

if [ -z "$COVERAGE" ]; then
    echo "❌ Failed to extract coverage percentage"
    exit 1
fi

echo "✅ Coverage: ${COVERAGE}%"

# Determine badge color based on coverage
if (( $(echo "$COVERAGE >= 80" | bc -l) )); then
    COLOR="brightgreen"
elif (( $(echo "$COVERAGE >= 60" | bc -l) )); then
    COLOR="green"
elif (( $(echo "$COVERAGE >= 40" | bc -l) )); then
    COLOR="yellow"
elif (( $(echo "$COVERAGE >= 20" | bc -l) )); then
    COLOR="orange"
else
    COLOR="red"
fi

# Generate shields.io badge URL
BADGE_URL="https://img.shields.io/badge/coverage-${COVERAGE}%25-${COLOR}?style=flat-square"

echo ""
echo "📊 Coverage Badge URL:"
echo "$BADGE_URL"
echo ""
echo "📝 Add this to your README.md:"
echo "![Coverage](${BADGE_URL})"
echo ""
echo "Or use this markdown:"
echo "[![Coverage](${BADGE_URL})](https://github.com/yourusername/station)"

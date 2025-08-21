#!/usr/bin/env bash
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
EX="$ROOT/examples/sbomb-example"

echo "Running SBOM bundle tests..."

# Generate SBOM
echo "1. Generating SBOM with Syft..."
ship mcp call syft_sbom --target "dir:$EX" --format cyclonedx-json --output_path "$EX/sbom.cdx.json"

# Scan for vulnerabilities
echo "2. Scanning vulnerabilities with Grype..."
ship mcp call grype_scan --input "sbom:$EX/sbom.cdx.json" --format json > "$EX/grype.json"

# Check OSV database
echo "3. Checking OSV database..."
ship mcp call osv_scan --mode source --path_or_ref "$EX" --format sarif > "$EX/osv.sarif"

# Scan licenses
echo "4. Scanning licenses with ScanCode..."
ship mcp call scancode_licenses --path "$EX" --output_path "$EX/scancode.json" > /dev/null

# Test policies
echo "5. Testing policies with Conftest..."
ship mcp call conftest_test --input_file "$EX/scancode.json" --policy_dir "$EX/policies" --data_dir "$EX/data" > "$EX/conftest.json"

echo ""
echo "Artifacts generated:"
ls -lh "$EX"/sbom.cdx.json "$EX"/grype.json "$EX"/osv.sarif "$EX"/scancode.json "$EX"/conftest.json
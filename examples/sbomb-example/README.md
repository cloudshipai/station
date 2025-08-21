# SBOM Example (tests)

This is a purposely "spicy" sample project designed to trigger SBOM, vulnerability, and license compliance checks.

## Quick run

```bash
# Generate SBOM
ship mcp call syft_sbom --target "dir:examples/sbomb-example" --format cyclonedx-json --output_path examples/sbomb-example/sbom.cdx.json

# Scan for vulnerabilities
ship mcp call grype_scan --input "sbom:examples/sbomb-example/sbom.cdx.json" --format json

# Check OSV database
ship mcp call osv_scan --mode source --path_or_ref examples/sbomb-example --format sarif

# Scan licenses
ship mcp call scancode_licenses --path examples/sbomb-example --output_path examples/sbomb-example/scancode.json

# Test policies
ship mcp call conftest_test --input_file examples/sbomb-example/scancode.json --policy_dir examples/sbomb-example/policies --data_dir examples/sbomb-example/data
```

## Test Expectations

- **Stable**: SBOM file created, Grype JSON has matches[], OSV SARIF file exists, ScanCode JSON exists, Conftest detects at least one denied license due to third_party/gpl_snippet.c
- **Non-deterministic counts**: Do not assert exact vuln counts; assert presence/shape
- **Offline OK**: Tests should pass even if scanners fetch DBs at runtime

## What this tests

1. **Dependencies with known vulnerabilities**:
   - lodash 4.17.19 (CVE-2020-28500)
   - minimist 0.0.8 (CVE-2020-7598)
   - requests 2.19.1 (CVE-2018-18074)
   - urllib3 1.24.1 (CVE-2019-11324)

2. **License compliance issues**:
   - Project is MIT licensed
   - Contains GPL-3.0 code in third_party/
   - Triggers license policy violations

3. **SBOM generation across ecosystems**:
   - JavaScript (npm)
   - Python (pip)
   - Container (Docker)
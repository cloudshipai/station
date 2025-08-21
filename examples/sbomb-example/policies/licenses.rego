package licensepolicy

default allow = true

allowed := {l | l := data.license_allowlist[_]}
denied  := {l | l := data.license_denied[_]}

# Example input for ScanCode:
# { "scan_type":"scancode", "files": [{ "path": "...", "declared_license": "GPL-3.0-only" }, ...] }

deny[msg] {
  input.scan_type == "scancode"
  some i
  lic := input.files[i].declared_license
  lic != ""  # ignore unknown
  lic == denied[_]
  msg := sprintf("Disallowed license %q in %q", [lic, input.files[i].path])
}

warn[msg] {
  input.scan_type == "scancode"
  some i
  lic := input.files[i].declared_license
  lic != "" ; not lic == denied[_]
  not lic == allowed[_]
  msg := sprintf("Unapproved license %q in %q", [lic, input.files[i].path])
}
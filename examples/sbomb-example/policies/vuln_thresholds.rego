package vulnpolicy

import future.keywords.in

default pass = true

critical := count([v | v in input.vulns; v.severity == "CRITICAL"])
high     := count([v | v in input.vulns; v.severity == "HIGH"])

deny[msg] {
  critical > data.thresholds.max_critical
  msg := sprintf("Too many critical vulns: %d > %d", [critical, data.thresholds.max_critical])
}

deny[msg] {
  high > data.thresholds.max_high
  msg := sprintf("Too many high vulns: %d > %d", [high, data.thresholds.max_high])
}
package mocks

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// NewGuardDutyMock creates a mock AWS GuardDuty MCP server for security threat detection
func NewGuardDutyMock() *MockServer {
	server := NewMockServer(
		"guardduty",
		"1.0.0",
		"Mock AWS GuardDuty for threat detection and security findings",
	)

	// get_findings - List security findings
	server.RegisterTool(mcp.Tool{
		Name:        "get_findings",
		Description: "Retrieve GuardDuty security findings with severity, resource, and threat details",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"severity": map[string]interface{}{
					"type":        "string",
					"description": "Filter by severity: Low, Medium, High, Critical",
					"enum":        []string{"Low", "Medium", "High", "Critical"},
				},
				"max_results": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of findings to return",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"findings": []map[string]interface{}{
				{
					"finding_id":  "finding-a1b2c3d4e5",
					"type":        "UnauthorizedAccess:EC2/SSHBruteForce",
					"severity":    "High",
					"region":      "us-east-1",
					"resource":    map[string]interface{}{
						"id":   "i-0123456789abcdef0",
						"type": "Instance",
						"tags": []map[string]string{
							{"Key": "Environment", "Value": "production"},
							{"Key": "Team", "Value": "platform"},
						},
					},
					"service": map[string]interface{}{
						"action": map[string]interface{}{
							"action_type":   "NETWORK_CONNECTION",
							"port_probe_action": map[string]interface{}{
								"blocked": false,
								"port_probe_details": []map[string]interface{}{
									{"local_port": 22, "remote_ip_details": map[string]interface{}{
										"ip_address": "198.51.100.42",
										"country":    "Unknown",
									}},
								},
							},
						},
						"count": 127,
					},
					"first_seen": time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
					"last_seen":  time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
					"description": "EC2 instance has been involved in SSH brute force attacks.",
				},
				{
					"finding_id": "finding-f6g7h8i9j0",
					"type":       "Recon:EC2/PortProbeUnprotectedPort",
					"severity":   "Medium",
					"region":     "us-west-2",
					"resource": map[string]interface{}{
						"id":   "i-9876543210fedcba0",
						"type": "Instance",
					},
					"service": map[string]interface{}{
						"action": map[string]interface{}{
							"action_type": "PORT_PROBE",
						},
						"count": 43,
					},
					"first_seen":  time.Now().Add(-72 * time.Hour).Format(time.RFC3339),
					"last_seen":   time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
					"description": "EC2 instance is being probed on unprotected ports.",
				},
				{
					"finding_id": "finding-k1l2m3n4o5",
					"type":       "CryptoCurrency:EC2/BitcoinTool.B!DNS",
					"severity":   "Critical",
					"region":     "us-east-1",
					"resource": map[string]interface{}{
						"id":   "i-abcd1234efgh5678",
						"type": "Instance",
					},
					"service": map[string]interface{}{
						"action": map[string]interface{}{
							"action_type": "DNS_REQUEST",
							"dns_request_action": map[string]interface{}{
								"domain":  "pool.minergate.com",
								"blocked": false,
							},
						},
						"count": 1523,
					},
					"first_seen":  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
					"last_seen":   time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
					"description": "EC2 instance is querying a domain associated with cryptocurrency mining.",
				},
			},
		}
		return SuccessResult(data)
	})

	return server
}

// NewIAMAccessAnalyzerMock creates a mock AWS IAM Access Analyzer MCP server
func NewIAMAccessAnalyzerMock() *MockServer {
	server := NewMockServer(
		"iam-access-analyzer",
		"1.0.0",
		"Mock AWS IAM Access Analyzer for identifying public resource exposure and overprivileged access",
	)

	// get_findings - Public exposure findings
	server.RegisterTool(mcp.Tool{
		Name:        "get_findings",
		Description: "Retrieve Access Analyzer findings showing publicly accessible resources",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"findings": []map[string]interface{}{
				{
					"finding_id":  "aa-finding-pub-s3-1",
					"resource":    "arn:aws:s3:::company-data-backup",
					"is_public":   true,
					"resource_type": "AWS::S3::Bucket",
					"principals":  []string{"*"},
					"actions":     []string{"s3:GetObject", "s3:ListBucket"},
					"condition":   map[string]interface{}{},
					"status":      "ACTIVE",
					"created_at":  time.Now().Add(-14 * 24 * time.Hour).Format(time.RFC3339),
					"analyzed_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				},
				{
					"finding_id":    "aa-finding-pub-iam-1",
					"resource":      "arn:aws:iam::123456789012:role/AdminRole",
					"is_public":     false,
					"resource_type": "AWS::IAM::Role",
					"principals":    []string{"arn:aws:iam::999888777666:root"},
					"actions":       []string{"sts:AssumeRole"},
					"condition":     map[string]interface{}{},
					"status":        "ACTIVE",
					"created_at":    time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339),
					"analyzed_at":   time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
				},
			},
		}
		return SuccessResult(data)
	})

	// analyze_policy - Analyze IAM policy for overprivileged access
	server.RegisterTool(mcp.Tool{
		Name:        "analyze_policy",
		Description: "Analyze IAM policy to identify overly permissive actions and suggest least privilege alternatives",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"policy_document": map[string]interface{}{
					"type":        "string",
					"description": "IAM policy JSON document to analyze",
				},
			},
			Required: []string{"policy_document"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"findings": []map[string]interface{}{
				{
					"finding_type": "OVERLY_PERMISSIVE_ACTIONS",
					"issue":        "Policy allows wildcard actions on all resources",
					"actions":      []string{"s3:*", "ec2:*", "iam:*"},
					"resources":    []string{"*"},
					"risk_level":   "HIGH",
					"recommendation": "Restrict actions to specific operations needed (e.g., s3:GetObject instead of s3:*)",
				},
				{
					"finding_type": "PRIVILEGE_ESCALATION_RISK",
					"issue":        "Policy allows creation of IAM users with full permissions",
					"actions":      []string{"iam:CreateUser", "iam:AttachUserPolicy"},
					"risk_level":   "CRITICAL",
					"recommendation": "Remove IAM user creation permissions or limit to specific policies",
				},
			},
			"suggested_policy": map[string]interface{}{
				"Version": "2012-10-17",
				"Statement": []map[string]interface{}{
					{
						"Effect": "Allow",
						"Action": []string{"s3:GetObject", "s3:PutObject"},
						"Resource": "arn:aws:s3:::specific-bucket/*",
					},
				},
			},
		}
		return SuccessResult(data)
	})

	return server
}

// NewTrivyMock creates a mock Trivy vulnerability scanner MCP server
func NewTrivyMock() *MockServer {
	server := NewMockServer(
		"trivy",
		"1.0.0",
		"Mock Trivy vulnerability scanner for container image and filesystem scanning",
	)

	// scan_image - Scan container image for vulnerabilities
	server.RegisterTool(mcp.Tool{
		Name:        "scan_image",
		Description: "Scan container image for OS and application vulnerabilities",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"image": map[string]interface{}{
					"type":        "string",
					"description": "Container image name and tag (e.g., nginx:latest)",
				},
			},
			Required: []string{"image"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"image":       "nginx:1.21.0",
			"scanned_at":  time.Now().Format(time.RFC3339),
			"vulnerabilities": []map[string]interface{}{
				{
					"vulnerability_id": "CVE-2023-12345",
					"package":          "libssl1.1",
					"installed_version": "1.1.1n-0+deb11u3",
					"fixed_version":    "1.1.1n-0+deb11u4",
					"severity":         "CRITICAL",
					"cvss_score":       9.8,
					"description":      "OpenSSL has a buffer overflow vulnerability allowing remote code execution",
					"layer":            "sha256:abcd1234...",
					"primary_url":      "https://nvd.nist.gov/vuln/detail/CVE-2023-12345",
				},
				{
					"vulnerability_id": "CVE-2023-45678",
					"package":          "curl",
					"installed_version": "7.74.0-1.3+deb11u3",
					"fixed_version":    "7.74.0-1.3+deb11u5",
					"severity":         "HIGH",
					"cvss_score":       7.5,
					"description":      "curl contains a heap buffer overflow in URL parsing",
					"layer":            "sha256:ef567890...",
					"primary_url":      "https://nvd.nist.gov/vuln/detail/CVE-2023-45678",
				},
				{
					"vulnerability_id": "CVE-2023-78901",
					"package":          "libc-bin",
					"installed_version": "2.31-13+deb11u5",
					"fixed_version":    "",
					"severity":         "MEDIUM",
					"cvss_score":       5.3,
					"description":      "GNU C Library has an information disclosure vulnerability",
					"layer":            "sha256:12345abc...",
					"primary_url":      "https://nvd.nist.gov/vuln/detail/CVE-2023-78901",
				},
			},
			"summary": map[string]interface{}{
				"total":    3,
				"critical": 1,
				"high":     1,
				"medium":   1,
				"low":      0,
			},
		}
		return SuccessResult(data)
	})

	// scan_filesystem - Scan filesystem for misconfigurations and secrets
	server.RegisterTool(mcp.Tool{
		Name:        "scan_filesystem",
		Description: "Scan filesystem for security misconfigurations, hardcoded secrets, and IaC issues",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Filesystem path to scan",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"path":       "/app",
			"scanned_at": time.Now().Format(time.RFC3339),
			"misconfigurations": []map[string]interface{}{
				{
					"id":          "DS002",
					"title":       "Image user should not be 'root'",
					"description": "Running containers as root increases attack surface",
					"severity":    "HIGH",
					"file":        "Dockerfile",
					"line":        15,
					"resolution":  "Add 'USER nonroot' to Dockerfile",
				},
				{
					"id":          "DS026",
					"title":       "Weak encryption algorithm",
					"description": "SHA-1 hash algorithm is considered weak",
					"severity":    "MEDIUM",
					"file":        "config/crypto.yaml",
					"line":        8,
					"resolution":  "Use SHA-256 or stronger",
				},
			},
			"secrets": []map[string]interface{}{
				{
					"rule_id":     "generic-api-key",
					"category":    "general",
					"severity":    "CRITICAL",
					"title":       "Generic API Key",
					"match":       "api_key = \"sk_live_1234567890abcdef\"",
					"file":        ".env.example",
					"line":        23,
					"start_column": 1,
					"end_column":   37,
				},
			},
		}
		return SuccessResult(data)
	})

	return server
}

// NewSemgrepMock creates a mock Semgrep static analysis MCP server
func NewSemgrepMock() *MockServer {
	server := NewMockServer(
		"semgrep",
		"1.0.0",
		"Mock Semgrep for static code analysis and security vulnerability detection",
	)

	// scan_code - Scan code for security vulnerabilities
	server.RegisterTool(mcp.Tool{
		Name:        "scan_code",
		Description: "Scan source code for security vulnerabilities, OWASP Top 10, and code quality issues",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to scan (directory or file)",
				},
				"rules": map[string]interface{}{
					"type":        "string",
					"description": "Rule set to use (auto, security, owasp-top-10)",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"scan_time": time.Now().Format(time.RFC3339),
			"findings": []map[string]interface{}{
				{
					"check_id": "python.lang.security.audit.sqli.pg-sqli",
					"path":     "src/api/user.py",
					"line":     42,
					"column":   15,
					"severity": "ERROR",
					"message":  "Potential SQL injection vulnerability. User input is directly interpolated into SQL query.",
					"code_snippet": `query = f"SELECT * FROM users WHERE id = {user_id}"`,
					"fix":          "Use parameterized queries: cursor.execute('SELECT * FROM users WHERE id = %s', (user_id,))",
					"cwe":          []string{"CWE-89"},
					"owasp":        []string{"A03:2021 - Injection"},
				},
				{
					"check_id": "javascript.express.security.audit.xss.mustache.explicit-unescape",
					"path":     "src/web/templates/profile.js",
					"line":     78,
					"column":   23,
					"severity": "WARNING",
					"message":  "Unescaped template variable could lead to XSS",
					"code_snippet": `<div>{{{ user.bio }}}</div>`,
					"fix":          "Use escaped variables: <div>{{ user.bio }}</div>",
					"cwe":          []string{"CWE-79"},
					"owasp":        []string{"A03:2021 - Injection"},
				},
				{
					"check_id": "generic.secrets.security.detected-private-key",
					"path":     "config/deploy.sh",
					"line":     15,
					"column":   1,
					"severity": "ERROR",
					"message":  "Hardcoded private key detected",
					"code_snippet": `SSH_KEY="-----BEGIN RSA PRIVATE KEY-----\nMIIE..."`,
					"fix":          "Use environment variables or secret management service",
					"cwe":          []string{"CWE-798"},
				},
			},
			"summary": map[string]interface{}{
				"total":    3,
				"error":    2,
				"warning":  1,
				"files_scanned": 47,
			},
		}
		return SuccessResult(data)
	})

	return server
}

// NewMacieMock creates a mock AWS Macie MCP server for data security
func NewMacieMock() *MockServer {
	server := NewMockServer(
		"macie",
		"1.0.0",
		"Mock AWS Macie for sensitive data discovery and data security posture management",
	)

	// get_findings - Data classification findings
	server.RegisterTool(mcp.Tool{
		Name:        "get_findings",
		Description: "Retrieve Macie findings about sensitive data discovery and classification",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"findings": []map[string]interface{}{
				{
					"finding_id":   "macie-finding-pii-1",
					"finding_type": "SensitiveData:S3Object/Personal",
					"severity":     "HIGH",
					"bucket":       "company-user-uploads",
					"object_key":   "uploads/2023/11/user_data_export.csv",
					"data_identifiers": []map[string]interface{}{
						{
							"type":        "USA_SOCIAL_SECURITY_NUMBER",
							"count":       1247,
							"sample":      "***-**-6789",
						},
						{
							"type":        "EMAIL_ADDRESS",
							"count":       1247,
							"sample":      "user@example.com",
						},
						{
							"type":        "CREDIT_CARD_NUMBER",
							"count":       82,
							"sample":      "****-****-****-4321",
						},
					},
					"sample_count": 1247,
					"created_at":   time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
				},
				{
					"finding_id":   "macie-finding-cred-1",
					"finding_type": "SensitiveData:S3Object/Credentials",
					"severity":     "CRITICAL",
					"bucket":       "dev-backup-bucket",
					"object_key":   "backups/db_dump_20231115.sql",
					"data_identifiers": []map[string]interface{}{
						{
							"type":   "AWS_SECRET_KEY",
							"count":  3,
							"sample": "aws_secret_access_key = **********************",
						},
						{
							"type":   "PRIVATE_KEY",
							"count":  1,
							"sample": "-----BEGIN RSA PRIVATE KEY-----",
						},
					},
					"sample_count": 4,
					"created_at":   time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339),
				},
			},
		}
		return SuccessResult(data)
	})

	return server
}

// NewFalcoMock creates a mock Falco runtime security MCP server
func NewFalcoMock() *MockServer {
	server := NewMockServer(
		"falco",
		"1.0.0",
		"Mock Falco for runtime security monitoring and threat detection in containers and Kubernetes",
	)

	// get_alerts - Runtime security alerts
	server.RegisterTool(mcp.Tool{
		Name:        "get_alerts",
		Description: "Retrieve Falco runtime security alerts for suspicious container/K8s behavior",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"alerts": []map[string]interface{}{
				{
					"rule":        "Terminal shell in container",
					"priority":    "WARNING",
					"output":      "A shell was spawned in a container with an attached terminal (user=root container_id=abc123 shell=bash parent=node)",
					"time":        time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
					"container_id": "abc123",
					"container_name": "nginx-app",
					"namespace":    "production",
					"pod":          "nginx-app-7d8f9c-xk2mn",
					"process":      "/bin/bash",
					"user":         "root",
				},
				{
					"rule":        "Write below etc",
					"priority":    "ERROR",
					"output":      "File below /etc opened for writing (user=www-data command=php file=/etc/nginx/nginx.conf)",
					"time":        time.Now().Add(-15 * time.Minute).Format(time.RFC3339),
					"container_id": "def456",
					"container_name": "web-backend",
					"namespace":    "production",
					"pod":          "web-backend-5f6g7h-p9q2r",
					"process":      "php",
					"user":         "www-data",
					"file":         "/etc/nginx/nginx.conf",
				},
				{
					"rule":        "Outbound Connection to C2 Server",
					"priority":    "CRITICAL",
					"output":      "Outbound connection to known C2 server (container=app-worker dest_ip=198.51.100.42 dest_port=4444)",
					"time":        time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
					"container_id": "ghi789",
					"container_name": "app-worker",
					"namespace":    "staging",
					"pod":          "app-worker-8i9j0k-s3t4u",
					"dest_ip":      "198.51.100.42",
					"dest_port":    4444,
				},
			},
		}
		return SuccessResult(data)
	})

	return server
}

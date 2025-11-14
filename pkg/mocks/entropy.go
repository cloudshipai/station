package mocks

import (
	"fmt"
	"math/rand"
	"time"
)

// EntropyHelper provides utilities for generating variable mock data suitable for evals
type EntropyHelper struct {
	rand *rand.Rand
}

// NewEntropyHelper creates a new entropy helper with random seed
func NewEntropyHelper() *EntropyHelper {
	return &EntropyHelper{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// RandomFloat generates a random float in range [min, max)
func (e *EntropyHelper) RandomFloat(min, max float64) float64 {
	return min + e.rand.Float64()*(max-min)
}

// RandomInt generates a random int in range [min, max)
func (e *EntropyHelper) RandomInt(min, max int) int {
	return min + e.rand.Intn(max-min)
}

// RandomChoice selects a random element from a slice
func (e *EntropyHelper) RandomChoice(choices []string) string {
	if len(choices) == 0 {
		return ""
	}
	return choices[e.rand.Intn(len(choices))]
}

// RandomBool returns a random boolean
func (e *EntropyHelper) RandomBool() bool {
	return e.rand.Intn(2) == 1
}

// RandomDate generates a random date within the past n days
func (e *EntropyHelper) RandomDate(daysBack int) time.Time {
	days := e.rand.Intn(daysBack)
	hours := e.rand.Intn(24)
	minutes := e.rand.Intn(60)
	return time.Now().AddDate(0, 0, -days).Add(-time.Duration(hours) * time.Hour).Add(-time.Duration(minutes) * time.Minute)
}

// RandomAWSService returns a random AWS service name
func (e *EntropyHelper) RandomAWSService() string {
	services := []string{
		"Amazon EC2", "Amazon S3", "Amazon RDS", "AWS Lambda",
		"Amazon CloudFront", "Amazon DynamoDB", "Amazon ECS",
		"Amazon EKS", "Amazon ElastiCache", "Amazon SNS",
		"Amazon SQS", "AWS Fargate", "Amazon CloudWatch",
		"AWS CloudTrail", "Amazon VPC", "Amazon Route53",
	}
	return e.RandomChoice(services)
}

// RandomAWSRegion returns a random AWS region
func (e *EntropyHelper) RandomAWSRegion() string {
	regions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-central-1", "ap-southeast-1",
		"ap-southeast-2", "ap-northeast-1", "sa-east-1",
	}
	return e.RandomChoice(regions)
}

// RandomIP generates a random IP address
func (e *EntropyHelper) RandomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		e.RandomInt(1, 255),
		e.RandomInt(0, 255),
		e.RandomInt(0, 255),
		e.RandomInt(1, 255))
}

// RandomInstanceType returns a random EC2 instance type
func (e *EntropyHelper) RandomInstanceType() string {
	types := []string{
		"t3.micro", "t3.small", "t3.medium", "t3.large", "t3.xlarge",
		"m5.large", "m5.xlarge", "m5.2xlarge", "c5.large", "c5.xlarge",
		"r5.large", "r5.xlarge",
	}
	return e.RandomChoice(types)
}

// RandomCVE generates a random CVE identifier
func (e *EntropyHelper) RandomCVE() string {
	year := e.RandomInt(2020, 2025)
	number := e.RandomInt(1000, 99999)
	return fmt.Sprintf("CVE-%d-%d", year, number)
}

// RandomSeverity returns a random severity level
func (e *EntropyHelper) RandomSeverity() string {
	severities := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}
	weights := []int{30, 40, 20, 10} // 30% LOW, 40% MEDIUM, 20% HIGH, 10% CRITICAL

	total := 0
	for _, w := range weights {
		total += w
	}

	r := e.RandomInt(0, total)
	cumulative := 0
	for i, w := range weights {
		cumulative += w
		if r < cumulative {
			return severities[i]
		}
	}
	return "MEDIUM"
}

// RandomCVSSScore generates a CVSS score appropriate for severity
func (e *EntropyHelper) RandomCVSSScore(severity string) float64 {
	switch severity {
	case "CRITICAL":
		return e.RandomFloat(9.0, 10.0)
	case "HIGH":
		return e.RandomFloat(7.0, 8.9)
	case "MEDIUM":
		return e.RandomFloat(4.0, 6.9)
	case "LOW":
		return e.RandomFloat(0.1, 3.9)
	default:
		return e.RandomFloat(4.0, 6.9)
	}
}

// RandomCost generates a random cost value with realistic distribution
func (e *EntropyHelper) RandomCost(baseMin, baseMax float64) float64 {
	// Add some spike probability for realistic cost anomalies (10% chance of 2-5x spike)
	cost := e.RandomFloat(baseMin, baseMax)
	if e.rand.Float64() < 0.1 {
		spikeMultiplier := e.RandomFloat(2.0, 5.0)
		cost *= spikeMultiplier
	}
	return cost
}

// RandomPercentChange generates a realistic percent change value
func (e *EntropyHelper) RandomPercentChange() float64 {
	// 60% chance positive, 40% negative
	change := e.RandomFloat(0, 50.0)
	if e.rand.Float64() < 0.4 {
		change = -change
	}
	return change
}

// RandomContainerName returns a random container/pod name
func (e *EntropyHelper) RandomContainerName() string {
	prefixes := []string{"web", "api", "worker", "nginx", "redis", "db", "app", "service"}
	suffixes := []string{"backend", "frontend", "processor", "handler", "manager"}

	name := e.RandomChoice(prefixes)
	if e.RandomBool() {
		name += "-" + e.RandomChoice(suffixes)
	}
	return name
}

// RandomK8sNamespace returns a random Kubernetes namespace
func (e *EntropyHelper) RandomK8sNamespace() string {
	namespaces := []string{"production", "staging", "development", "qa", "demo", "default"}
	return e.RandomChoice(namespaces)
}

// RandomErrorMessage returns a random error message
func (e *EntropyHelper) RandomErrorMessage() string {
	messages := []string{
		"Connection timeout after 30 seconds",
		"Rate limit exceeded",
		"Internal server error",
		"Database connection failed",
		"Authentication failed",
		"Permission denied",
		"Resource not found",
		"Out of memory",
		"Disk space low",
		"Network unreachable",
	}
	return e.RandomChoice(messages)
}

// RandomMetricValue generates a realistic metric value with trend and noise
func (e *EntropyHelper) RandomMetricValue(base, trendSlope, noisePercent float64, timeOffset int) float64 {
	// base + (trend * time) + noise
	trend := trendSlope * float64(timeOffset)
	noise := base * noisePercent * (e.rand.Float64()*2 - 1) // Random noise in [-noisePercent, +noisePercent]
	return base + trend + noise
}

// RandomLogLevel returns a random log level
func (e *EntropyHelper) RandomLogLevel() string {
	levels := []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
	weights := []int{10, 50, 25, 12, 3} // Realistic distribution

	total := 0
	for _, w := range weights {
		total += w
	}

	r := e.RandomInt(0, total)
	cumulative := 0
	for i, w := range weights {
		cumulative += w
		if r < cumulative {
			return levels[i]
		}
	}
	return "INFO"
}

// RandomDuration generates a random duration in milliseconds
func (e *EntropyHelper) RandomDuration(minMs, maxMs int) int {
	return e.RandomInt(minMs, maxMs)
}

// RandomMemoryMB generates a random memory usage in MB
func (e *EntropyHelper) RandomMemoryMB(minMB, maxMB int) int {
	return e.RandomInt(minMB, maxMB)
}

// RandomPort generates a random port number
func (e *EntropyHelper) RandomPort() int {
	// Common ports + random high ports
	commonPorts := []int{22, 80, 443, 3000, 3306, 5432, 6379, 8080, 8443, 9000}
	if e.rand.Float64() < 0.7 {
		return commonPorts[e.rand.Intn(len(commonPorts))]
	}
	return e.RandomInt(1024, 65535)
}

// RandomAccountID generates a random AWS account ID
func (e *EntropyHelper) RandomAccountID() string {
	return fmt.Sprintf("%012d", e.RandomInt(100000000000, 999999999999))
}

// RandomARN generates a random AWS ARN
func (e *EntropyHelper) RandomARN(service, resourceType string) string {
	region := e.RandomAWSRegion()
	accountID := e.RandomAccountID()
	resourceID := fmt.Sprintf("%s-%d", resourceType, e.RandomInt(1000, 9999))
	return fmt.Sprintf("arn:aws:%s:%s:%s:%s/%s", service, region, accountID, resourceType, resourceID)
}

// RandomInstanceID generates a random EC2 instance ID
func (e *EntropyHelper) RandomInstanceID() string {
	return fmt.Sprintf("i-%016x", e.rand.Int63())
}

// RandomDockerImageTag returns a random Docker image tag
func (e *EntropyHelper) RandomDockerImageTag() string {
	// Mix of semantic versions and tags
	if e.RandomBool() {
		return fmt.Sprintf("v%d.%d.%d", e.RandomInt(0, 2), e.RandomInt(0, 20), e.RandomInt(0, 10))
	}
	tags := []string{"latest", "stable", "prod", "dev", "main"}
	return e.RandomChoice(tags)
}

// RandomUserAgent returns a random user agent string
func (e *EntropyHelper) RandomUserAgent() string {
	agents := []string{
		"aws-cli/2.13.0",
		"boto3/1.28.0",
		"terraform/1.5.0",
		"kubectl/1.27.0",
		"Mozilla/5.0",
	}
	return e.RandomChoice(agents)
}

// RandomFileName generates a random file name
func (e *EntropyHelper) RandomFileName() string {
	names := []string{
		"config.yaml", "secrets.env", "docker-compose.yml", "main.tf",
		"app.py", "index.js", "database.sql", ".env.production",
	}
	return e.RandomChoice(names)
}

// RandomFilePath generates a random file path
func (e *EntropyHelper) RandomFilePath() string {
	dirs := []string{"/app", "/src", "/config", "/data", "/var/log", "/etc"}
	return e.RandomChoice(dirs) + "/" + e.RandomFileName()
}
